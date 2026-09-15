package services

import (
	"bytes"
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"image"
	"image/png"
	"io"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

type coverMemoryStore struct {
	puts    int
	deletes int
	lastKey string
}

func (s *coverMemoryStore) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		return err
	}
	s.puts++
	s.lastKey = key
	return nil
}

func (s *coverMemoryStore) Delete(_ context.Context, key string) error {
	s.deletes++
	s.lastKey = key
	return nil
}

func coverPNG(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 4, 5))); err != nil {
		t.Fatalf("encode cover: %v", err)
	}
	return data.Bytes()
}

func TestBookCoverUploadPersistsOutboxAndEnforcesLibraryScope(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "covers.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err = db.Exec(`
		INSERT INTO users(id,username,password_hash,role,status,created_at,updated_at) VALUES
		('owner-1','owner1','hash','OWNER_LIBRARY','ACTIVE','now','now'),
		('owner-2','owner2','hash','OWNER_LIBRARY','ACTIVE','now','now');
		INSERT INTO libraries(id,name,owner_user_id,status,created_at,updated_at) VALUES
		('library-1','One','owner-1','ACTIVE','now','now'),
		('library-2','Two','owner-2','ACTIVE','now','now');
	`); err != nil {
		t.Fatalf("seed identities: %v", err)
	}

	bookService := NewBookService(repositories.NewBookRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	book, err := bookService.Create(context.Background(), ownerOne, models.BookInput{
		Title: "Livre couverture", Price: 1000, Volume: 1,
	})
	if err != nil {
		t.Fatalf("create book: %v", err)
	}

	store := &coverMemoryStore{}
	sourceUploader, err := covers.NewSourceUploader(
		store,
		covers.NewValidator(1024*1024, 100),
		func() (string, error) { return "cover-fixed", nil },
	)
	if err != nil {
		t.Fatalf("new uploader: %v", err)
	}
	service := NewBookCoverService(
		true, bookService, repositories.NewCoverRepository(db), sourceUploader,
	)
	ids := []string{"event-1", "audit-1", "event-2", "audit-2"}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}

	pending, err := service.Upload(
		context.Background(), ownerOne, book.ID, "image/png", bytes.NewReader(coverPNG(t)),
	)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if pending.Status != "PENDING" || pending.LibraryID != "library-1" ||
		pending.BookID != book.ID || store.puts != 1 || store.deletes != 0 {
		t.Fatalf("unexpected pending=%+v store=%+v", pending, store)
	}

	for table, want := range map[string]int{
		"book_covers": 1, "cover_processing_outbox": 1,
	} {
		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != want {
			t.Fatalf("%s count=%d err=%v", table, count, err)
		}
	}
	var auditCount int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM audit_logs
		WHERE action='UPLOAD_BOOK_COVER' AND resource_id=?
	`, book.ID).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit count=%d err=%v", auditCount, err)
	}

	_, err = service.Upload(
		context.Background(), ownerTwo, book.ID, "image/png", bytes.NewReader(coverPNG(t)),
	)
	if !errors.Is(err, repositories.ErrBookNotFound) || store.puts != 1 {
		t.Fatalf("cross-library err=%v puts=%d", err, store.puts)
	}

	_, err = service.Upload(
		context.Background(), ownerOne, book.ID, "image/png", bytes.NewReader(coverPNG(t)),
	)
	if !errors.Is(err, ErrCoverPersistence) || store.puts != 2 || store.deletes != 1 {
		t.Fatalf("compensation err=%v store=%+v", err, store)
	}
}

func TestBookCoverUploadDisabled(t *testing.T) {
	service := NewBookCoverService(false, nil, nil, nil)
	_, err := service.Upload(context.Background(), nil, 1, "image/png", bytes.NewReader(nil))
	if !errors.Is(err, ErrCoversDisabled) {
		t.Fatalf("err=%v", err)
	}
}
