package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestBookCoverStatusAndRetryRespectLibraryScope(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cover-status.db")+"?_foreign_keys=on")
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
		Title: "Livre statut couverture", Price: 1000, Volume: 1,
	})
	if err != nil {
		t.Fatalf("create book: %v", err)
	}
	if _, err = db.Exec(`
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key,
			source_content_type, source_format, source_width, source_height,
			source_size, error_code, active, processing_by,
			processing_until, processing_attempts, created_at, updated_at
		) VALUES (
			'failed-cover', ?, 'library-1', 'FAILED',
			'sources/library-1/book/failed-cover.png',
			'image/png', 'png', 800, 1200, 1024, 'decode_failed', 0,
			'worker-a', '2026-09-16T12:00:00Z', 5,
			'2026-09-16T11:00:00Z', '2026-09-16T11:30:00Z'
		)
	`, book.ID); err != nil {
		t.Fatalf("insert failed cover: %v", err)
	}

	service := NewBookCoverService(
		true,
		bookService,
		repositories.NewCoverRepository(db),
		nil,
	)
	ids := []string{"retry-event", "retry-audit"}
	service.newID = func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	}
	service.now = func() time.Time {
		return time.Date(2026, 9, 16, 12, 30, 0, 0, time.UTC)
	}

	status, err := service.Status(context.Background(), ownerOne, book.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.ID != "failed-cover" || status.Status != "FAILED" ||
		status.ErrorCode != "decode_failed" || !status.CanRetry {
		t.Fatalf("status=%+v", status)
	}
	_, err = service.Status(context.Background(), ownerTwo, book.ID)
	if !errors.Is(err, repositories.ErrBookNotFound) {
		t.Fatalf("cross-library status err=%v", err)
	}

	status, err = service.Retry(context.Background(), ownerOne, book.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if status.Status != "PENDING" || status.ErrorCode != "" || status.CanRetry {
		t.Fatalf("retried status=%+v", status)
	}

	var payloadText string
	if err = db.QueryRow(`
		SELECT payload FROM cover_processing_outbox
		WHERE event_id='retry-event'
	`).Scan(&payloadText); err != nil {
		t.Fatalf("read retry payload: %v", err)
	}
	var payload struct {
		SchemaVersion   int    `json:"schemaVersion"`
		EventID         string `json:"eventId"`
		CoverID         string `json:"coverId"`
		BookID          int    `json:"bookId"`
		LibraryID       string `json:"libraryId"`
		SourceObjectKey string `json:"sourceObjectKey"`
		Attempt         int    `json:"attempt"`
	}
	if err = json.Unmarshal([]byte(payloadText), &payload); err != nil {
		t.Fatalf("decode retry payload: %v", err)
	}
	if payload.SchemaVersion != 1 || payload.EventID != "retry-event" ||
		payload.CoverID != "failed-cover" || payload.BookID != book.ID ||
		payload.LibraryID != "library-1" || payload.Attempt != 1 ||
		payload.SourceObjectKey == "" {
		t.Fatalf("payload=%+v", payload)
	}

	var auditCount int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM audit_logs
		WHERE action='RETRY_BOOK_COVER' AND actor_user_id='owner-1'
	`).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audit count=%d err=%v", auditCount, err)
	}

	_, err = service.Retry(context.Background(), ownerOne, book.ID)
	if !errors.Is(err, repositories.ErrBookCoverNotRetryable) {
		t.Fatalf("second retry err=%v", err)
	}
}

func TestBookCoverStatusDisabled(t *testing.T) {
	service := NewBookCoverService(false, nil, nil, nil)
	if _, err := service.Status(context.Background(), nil, 1); !errors.Is(err, ErrCoversDisabled) {
		t.Fatalf("status err=%v", err)
	}
	if _, err := service.Retry(context.Background(), nil, 1); !errors.Is(err, ErrCoversDisabled) {
		t.Fatalf("retry err=%v", err)
	}
}
