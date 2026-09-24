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

		CREATE TABLE book_submissions (
			id TEXT PRIMARY KEY,
			library_id TEXT NOT NULL,
			source_object_key TEXT NOT NULL,
			moderation_status TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			decision_code TEXT,
			updated_at TEXT NOT NULL
		);

		CREATE TABLE book_covers (
			id TEXT PRIMARY KEY,
			book_id TEXT NOT NULL,
			library_id TEXT NOT NULL,
			status TEXT NOT NULL,
			source_object_key TEXT,
			master_object_key TEXT,
			large_jpeg_object_key TEXT,
			large_webp_object_key TEXT,
			thumb_jpeg_object_key TEXT,
			thumb_webp_object_key TEXT,
			active INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL
		);

	`); err != nil {
		t.Fatalf("create cover cleanup schema: %v", err)
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

func TestCoverCleanupFailureReleasesLeaseAndDelaysRetry(t *testing.T) {
	db := openCoverCleanupTestDB(t)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`
		INSERT INTO cover_object_cleanup_jobs(
			cover_id, library_id, object_key, object_kind,
			available_at, created_at
		) VALUES ('cover-2', 'library-1',
		          'variants/library-1/7/cover-2/thumb.webp',
		          'GENERATED', ?, ?)
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
	retryAt := now.Add(30 * time.Second)
	if err = repository.MarkFailed(
		context.Background(),
		job.ID,
		"worker-a",
		retryAt,
		"minio unavailable",
	); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	var attempts int
	var availableAt, lastError string
	var lockedBy sql.NullString
	if err = db.QueryRow(`
		SELECT attempts, available_at, last_error, locked_by
		FROM cover_object_cleanup_jobs WHERE id=?
	`, job.ID).Scan(&attempts, &availableAt, &lastError, &lockedBy); err != nil {
		t.Fatalf("read retry: %v", err)
	}
	if attempts != 1 ||
		availableAt != retryAt.Format(time.RFC3339Nano) ||
		lastError != "minio unavailable" ||
		lockedBy.Valid {
		t.Fatalf(
			"attempts=%d available=%q error=%q locked=%v",
			attempts,
			availableAt,
			lastError,
			lockedBy,
		)
	}
}

func TestCoverCleanupReconcileIsIdempotentAndPreservesActiveVariants(t *testing.T) {
	db := openCoverCleanupTestDB(t)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-48 * time.Hour).Format(time.RFC3339Nano)

	if _, err := db.Exec(`
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key,
			master_object_key, large_jpeg_object_key,
			large_webp_object_key, thumb_jpeg_object_key,
			thumb_webp_object_key, active, updated_at
		) VALUES
		(
			'cover-old', 'book-1', 'library-1', 'READY',
			'sources/library-1/book-1/cover-old.jpg',
			'masters/library-1/book-1/cover-old/master.jpg',
			'variants/library-1/book-1/cover-old/large.jpg',
			'variants/library-1/book-1/cover-old/large.webp',
			'variants/library-1/book-1/cover-old/thumb.jpg',
			'variants/library-1/book-1/cover-old/thumb.webp',
			0, ?
		),
		(
			'cover-active', 'book-1', 'library-1', 'READY',
			'sources/library-1/book-1/cover-active.jpg',
			'masters/library-1/book-1/cover-active/master.jpg',
			'variants/library-1/book-1/cover-active/large.jpg',
			'variants/library-1/book-1/cover-active/large.webp',
			'variants/library-1/book-1/cover-active/thumb.jpg',
			'variants/library-1/book-1/cover-active/thumb.webp',
			1, ?
		),
		(
			'cover-failed', 'book-1', 'library-1', 'FAILED',
			'sources/library-1/book-1/cover-failed.png',
			NULL, NULL, NULL, NULL, NULL,
			0, ?
		)
	`, updatedAt, updatedAt, updatedAt); err != nil {
		t.Fatalf("insert covers: %v", err)
	}

	repository := NewCoverCleanupRepository(db)
	inserted, err := repository.Reconcile(
		context.Background(),
		now,
		now.Add(-24*time.Hour),
	)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if inserted != 8 {
		t.Fatalf("inserted=%d want=8", inserted)
	}

	inserted, err = repository.Reconcile(
		context.Background(),
		now,
		now.Add(-24*time.Hour),
	)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if inserted != 0 {
		t.Fatalf("second inserted=%d want=0", inserted)
	}

	var jobs int
	if err = db.QueryRow(
		"SELECT COUNT(*) FROM cover_object_cleanup_jobs",
	).Scan(&jobs); err != nil {
		t.Fatalf("count cleanup jobs: %v", err)
	}
	if jobs != 8 {
		t.Fatalf("jobs=%d want=8", jobs)
	}

	var activeVariants int
	if err = db.QueryRow(`
		SELECT COUNT(*)
		FROM cover_object_cleanup_jobs
		WHERE object_key LIKE 'masters/library-1/book-1/cover-active/%'
		   OR object_key LIKE 'variants/library-1/book-1/cover-active/%'
	`).Scan(&activeVariants); err != nil {
		t.Fatalf("count active variants: %v", err)
	}
	if activeVariants != 0 {
		t.Fatalf("active variant cleanup jobs=%d want=0", activeVariants)
	}
}


func TestCoverCleanupReconcileQueuesRejectedAndExpiredSubmissions(t *testing.T) {
	db := openCoverCleanupTestDB(t)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour).Format(time.RFC3339Nano)
	future := now.Add(time.Hour).Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO book_submissions(
			id, library_id, source_object_key, moderation_status, expires_at, updated_at
		) VALUES
			('rejected', 'library-1', 'quarantine/library-1/rejected.jpg', 'REJECTED', ?, ?),
			('review-expired', 'library-1', 'quarantine/library-1/review.jpg', 'REVIEW_REQUIRED', ?, ?),
			('pending-expired', 'library-1', 'quarantine/library-1/pending.jpg', 'PENDING_SCAN', ?, ?),
			('failed-retryable', 'library-1', 'quarantine/library-1/retry.jpg', 'FAILED', ?, ?)
	`, future, now.Format(time.RFC3339Nano), expired, now.Format(time.RFC3339Nano),
		expired, now.Format(time.RFC3339Nano), future, now.Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("insert submissions: %v", err)
	}

	repository := NewCoverCleanupRepository(db)
	inserted, err := repository.Reconcile(context.Background(), now, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if inserted != 3 {
		t.Fatalf("inserted=%d want=3", inserted)
	}
	var status, code string
	if err = db.QueryRow(`SELECT moderation_status, decision_code FROM book_submissions WHERE id='pending-expired'`).Scan(&status, &code); err != nil {
		t.Fatalf("read expired submission: %v", err)
	}
	if status != "FAILED" || code != "SOURCE_EXPIRED" {
		t.Fatalf("expired submission status=%q code=%q", status, code)
	}
	var jobs int
	if err = db.QueryRow(`SELECT COUNT(*) FROM cover_object_cleanup_jobs WHERE cover_id LIKE 'submission:%'`).Scan(&jobs); err != nil {
		t.Fatalf("count submission jobs: %v", err)
	}
	if jobs != 3 {
		t.Fatalf("submission jobs=%d want=3", jobs)
	}
	inserted, err = repository.Reconcile(context.Background(), now, now.Add(-24*time.Hour))
	if err != nil || inserted != 0 {
		t.Fatalf("second reconcile inserted=%d err=%v", inserted, err)
	}
}
