package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCoverOutboxEmpty     = errors.New("cover outbox is empty")
	ErrCoverOutboxLeaseLost = errors.New("cover outbox lease was lost")
)

type CoverOutboxEvent struct {
	EventID       string
	CoverID       string
	BookID        int
	LibraryID     string
	EventType     string
	SchemaVersion int
	Payload       string
	Attempts      int
}

type CoverOutboxRepository struct{ db *sql.DB }

func NewCoverOutboxRepository(db *sql.DB) *CoverOutboxRepository {
	return &CoverOutboxRepository{db: db}
}

// ClaimNext réserve atomiquement le prochain événement disponible. Le bail
// permet à une autre instance de reprendre le travail après un arrêt brutal.
func (r *CoverOutboxRepository) ClaimNext(ctx context.Context, workerID string, now, leaseUntil time.Time) (CoverOutboxEvent, error) {
	if workerID == "" || !leaseUntil.After(now) {
		return CoverOutboxEvent{}, fmt.Errorf("invalid cover outbox lease")
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	leaseText := leaseUntil.UTC().Format(time.RFC3339Nano)
	var event CoverOutboxEvent
	err := r.db.QueryRowContext(ctx, `
		UPDATE cover_processing_outbox
		SET locked_by = ?, locked_until = ?
		WHERE event_id = (
			SELECT event_id FROM cover_processing_outbox
			WHERE published_at IS NULL
			  AND available_at <= ?
			  AND (locked_until IS NULL OR locked_until <= ?)
			ORDER BY available_at, created_at, event_id
			LIMIT 1
		)
		RETURNING event_id, cover_id, book_id, library_id, event_type,
		          schema_version, payload, attempts
	`, workerID, leaseText, nowText, nowText).Scan(
		&event.EventID, &event.CoverID, &event.BookID, &event.LibraryID,
		&event.EventType, &event.SchemaVersion, &event.Payload, &event.Attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CoverOutboxEvent{}, ErrCoverOutboxEmpty
	}
	if err != nil {
		return CoverOutboxEvent{}, fmt.Errorf("claim cover outbox event: %w", err)
	}
	return event, nil
}

func (r *CoverOutboxRepository) MarkPublished(ctx context.Context, eventID, workerID string, publishedAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cover_processing_outbox
		SET published_at = ?, last_error = NULL, locked_by = NULL, locked_until = NULL
		WHERE event_id = ? AND locked_by = ? AND published_at IS NULL
	`, publishedAt.UTC().Format(time.RFC3339Nano), eventID, workerID)
	if err != nil {
		return fmt.Errorf("mark cover outbox event published: %w", err)
	}
	return requireCoverOutboxLease(result)
}

func (r *CoverOutboxRepository) MarkFailed(ctx context.Context, eventID, workerID string, availableAt time.Time, lastError string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cover_processing_outbox
		SET attempts = attempts + 1, available_at = ?, last_error = ?,
		    locked_by = NULL, locked_until = NULL
		WHERE event_id = ? AND locked_by = ? AND published_at IS NULL
	`, availableAt.UTC().Format(time.RFC3339Nano), lastError, eventID, workerID)
	if err != nil {
		return fmt.Errorf("reschedule cover outbox event: %w", err)
	}
	return requireCoverOutboxLease(result)
}

func requireCoverOutboxLease(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect cover outbox update: %w", err)
	}
	if affected != 1 {
		return ErrCoverOutboxLeaseLost
	}
	return nil
}
