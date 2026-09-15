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

func openCoverOutboxTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil { t.Fatalf("open database: %v", err) }
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE cover_processing_outbox (
			event_id TEXT PRIMARY KEY, cover_id TEXT NOT NULL,
			book_id INTEGER NOT NULL, library_id TEXT NOT NULL,
			event_type TEXT NOT NULL, schema_version INTEGER NOT NULL,
			payload TEXT NOT NULL, attempts INTEGER NOT NULL DEFAULT 0,
			available_at TEXT NOT NULL, published_at TEXT, last_error TEXT,
			created_at TEXT NOT NULL, locked_by TEXT, locked_until TEXT
		);
	`); err != nil { t.Fatalf("create outbox: %v", err) }
	return db
}

func insertCoverOutboxEvent(t *testing.T, db *sql.DB, eventID string, availableAt time.Time) {
	t.Helper()
	stamp := availableAt.UTC().Format(time.RFC3339Nano)
	if _, err := db.Exec(`
		INSERT INTO cover_processing_outbox(
			event_id, cover_id, book_id, library_id, event_type,
			schema_version, payload, available_at, created_at
		) VALUES (?, ?, 7, 'library-1', 'book.covers.process.v1', 1, '{}', ?, ?)
	`, eventID, "cover-"+eventID, stamp, stamp); err != nil {
		t.Fatalf("insert event: %v", err)
	}
}

func TestCoverOutboxClaimIsExclusiveAndCanBePublished(t *testing.T) {
	db := openCoverOutboxTestDB(t)
	now := time.Date(2026, 9, 15, 17, 0, 0, 0, time.UTC)
	insertCoverOutboxEvent(t, db, "event-1", now)
	repository := NewCoverOutboxRepository(db)

	event, err := repository.ClaimNext(context.Background(), "publisher-a", now, now.Add(time.Minute))
	if err != nil { t.Fatalf("claim: %v", err) }
	if event.EventID != "event-1" || event.Attempts != 0 { t.Fatalf("unexpected event: %+v", event) }

	_, err = repository.ClaimNext(context.Background(), "publisher-b", now, now.Add(time.Minute))
	if !errors.Is(err, ErrCoverOutboxEmpty) { t.Fatalf("second claim err=%v", err) }
	if err = repository.MarkPublished(context.Background(), "event-1", "publisher-b", now); !errors.Is(err, ErrCoverOutboxLeaseLost) {
		t.Fatalf("foreign lease err=%v", err)
	}
	if err = repository.MarkPublished(context.Background(), "event-1", "publisher-a", now); err != nil {
		t.Fatalf("mark published: %v", err)
	}

	var publishedAt, lockedBy sql.NullString
	if err = db.QueryRow(`SELECT published_at, locked_by FROM cover_processing_outbox WHERE event_id='event-1'`).
		Scan(&publishedAt, &lockedBy); err != nil { t.Fatalf("read published event: %v", err) }
	if !publishedAt.Valid || lockedBy.Valid { t.Fatalf("published=%v lockedBy=%v", publishedAt, lockedBy) }
}

func TestCoverOutboxExpiredLeaseAndFailureAreRecoverable(t *testing.T) {
	db := openCoverOutboxTestDB(t)
	now := time.Date(2026, 9, 15, 17, 0, 0, 0, time.UTC)
	insertCoverOutboxEvent(t, db, "event-2", now.Add(-time.Hour))
	if _, err := db.Exec(`UPDATE cover_processing_outbox SET locked_by='dead-worker', locked_until=? WHERE event_id='event-2'`,
		now.Add(-time.Second).Format(time.RFC3339Nano)); err != nil { t.Fatalf("expire lease: %v", err) }

	repository := NewCoverOutboxRepository(db)
	event, err := repository.ClaimNext(context.Background(), "publisher-b", now, now.Add(time.Minute))
	if err != nil { t.Fatalf("reclaim: %v", err) }
	if event.EventID != "event-2" { t.Fatalf("unexpected event: %+v", event) }

	nextAttempt := now.Add(30 * time.Second)
	if err = repository.MarkFailed(context.Background(), event.EventID, "publisher-b", nextAttempt, "nats unavailable"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	var attempts int
	var availableAt, lastError string
	var lockedBy sql.NullString
	if err = db.QueryRow(`
		SELECT attempts, available_at, last_error, locked_by
		FROM cover_processing_outbox WHERE event_id='event-2'
	`).Scan(&attempts, &availableAt, &lastError, &lockedBy); err != nil {
		t.Fatalf("read failed event: %v", err)
	}
	if attempts != 1 || availableAt != nextAttempt.Format(time.RFC3339Nano) ||
		lastError != "nats unavailable" || lockedBy.Valid {
		t.Fatalf("attempts=%d available=%q error=%q locked=%v", attempts, availableAt, lastError, lockedBy)
	}

	_, err = repository.ClaimNext(context.Background(), "publisher-c", now, now.Add(time.Minute))
	if !errors.Is(err, ErrCoverOutboxEmpty) { t.Fatalf("early retry err=%v", err) }
}
