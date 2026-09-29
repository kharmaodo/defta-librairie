package repositories

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCoverImportListReportsScopedJobProgress(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "progress.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE cover_imports(id TEXT PRIMARY KEY,library_id TEXT,status TEXT,total_files INTEGER,accepted_files INTEGER,rejected_files INTEGER,created_at TEXT,updated_at TEXT,completed_at TEXT);
		CREATE TABLE cover_import_jobs(id TEXT PRIMARY KEY,import_id TEXT,library_id TEXT,status TEXT,source_content_type TEXT,source_format TEXT,source_width INTEGER,source_height INTEGER,source_size INTEGER,nsfw_decision TEXT,decision_code TEXT,failure_code TEXT,target_book_id INTEGER,expires_at TEXT,created_at TEXT,updated_at TEXT,source_object_key TEXT);
		INSERT INTO cover_imports VALUES('batch-a','library-a','PENDING',2,0,0,'now','now',NULL),('batch-b','library-b','PENDING',1,0,0,'now','now',NULL);
		INSERT INTO cover_import_jobs(id,import_id,library_id,status,source_content_type,source_format,source_width,source_height,source_size,nsfw_decision,decision_code,target_book_id,expires_at,created_at,updated_at,source_object_key) VALUES
		('job-a','batch-a','library-a','REVIEW_REQUIRED','image/jpeg','jpeg',20,30,100,'SAFE',NULL,NULL,'later','now','now','private/a'),
		('job-b','batch-a','library-a','READY','image/png','png',20,30,100,'SAFE','OWNER_ACCEPTED',1,'later','now','now','private/b'),
		('job-foreign','batch-b','library-b','QUARANTINED','image/jpeg','jpeg',20,30,100,'REVIEW',NULL,NULL,'later','now','now','private/foreign');
	`)
	if err != nil {
		t.Fatal(err)
	}
	items, err := NewCoverImportRepository(db).List(context.Background(), "library-a", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || len(items[0].Jobs) != 2 {
		t.Fatalf("scoped progress: %+v", items)
	}
	if items[0].Jobs[0].Status != "REVIEW_REQUIRED" || items[0].Jobs[1].TargetBookID == nil || *items[0].Jobs[1].TargetBookID != 1 {
		t.Fatalf("job state: %+v", items[0].Jobs)
	}
	if items[0].Status != "PROCESSING" || items[0].AcceptedFiles != 1 || items[0].RejectedFiles != 0 {
		t.Fatalf("summary: %+v", items[0])
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	active, err := countActiveCoverImports(context.Background(), tx, "library-a")
	_ = tx.Rollback()
	if err != nil || active != 1 {
		t.Fatalf("active imports before decision: %d, %v", active, err)
	}
	if _, err = db.Exec(`UPDATE cover_import_jobs SET status='REJECTED', updated_at='tomorrow' WHERE id='job-a'`); err != nil {
		t.Fatal(err)
	}
	items, err = NewCoverImportRepository(db).List(context.Background(), "library-a", 30)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].Status != "COMPLETED" || items[0].AcceptedFiles != 1 || items[0].RejectedFiles != 1 || items[0].CompletedAt == nil {
		t.Fatalf("final summary: %+v", items[0])
	}
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	active, err = countActiveCoverImports(context.Background(), tx, "library-a")
	_ = tx.Rollback()
	if err != nil || active != 0 {
		t.Fatalf("active imports after decision: %d, %v", active, err)
	}
}
