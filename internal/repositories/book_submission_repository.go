package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
)

var ErrInvalidBookSubmission = errors.New("invalid book submission")

type PendingBookSubmission struct {
	ID                string
	LibraryID         string
	ActorUserID       string
	Book              models.BookInput
	SourceObjectKey   string
	SourceContentType string
	SourceFormat      string
	SourceWidth       int
	SourceHeight      int
	SourceSize        int64
	ExpiresAt         string
}

// BookSubmissionRepository owns the quarantine persistence boundary. It never
// inserts into defta; only an approved moderation decision may create a book.
type BookSubmissionRepository struct {
	db *sql.DB
}

func NewBookSubmissionRepository(db *sql.DB) *BookSubmissionRepository {
	return &BookSubmissionRepository{db: db}
}

func (r *BookSubmissionRepository) CreatePending(
	ctx context.Context,
	submission PendingBookSubmission,
	eventID string,
	payload string,
	auditID string,
	now string,
) error {
	if submission.ID == "" || submission.LibraryID == "" ||
		submission.ActorUserID == "" || submission.Book.Title == "" ||
		submission.SourceObjectKey == "" || submission.ExpiresAt == "" ||
		submission.SourceWidth < 1 || submission.SourceHeight < 1 ||
		submission.SourceSize < 1 {
		return ErrInvalidBookSubmission
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin book submission: %w", err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO book_submissions(
			id, library_id, actor_user_id, title, auteur, editeur, price,
			volume, status, tags, categorie, cover_url, source_object_key,
			source_content_type, source_format, source_width, source_height,
			source_size, moderation_status, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
		          'PENDING_SCAN', ?, ?, ?)
	`, submission.ID, submission.LibraryID, submission.ActorUserID,
		submission.Book.Title, submission.Book.Auteur, submission.Book.Editeur,
		submission.Book.Price, submission.Book.Volume, submission.Book.Status,
		submission.Book.Tags, submission.Book.Categorie, submission.Book.CoverURL,
		submission.SourceObjectKey, submission.SourceContentType,
		submission.SourceFormat, submission.SourceWidth, submission.SourceHeight,
		submission.SourceSize, submission.ExpiresAt, now, now); err != nil {
		return fmt.Errorf("insert pending book submission: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO book_submission_outbox(
			event_id, submission_id, library_id, event_type, schema_version,
			payload, attempts, available_at, created_at
		) VALUES (?, ?, ?, 'book.submissions.moderate.v1', 1, ?, 0, ?, ?)
	`, eventID, submission.ID, submission.LibraryID, payload, now, now); err != nil {
		return fmt.Errorf("insert book submission outbox: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(
			id, actor_user_id, action, resource_type, resource_id,
			new_values, success, created_at
		) VALUES (?, ?, 'CREATE_BOOK_SUBMISSION', 'BOOK_SUBMISSION', ?, ?, 1, ?)
	`, auditID, submission.ActorUserID, submission.ID, payload, now); err != nil {
		return fmt.Errorf("audit book submission: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit book submission: %w", err)
	}
	return nil
}
