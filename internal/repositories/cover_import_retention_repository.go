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

// PurgeMetadata removes at most 100 old terminal jobs per pass. The source
// deletion must already be acknowledged; a failed object deletion retains its
// metadata for retry and investigation. The batch row is removed only after
// all of its jobs have been purged.
func (r *CoverImportRetentionRepository) PurgeMetadata(ctx context.Context, now time.Time) (int, error) {
	cutoff := now.UTC().AddDate(-2, 0, 0).Format(time.RFC3339Nano)
	nowText := now.UTC().Format(time.RFC3339Nano)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin import metadata purge: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT j.id FROM cover_import_jobs j
		JOIN cover_object_cleanup_jobs c ON c.cover_id='import:' || j.id AND c.object_key=j.source_object_key
		WHERE j.created_at <= ? AND j.status IN ('READY','REJECTED','FAILED','CANCELLED')
		AND c.completed_at IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM cover_import_legal_holds h WHERE h.job_id=j.id AND h.expires_at > ?)
		ORDER BY j.created_at,j.id LIMIT 100`, cutoff, nowText)
	if err != nil {
		return 0, fmt.Errorf("select expired import metadata: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	if closeErr := rows.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, fmt.Errorf("read expired import metadata: %w", err)
	}
	for _, id := range ids {
		for _, query := range []string{
			`DELETE FROM cover_import_candidate_matches WHERE job_id=?`,
			`DELETE FROM cover_import_review_decisions WHERE job_id=?`,
			`DELETE FROM cover_import_ocr_results WHERE job_id=?`,
			`DELETE FROM cover_import_outbox WHERE job_id=?`,
			`DELETE FROM cover_import_legal_holds WHERE job_id=?`,
			`DELETE FROM audit_logs WHERE resource_type='COVER_IMPORT_JOB' AND resource_id=? AND created_at <= ?`,
			`DELETE FROM cover_import_jobs WHERE id=?`,
			`DELETE FROM cover_object_cleanup_jobs WHERE cover_id='import:' || ? AND completed_at IS NOT NULL`,
		} {
			arguments := []any{id}
			if query == `DELETE FROM audit_logs WHERE resource_type='COVER_IMPORT_JOB' AND resource_id=? AND created_at <= ?` {
				arguments = append(arguments, cutoff)
			}
			if _, err = tx.ExecContext(ctx, query, arguments...); err != nil {
				return 0, fmt.Errorf("purge import job %s: %w", id, err)
			}
		}
	}
	// Keep only the aggregate audit of the purge; the per-object queue row
	// is also metadata and expires with the import job.
	if _, err = tx.ExecContext(ctx, `DELETE FROM audit_logs WHERE resource_type='COVER_IMPORT' AND created_at <= ?
		AND resource_id IN (SELECT id FROM cover_imports WHERE created_at <= ?
		AND NOT EXISTS (SELECT 1 FROM cover_import_jobs WHERE import_id=cover_imports.id))`, cutoff, cutoff); err != nil {
		return 0, fmt.Errorf("purge import audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM audit_logs WHERE created_at <= ? AND
		(resource_type='COVER_IMPORT_RETENTION' OR
		 (resource_type='COVER_IMPORT_JOB' AND NOT EXISTS
		 (SELECT 1 FROM cover_import_jobs j WHERE j.id=audit_logs.resource_id)))`, cutoff); err != nil {
		return 0, fmt.Errorf("purge expired retention audit: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM cover_imports WHERE created_at <= ?
		AND NOT EXISTS (SELECT 1 FROM cover_import_jobs WHERE import_id=cover_imports.id)`, cutoff); err != nil {
		return 0, fmt.Errorf("purge empty import batches: %w", err)
	}
	if len(ids) > 0 {
		if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,action,resource_type,new_values,success,created_at)
			VALUES(lower(hex(randomblob(16))),'PURGE_COVER_IMPORT_METADATA','COVER_IMPORT_RETENTION',json_object('purgedJobs',?),1,?)`, len(ids), nowText); err != nil {
			return 0, fmt.Errorf("audit import metadata purge: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit import metadata purge: %w", err)
	}
	return len(ids), nil
}
