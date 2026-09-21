package repositories

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func openCoverStatusTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cover-status.db"))
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
			active INTEGER NOT NULL DEFAULT 0,
			processing_by TEXT,
			processing_until TEXT,
			processing_attempts INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE cover_processing_outbox (
			event_id TEXT PRIMARY KEY,
			cover_id TEXT NOT NULL,
			book_id INTEGER NOT NULL,
			library_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			available_at TEXT NOT NULL,
			published_at TEXT,
			last_error TEXT,
			created_at TEXT NOT NULL
		);
		CREATE TABLE cover_object_cleanup_jobs (
			object_key TEXT NOT NULL UNIQUE
		);
		CREATE TABLE audit_logs (
			id TEXT PRIMARY KEY,
			actor_user_id TEXT,
			action TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			resource_id TEXT,
			new_values TEXT,
			success INTEGER NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestCoverStatusLatestIsScopedByLibrary(t *testing.T) {
	db := openCoverStatusTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key,
			error_code, created_at, updated_at
		) VALUES
		('old', 7, 'library-1', 'READY', 'sources/old.jpg', NULL,
		 '2026-09-16T10:00:00Z', '2026-09-16T10:00:00Z'),
		('latest', 7, 'library-1', 'FAILED', 'sources/latest.jpg', 'decode_failed',
		 '2026-09-16T11:00:00Z', '2026-09-16T11:30:00Z'),
		('foreign', 7, 'library-2', 'READY', 'sources/foreign.jpg', NULL,
		 '2026-09-16T12:00:00Z', '2026-09-16T12:00:00Z')
	`); err != nil {
		t.Fatalf("insert covers: %v", err)
	}

	repository := NewCoverRepository(db)
	status, err := repository.LatestStatus(context.Background(), 7, "library-1")
	if err != nil {
		t.Fatalf("latest status: %v", err)
	}
	if status.ID != "latest" || status.Status != "FAILED" ||
		status.ErrorCode != "decode_failed" || status.LibraryID != "library-1" {
		t.Fatalf("status=%+v", status)
	}
	_, err = repository.LatestStatus(context.Background(), 7, "library-missing")
	if !errors.Is(err, ErrBookCoverNotFound) {
		t.Fatalf("missing library err=%v", err)
	}
}

func TestCoverRetryFailedIsAtomicAndIdempotent(t *testing.T) {
	db := openCoverStatusTestDB(t)
	if _, err := db.Exec(`
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key,
			error_code, processing_by, processing_until, processing_attempts,
			created_at, updated_at
		) VALUES (
			'failed-cover', 7, 'library-1', 'FAILED', 'sources/failed.jpg',
			'worker_failed', 'worker-a', '2026-09-16T12:00:00Z', 5,
			'2026-09-16T11:00:00Z', '2026-09-16T11:30:00Z'
		)
	`); err != nil {
		t.Fatalf("insert failed cover: %v", err)
	}

	repository := NewCoverRepository(db)
	payload := `{"schemaVersion":1,"eventId":"retry-event","coverId":"failed-cover"}`
	status, err := repository.RetryFailed(
		context.Background(),
		7,
		"library-1",
		"retry-event",
		payload,
		"actor-1",
		"audit-1",
		"2026-09-16T12:30:00Z",
	)
	if err != nil {
		t.Fatalf("retry failed cover: %v", err)
	}
	if status.Status != "PENDING" || status.ErrorCode != "" {
		t.Fatalf("status=%+v", status)
	}

	var persistedStatus string
	var errorCode, processingBy, processingUntil sql.NullString
	var processingAttempts int
	if err = db.QueryRow(`
		SELECT status, error_code, processing_by, processing_until,
		       processing_attempts
		FROM book_covers WHERE id = 'failed-cover'
	`).Scan(
		&persistedStatus,
		&errorCode,
		&processingBy,
		&processingUntil,
		&processingAttempts,
	); err != nil {
		t.Fatalf("read retried cover: %v", err)
	}
	if persistedStatus != "PENDING" || errorCode.Valid || processingBy.Valid ||
		processingUntil.Valid || processingAttempts != 0 {
		t.Fatalf(
			"status=%q error=%v worker=%v until=%v attempts=%d",
			persistedStatus,
			errorCode,
			processingBy,
			processingUntil,
			processingAttempts,
		)
	}

	var outboxCount, auditCount int
	if err = db.QueryRow(
		"SELECT COUNT(*) FROM cover_processing_outbox WHERE cover_id='failed-cover'",
	).Scan(&outboxCount); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM audit_logs
		WHERE action='RETRY_BOOK_COVER' AND resource_id='7'
	`).Scan(&auditCount); err != nil {
		t.Fatalf("count audit: %v", err)
	}
	if outboxCount != 1 || auditCount != 1 {
		t.Fatalf("outbox=%d audit=%d", outboxCount, auditCount)
	}

	_, err = repository.RetryFailed(
		context.Background(),
		7,
		"library-1",
		"retry-event-2",
		payload,
		"actor-1",
		"audit-2",
		"2026-09-16T12:31:00Z",
	)
	if !errors.Is(err, ErrBookCoverNotRetryable) {
		t.Fatalf("second retry err=%v", err)
	}
}

func TestCoverRetryRejectsSourceQueuedForCleanup(t *testing.T) {
	db := openCoverStatusTestDB(t)
	if _, err := db.Exec(`
        INSERT INTO book_covers(id, book_id, library_id, status,
                                source_object_key, created_at, updated_at)
        VALUES ('expired', 7, 'library-1', 'FAILED', 'sources/expired.jpg',
                '2026-09-16T10:00:00Z', '2026-09-16T10:00:00Z');
        INSERT INTO cover_object_cleanup_jobs(object_key)
        VALUES ('sources/expired.jpg');
    `); err != nil {
		t.Fatalf("seed expired cover: %v", err)
	}
	repository := NewCoverRepository(db)
	current, err := repository.LatestStatus(context.Background(), 7, "library-1")
	if err != nil || current.SourceRetained {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	_, err = repository.RetryFailed(context.Background(), 7, "library-1",
		"event-2", `{}`, "actor-1", "audit-2", "2026-09-16T12:30:00Z")
	if !errors.Is(err, ErrBookCoverNotRetryable) {
		t.Fatalf("retry error=%v", err)
	}
	var events int
	if err := db.QueryRow("SELECT COUNT(*) FROM cover_processing_outbox").Scan(&events); err != nil || events != 0 {
		t.Fatalf("events=%d err=%v", events, err)
	}
}
