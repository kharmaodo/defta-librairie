package repositories

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func reviewDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "review.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
      CREATE TABLE defta(id INTEGER PRIMARY KEY,library_id TEXT,title TEXT,deleted_at TEXT,auteur TEXT);
      CREATE TABLE cover_import_review_suggestions(job_id TEXT,book_id INTEGER,origin TEXT,rejected INTEGER DEFAULT 0,actor_user_id TEXT,updated_at TEXT,PRIMARY KEY(job_id,book_id));
      CREATE TABLE cover_import_jobs(id TEXT PRIMARY KEY,library_id TEXT,status TEXT,nsfw_decision TEXT,nsfw_policy_version TEXT,source_object_key TEXT,source_content_type TEXT,source_format TEXT,source_width INTEGER,source_height INTEGER,source_size INTEGER,target_book_id INTEGER,decision_code TEXT,terminal_at TEXT,updated_at TEXT);
      CREATE TABLE cover_import_candidate_matches(job_id TEXT,book_id INTEGER,rank INTEGER,fts_score REAL,PRIMARY KEY(job_id,book_id));
      CREATE TABLE cover_import_review_decisions(job_id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,book_id INTEGER,reason TEXT,created_at TEXT);
      CREATE TABLE book_covers(id TEXT PRIMARY KEY,book_id INTEGER,library_id TEXT,status TEXT,source_object_key TEXT UNIQUE,source_content_type TEXT,source_format TEXT,source_width INTEGER,source_height INTEGER,source_size INTEGER,active INTEGER,created_at TEXT,updated_at TEXT);
      CREATE TABLE cover_processing_outbox(event_id TEXT PRIMARY KEY,cover_id TEXT,book_id INTEGER,library_id TEXT,event_type TEXT,schema_version INTEGER,payload TEXT,attempts INTEGER,available_at TEXT,created_at TEXT);
      CREATE TABLE cover_import_outbox(event_id TEXT PRIMARY KEY,job_id TEXT,library_id TEXT,event_type TEXT,schema_version INTEGER,payload TEXT,attempts INTEGER,available_at TEXT,created_at TEXT);
      CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,new_values TEXT,success INTEGER,created_at TEXT);
      INSERT INTO defta(id,library_id,title,deleted_at) VALUES(1,'library-a','Book A',NULL),(2,'library-b','Book B',NULL),(3,'library-a','Deleted','today');
      INSERT INTO cover_import_jobs(id,library_id,status,nsfw_decision,source_object_key,source_content_type,source_format,source_width,source_height,source_size,updated_at) VALUES
      ('safe','library-a','REVIEW_REQUIRED','SAFE','private/safe','image/jpeg','jpeg',100,200,1000,'before'),
      ('quarantine','library-a','QUARANTINED','REVIEW','private/review','image/jpeg','jpeg',100,200,1000,'before');
      INSERT INTO cover_import_candidate_matches VALUES('safe',1,1,0.25),('safe',2,2,0.5),('safe',3,3,0.7);
    `)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func reviewCount(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestCoverImportReviewScopedAcceptAndIdempotence(t *testing.T) {
	db := reviewDB(t)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	if _, err := repo.ReviewJob(ctx, "safe", "library-b"); !errors.Is(err, ErrCoverImportJobNotFound) {
		t.Fatalf("foreign job: %v", err)
	}
	job, err := repo.ReviewJob(ctx, "safe", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Candidates) != 1 || job.Candidates[0].BookID != 1 {
		t.Fatalf("candidates: %+v", job.Candidates)
	}
	cover := PendingCover{ID: "cover-1", BookID: 1, LibraryID: "library-a", SourceObjectKey: "library-a/book-1/cover-1.jpg", SourceContentType: "image/jpeg", SourceFormat: "jpeg", SourceWidth: 100, SourceHeight: 200, SourceSize: 1000}
	foreignCover := cover
	foreignCover.BookID = 2
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "ACCEPT", "approved", 2, foreignCover, "event-bad", "audit-bad", "now"); !errors.Is(err, ErrCoverImportCandidateNotFound) {
		t.Fatalf("foreign book: %v", err)
	}
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "ACCEPT", "approved", 1, cover, "event-1", "audit-1", "now"); err != nil {
		t.Fatal(err)
	}
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "ACCEPT", "again", 1, cover, "event-2", "audit-2", "later"); !errors.Is(err, ErrCoverImportReviewState) {
		t.Fatalf("duplicate: %v", err)
	}
	for _, item := range []struct {
		table string
		want  int
	}{{"cover_import_review_decisions", 1}, {"book_covers", 1}, {"cover_processing_outbox", 1}, {"audit_logs", 1}} {
		if got := reviewCount(t, db, item.table); got != item.want {
			t.Errorf("%s=%d", item.table, got)
		}
	}
	var status, code string
	var bookID int
	if err = db.QueryRow(`SELECT status,decision_code,target_book_id FROM cover_import_jobs WHERE id='safe'`).Scan(&status, &code, &bookID); err != nil {
		t.Fatal(err)
	}
	if status != "READY" || code != "OWNER_ACCEPTED" || bookID != 1 {
		t.Fatalf("state %s %s %d", status, code, bookID)
	}
	var auditRole, correlation string
	if err = db.QueryRow(`SELECT json_extract(new_values,'$.actorRole'),json_extract(new_values,'$.correlationId') FROM audit_logs WHERE id='audit-1'`).Scan(&auditRole, &correlation); err != nil {
		t.Fatal(err)
	}
	if auditRole != "OWNER_LIBRARY" || correlation != "audit-1" {
		t.Fatalf("audit %s/%s", auditRole, correlation)
	}
}

func TestCoverImportReviewRollbackAndReject(t *testing.T) {
	db := reviewDB(t)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	job, err := repo.ReviewJob(ctx, "safe", "library-a")
	if err != nil {
		t.Fatal(err)
	}
	cover := PendingCover{ID: "cover-1", BookID: 1, LibraryID: "library-a", SourceObjectKey: "private/new", SourceContentType: "image/jpeg", SourceFormat: "jpeg", SourceWidth: 100, SourceHeight: 200, SourceSize: 1000}
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "ACCEPT", "", 1, cover, "event-1", "audit-1", "now"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`DELETE FROM audit_logs; DELETE FROM cover_processing_outbox; DELETE FROM book_covers; DELETE FROM cover_import_review_decisions; UPDATE cover_import_jobs SET status='REVIEW_REQUIRED' WHERE id='safe'; INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES('audit-1','x','x','x','x','{}',1,'x')`); err != nil {
		t.Fatal(err)
	}
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "ACCEPT", "", 1, cover, "event-2", "audit-1", "now"); err == nil {
		t.Fatal("expected audit collision")
	}
	if got := reviewCount(t, db, "book_covers"); got != 0 {
		t.Fatalf("uncommitted cover: %d", got)
	}
	if err = repo.DecideReview(ctx, job, "owner-a", "OWNER_LIBRARY", "REJECT", "not a match", 0, PendingCover{}, "", "audit-2", "now"); err != nil {
		t.Fatal(err)
	}
	if got := reviewCount(t, db, "cover_processing_outbox"); got != 0 {
		t.Fatalf("unexpected cover event: %d", got)
	}
}

func TestCoverImportQuarantineRootTransitionIdempotence(t *testing.T) {
	db := reviewDB(t)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	if err := repo.DecideQuarantine(ctx, "quarantine", "library-b", "root", "SUPER_ADMIN_ROOT", "APPROVE", "other", "audit-other", "now"); !errors.Is(err, ErrCoverImportReviewState) {
		t.Fatalf("foreign quarantine: %v", err)
	}
	if err := repo.DecideQuarantine(ctx, "quarantine", "library-a", "root", "SUPER_ADMIN_ROOT", "APPROVE", "ocr-1", "audit-1", "now"); err != nil {
		t.Fatal(err)
	}
	if err := repo.DecideQuarantine(ctx, "quarantine", "library-a", "root", "SUPER_ADMIN_ROOT", "APPROVE", "ocr-2", "audit-2", "later"); !errors.Is(err, ErrCoverImportReviewState) {
		t.Fatalf("duplicate: %v", err)
	}
	if got := reviewCount(t, db, "cover_import_outbox"); got != 1 {
		t.Fatalf("ocr events: %d", got)
	}
	var status, decision string
	if err := db.QueryRow(`SELECT status,nsfw_decision FROM cover_import_jobs WHERE id='quarantine'`).Scan(&status, &decision); err != nil {
		t.Fatal(err)
	}
	if status != "OCR_PENDING" || decision != "SAFE" {
		t.Fatalf("state: %s/%s", status, decision)
	}
}

func TestCoverImportQuarantineRejectHasNoOCREvent(t *testing.T) {
	db := reviewDB(t)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	if err := repo.DecideQuarantine(ctx, "quarantine", "library-a", "root", "SUPER_ADMIN_ROOT", "REJECT", "", "audit-reject", "now"); err != nil {
		t.Fatal(err)
	}
	if got := reviewCount(t, db, "cover_import_outbox"); got != 0 {
		t.Fatalf("unexpected OCR events: %d", got)
	}
	var status, decision string
	if err := db.QueryRow(`SELECT status,nsfw_decision FROM cover_import_jobs WHERE id='quarantine'`).Scan(&status, &decision); err != nil {
		t.Fatal(err)
	}
	if status != "REJECTED" || decision != "REVIEW" {
		t.Fatalf("state: %s/%s", status, decision)
	}
}
