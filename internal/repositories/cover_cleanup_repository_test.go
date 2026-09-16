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

func openCoverCleanupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE cover_object_cleanup_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cover_id TEXT NOT NULL,
			library_id TEXT NOT NULL,
			object_key TEXT NOT NULL UNIQUE,
			object_kind TEXT NOT NULL,
			available_at TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			locked_by TEXT,
			locked_until TEXT,
			completed_at TEXT,
			last_error TEXT,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create cleanup queue: %v", err)
	}
	return db
}

func TestCoverCleanupClaimIsExclusiveAndRecoverable(t *testing.T) {
	db := openCoverCleanupTestDB(t)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`
		INSERT INTO cover_object_cleanup_jobs(
			cover_id, library_id, object_key, object_kind,
			available_at, created_at
		) VALUES ('cover-1', 'library-1', 'sources/library-1/7/cover-1.jpg',
		          'SOURCE', ?, ?)
	`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert job: %v", err)
	}

	repository := NewCoverCleanupRepository(db)
	job, err := repository.ClaimNext(
		context.Background(),
		"worker-a",
		now,
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	_, err = repository.ClaimNext(
		context.Background(),
		"worker-b",
		now,
		now.Add(time.Minute),
	)
	if !errors.Is(err, ErrCoverCleanupEmpty) {
		t.Fatalf("concurrent claim err=%v", err)
	}

	job, err = repository.ClaimNext(
		context.Background(),
		"worker-b",
		now.Add(2*time.Minute),
		now.Add(3*time.Minute),
	)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if err = repository.MarkCompleted(
		context.Background(),
		job.ID,
		"worker-b",
		now.Add(2*time.Minute),
	); err != nil {
		t.Fatalf("complete: %v", err)
	}

	var completedAt sql.NullString
	if err = db.QueryRow(
		"SELECT completed_at FROM cover_object_cleanup_jobs WHERE id=?",
		job.ID,
	).Scan(&completedAt); err != nil {
		t.Fatalf("read completion: %v", err)
	}
	if !completedAt.Valid {
		t.Fatal("cleanup job was not completed")
	}
}
