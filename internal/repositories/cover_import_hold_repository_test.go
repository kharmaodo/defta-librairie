package repositories

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestLegalHoldIsAuditedAndCannotRaceWithClaimedCleanup(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "holds.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE cover_import_jobs(id TEXT PRIMARY KEY);
	CREATE TABLE cover_import_legal_holds(job_id TEXT PRIMARY KEY,reason TEXT,actor_user_id TEXT,expires_at TEXT,created_at TEXT);
	CREATE TABLE cover_object_cleanup_jobs(id INTEGER PRIMARY KEY,cover_id TEXT,library_id TEXT,object_key TEXT,object_kind TEXT,available_at TEXT,created_at TEXT,completed_at TEXT,locked_by TEXT,locked_until TEXT,attempts INTEGER DEFAULT 0,last_error TEXT);
	CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,new_values TEXT,success INTEGER,created_at TEXT);
	INSERT INTO cover_import_jobs VALUES('job-1');`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	repo := NewCoverImportRetentionRepository(db)
	ctx := context.Background()
	if err = repo.SetLegalHold(ctx, "job-1", "root", "Litige", now.Add(48*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetLegalHold(ctx, "job-1", "root", "Litige prolongé", now.Add(72*time.Hour), now); err != nil {
		t.Fatalf("renew hold: %v", err)
	}
	_, err = db.Exec(`INSERT INTO cover_object_cleanup_jobs(cover_id,library_id,object_key,object_kind,available_at,created_at) VALUES('import:job-1','library-1','imports/job-1','SOURCE',?,?)`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewCoverCleanupRepository(db).ClaimNext(ctx, "worker", now, now.Add(time.Minute)); !errors.Is(err, ErrCoverCleanupEmpty) {
		t.Fatalf("claim under hold: %v", err)
	}
	if err = repo.ReleaseLegalHold(ctx, "job-1", "root", now); err != nil {
		t.Fatal(err)
	}
	if err = repo.ReleaseLegalHold(ctx, "job-1", "root", now); !errors.Is(err, ErrCoverImportHoldNotFound) {
		t.Fatalf("repeat release: %v", err)
	}
	job, err := NewCoverCleanupRepository(db).ClaimNext(ctx, "worker", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.SetLegalHold(ctx, "job-1", "root", "Too late", now.Add(time.Hour), now); !errors.Is(err, ErrCoverImportHoldConflict) {
		t.Fatalf("hold during lease: %v", err)
	}
	if err = NewCoverCleanupRepository(db).MarkCompleted(ctx, job.ID, "worker", now); err != nil {
		t.Fatal(err)
	}
	if err = repo.SetLegalHold(ctx, "job-1", "root", "Too late", now.Add(time.Hour), now); !errors.Is(err, ErrCoverImportHoldConflict) {
		t.Fatalf("hold after deletion: %v", err)
	}
	var audits int
	if err = db.QueryRow(`SELECT count(*) FROM audit_logs WHERE resource_id='job-1'`).Scan(&audits); err != nil || audits != 3 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
}
