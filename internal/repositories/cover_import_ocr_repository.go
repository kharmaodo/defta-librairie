package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type CoverImportOCRJob struct {
	ID, LibraryID, ActorUserID, SourceObjectKey, SourceContentType string
	SourceSize                                                   int64
}

func (r *CoverImportRepository) PendingOCROutbox(ctx context.Context, now string, limit int) ([]PendingCoverImportOutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidCoverImport
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_id,payload FROM cover_import_outbox WHERE event_type='cover.imports.ocr.v1' AND published_at IS NULL AND available_at<=? ORDER BY created_at,event_id LIMIT ?`, now, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	events := []PendingCoverImportOutboxEvent{}
	for rows.Next() { var event PendingCoverImportOutboxEvent; if err=rows.Scan(&event.EventID,&event.Payload);err!=nil{return nil,err};events=append(events,event) }
	return events, rows.Err()
}

func (r *CoverImportRepository) MarkOCROutboxPublished(ctx context.Context,eventID,now string) error { result,err:=r.db.ExecContext(ctx,`UPDATE cover_import_outbox SET published_at=?,last_error=NULL WHERE event_id=? AND event_type='cover.imports.ocr.v1' AND published_at IS NULL`,now,eventID);if err!=nil{return err};rows,_:=result.RowsAffected();if rows!=1{return ErrCoverImportJobNotFound};return nil }

func (r *CoverImportRepository) RecordOCROutboxFailure(ctx context.Context,eventID,message,now string) error { _,err:=r.db.ExecContext(ctx,`UPDATE cover_import_outbox SET attempts=attempts+1,last_error=?,available_at=? WHERE event_id=? AND event_type='cover.imports.ocr.v1' AND published_at IS NULL`,message,now,eventID);return err }

func (r *CoverImportRepository) ClaimForOCR(ctx context.Context, jobID, now string) (CoverImportOCRJob, error) {
	var job CoverImportOCRJob
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil { return job, fmt.Errorf("begin OCR claim: %w", err) }
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT id,library_id,actor_user_id,source_object_key,source_content_type,source_size FROM cover_import_jobs WHERE id=? AND status='OCR_PENDING'`, jobID).Scan(&job.ID,&job.LibraryID,&job.ActorUserID,&job.SourceObjectKey,&job.SourceContentType,&job.SourceSize)
	if errors.Is(err, sql.ErrNoRows) { return job, ErrCoverImportJobNotFound }
	if err != nil { return job, fmt.Errorf("read OCR job: %w", err) }
	result, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='OCR_PROCESSING',updated_at=? WHERE id=? AND status='OCR_PENDING'`, now, jobID)
	if err != nil { return job, fmt.Errorf("claim OCR job: %w", err) }
	rows, _ := result.RowsAffected(); if rows != 1 { return job, ErrCoverImportJobState }
	if err = tx.Commit(); err != nil { return job, fmt.Errorf("commit OCR claim: %w", err) }
	return job, nil
}

func (r *CoverImportRepository) CompleteOCR(ctx context.Context, job CoverImportOCRJob, engine, version, language, raw, normalized, eventID, auditID, now string) error {
	if job.ID=="" || engine=="" || version=="" || language!="ara" || eventID=="" || auditID=="" { return ErrInvalidCoverImport }
	tx, err := r.db.BeginTx(ctx,nil); if err != nil { return fmt.Errorf("begin OCR completion: %w",err) }; defer tx.Rollback()
	result,err:=tx.ExecContext(ctx,`UPDATE cover_import_jobs SET status='MATCHING',updated_at=? WHERE id=? AND status='OCR_PROCESSING'`,now,job.ID); if err!=nil{return err}; rows,_:=result.RowsAffected();if rows!=1{return ErrCoverImportJobState}
	if _,err=tx.ExecContext(ctx,`INSERT INTO cover_import_ocr_results(job_id,engine,engine_version,language,text_raw,text_normalized,completed_at) VALUES(?,?,?,?,?,?,?)`,job.ID,engine,version,language,raw,normalized,now);err!=nil{return fmt.Errorf("persist OCR result: %w",err)}
	payload:=fmt.Sprintf(`{"schemaVersion":1,"eventId":%q,"jobId":%q,"libraryId":%q,"attempt":1}`,eventID,job.ID,job.LibraryID)
	if _,err=tx.ExecContext(ctx,`INSERT INTO cover_import_outbox(event_id,job_id,library_id,event_type,schema_version,payload,attempts,available_at,created_at) VALUES(?,?,?,'cover.imports.match.v1',1,?,0,?,?)`,eventID,job.ID,job.LibraryID,payload,now,now);err!=nil{return fmt.Errorf("queue matching: %w",err)}
	if _,err=tx.ExecContext(ctx,`INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?, 'COMPLETE_COVER_IMPORT_OCR','COVER_IMPORT_JOB',?,?,1,?)`,auditID,job.ActorUserID,job.ID,fmt.Sprintf(`{"engine":%q,"language":%q}`,engine,language),now);err!=nil{return fmt.Errorf("audit OCR: %w",err)}
	return tx.Commit()
}

func (r *CoverImportRepository) FailOCR(ctx context.Context, jobID, code, auditID, now string) error {
	tx,err:=r.db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback();var actor string
	err=tx.QueryRowContext(ctx,`SELECT actor_user_id FROM cover_import_jobs WHERE id=? AND status='OCR_PROCESSING'`,jobID).Scan(&actor);if errors.Is(err,sql.ErrNoRows){return ErrCoverImportJobState};if err!=nil{return err}
	result,err:=tx.ExecContext(ctx,`UPDATE cover_import_jobs SET status='FAILED',failure_code=?,updated_at=?,terminal_at=? WHERE id=? AND status='OCR_PROCESSING'`,code,now,now,jobID);if err!=nil{return err};rows,_:=result.RowsAffected();if rows!=1{return ErrCoverImportJobState}
	_,err=tx.ExecContext(ctx,`INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?, 'COMPLETE_COVER_IMPORT_OCR','COVER_IMPORT_JOB',?,?,0,?)`,auditID,actor,jobID,fmt.Sprintf(`{"failureCode":%q}`,code),now);if err!=nil{return err};return tx.Commit()
}
