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


var (
	ErrBookSubmissionNotFound = errors.New("book submission not found")
	ErrBookSubmissionState    = errors.New("book submission is not pending moderation")
)

type ModerationSubmission struct {
	ID                string
	LibraryID         string
	ActorUserID       string
	Book              models.BookInput
	SourceObjectKey   string
	SourceContentType string
	SourceSize        int64
}

// ManualReviewDecision is a terminal decision made by an authorized reviewer
// after the model has returned an ambiguous result.
type ManualReviewDecision string

const (
	ManualReviewApprove ManualReviewDecision = "APPROVE"
	ManualReviewReject  ManualReviewDecision = "REJECT"
)

func (r *BookSubmissionRepository) ClaimForModeration(ctx context.Context, id, now string) (ModerationSubmission, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ModerationSubmission{}, fmt.Errorf("begin claim submission: %w", err)
	}
	defer tx.Rollback()

	var submission ModerationSubmission
	err = tx.QueryRowContext(ctx, `
		SELECT id, library_id, actor_user_id, title, auteur, editeur, price, volume,
		       status, tags, categorie, cover_url, source_object_key,
		       source_content_type, source_size
		FROM book_submissions
		WHERE id=? AND moderation_status='PENDING_SCAN'
	`, id).Scan(
		&submission.ID, &submission.LibraryID, &submission.ActorUserID,
		&submission.Book.Title, &submission.Book.Auteur, &submission.Book.Editeur,
		&submission.Book.Price, &submission.Book.Volume, &submission.Book.Status,
		&submission.Book.Tags, &submission.Book.Categorie, &submission.Book.CoverURL,
		&submission.SourceObjectKey, &submission.SourceContentType, &submission.SourceSize,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ModerationSubmission{}, ErrBookSubmissionNotFound
	}
	if err != nil {
		return ModerationSubmission{}, fmt.Errorf("read pending submission: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_submissions
		SET moderation_status='SCANNING', updated_at=?
		WHERE id=? AND moderation_status='PENDING_SCAN'
	`, now, id)
	if err != nil {
		return ModerationSubmission{}, fmt.Errorf("claim pending submission: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return ModerationSubmission{}, ErrBookSubmissionState
	}
	if err = tx.Commit(); err != nil {
		return ModerationSubmission{}, fmt.Errorf("commit claim submission: %w", err)
	}
	return submission, nil
}

func (r *BookSubmissionRepository) CompleteModeration(
	ctx context.Context, submission ModerationSubmission, class string, score float64,
	modelVersion, decisionCode, decisionAuditID, bookAuditID, now string,
) (int, error) {
	if submission.ID == "" || decisionAuditID == "" || modelVersion == "" || now == "" ||
		(score < 0 || score > 1) || (class != "SAFE" && class != "REVIEW" && class != "UNSAFE") {
		return 0, ErrInvalidBookSubmission
	}
	status := "REVIEW_REQUIRED"
	if class == "UNSAFE" {
		status = "REJECTED"
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin moderation decision: %w", err)
	}
	defer tx.Rollback()

	createdBookID := 0
	if class == "SAFE" {
		if bookAuditID == "" {
			return 0, ErrInvalidBookSubmission
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO defta(title, auteur, editeur, price, volume, status, tags, categorie, coverUrl,
			                  library_id, created_at, updated_at, version)
			VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''),
			        NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, 1)
		`, submission.Book.Title, submission.Book.Auteur, submission.Book.Editeur,
			submission.Book.Price, submission.Book.Volume, submission.Book.Status,
			submission.Book.Tags, submission.Book.Categorie, submission.Book.CoverURL,
			submission.LibraryID, now, now)
		if err != nil {
			return 0, fmt.Errorf("create approved book: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("read approved book id: %w", err)
		}
		createdBookID = int(id)
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO book_inventory(book_id, library_id, quantity, low_stock_threshold, version, updated_at)
			VALUES (?, ?, 0, COALESCE((SELECT default_low_stock_threshold FROM library_settings WHERE library_id=?),5), 1, ?)
		`, createdBookID, submission.LibraryID, submission.LibraryID, now); err != nil {
			return 0, fmt.Errorf("initialize approved book inventory: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
			VALUES (?, ?, 'CREATE_BOOK', 'BOOK', ?, ?, 1, ?)
		`, bookAuditID, submission.ActorUserID, createdBookID, `{"origin":"approved_book_submission"}`, now); err != nil {
			return 0, fmt.Errorf("audit approved book: %w", err)
		}
		status = "APPROVED"
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_submissions
		SET moderation_status=?, moderation_score=?, moderation_model_version=?,
		    decision_code=?, created_book_id=NULLIF(?, 0), updated_at=?
		WHERE id=? AND moderation_status='SCANNING'
	`, status, score, modelVersion, decisionCode, createdBookID, now, submission.ID)
	if err != nil {
		return 0, fmt.Errorf("record moderation decision: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return 0, ErrBookSubmissionState
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'DECIDE_BOOK_SUBMISSION_MODERATION', 'BOOK_SUBMISSION', ?, ?, 1, ?)
	`, decisionAuditID, submission.ActorUserID, submission.ID,
		fmt.Sprintf(`{"class":%q,"score":%g,"modelVersion":%q,"decisionCode":%q}`, class, score, modelVersion, decisionCode), now); err != nil {
		return 0, fmt.Errorf("audit moderation decision: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit moderation decision: %w", err)
	}
	return createdBookID, nil
}

// DecideReview records a human decision on a REVIEW_REQUIRED submission.  The
// compare-and-set state check makes the operation idempotent from the caller's
// perspective: a second decision cannot create a second book.
func (r *BookSubmissionRepository) DecideReview(
	ctx context.Context, submissionID, reviewerUserID string, decision ManualReviewDecision,
	decisionAuditID, bookAuditID, now string,
) (int, error) {
	if submissionID == "" || reviewerUserID == "" || decisionAuditID == "" || now == "" ||
		(decision != ManualReviewApprove && decision != ManualReviewReject) {
		return 0, ErrInvalidBookSubmission
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin manual book submission decision: %w", err)
	}
	defer tx.Rollback()

	var submission ModerationSubmission
	err = tx.QueryRowContext(ctx, `
		SELECT id, library_id, actor_user_id, title, auteur, editeur, price, volume,
		       status, tags, categorie, cover_url, source_object_key,
		       source_content_type, source_size
		FROM book_submissions
		WHERE id=? AND moderation_status='REVIEW_REQUIRED'
	`, submissionID).Scan(
		&submission.ID, &submission.LibraryID, &submission.ActorUserID,
		&submission.Book.Title, &submission.Book.Auteur, &submission.Book.Editeur,
		&submission.Book.Price, &submission.Book.Volume, &submission.Book.Status,
		&submission.Book.Tags, &submission.Book.Categorie, &submission.Book.CoverURL,
		&submission.SourceObjectKey, &submission.SourceContentType, &submission.SourceSize,
	)
	if errors.Is(err, sql.ErrNoRows) {
		var exists int
		if checkErr := tx.QueryRowContext(ctx, `SELECT 1 FROM book_submissions WHERE id=?`, submissionID).Scan(&exists); errors.Is(checkErr, sql.ErrNoRows) {
			return 0, ErrBookSubmissionNotFound
		}
		return 0, ErrBookSubmissionState
	}
	if err != nil {
		return 0, fmt.Errorf("read review-required submission: %w", err)
	}

	status, code, createdBookID := "REJECTED", "MANUAL_REJECTED", 0
	if decision == ManualReviewApprove {
		if bookAuditID == "" {
			return 0, ErrInvalidBookSubmission
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO defta(title, auteur, editeur, price, volume, status, tags, categorie, coverUrl,
			                  library_id, created_at, updated_at, version)
			VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''),
			        NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, 1)
		`, submission.Book.Title, submission.Book.Auteur, submission.Book.Editeur,
			submission.Book.Price, submission.Book.Volume, submission.Book.Status,
			submission.Book.Tags, submission.Book.Categorie, submission.Book.CoverURL,
			submission.LibraryID, now, now)
		if err != nil {
			return 0, fmt.Errorf("create manually approved book: %w", err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return 0, fmt.Errorf("read manually approved book id: %w", err)
		}
		createdBookID = int(id)
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO book_inventory(book_id, library_id, quantity, low_stock_threshold, version, updated_at)
			VALUES (?, ?, 0, COALESCE((SELECT default_low_stock_threshold FROM library_settings WHERE library_id=?),5), 1, ?)
		`, createdBookID, submission.LibraryID, submission.LibraryID, now); err != nil {
			return 0, fmt.Errorf("initialize manually approved book inventory: %w", err)
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
			VALUES (?, ?, 'CREATE_BOOK', 'BOOK', ?, ?, 1, ?)
		`, bookAuditID, reviewerUserID, createdBookID, `{"origin":"manually_approved_book_submission"}`, now); err != nil {
			return 0, fmt.Errorf("audit manually approved book: %w", err)
		}
		status, code = "APPROVED", "MANUAL_APPROVED"
	}

	result, err := tx.ExecContext(ctx, `
		UPDATE book_submissions
		SET moderation_status=?, decision_code=?, created_book_id=NULLIF(?, 0), updated_at=?
		WHERE id=? AND moderation_status='REVIEW_REQUIRED'
	`, status, code, createdBookID, now, submissionID)
	if err != nil {
		return 0, fmt.Errorf("record manual decision: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return 0, ErrBookSubmissionState
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'MANUALLY_DECIDE_BOOK_SUBMISSION_MODERATION', 'BOOK_SUBMISSION', ?, ?, 1, ?)
	`, decisionAuditID, reviewerUserID, submissionID,
		fmt.Sprintf(`{"decision":%q,"decisionCode":%q}`, decision, code), now); err != nil {
		return 0, fmt.Errorf("audit manual moderation decision: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit manual moderation decision: %w", err)
	}
	return createdBookID, nil
}

func (r *BookSubmissionRepository) FailModeration(ctx context.Context, id, decisionCode, auditID, now string) error {
	if id == "" || decisionCode == "" || auditID == "" || now == "" {
		return ErrInvalidBookSubmission
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin failed moderation: %w", err)
	}
	defer tx.Rollback()
	var actorUserID string
	err = tx.QueryRowContext(ctx, `SELECT actor_user_id FROM book_submissions WHERE id=? AND moderation_status='SCANNING'`, id).Scan(&actorUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBookSubmissionState
	}
	if err != nil {
		return fmt.Errorf("read failed moderation submission: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_submissions SET moderation_status='FAILED', decision_code=?, updated_at=?
		WHERE id=? AND moderation_status='SCANNING'
	`, decisionCode, now, id)
	if err != nil {
		return fmt.Errorf("mark failed moderation: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return ErrBookSubmissionState
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'DECIDE_BOOK_SUBMISSION_MODERATION', 'BOOK_SUBMISSION', ?, ?, 0, ?)
	`, auditID, actorUserID, id, fmt.Sprintf(`{"decisionCode":%q}`, decisionCode), now); err != nil {
		return fmt.Errorf("audit failed moderation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit failed moderation: %w", err)
	}
	return nil
}

// RetryFailed returns a technical failure to the moderation queue. Only a
// FAILED submission whose quarantined source has not expired can be retried.
func (r *BookSubmissionRepository) RetryFailed(ctx context.Context, submissionID, reviewerUserID, eventID, payload, auditID, now string) error {
	if submissionID == "" || reviewerUserID == "" || eventID == "" || payload == "" || auditID == "" || now == "" {
		return ErrInvalidBookSubmission
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return fmt.Errorf("begin retry book submission: %w", err) }
	defer tx.Rollback()
	var libraryID string
	err = tx.QueryRowContext(ctx, `SELECT library_id FROM book_submissions WHERE id=? AND moderation_status='FAILED' AND expires_at>?`, submissionID, now).Scan(&libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		var exists int
		if checkErr := tx.QueryRowContext(ctx, `SELECT 1 FROM book_submissions WHERE id=?`, submissionID).Scan(&exists); errors.Is(checkErr, sql.ErrNoRows) { return ErrBookSubmissionNotFound }
		return ErrBookSubmissionState
	}
	if err != nil { return fmt.Errorf("read retryable book submission: %w", err) }
	result, err := tx.ExecContext(ctx, `UPDATE book_submissions SET moderation_status='PENDING_SCAN', decision_code='RETRY_REQUESTED', updated_at=? WHERE id=? AND moderation_status='FAILED'`, now, submissionID)
	if err != nil { return fmt.Errorf("requeue book submission: %w", err) }
	if rows, rowErr := result.RowsAffected(); rowErr != nil || rows != 1 { return ErrBookSubmissionState }
	if _, err = tx.ExecContext(ctx, `INSERT INTO book_submission_outbox(event_id, submission_id, library_id, event_type, schema_version, payload, attempts, available_at, created_at) VALUES (?, ?, ?, 'book.submissions.moderate.v1', 1, ?, 0, ?, ?)`, eventID, submissionID, libraryID, payload, now, now); err != nil { return fmt.Errorf("queue book submission retry: %w", err) }
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at) VALUES (?, ?, 'RETRY_BOOK_SUBMISSION_MODERATION', 'BOOK_SUBMISSION', ?, ?, 1, ?)`, auditID, reviewerUserID, submissionID, `{"reason":"manual_retry"}`, now); err != nil { return fmt.Errorf("audit book submission retry: %w", err) }
	if err = tx.Commit(); err != nil { return fmt.Errorf("commit book submission retry: %w", err) }
	return nil
}


type PendingSubmissionOutboxEvent struct {
	EventID string
	Payload string
}

func (r *BookSubmissionRepository) PendingOutbox(ctx context.Context, now string, limit int) ([]PendingSubmissionOutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidBookSubmission
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_id, payload
		FROM book_submission_outbox
		WHERE published_at IS NULL AND available_at <= ?
		ORDER BY created_at, event_id
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending submission outbox: %w", err)
	}
	defer rows.Close()
	events := make([]PendingSubmissionOutboxEvent, 0)
	for rows.Next() {
		var event PendingSubmissionOutboxEvent
		if err = rows.Scan(&event.EventID, &event.Payload); err != nil {
			return nil, fmt.Errorf("scan pending submission outbox: %w", err)
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *BookSubmissionRepository) MarkOutboxPublished(ctx context.Context, eventID, now string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE book_submission_outbox
		SET published_at=?, last_error=NULL
		WHERE event_id=? AND published_at IS NULL
	`, now, eventID)
	if err != nil {
		return fmt.Errorf("mark submission outbox published: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return ErrBookSubmissionNotFound
	}
	return nil
}

func (r *BookSubmissionRepository) RecordOutboxFailure(ctx context.Context, eventID, message, now string) error {
	if message == "" {
		message = "publish_failed"
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE book_submission_outbox
		SET attempts=attempts+1, last_error=?, available_at=?
		WHERE event_id=? AND published_at IS NULL
	`, message, now, eventID)
	if err != nil {
		return fmt.Errorf("record submission outbox failure: %w", err)
	}
	return nil
}


type ApprovedSubmissionForPromotion struct {
	ID                string
	BookID            int
	LibraryID         string
	ActorUserID       string
	SourceObjectKey   string
	SourceContentType string
	SourceFormat      string
	SourceWidth       int
	SourceHeight      int
	SourceSize        int64
}

func (r *BookSubmissionRepository) ApprovedWithoutCover(ctx context.Context, limit int) ([]ApprovedSubmissionForPromotion, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidBookSubmission
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.created_book_id, s.library_id, s.actor_user_id,
		       s.source_object_key, s.source_content_type, s.source_format,
		       s.source_width, s.source_height, s.source_size
		FROM book_submissions s
		LEFT JOIN book_covers c ON c.book_id=s.created_book_id
		WHERE s.moderation_status='APPROVED'
		  AND s.created_book_id IS NOT NULL
		  AND c.id IS NULL
		ORDER BY s.updated_at, s.id
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list approved submissions without cover: %w", err)
	}
	defer rows.Close()
	result := make([]ApprovedSubmissionForPromotion, 0)
	for rows.Next() {
		var item ApprovedSubmissionForPromotion
		if err = rows.Scan(&item.ID, &item.BookID, &item.LibraryID, &item.ActorUserID,
			&item.SourceObjectKey, &item.SourceContentType, &item.SourceFormat,
			&item.SourceWidth, &item.SourceHeight, &item.SourceSize); err != nil {
			return nil, fmt.Errorf("scan approved submission without cover: %w", err)
		}
		result = append(result, item)
	}
	return result, rows.Err()
}


func (r *BookSubmissionRepository) List(ctx context.Context, libraryID string, limit int) ([]models.BookSubmission, error) {
	if limit < 1 || limit > 100 { return nil, ErrInvalidBookSubmission }
	query := `SELECT id, library_id, title, auteur, editeur, price, volume, status, tags, categorie, cover_url, source_object_key, source_content_type, source_format, source_width, source_height, source_size, moderation_status, moderation_score, COALESCE(moderation_model_version,''), COALESCE(decision_code,''), created_book_id, expires_at, created_at, updated_at FROM book_submissions`
	args := []interface{}{}
	if libraryID != "" { query += " WHERE library_id=?"; args = append(args, libraryID) }
	query += " ORDER BY created_at DESC LIMIT ?"; args = append(args, limit)
	rows, err := r.db.QueryContext(ctx, query, args...); if err != nil { return nil, fmt.Errorf("list book submissions: %w", err) }; defer rows.Close()
	items := []models.BookSubmission{}
	for rows.Next() { var item models.BookSubmission; if err=rows.Scan(&item.ID,&item.LibraryID,&item.Title,&item.Auteur,&item.Editeur,&item.Price,&item.Volume,&item.Status,&item.Tags,&item.Categorie,&item.CoverURL,&item.SourceObjectKey,&item.SourceContentType,&item.SourceFormat,&item.SourceWidth,&item.SourceHeight,&item.SourceSize,&item.ModerationStatus,&item.ModerationScore,&item.ModerationModelVersion,&item.DecisionCode,&item.CreatedBookID,&item.ExpiresAt,&item.CreatedAt,&item.UpdatedAt); err != nil { return nil, fmt.Errorf("scan book submission: %w",err) }; items=append(items,item) }
	return items, rows.Err()
}
