package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/ocr"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrCoverImportOCRLeaseHeld = errors.New("OCR claim is still active")

type ExperimentalOCRJob struct {
	CoverImportOCRJob
	Token    string
	Attempts int
}

func (r *CoverImportRepository) ClaimExperimentalOCR(ctx context.Context, jobID, token, now, until string) (ExperimentalOCRJob, error) {
	var job ExperimentalOCRJob
	if jobID == "" || token == "" || now == "" || until == "" {
		return job, ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return job, err
	}
	defer tx.Rollback()
	var status string
	var currentToken sql.NullString
	var active bool
	err = tx.QueryRowContext(ctx, `SELECT id,library_id,actor_user_id,source_object_key,source_content_type,source_size,status,ocr_claim_token,ocr_attempts,COALESCE(julianday(ocr_claim_until)>julianday(?),0) FROM cover_import_jobs WHERE id=?`, now, jobID).Scan(&job.ID, &job.LibraryID, &job.ActorUserID, &job.SourceObjectKey, &job.SourceContentType, &job.SourceSize, &status, &currentToken, &job.Attempts, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrCoverImportJobNotFound
	}
	if err != nil {
		return job, err
	}
	if status != "OCR_PENDING" && status != "OCR_PROCESSING" {
		return job, ErrCoverImportJobNotFound
	}
	if status == "OCR_PROCESSING" && (active || !currentToken.Valid) {
		return job, ErrCoverImportOCRLeaseHeld
	}
	updated, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='OCR_PROCESSING',ocr_claim_token=?,ocr_claim_until=?,ocr_attempts=ocr_attempts+1,failure_code=NULL,updated_at=? WHERE id=? AND (status='OCR_PENDING' OR (status='OCR_PROCESSING' AND ocr_claim_token IS NOT NULL AND (ocr_claim_until IS NULL OR julianday(ocr_claim_until)<=julianday(?))))`, token, until, now, jobID, now)
	if err != nil {
		return job, err
	}
	count, _ := updated.RowsAffected()
	if count != 1 {
		return job, ErrCoverImportOCRLeaseHeld
	}
	job.Token = token
	job.Attempts++
	if err = tx.Commit(); err != nil {
		return job, err
	}
	return job, nil
}

func (r *CoverImportRepository) RetryExperimentalOCR(ctx context.Context, job ExperimentalOCRJob, code, now string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE cover_import_jobs SET status='OCR_PENDING',failure_code=?,ocr_claim_token=NULL,ocr_claim_until=NULL,updated_at=? WHERE id=? AND status='OCR_PROCESSING' AND ocr_claim_token=?`, code, now, job.ID, job.Token)
	if err != nil {
		return err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return ErrCoverImportJobState
	}
	return nil
}

func (r *CoverImportRepository) CompleteExperimentalOCR(ctx context.Context, job ExperimentalOCRJob, result ocr.Result, eventID, auditID, now string) error {
	if job.ID == "" || job.Token == "" || result.Engine != "tesseract-experimental" || result.Language != "ara" || eventID == "" || auditID == "" {
		return ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changed, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='MATCHING',ocr_claim_token=NULL,ocr_claim_until=NULL,failure_code=NULL,updated_at=? WHERE id=? AND library_id=? AND status='OCR_PROCESSING' AND ocr_claim_token=?`, now, job.ID, job.LibraryID, job.Token)
	if err != nil {
		return err
	}
	count, _ := changed.RowsAffected()
	if count != 1 {
		return ErrCoverImportJobState
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cover_import_ocr_results(job_id,engine,engine_version,language,text_raw,text_normalized,confidence,policy_version,psm,preprocessing,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, job.ID, result.Engine, result.EngineVersion, result.Language, result.TextRaw, result.TextNormalized, result.Confidence, result.PolicyVersion, result.PSM, result.Preprocessing, now)
	if err != nil {
		return fmt.Errorf("persist experimental OCR: %w", err)
	}
	payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "eventId": eventID, "jobId": job.ID, "libraryId": job.LibraryID, "attempt": 1})
	_, err = tx.ExecContext(ctx, `INSERT INTO cover_import_outbox(event_id,job_id,library_id,event_type,schema_version,payload,attempts,available_at,created_at) VALUES(?,?,?,'cover.imports.match.v1',1,?,0,?,?)`, eventID, job.ID, job.LibraryID, string(payload), now, now)
	if err != nil {
		return err
	}
	metadata, _ := json.Marshal(map[string]any{"engine": result.Engine, "engineVersion": result.EngineVersion, "policyVersion": result.PolicyVersion, "language": result.Language, "psm": result.PSM, "preprocessing": result.Preprocessing, "libraryId": job.LibraryID, "correlationId": job.ID})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'COMPLETE_COVER_IMPORT_OCR','COVER_IMPORT_JOB',?,?,1,?)`, auditID, job.ActorUserID, job.ID, string(metadata), now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *CoverImportRepository) FailExperimentalOCR(ctx context.Context, job ExperimentalOCRJob, code, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	changed, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='FAILED',failure_code=?,updated_at=?,terminal_at=?,ocr_claim_token=NULL,ocr_claim_until=NULL WHERE id=? AND status='OCR_PROCESSING' AND ocr_claim_token=?`, code, now, now, job.ID, job.Token)
	if err != nil {
		return err
	}
	count, _ := changed.RowsAffected()
	if count != 1 {
		return ErrCoverImportJobState
	}
	metadata, _ := json.Marshal(map[string]any{"failureCode": code, "libraryId": job.LibraryID, "correlationId": job.ID, "attempts": job.Attempts})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'COMPLETE_COVER_IMPORT_OCR','COVER_IMPORT_JOB',?,?,0,?)`, auditID, job.ActorUserID, job.ID, string(metadata), now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// A rollback recovers stale experimental claims without stealing active ones.
func (r *CoverImportRepository) RecoverExpiredExperimentalOCR(ctx context.Context, jobID, now string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE cover_import_jobs SET status='OCR_PENDING',ocr_claim_token=NULL,ocr_claim_until=NULL,updated_at=? WHERE id=? AND status='OCR_PROCESSING' AND ocr_claim_token IS NOT NULL AND (ocr_claim_until IS NULL OR julianday(ocr_claim_until)<=julianday(?))`, now, jobID, now)
	if err != nil {
		return err
	}
	var active int
	err = r.db.QueryRowContext(ctx, `SELECT 1 FROM cover_import_jobs WHERE id=? AND status='OCR_PROCESSING' AND ocr_claim_token IS NOT NULL`, jobID).Scan(&active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return ErrCoverImportOCRLeaseHeld
}
