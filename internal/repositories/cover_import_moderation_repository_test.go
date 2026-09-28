package repositories

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openCoverImportModerationDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cover-import-moderation.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE cover_import_jobs (
			id TEXT PRIMARY KEY, library_id TEXT NOT NULL, actor_user_id TEXT NOT NULL,
			source_object_key TEXT NOT NULL, source_content_type TEXT NOT NULL,
			source_size INTEGER NOT NULL, status TEXT NOT NULL, nsfw_decision TEXT,
			nsfw_policy_version TEXT, decision_code TEXT, failure_code TEXT,
			updated_at TEXT NOT NULL, terminal_at TEXT
		);
		CREATE TABLE cover_import_outbox (
			event_id TEXT PRIMARY KEY, job_id TEXT NOT NULL, library_id TEXT NOT NULL,
			event_type TEXT NOT NULL, schema_version INTEGER NOT NULL, payload TEXT NOT NULL,
			attempts INTEGER NOT NULL, available_at TEXT NOT NULL, published_at TEXT,
			last_error TEXT, created_at TEXT NOT NULL
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY, actor_user_id TEXT NOT NULL, action TEXT NOT NULL,
			resource_type TEXT NOT NULL, resource_id TEXT NOT NULL, new_values TEXT NOT NULL,
			success INTEGER NOT NULL, created_at TEXT NOT NULL
		);
		CREATE TABLE cover_import_ocr_results (
			job_id TEXT PRIMARY KEY, engine TEXT NOT NULL, engine_version TEXT NOT NULL,
			language TEXT NOT NULL, text_raw TEXT NOT NULL, text_normalized TEXT NOT NULL,
			confidence REAL, title TEXT, auteur TEXT, editeur TEXT, isbn13 TEXT,
			completed_at TEXT NOT NULL
		);
	`); err != nil { t.Fatal(err) }
	return db
}

func TestCoverImportOCRCompletionIsIdempotent(t *testing.T) {
	db := openCoverImportModerationDB(t)
	seedModerationJob(t, db, "job-ocr")
	if _, err := db.Exec(`UPDATE cover_import_jobs SET status='OCR_PENDING' WHERE id='job-ocr'`); err != nil {
		t.Fatal(err)
	}
	repo := NewCoverImportRepository(db)
	job, err := repo.ClaimForOCR(context.Background(), "job-ocr", "now")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteOCR(context.Background(), job, "tesseract", "tesseract-5", "ara", "عنوان", "عنوان", "event-match-1", "audit-ocr-1", "later"); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimForOCR(context.Background(), "job-ocr", "again"); !errors.Is(err, ErrCoverImportJobNotFound) {
		t.Fatalf("expected idempotent claim rejection, got %v", err)
	}
	var results, events, audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM cover_import_ocr_results WHERE job_id='job-ocr'`).Scan(&results); err != nil { t.Fatal(err) }
	if err = db.QueryRow(`SELECT COUNT(*) FROM cover_import_outbox WHERE job_id='job-ocr' AND event_type='cover.imports.match.v1'`).Scan(&events); err != nil { t.Fatal(err) }
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_id='job-ocr' AND action='COMPLETE_COVER_IMPORT_OCR'`).Scan(&audits); err != nil { t.Fatal(err) }
	if results != 1 || events != 1 || audits != 1 { t.Fatalf("results=%d events=%d audits=%d", results, events, audits) }
}

func seedModerationJob(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO cover_import_jobs(id,library_id,actor_user_id,source_object_key,source_content_type,source_size,status,updated_at) VALUES(?, 'library-1','owner-1','quarantine/library-1/import-1/source.jpg','image/jpeg',123,'PENDING_SCAN','before')`, id)
	if err != nil { t.Fatal(err) }
}

func TestCoverImportSafeModerationQueuesOCRWithoutBookMutation(t *testing.T) {
	db := openCoverImportModerationDB(t); seedModerationJob(t, db, "job-safe")
	repo := NewCoverImportRepository(db)
	job, err := repo.ClaimForModeration(context.Background(), "job-safe", "now")
	if err != nil { t.Fatal(err) }
	if err = repo.CompleteModeration(context.Background(), job, "SAFE", "local-model-1", "event-ocr-1", "audit-1", "later"); err != nil { t.Fatal(err) }
	var status, decision, policy string
	if err = db.QueryRow(`SELECT status,nsfw_decision,nsfw_policy_version FROM cover_import_jobs WHERE id='job-safe'`).Scan(&status,&decision,&policy); err != nil { t.Fatal(err) }
	if status != "OCR_PENDING" || decision != "SAFE" || policy != "local-model-1" { t.Fatalf("status=%s decision=%s policy=%s", status, decision, policy) }
	var events, audits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM cover_import_outbox WHERE job_id='job-safe' AND event_type='cover.imports.ocr.v1'`).Scan(&events)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_id='job-safe' AND success=1`).Scan(&audits)
	if events != 1 || audits != 1 { t.Fatalf("events=%d audits=%d", events, audits) }
}

func TestCoverImportReviewStaysQuarantinedWithoutOCR(t *testing.T) {
	db := openCoverImportModerationDB(t); seedModerationJob(t, db, "job-review")
	repo := NewCoverImportRepository(db)
	job, err := repo.ClaimForModeration(context.Background(), "job-review", "now")
	if err != nil { t.Fatal(err) }
	if err = repo.CompleteModeration(context.Background(), job, "REVIEW", "local-model-1", "", "audit-2", "later"); err != nil { t.Fatal(err) }
	var status string; var events int
	_ = db.QueryRow(`SELECT status FROM cover_import_jobs WHERE id='job-review'`).Scan(&status)
	_ = db.QueryRow(`SELECT COUNT(*) FROM cover_import_outbox WHERE job_id='job-review'`).Scan(&events)
	if status != "QUARANTINED" || events != 0 { t.Fatalf("status=%s events=%d", status, events) }
}
