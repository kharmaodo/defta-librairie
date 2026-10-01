package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type CoverImportMatchingJob struct {
	ID, LibraryID, ActorUserID, Text string
}

type CoverImportCandidate struct {
	BookID int64
	Score  float64
}

func (r *CoverImportRepository) MatchingJob(ctx context.Context, jobID, libraryID string) (CoverImportMatchingJob, error) {
	if jobID == "" || libraryID == "" {
		return CoverImportMatchingJob{}, ErrInvalidCoverImport
	}
	var job CoverImportMatchingJob
	err := r.db.QueryRowContext(ctx, `SELECT j.id,j.library_id,j.actor_user_id,o.text_raw
		FROM cover_import_jobs j JOIN cover_import_ocr_results o ON o.job_id=j.id
		WHERE j.id=? AND j.library_id=? AND j.status='MATCHING' AND j.nsfw_decision='SAFE'`, jobID, libraryID).
		Scan(&job.ID, &job.LibraryID, &job.ActorUserID, &job.Text)
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrCoverImportJobNotFound
	}
	if err != nil {
		return job, fmt.Errorf("read matching job: %w", err)
	}
	return job, nil
}

// SearchCoverImportCandidates uses the existing catalogue FTS5 index. The
// library condition is mandatory even for root-owned imports.
func (r *CoverImportRepository) SearchCoverImportCandidates(ctx context.Context, libraryID, query string) ([]CoverImportCandidate, error) {
	if libraryID == "" {
		return nil, ErrInvalidCoverImport
	}
	if query == "" {
		return []CoverImportCandidate{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `SELECT d.id,bm25(defta_fts,5.0,1.0,2.0,0.5,0.5) AS score
		FROM defta_fts JOIN defta d ON d.id=defta_fts.rowid
		WHERE defta_fts MATCH ? AND d.library_id=? AND d.deleted_at IS NULL
		ORDER BY score ASC,d.id ASC LIMIT 5`, query, libraryID)
	if err != nil {
		return nil, fmt.Errorf("search cover import candidates: %w", err)
	}
	defer rows.Close()
	candidates := make([]CoverImportCandidate, 0, 5)
	for rows.Next() {
		var candidate CoverImportCandidate
		if err = rows.Scan(&candidate.BookID, &candidate.Score); err != nil {
			return nil, fmt.Errorf("scan cover import candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

// CompleteMatching commits candidates, the state transition and the audit as
// one unit. Re-delivery cannot add a second set of candidates or audit entry.
func (r *CoverImportRepository) CompleteMatching(ctx context.Context, job CoverImportMatchingJob, candidates []CoverImportCandidate, auditID, now string) error {
	return r.completeMatching(ctx, job, candidates, auditID, now, "fts5_bm25", "v1")
}

func (r *CoverImportRepository) CompleteQualityMatching(ctx context.Context, job CoverImportMatchingJob, candidates []CoverImportCandidate, auditID, now string) error {
	return r.completeMatching(ctx, job, candidates, auditID, now, "scoped_fts5_bm25", "v2")
}

func (r *CoverImportRepository) completeMatching(ctx context.Context, job CoverImportMatchingJob, candidates []CoverImportCandidate, auditID, now, algorithm, policy string) error {
	if job.ID == "" || job.LibraryID == "" || job.ActorUserID == "" || auditID == "" || len(candidates) > 5 {
		return ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin matching: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='REVIEW_REQUIRED',updated_at=? WHERE id=? AND library_id=? AND status='MATCHING' AND nsfw_decision='SAFE'`, now, job.ID, job.LibraryID)
	if err != nil {
		return fmt.Errorf("update matching status: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCoverImportJobState
	}
	for rank, candidate := range candidates {
		if candidate.BookID <= 0 {
			return ErrInvalidCoverImport
		}
		// Re-check scope inside the transaction, including deleted books.
		var bookID int64
		err = tx.QueryRowContext(ctx, `SELECT id FROM defta WHERE id=? AND library_id=? AND deleted_at IS NULL`, candidate.BookID, job.LibraryID).Scan(&bookID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidCoverImport
		}
		if err != nil {
			return err
		}
		explanation, _ := json.Marshal(map[string]string{"algorithm": algorithm, "policyVersion": policy})
		_, err = tx.ExecContext(ctx, `INSERT INTO cover_import_candidate_matches(job_id,book_id,rank,fts_score,explanation_json,created_at) VALUES(?,?,?,?,?,?)`, job.ID, bookID, rank+1, candidate.Score, string(explanation), now)
		if err != nil {
			return fmt.Errorf("persist matching candidate: %w", err)
		}
	}
	values, _ := json.Marshal(map[string]any{"algorithm": algorithm, "policyVersion": policy, "candidateCount": len(candidates), "libraryId": job.LibraryID, "correlationId": job.ID})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?, 'COMPLETE_COVER_IMPORT_MATCHING','COVER_IMPORT_JOB',?,?,1,?)`, auditID, job.ActorUserID, job.ID, string(values), now)
	if err != nil {
		return fmt.Errorf("audit matching: %w", err)
	}
	return tx.Commit()
}

func (r *CoverImportRepository) FailMatching(ctx context.Context, jobID, libraryID, auditID, now string) error {
	if jobID == "" || libraryID == "" || auditID == "" {
		return ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var actor string
	err = tx.QueryRowContext(ctx, `SELECT actor_user_id FROM cover_import_jobs WHERE id=? AND library_id=? AND status='MATCHING'`, jobID, libraryID).Scan(&actor)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrCoverImportJobState
	}
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status='FAILED',failure_code='MATCHING_FAILED',updated_at=?,terminal_at=? WHERE id=? AND library_id=? AND status='MATCHING'`, now, now, jobID, libraryID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCoverImportJobState
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'FAIL_COVER_IMPORT_MATCHING','COVER_IMPORT_JOB',?,'{"failureCode":"MATCHING_FAILED"}',0,?)`, auditID, actor, jobID, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *CoverImportRepository) PendingMatchOutbox(ctx context.Context, now string, limit int) ([]PendingCoverImportOutboxEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidCoverImport
	}
	rows, err := r.db.QueryContext(ctx, `SELECT event_id,payload FROM cover_import_outbox WHERE event_type='cover.imports.match.v1' AND published_at IS NULL AND available_at<=? ORDER BY created_at,event_id LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]PendingCoverImportOutboxEvent, 0)
	for rows.Next() {
		var event PendingCoverImportOutboxEvent
		if err = rows.Scan(&event.EventID, &event.Payload); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *CoverImportRepository) MarkMatchOutboxPublished(ctx context.Context, eventID, now string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE cover_import_outbox SET published_at=?,last_error=NULL WHERE event_id=? AND event_type='cover.imports.match.v1' AND published_at IS NULL`, now, eventID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrCoverImportJobNotFound
	}
	return nil
}

func (r *CoverImportRepository) RecordMatchOutboxFailure(ctx context.Context, eventID, message, availableAt string) error {
	if message == "" {
		message = "publish_failed"
	}
	_, err := r.db.ExecContext(ctx, `UPDATE cover_import_outbox SET attempts=attempts+1,last_error=?,available_at=? WHERE event_id=? AND event_type='cover.imports.match.v1' AND published_at IS NULL`, message, availableAt, eventID)
	return err
}
