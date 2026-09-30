package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	_ "github.com/mattn/go-sqlite3"
)

type reviewMemoryStore struct{ copied, deleted int }

func (s *reviewMemoryStore) CopySource(context.Context, string, string, int64, string) error {
	s.copied++
	return nil
}
func (s *reviewMemoryStore) DeleteObject(context.Context, string) error { s.deleted++; return nil }
func (s *reviewMemoryStore) OpenSource(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("private")), nil
}

func reviewServiceDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "review-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
      CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT);
      CREATE TABLE defta(id INTEGER PRIMARY KEY,library_id TEXT,title TEXT,deleted_at TEXT,auteur TEXT);
      CREATE TABLE cover_import_review_suggestions(job_id TEXT,book_id INTEGER,origin TEXT,rejected INTEGER DEFAULT 0,actor_user_id TEXT,updated_at TEXT,PRIMARY KEY(job_id,book_id));
      CREATE TABLE cover_import_jobs(id TEXT PRIMARY KEY,library_id TEXT,status TEXT,nsfw_decision TEXT,nsfw_policy_version TEXT,source_object_key TEXT,source_content_type TEXT,source_format TEXT,source_width INTEGER,source_height INTEGER,source_size INTEGER,target_book_id INTEGER,decision_code TEXT,terminal_at TEXT,updated_at TEXT);
      CREATE TABLE cover_import_candidate_matches(job_id TEXT,book_id INTEGER,rank INTEGER,fts_score REAL);
      CREATE TABLE cover_import_review_decisions(job_id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,book_id INTEGER,reason TEXT,created_at TEXT);
      CREATE TABLE book_covers(id TEXT PRIMARY KEY,book_id INTEGER,library_id TEXT,status TEXT,source_object_key TEXT,source_content_type TEXT,source_format TEXT,source_width INTEGER,source_height INTEGER,source_size INTEGER,active INTEGER,created_at TEXT,updated_at TEXT);
      CREATE TABLE cover_processing_outbox(event_id TEXT PRIMARY KEY,cover_id TEXT,book_id INTEGER,library_id TEXT,event_type TEXT,schema_version INTEGER,payload TEXT,attempts INTEGER,available_at TEXT,created_at TEXT);
      CREATE TABLE cover_import_outbox(event_id TEXT PRIMARY KEY,job_id TEXT,library_id TEXT,event_type TEXT,schema_version INTEGER,payload TEXT,attempts INTEGER,available_at TEXT,created_at TEXT);
      CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,new_values TEXT,success INTEGER,created_at TEXT);
      INSERT INTO libraries VALUES('library-a','ACTIVE'),('library-b','ACTIVE');
      INSERT INTO defta(id,library_id,title,deleted_at) VALUES(1,'library-a','Book A',NULL);
      INSERT INTO cover_import_jobs(id,library_id,status,nsfw_decision,source_object_key,source_content_type,source_format,source_width,source_height,source_size,updated_at) VALUES('safe','library-a','REVIEW_REQUIRED','SAFE','private/source','image/jpeg','jpeg',10,10,200,'before'),('quarantine','library-a','QUARANTINED','REVIEW','private/quarantine','image/jpeg','jpeg',10,10,200,'before');
      INSERT INTO cover_import_candidate_matches VALUES('safe',1,1,0.4);
    `)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCoverImportReviewAuthorizationAndSource(t *testing.T) {
	db := reviewServiceDB(t)
	store := &reviewMemoryStore{}
	service := NewCoverImportReviewService(NewBookService(repositories.NewBookRepository(db)), repositories.NewCoverImportRepository(db), store)
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-a", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner-a"}}
	foreign := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-b", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner-b"}}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	ctx := context.Background()
	if _, err := service.Job(ctx, foreign, "safe", ""); !errors.Is(err, repositories.ErrCoverImportJobNotFound) {
		t.Fatalf("foreign job: %v", err)
	}
	if _, _, err := service.OpenSource(ctx, owner, "quarantine", ""); !errors.Is(err, repositories.ErrCoverImportReviewState) {
		t.Fatalf("owner quarantine preview: %v", err)
	}
	if _, err := service.DecideQuarantine(ctx, owner, "quarantine", "", "APPROVE"); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("owner quarantine decision: %v", err)
	}
	reader, ctype, err := service.OpenSource(ctx, root, "quarantine", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	reader.Close()
	if ctype != "image/jpeg" {
		t.Fatalf("content type %s", ctype)
	}
	if _, err := service.Decide(ctx, owner, "safe", "", "ACCEPT", "", 2); !errors.Is(err, repositories.ErrCoverImportCandidateNotFound) {
		t.Fatalf("uncandidate book: %v", err)
	}
	if store.copied != 0 {
		t.Fatalf("copied without authorization: %d", store.copied)
	}
	if _, err := service.Decide(ctx, owner, "safe", "", "ACCEPT", "", 1); err != nil {
		t.Fatal(err)
	}
	if store.copied != 1 || store.deleted != 0 {
		t.Fatalf("store copied=%d deleted=%d", store.copied, store.deleted)
	}
	if _, err := service.Decide(ctx, owner, "safe", "", "ACCEPT", "", 1); !errors.Is(err, repositories.ErrCoverImportReviewState) {
		t.Fatalf("duplicate: %v", err)
	}
	if store.copied != 1 {
		t.Fatalf("duplicate copy: %d", store.copied)
	}
}
