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
	"io"
	"path/filepath"
	"testing"
)

type importErrorStore struct {
	coverMemoryStore
	deleteErr error
	putErr    error
}

func (s *importErrorStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	if s.putErr != nil {
		return s.putErr
	}
	return s.coverMemoryStore.Put(ctx, key, body, size, contentType)
}

func (s *importErrorStore) Delete(ctx context.Context, key string) error {
	_ = s.coverMemoryStore.Delete(ctx, key)
	return s.deleteErr
}

func importErrorFixture(t *testing.T, store *importErrorStore) (*CoverImportService, *auth.Claims, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "imports.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
 INSERT INTO users(id,username,password_hash,role,status,created_at,updated_at)
 VALUES('owner-a','owner-a','hash','OWNER_LIBRARY','ACTIVE','now','now');
 INSERT INTO libraries(id,name,owner_user_id,status,created_at,updated_at)
 VALUES('library-a','Library A','owner-a','ACTIVE','now','now');
 `)
	if err != nil {
		t.Fatal(err)
	}
	uploader, err := covers.NewSourceUploader(store, covers.NewValidator(1024*1024, 100), func() (string, error) { return "unused", nil })
	if err != nil {
		t.Fatal(err)
	}
	claims := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-a"}
	claims.Subject = "owner-a"
	return NewCoverImportService(true, NewBookService(repositories.NewBookRepository(db)), repositories.NewCoverImportRepository(db), uploader), claims, db
}

func importPNGFile(t *testing.T) CoverImportFile {
	return CoverImportFile{DeclaredContentType: "image/png", Body: bytes.NewReader(coverPNG(t))}
}

func TestCoverImportQuotaSurvivesRollback(t *testing.T) {
	for _, cleanupFailure := range []bool{false, true} {
		name := "cleanup-success"
		if cleanupFailure {
			name = "cleanup-failure"
		}
		t.Run(name, func(t *testing.T) {
			store := &importErrorStore{}
			service, claims, db := importErrorFixture(t, store)
			for _, key := range []string{"first", "second"} {
				if _, err := service.Create(context.Background(), claims, "", key, []CoverImportFile{importPNGFile(t)}); err != nil {
					t.Fatal(err)
				}
			}
			cleanupErr := errors.New("cleanup failed")
			if cleanupFailure {
				store.deleteErr = cleanupErr
			}
			_, err := service.Create(context.Background(), claims, "", "third", []CoverImportFile{importPNGFile(t)})
			if !errors.Is(err, repositories.ErrCoverImportQuota) {
				t.Fatalf("lost quota error: %v", err)
			}
			if cleanupFailure && !errors.Is(err, covers.ErrStoreUnavailable) {
				t.Fatalf("lost cleanup error: %v", err)
			}
			if store.puts != 3 || store.deletes != 1 {
				t.Fatalf("puts=%d deletes=%d", store.puts, store.deletes)
			}
			for _, table := range []string{"cover_imports", "cover_import_jobs", "cover_import_outbox"} {
				var count int
				if err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 2 {
					t.Fatalf("%s: expected two committed rows, got %d", table, count)
				}
			}
		})
	}
}

func TestCoverImportValidationSurvivesPartialUploadRollback(t *testing.T) {
	store := &importErrorStore{deleteErr: errors.New("cleanup failed")}
	service, claims, db := importErrorFixture(t, store)
	files := []CoverImportFile{importPNGFile(t), {DeclaredContentType: "image/png", Body: bytes.NewReader([]byte("invalid"))}}
	_, err := service.Create(context.Background(), claims, "", "invalid", files)
	if !errors.Is(err, covers.ErrUnsupportedFormat) {
		t.Fatalf("lost image validation: %v", err)
	}
	if !errors.Is(err, covers.ErrStoreUnavailable) {
		t.Fatalf("lost cleanup error: %v", err)
	}
	if store.puts != 1 || store.deletes != 1 {
		t.Fatalf("puts=%d deletes=%d", store.puts, store.deletes)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM cover_imports").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unexpected committed import: %d", count)
	}
}

func TestCoverImportStoreFailureRemainsUnavailable(t *testing.T) {
	store := &importErrorStore{putErr: errors.New("store offline")}
	service, claims, _ := importErrorFixture(t, store)
	_, err := service.Create(context.Background(), claims, "", "offline", []CoverImportFile{importPNGFile(t)})
	if !errors.Is(err, covers.ErrStoreUnavailable) || !errors.Is(err, ErrCoverPersistence) {
		t.Fatalf("missing infrastructure error: %v", err)
	}
	if store.deletes != 0 {
		t.Fatalf("deleted an object not uploaded: %d", store.deletes)
	}
}
