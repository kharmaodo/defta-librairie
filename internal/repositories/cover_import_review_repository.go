package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrCoverImportReviewState = errors.New("cover import review is not actionable")
var ErrCoverImportCandidateNotFound = errors.New("cover import candidate not found")

type CoverImportReviewJob struct {
	ID, LibraryID, Status, NSFWDecision, NSFWPolicyVersion, SourceKey, ContentType, Format string
	Width, Height                                                                          int
	Size                                                                                   int64
	Candidates                                                                             []CoverImportReviewCandidate
}

type CoverImportReviewCandidate struct {
	BookID int     `json:"bookId"`
	Rank   int     `json:"rank"`
	Title  string  `json:"title"`
	Score  float64 `json:"ftsScore"`
}

func (r *CoverImportRepository) ReviewJob(ctx context.Context, jobID, libraryID string) (CoverImportReviewJob, error) {
	var job CoverImportReviewJob
	if jobID == "" || libraryID == "" {
		return job, ErrInvalidCoverImport
	}
	err := r.db.QueryRowContext(ctx, `SELECT id,library_id,status,COALESCE(nsfw_decision,''),COALESCE(nsfw_policy_version,''),source_object_key,source_content_type,source_format,source_width,source_height,source_size FROM cover_import_jobs WHERE id=? AND library_id=?`, jobID, libraryID).Scan(&job.ID, &job.LibraryID, &job.Status, &job.NSFWDecision, &job.NSFWPolicyVersion, &job.SourceKey, &job.ContentType, &job.Format, &job.Width, &job.Height, &job.Size)
	if errors.Is(err, sql.ErrNoRows) {
		return job, ErrCoverImportJobNotFound
	}
	if err != nil {
		return job, fmt.Errorf("read review job: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT c.book_id,c.rank,d.title,c.fts_score FROM cover_import_candidate_matches c JOIN defta d ON d.id=c.book_id AND d.library_id=? AND d.deleted_at IS NULL WHERE c.job_id=? ORDER BY c.rank`, libraryID, jobID)
	if err != nil {
		return job, err
	}
	defer rows.Close()
	job.Candidates = []CoverImportReviewCandidate{}
	for rows.Next() {
		var item CoverImportReviewCandidate
		if err = rows.Scan(&item.BookID, &item.Rank, &item.Title, &item.Score); err != nil {
			return job, err
		}
		job.Candidates = append(job.Candidates, item)
	}
	return job, rows.Err()
}

func (r *CoverImportRepository) DecideReview(ctx context.Context, job CoverImportReviewJob, actorID, actorRole, action, reason string, bookID int, cover PendingCover, eventID, auditID, now string) error {
	if job.ID == "" || job.LibraryID == "" || actorID == "" || auditID == "" || (action != "ACCEPT" && action != "REJECT") {
		return ErrInvalidCoverImport
	}
	if action == "ACCEPT" && (bookID < 1 || cover.ID == "" || eventID == "" || cover.BookID != bookID || cover.LibraryID != job.LibraryID) {
		return ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if action == "ACCEPT" {
		var count int
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cover_import_candidate_matches c JOIN defta d ON d.id=c.book_id AND d.library_id=? AND d.deleted_at IS NULL WHERE c.job_id=? AND c.book_id=?`, job.LibraryID, job.ID, bookID).Scan(&count)
		if err != nil {
			return err
		}
		if count != 1 {
			return ErrCoverImportCandidateNotFound
		}
	}
	status, code := "REJECTED", "OWNER_REJECTED"
	if action == "ACCEPT" {
		status, code = "READY", "OWNER_ACCEPTED"
	}
	result, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status=?,target_book_id=?,decision_code=?,terminal_at=?,updated_at=? WHERE id=? AND library_id=? AND status='REVIEW_REQUIRED' AND nsfw_decision='SAFE'`, status, reviewBookID(action, bookID), code, now, now, job.ID, job.LibraryID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCoverImportReviewState
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO cover_import_review_decisions(job_id,actor_user_id,action,book_id,reason,created_at) VALUES(?,?,?,?,?,?)`, job.ID, actorID, action, reviewBookID(action, bookID), reason, now)
	if err != nil {
		return err
	}
	if action == "ACCEPT" {
		_, err = tx.ExecContext(ctx, `INSERT INTO book_covers(id,book_id,library_id,status,source_object_key,source_content_type,source_format,source_width,source_height,source_size,active,created_at,updated_at) VALUES(?,?,?,'PENDING',?,?,?,?,?,?,0,?,?)`, cover.ID, bookID, job.LibraryID, cover.SourceObjectKey, cover.SourceContentType, cover.SourceFormat, cover.SourceWidth, cover.SourceHeight, cover.SourceSize, now, now)
		if err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "eventId": eventID, "coverId": cover.ID, "bookId": bookID, "libraryId": job.LibraryID, "sourceObjectKey": cover.SourceObjectKey, "attempt": 1})
		_, err = tx.ExecContext(ctx, `INSERT INTO cover_processing_outbox(event_id,cover_id,book_id,library_id,event_type,schema_version,payload,attempts,available_at,created_at) VALUES(?,?,?,?,'book.covers.process.v1',1,?,0,?,?)`, eventID, cover.ID, bookID, job.LibraryID, string(payload), now, now)
		if err != nil {
			return err
		}
	}
	values, _ := json.Marshal(map[string]any{"action": action, "bookId": reviewBookID(action, bookID), "libraryId": job.LibraryID, "actorRole": actorRole, "correlationId": auditID, "nsfwPolicyVersion": job.NSFWPolicyVersion})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'DECIDE_COVER_IMPORT_REVIEW','COVER_IMPORT_JOB',?,?,1,?)`, auditID, actorID, job.ID, string(values), now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func reviewBookID(action string, id int) any {
	if action == "ACCEPT" {
		return id
	}
	return nil
}

func (r *CoverImportRepository) DecideQuarantine(ctx context.Context, jobID, libraryID, actorID, actorRole, decision, eventID, auditID, now string) error {
	if jobID == "" || libraryID == "" || actorID == "" || auditID == "" || (decision != "APPROVE" && decision != "REJECT") || (decision == "APPROVE" && eventID == "") {
		return ErrInvalidCoverImport
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	status, code := "REJECTED", "ROOT_REJECTED_REVIEW"
	if decision == "APPROVE" {
		status, code = "OCR_PENDING", "ROOT_APPROVED_REVIEW"
	}
	result, err := tx.ExecContext(ctx, `UPDATE cover_import_jobs SET status=?,nsfw_decision=CASE WHEN ?='APPROVE' THEN 'SAFE' ELSE nsfw_decision END,decision_code=?,updated_at=?,terminal_at=CASE WHEN ?='REJECT' THEN ? ELSE NULL END WHERE id=? AND library_id=? AND status='QUARANTINED' AND nsfw_decision='REVIEW'`, status, decision, code, now, decision, now, jobID, libraryID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCoverImportReviewState
	}
	if decision == "APPROVE" {
		payload, _ := json.Marshal(map[string]any{"schemaVersion": 1, "eventId": eventID, "jobId": jobID, "libraryId": libraryID, "attempt": 1})
		_, err = tx.ExecContext(ctx, `INSERT INTO cover_import_outbox(event_id,job_id,library_id,event_type,schema_version,payload,attempts,available_at,created_at) VALUES(?,?,?,'cover.imports.ocr.v1',1,?,0,?,?)`, eventID, jobID, libraryID, string(payload), now, now)
		if err != nil {
			return err
		}
	}
	var policyVersion string
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(nsfw_policy_version,'') FROM cover_import_jobs WHERE id=? AND library_id=?`, jobID, libraryID).Scan(&policyVersion); err != nil {
		return err
	}
	values, _ := json.Marshal(map[string]string{"decision": decision, "libraryId": libraryID, "actorRole": actorRole, "correlationId": auditID, "nsfwPolicyVersion": policyVersion})
	_, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'DECIDE_COVER_IMPORT_QUARANTINE','COVER_IMPORT_JOB',?,?,1,?)`, auditID, actorID, jobID, string(values), now)
	if err != nil {
		return err
	}
	return tx.Commit()
}
