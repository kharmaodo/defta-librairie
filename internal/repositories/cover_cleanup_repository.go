package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCoverCleanupEmpty     = errors.New("cover cleanup queue is empty")
	ErrCoverCleanupLeaseLost = errors.New("cover cleanup lease was lost")
)

type CoverCleanupJob struct {
	ID        int64
	CoverID   string
	LibraryID string
	ObjectKey string
	ObjectKind string
	Attempts  int
}

type CoverCleanupRepository struct {
	db *sql.DB
}

func NewCoverCleanupRepository(db *sql.DB) *CoverCleanupRepository {
	return &CoverCleanupRepository{db: db}
}

func (r *CoverCleanupRepository) ClaimNext(
	ctx context.Context,
	workerID string,
	now time.Time,
	leaseUntil time.Time,
) (CoverCleanupJob, error) {
	if workerID == "" || !leaseUntil.After(now) {
		return CoverCleanupJob{}, fmt.Errorf("invalid cover cleanup lease")
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	leaseText := leaseUntil.UTC().Format(time.RFC3339Nano)
	var job CoverCleanupJob
	err := r.db.QueryRowContext(ctx, `
		UPDATE cover_object_cleanup_jobs
		SET locked_by = ?, locked_until = ?
		WHERE id = (
			SELECT id FROM cover_object_cleanup_jobs
			WHERE completed_at IS NULL
			  AND available_at <= ?
			  AND (locked_until IS NULL OR locked_until <= ?)
			ORDER BY available_at, id
			LIMIT 1
		)
		RETURNING id, cover_id, library_id, object_key, object_kind, attempts
	`, workerID, leaseText, nowText, nowText).Scan(
		&job.ID,
		&job.CoverID,
		&job.LibraryID,
		&job.ObjectKey,
		&job.ObjectKind,
		&job.Attempts,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return CoverCleanupJob{}, ErrCoverCleanupEmpty
	}
	if err != nil {
		return CoverCleanupJob{}, fmt.Errorf("claim cover cleanup job: %w", err)
	}
	return job, nil
}

func (r *CoverCleanupRepository) MarkCompleted(
	ctx context.Context,
	jobID int64,
	workerID string,
	completedAt time.Time,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cover_object_cleanup_jobs
		SET completed_at = ?, last_error = NULL,
		    locked_by = NULL, locked_until = NULL
		WHERE id = ? AND locked_by = ? AND completed_at IS NULL
	`, completedAt.UTC().Format(time.RFC3339Nano), jobID, workerID)
	if err != nil {
		return fmt.Errorf("complete cover cleanup job: %w", err)
	}
	return requireCoverCleanupLease(result)
}

func (r *CoverCleanupRepository) MarkFailed(
	ctx context.Context,
	jobID int64,
	workerID string,
	availableAt time.Time,
	lastError string,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE cover_object_cleanup_jobs
		SET attempts = attempts + 1, available_at = ?, last_error = ?,
		    locked_by = NULL, locked_until = NULL
		WHERE id = ? AND locked_by = ? AND completed_at IS NULL
	`, availableAt.UTC().Format(time.RFC3339Nano), lastError, jobID, workerID)
	if err != nil {
		return fmt.Errorf("reschedule cover cleanup job: %w", err)
	}
	return requireCoverCleanupLease(result)
}

func requireCoverCleanupLease(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect cover cleanup update: %w", err)
	}
	if affected != 1 {
		return ErrCoverCleanupLeaseLost
	}
	return nil
}
