package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCoverProcessingNotFound = errors.New("cover processing target not found")
	ErrCoverProcessingBusy     = errors.New("cover processing lease is active")
	ErrCoverProcessingTerminal = errors.New("cover processing is terminal")
)

type CoverProcessingClaim int

const (
	CoverProcessingClaimed CoverProcessingClaim = iota + 1
	CoverProcessingAlreadyReady
)

type CoverProcessingRepository struct {
	db *sql.DB
}

func NewCoverProcessingRepository(db *sql.DB) *CoverProcessingRepository {
	return &CoverProcessingRepository{db: db}
}

func (r *CoverProcessingRepository) Claim(
	ctx context.Context,
	coverID string,
	bookID int,
	libraryID string,
	sourceObjectKey string,
	workerID string,
	now time.Time,
	leaseUntil time.Time,
) (CoverProcessingClaim, error) {
	if coverID == "" || bookID < 1 || libraryID == "" ||
		sourceObjectKey == "" || workerID == "" || !leaseUntil.After(now) {
		return 0, fmt.Errorf("invalid cover processing claim")
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	leaseText := leaseUntil.UTC().Format(time.RFC3339Nano)
	var claimedID string
	err := r.db.QueryRowContext(ctx, `
		UPDATE book_covers
		SET status = 'PROCESSING',
		    processing_by = ?,
		    processing_until = ?,
		    processing_attempts = processing_attempts + 1,
		    error_code = NULL,
		    updated_at = ?
		WHERE id = ?
		  AND book_id = ?
		  AND library_id = ?
		  AND source_object_key = ?
		  AND (
		    status = 'PENDING'
		    OR (
		      status = 'PROCESSING'
		      AND processing_until IS NOT NULL
		      AND processing_until <= ?
		    )
		  )
		RETURNING id
	`,
		workerID,
		leaseText,
		nowText,
		coverID,
		bookID,
		libraryID,
		sourceObjectKey,
		nowText,
	).Scan(&claimedID)
	if err == nil {
		return CoverProcessingClaimed, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("claim cover processing: %w", err)
	}

	var status, storedSource string
	err = r.db.QueryRowContext(ctx, `
		SELECT status, source_object_key
		FROM book_covers
		WHERE id = ? AND book_id = ? AND library_id = ?
	`, coverID, bookID, libraryID).Scan(&status, &storedSource)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrCoverProcessingNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("inspect cover processing claim: %w", err)
	}
	if storedSource != sourceObjectKey {
		return 0, ErrCoverProcessingNotFound
	}
	switch status {
	case "READY":
		return CoverProcessingAlreadyReady, nil
	case "PROCESSING":
		return 0, ErrCoverProcessingBusy
	case "FAILED":
		return 0, ErrCoverProcessingTerminal
	default:
		return 0, fmt.Errorf("claim cover processing in status %q", status)
	}
}

func (r *CoverProcessingRepository) Release(
	ctx context.Context,
	coverID string,
	workerID string,
	errorCode string,
	now time.Time,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE book_covers
		SET status = 'PENDING',
		    processing_by = NULL,
		    processing_until = NULL,
		    error_code = ?,
		    updated_at = ?
		WHERE id = ?
		  AND status = 'PROCESSING'
		  AND processing_by = ?
	`, errorCode, now.UTC().Format(time.RFC3339Nano), coverID, workerID)
	if err != nil {
		return fmt.Errorf("release cover processing: %w", err)
	}
	return requireCoverProcessingLease(result)
}

func (r *CoverProcessingRepository) MarkFailed(
	ctx context.Context,
	coverID string,
	errorCode string,
	now time.Time,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE book_covers
		SET status = 'FAILED',
		    processing_by = NULL,
		    processing_until = NULL,
		    error_code = ?,
		    updated_at = ?
		WHERE id = ? AND status IN ('PENDING', 'PROCESSING')
	`, errorCode, now.UTC().Format(time.RFC3339Nano), coverID)
	if err != nil {
		return fmt.Errorf("mark cover processing failed: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect failed cover processing: %w", err)
	}
	if affected == 0 {
		return ErrCoverProcessingTerminal
	}
	return nil
}

type ProcessedCover struct {
	MasterObjectKey    string
	LargeJPEGObjectKey string
	LargeWebPObjectKey string
	ThumbJPEGObjectKey string
	ThumbWebPObjectKey string
}

func (r *CoverProcessingRepository) Complete(
	ctx context.Context,
	coverID string,
	libraryID string,
	workerID string,
	processed ProcessedCover,
	now time.Time,
) error {
	if coverID == "" || libraryID == "" || workerID == "" ||
		processed.MasterObjectKey == "" ||
		processed.LargeJPEGObjectKey == "" ||
		processed.LargeWebPObjectKey == "" ||
		processed.ThumbJPEGObjectKey == "" ||
		processed.ThumbWebPObjectKey == "" {
		return fmt.Errorf("invalid processed cover")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cover completion: %w", err)
	}
	defer tx.Rollback()

	var bookID int
	err = tx.QueryRowContext(ctx, `
		SELECT book_id
		FROM book_covers
		WHERE id = ? AND library_id = ?
		  AND status = 'PROCESSING' AND processing_by = ?
	`, coverID, libraryID, workerID).Scan(&bookID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCoverProcessingBusy
	}
	if err != nil {
		return fmt.Errorf("inspect cover completion: %w", err)
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO cover_object_cleanup_jobs(
			cover_id, library_id, object_key, object_kind,
			available_at, created_at
		)
		SELECT id, library_id, source_object_key, 'SOURCE', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		UNION ALL
		SELECT id, library_id, master_object_key, 'GENERATED', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		  AND master_object_key IS NOT NULL
		UNION ALL
		SELECT id, library_id, large_jpeg_object_key, 'GENERATED', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		  AND large_jpeg_object_key IS NOT NULL
		UNION ALL
		SELECT id, library_id, large_webp_object_key, 'GENERATED', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		  AND large_webp_object_key IS NOT NULL
		UNION ALL
		SELECT id, library_id, thumb_jpeg_object_key, 'GENERATED', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		  AND thumb_jpeg_object_key IS NOT NULL
		UNION ALL
		SELECT id, library_id, thumb_webp_object_key, 'GENERATED', ?, ?
		FROM book_covers
		WHERE book_id = ? AND active = 1 AND id <> ?
		  AND thumb_webp_object_key IS NOT NULL
	`,
		nowText, nowText, bookID, coverID,
		nowText, nowText, bookID, coverID,
		nowText, nowText, bookID, coverID,
		nowText, nowText, bookID, coverID,
		nowText, nowText, bookID, coverID,
		nowText, nowText, bookID, coverID,
	); err != nil {
		return fmt.Errorf("queue previous cover cleanup: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE book_covers
		SET active = 0, updated_at = ?
		WHERE book_id = ? AND active = 1 AND id <> ?
	`, nowText, bookID, coverID); err != nil {
		return fmt.Errorf("deactivate previous cover: %w", err)
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE book_covers
		SET status = 'READY',
		    master_object_key = ?,
		    large_jpeg_object_key = ?,
		    large_webp_object_key = ?,
		    thumb_jpeg_object_key = ?,
		    thumb_webp_object_key = ?,
		    active = 1,
		    error_code = NULL,
		    processing_by = NULL,
		    processing_until = NULL,
		    updated_at = ?
		WHERE id = ? AND library_id = ?
		  AND status = 'PROCESSING' AND processing_by = ?
	`,
		processed.MasterObjectKey,
		processed.LargeJPEGObjectKey,
		processed.LargeWebPObjectKey,
		processed.ThumbJPEGObjectKey,
		processed.ThumbWebPObjectKey,
		nowText,
		coverID,
		libraryID,
		workerID,
	)
	if err != nil {
		return fmt.Errorf("complete cover processing: %w", err)
	}
	if err = requireCoverProcessingLease(result); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit cover completion: %w", err)
	}
	return nil
}

func requireCoverProcessingLease(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect cover processing update: %w", err)
	}
	if affected != 1 {
		return ErrCoverProcessingBusy
	}
	return nil
}
