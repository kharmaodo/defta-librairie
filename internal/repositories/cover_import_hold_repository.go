package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"defta-librairie/internal/identity"
)

var ErrCoverImportHoldConflict = errors.New("import source cleanup is already running or completed")
var ErrCoverImportHoldNotFound = errors.New("import or legal hold not found")
var ErrCoverImportHoldInvalid = errors.New("invalid legal hold")

func (r *CoverImportRetentionRepository) SetLegalHold(ctx context.Context, jobID, actorID, reason string, expiresAt, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if jobID == "" || actorID == "" || reason == "" || len([]rune(reason)) > 500 || !expiresAt.After(now) {
		return ErrCoverImportHoldInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legal hold: %w", err)
	}
	defer tx.Rollback()
	var exists int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cover_import_jobs WHERE id=?`, jobID).Scan(&exists); err != nil {
		return fmt.Errorf("find import for legal hold: %w", err)
	}
	if exists == 0 {
		return ErrCoverImportHoldNotFound
	}
	var blocked int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM cover_object_cleanup_jobs
		WHERE cover_id='import:' || ? AND (completed_at IS NOT NULL OR locked_by IS NOT NULL)`, jobID).Scan(&blocked); err != nil {
		return fmt.Errorf("check source cleanup: %w", err)
	}
	if blocked > 0 {
		return ErrCoverImportHoldConflict
	}
	auditID, err := identity.NewID()
	if err != nil {
		return fmt.Errorf("create legal hold audit id: %w", err)
	}
	nowText := now.UTC().Format(time.RFC3339Nano)
	expiresText := expiresAt.UTC().Format(time.RFC3339Nano)
	if _, err = tx.ExecContext(ctx, `INSERT INTO cover_import_legal_holds(job_id,reason,actor_user_id,expires_at,created_at)
		VALUES(?,?,?,?,?) ON CONFLICT(job_id) DO UPDATE SET reason=excluded.reason,actor_user_id=excluded.actor_user_id,
		expires_at=excluded.expires_at,created_at=excluded.created_at`, jobID, reason, actorID, expiresText, nowText); err != nil {
		return fmt.Errorf("persist legal hold: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES(?,?,'SET_COVER_IMPORT_LEGAL_HOLD','COVER_IMPORT_JOB',?,json_object('expiresAt',?),1,?)`, auditID, actorID, jobID, expiresText, nowText); err != nil {
		return fmt.Errorf("audit legal hold: %w", err)
	}
	return tx.Commit()
}

func (r *CoverImportRetentionRepository) ReleaseLegalHold(ctx context.Context, jobID, actorID string, now time.Time) error {
	if jobID == "" || actorID == "" {
		return ErrCoverImportHoldInvalid
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legal hold release: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM cover_import_legal_holds WHERE job_id=?`, jobID)
	if err != nil {
		return fmt.Errorf("release legal hold: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCoverImportHoldNotFound
	}
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at)
		VALUES(?,?,'RELEASE_COVER_IMPORT_LEGAL_HOLD','COVER_IMPORT_JOB',?,'{}',1,?)`, auditID, actorID, jobID, now.UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("audit legal hold release: %w", err)
	}
	return tx.Commit()
}
