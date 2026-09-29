package repositories

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestCoverImportRetentionBoundariesHoldAndReplay(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE cover_import_jobs (
	 id TEXT PRIMARY KEY, library_id TEXT NOT NULL, source_object_key TEXT NOT NULL,
	 status TEXT NOT NULL, nsfw_decision TEXT, failure_code TEXT,
	 expires_at TEXT NOT NULL, updated_at TEXT NOT NULL, terminal_at TEXT);
	 CREATE TABLE cover_import_legal_holds (job_id TEXT PRIMARY KEY, expires_at TEXT NOT NULL);
	 CREATE TABLE cover_object_cleanup_jobs (
	 id INTEGER PRIMARY KEY, cover_id TEXT NOT NULL, library_id TEXT NOT NULL,
	 object_key TEXT NOT NULL UNIQUE, object_kind TEXT NOT NULL,
	 available_at TEXT NOT NULL, created_at TEXT NOT NULL, completed_at TEXT,
	 locked_by TEXT, locked_until TEXT, attempts INTEGER NOT NULL DEFAULT 0, last_error TEXT);`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	stamp := func(d int) string { return now.AddDate(0, 0, -d).Format(time.RFC3339Nano) }
	for _, item := range []struct {
		id, status, decision string
		days                 int
		want                 bool
	}{
		{"failure-due", "FAILED", "", 7, true}, {"failure-young", "FAILED", "", 6, false},
		{"unsafe-due", "REJECTED", "UNSAFE", 14, true}, {"unsafe-young", "REJECTED", "UNSAFE", 13, false},
		{"safe-due", "REJECTED", "SAFE", 30, true}, {"safe-young", "REJECTED", "SAFE", 29, false},
		{"accepted-due", "READY", "SAFE", 90, true}, {"accepted-young", "READY", "SAFE", 89, false},
		{"review-due", "QUARANTINED", "REVIEW", 30, true}, {"review-young", "QUARANTINED", "REVIEW", 29, false},
		{"pending-expired", "OCR_PENDING", "SAFE", 30, true},
		{"hold", "READY", "SAFE", 91, false},
	} {
		expires := stamp(item.days)
		if item.id == "review-young" {
			expires = now.Add(24 * time.Hour).Format(time.RFC3339Nano)
		}
		_, err = db.Exec(`INSERT INTO cover_import_jobs(id,library_id,source_object_key,status,nsfw_decision,expires_at,updated_at,terminal_at)
		VALUES(?,'library-1',?,?,?,?,?,?)`, item.id, "imports/"+item.id, item.status, item.decision, expires, stamp(item.days), stamp(item.days))
		if err != nil {
			t.Fatal(err)
		}
		if item.id == "hold" {
			_, err = db.Exec(`INSERT INTO cover_import_legal_holds VALUES (?,?)`, item.id, now.Add(time.Hour).Format(time.RFC3339Nano))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	repo := NewCoverImportRetentionRepository(db)
	count, err := repo.Reconcile(context.Background(), now)
	if err != nil || count != 6 {
		t.Fatalf("first reconciliation: count=%d err=%v", count, err)
	}
	count, err = repo.Reconcile(context.Background(), now)
	if err != nil || count != 0 {
		t.Fatalf("replay: count=%d err=%v", count, err)
	}
	var status, failure string
	if err = db.QueryRow(`SELECT status, failure_code FROM cover_import_jobs WHERE id='pending-expired'`).Scan(&status, &failure); err != nil || status != "CANCELLED" || failure != "SOURCE_EXPIRED" {
		t.Fatalf("expired worker state: %q %q %v", status, failure, err)
	}
	for _, id := range []string{"failure-young", "unsafe-young", "safe-young", "accepted-young", "review-young", "hold"} {
		var n int
		if err = db.QueryRow(`SELECT count(*) FROM cover_object_cleanup_jobs WHERE cover_id=?`, "import:"+id).Scan(&n); err != nil || n != 0 {
			t.Fatalf("premature cleanup %s: %d %v", id, n, err)
		}
	}
	if _, err = db.Exec(`UPDATE cover_import_legal_holds SET expires_at=? WHERE job_id='hold'`, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	count, err = repo.Reconcile(context.Background(), now)
	if err != nil || count != 1 {
		t.Fatalf("expired hold: %d %v", count, err)
	}
}

func TestCoverImportHoldBlocksAlreadyQueuedCleanup(t *testing.T) {
	db := openCoverCleanupTestDB(t)
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO cover_object_cleanup_jobs(cover_id,library_id,object_key,object_kind,available_at,created_at)
	 VALUES('import:job-1','library-1','imports/job-1','SOURCE',?,?)`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO cover_import_legal_holds VALUES ('job-1',?)`, now.Add(time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	repo := NewCoverCleanupRepository(db)
	if _, err = repo.ClaimNext(context.Background(), "worker", now, now.Add(time.Minute)); err != ErrCoverCleanupEmpty {
		t.Fatalf("active hold claimed: %v", err)
	}
	if _, err = repo.ClaimNext(context.Background(), "worker", now.Add(2*time.Hour), now.Add(2*time.Hour+time.Minute)); err != nil {
		t.Fatalf("expired hold blocked: %v", err)
	}
}
