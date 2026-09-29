package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// CoverImportRetentionRepository schedules deletion through the existing retryable
// object cleanup queue. Timestamps use the same UTC RFC3339Nano representation as
// the import and cleanup repositories.
type CoverImportRetentionRepository struct{ db *sql.DB }

func NewCoverImportRetentionRepository(db *sql.DB) *CoverImportRetentionRepository {
	return &CoverImportRetentionRepository{db: db}
}

func (r *CoverImportRetentionRepository) Reconcile(ctx context.Context, now time.Time) (int64, error) {
	nowText := now.UTC().Format(time.RFC3339Nano)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import retention: %w", err)
	}
	defer tx.Rollback()
	// A delayed worker or reviewer must never resume an expired source.
	if _, err = tx.ExecContext(ctx, `UPDATE cover_import_jobs
		SET status='CANCELLED', failure_code='SOURCE_EXPIRED', updated_at=?, terminal_at=?
		WHERE status IN ('PENDING_SCAN','NSFW_SCANNING','NSFW_DECIDED','OCR_PENDING','OCR_PROCESSING','MATCHING','REVIEW_REQUIRED','QUARANTINED')
		AND expires_at <= ?`, nowText, nowText, nowText); err != nil {
		return 0, fmt.Errorf("expire import jobs: %w", err)
	}
	result, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO cover_object_cleanup_jobs
		(cover_id, library_id, object_key, object_kind, available_at, created_at)
		SELECT 'import:' || j.id, j.library_id, j.source_object_key, 'SOURCE', ?, ?
		FROM cover_import_jobs j
		WHERE j.source_object_key <> ''
		AND NOT EXISTS (SELECT 1 FROM cover_import_legal_holds h WHERE h.job_id=j.id AND h.expires_at > ?)
		AND ((j.status='FAILED' AND j.terminal_at <= ?)
		OR (j.status='CANCELLED' AND j.failure_code='SOURCE_EXPIRED' AND j.expires_at <= ?)
		OR (j.status='REJECTED' AND j.nsfw_decision IN ('REVIEW','UNSAFE') AND j.terminal_at <= ?)
		OR (j.status IN ('REJECTED','CANCELLED') AND (j.nsfw_decision IS NULL OR j.nsfw_decision='SAFE') AND j.terminal_at <= ?)
		OR (j.status='QUARANTINED' AND j.expires_at <= ?)
		OR (j.status='REVIEW_REQUIRED' AND j.expires_at <= ?)
		OR (j.status='READY' AND j.terminal_at <= ?))`,
		nowText, nowText, nowText,
		now.AddDate(0, 0, -7).UTC().Format(time.RFC3339Nano), nowText,
		now.AddDate(0, 0, -14).UTC().Format(time.RFC3339Nano),
		now.AddDate(0, 0, -30).UTC().Format(time.RFC3339Nano),
		nowText, nowText,
		now.AddDate(0, 0, -90).UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, fmt.Errorf("enqueue import source cleanup: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count import cleanup jobs: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import retention: %w", err)
	}
	return count, nil
}
