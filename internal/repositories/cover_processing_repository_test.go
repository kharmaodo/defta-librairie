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

func openCoverProcessingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "processing.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE book_covers (
			id TEXT PRIMARY KEY,
			book_id INTEGER NOT NULL,
			library_id TEXT NOT NULL,
			status TEXT NOT NULL,
			source_object_key TEXT NOT NULL,
			error_code TEXT,
			processing_by TEXT,
			processing_until TEXT,
			processing_attempts INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create covers: %v", err)
	}
	return db
}

func insertProcessingCover(t *testing.T, db *sql.DB, id, status string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key, updated_at
		) VALUES (?, 7, 'library-1', ?, ?, 'initial')
	`, id, status, "sources/library-1/7/"+id+".png"); err != nil {
		t.Fatalf("insert cover: %v", err)
	}
}

func TestCoverProcessingClaimIsExclusiveAndRecoverable(t *testing.T) {
	db := openCoverProcessingTestDB(t)
	insertProcessingCover(t, db, "cover-1", "PENDING")
	repository := NewCoverProcessingRepository(db)
	now := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)
	source := "sources/library-1/7/cover-1.png"

	claim, err := repository.Claim(
		context.Background(),
		"cover-1",
		7,
		"library-1",
		source,
		"worker-a",
		now,
		now.Add(time.Minute),
	)
	if err != nil || claim != CoverProcessingClaimed {
		t.Fatalf("claim=%v err=%v", claim, err)
	}
	_, err = repository.Claim(
		context.Background(),
		"cover-1",
		7,
		"library-1",
		source,
		"worker-b",
		now,
		now.Add(time.Minute),
	)
	if !errors.Is(err, ErrCoverProcessingBusy) {
		t.Fatalf("concurrent claim err=%v", err)
	}

	if _, err = db.Exec(`
		UPDATE book_covers SET processing_until=? WHERE id='cover-1'
	`, now.Add(-time.Second).Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("expire lease: %v", err)
	}
	claim, err = repository.Claim(
		context.Background(),
		"cover-1",
		7,
		"library-1",
		source,
		"worker-b",
		now,
		now.Add(time.Minute),
	)
	if err != nil || claim != CoverProcessingClaimed {
		t.Fatalf("reclaim=%v err=%v", claim, err)
	}

	var attempts int
	var processingBy string
	if err = db.QueryRow(`
		SELECT processing_attempts, processing_by
		FROM book_covers WHERE id='cover-1'
	`).Scan(&attempts, &processingBy); err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if attempts != 2 || processingBy != "worker-b" {
		t.Fatalf("attempts=%d worker=%q", attempts, processingBy)
	}
}

func TestCoverProcessingReleaseAndTerminalStates(t *testing.T) {
	db := openCoverProcessingTestDB(t)
	insertProcessingCover(t, db, "cover-pending", "PENDING")
	insertProcessingCover(t, db, "cover-ready", "READY")
	insertProcessingCover(t, db, "cover-failed", "FAILED")
	repository := NewCoverProcessingRepository(db)
	now := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)

	_, err := repository.Claim(
		context.Background(),
		"cover-pending",
		7,
		"library-1",
		"sources/library-1/7/cover-pending.png",
		"worker-a",
		now,
		now.Add(time.Minute),
	)
	if err != nil {
		t.Fatalf("claim pending: %v", err)
	}
	if err = repository.Release(
		context.Background(),
		"cover-pending",
		"worker-a",
		"PROCESSOR_UNAVAILABLE",
		now,
	); err != nil {
		t.Fatalf("release: %v", err)
	}

	var status, errorCode string
	var processingBy sql.NullString
	if err = db.QueryRow(`
		SELECT status, error_code, processing_by
		FROM book_covers WHERE id='cover-pending'
	`).Scan(&status, &errorCode, &processingBy); err != nil {
		t.Fatalf("read released cover: %v", err)
	}
	if status != "PENDING" || errorCode != "PROCESSOR_UNAVAILABLE" || processingBy.Valid {
		t.Fatalf("status=%q error=%q worker=%v", status, errorCode, processingBy)
	}

	claim, err := repository.Claim(
		context.Background(),
		"cover-ready",
		7,
		"library-1",
		"sources/library-1/7/cover-ready.png",
		"worker-a",
		now,
		now.Add(time.Minute),
	)
	if err != nil || claim != CoverProcessingAlreadyReady {
		t.Fatalf("ready claim=%v err=%v", claim, err)
	}
	_, err = repository.Claim(
		context.Background(),
		"cover-failed",
		7,
		"library-1",
		"sources/library-1/7/cover-failed.png",
		"worker-a",
		now,
		now.Add(time.Minute),
	)
	if !errors.Is(err, ErrCoverProcessingTerminal) {
		t.Fatalf("failed claim err=%v", err)
	}
}

func TestCoverProcessingMarkFailedIsIdempotentForTerminalCover(t *testing.T) {
	db := openCoverProcessingTestDB(t)
	insertProcessingCover(t, db, "cover-3", "PENDING")
	repository := NewCoverProcessingRepository(db)
	now := time.Date(2026, 9, 16, 9, 0, 0, 0, time.UTC)

	if err := repository.MarkFailed(
		context.Background(), "cover-3", "MAX_DELIVERIES", now,
	); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if err := repository.MarkFailed(
		context.Background(), "cover-3", "MAX_DELIVERIES", now,
	); !errors.Is(err, ErrCoverProcessingTerminal) {
		t.Fatalf("second mark err=%v", err)
	}
}
