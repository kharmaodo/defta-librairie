package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrBookCoverNotFound     = errors.New("book cover not found")
	ErrBookCoverNotRetryable = errors.New("book cover is not retryable")
)

type BookCoverStatus struct {
	ID              string
	BookID          int
	LibraryID       string
	Status          string
	SourceObjectKey string
	SourceRetained  bool
	ErrorCode       string
	UpdatedAt       string
}

func (r *CoverRepository) LatestStatus(
	ctx context.Context,
	bookID int,
	libraryID string,
) (BookCoverStatus, error) {
	if bookID < 1 || libraryID == "" {
		return BookCoverStatus{}, ErrBookCoverNotFound
	}
	var status BookCoverStatus
	var errorCode sql.NullString
	var retained int
	err := r.db.QueryRowContext(ctx, `
		SELECT id, book_id, library_id, status, source_object_key,
		       error_code, updated_at,
		       NOT EXISTS (
		           SELECT 1 FROM cover_object_cleanup_jobs AS cleanup
		           WHERE cleanup.object_key = book_covers.source_object_key
		       )
		FROM book_covers
		WHERE book_id = ? AND library_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, bookID, libraryID).Scan(
		&status.ID,
		&status.BookID,
		&status.LibraryID,
		&status.Status,
		&status.SourceObjectKey,
		&errorCode,
		&status.UpdatedAt,
		&retained,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return BookCoverStatus{}, ErrBookCoverNotFound
	}
	if err != nil {
		return BookCoverStatus{}, fmt.Errorf("read latest book cover status: %w", err)
	}
	if errorCode.Valid {
		status.ErrorCode = errorCode.String
	}
	status.SourceRetained = retained == 1
	return status, nil
}

func (r *CoverRepository) RetryFailed(
	ctx context.Context,
	bookID int,
	libraryID, eventID, payload, actorID, auditID, now string,
) (BookCoverStatus, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BookCoverStatus{}, fmt.Errorf("begin book cover retry: %w", err)
	}
	defer tx.Rollback()

	var current BookCoverStatus
	if err = tx.QueryRowContext(ctx, `
		SELECT id, book_id, library_id, status, source_object_key, updated_at
		FROM book_covers
		WHERE book_id = ? AND library_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, bookID, libraryID).Scan(
		&current.ID,
		&current.BookID,
		&current.LibraryID,
		&current.Status,
		&current.SourceObjectKey,
		&current.UpdatedAt,
	); errors.Is(err, sql.ErrNoRows) {
		return BookCoverStatus{}, ErrBookCoverNotFound
	} else if err != nil {
		return BookCoverStatus{}, fmt.Errorf("read retryable book cover: %w", err)
	}
	if current.Status != "FAILED" {
		return BookCoverStatus{}, ErrBookCoverNotRetryable
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE book_covers
		SET status = 'PENDING', error_code = NULL,
		    processing_by = NULL, processing_until = NULL,
		    processing_attempts = 0, updated_at = ?
		WHERE id = ? AND library_id = ? AND status = 'FAILED'
		  AND NOT EXISTS (
		      SELECT 1 FROM cover_object_cleanup_jobs AS cleanup
		      WHERE cleanup.object_key = book_covers.source_object_key
		  )
	`, now, current.ID, libraryID)
	if err != nil {
		return BookCoverStatus{}, fmt.Errorf("reset failed book cover: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return BookCoverStatus{}, fmt.Errorf("inspect book cover retry: %w", err)
	}
	if updated != 1 {
		return BookCoverStatus{}, ErrBookCoverNotRetryable
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO cover_processing_outbox(
			event_id, cover_id, book_id, library_id, event_type,
			schema_version, payload, attempts, available_at, created_at
		) VALUES (?, ?, ?, ?, 'book.covers.process.v1', 1, ?, 0, ?, ?)
	`, eventID, current.ID, bookID, libraryID, payload, now, now); err != nil {
		return BookCoverStatus{}, fmt.Errorf("insert retry cover outbox: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(
			id, actor_user_id, action, resource_type, resource_id,
			new_values, success, created_at
		) VALUES (?, ?, 'RETRY_BOOK_COVER', 'BOOK', ?, ?, 1, ?)
	`, auditID, actorID, bookID, payload, now); err != nil {
		return BookCoverStatus{}, fmt.Errorf("audit book cover retry: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return BookCoverStatus{}, fmt.Errorf("commit book cover retry: %w", err)
	}
	current.Status = "PENDING"
	current.ErrorCode = ""
	current.UpdatedAt = now
	return current, nil
}
