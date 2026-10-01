package repositories

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
)

// SearchReviewBooks materializes suggestions, never a book attachment.
func (r *CoverImportRepository) SearchReviewBooks(ctx context.Context, job CoverImportReviewJob, query, actorID, now string) ([]CoverImportReviewCandidate, error) {
	tokens := strings.FieldsFunc(query, func(c rune) bool { return !unicode.IsLetter(c) && !unicode.IsDigit(c) })
	if len(tokens) == 0 || len(tokens) > 20 {
		return nil, ErrInvalidCoverImport
	}
	for i, t := range tokens {
		tokens[i] = `"` + t + `"`
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var valid int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cover_import_jobs WHERE id=? AND library_id=? AND status='REVIEW_REQUIRED' AND nsfw_decision='SAFE'`, job.ID, job.LibraryID).Scan(&valid); err != nil {
		return nil, err
	}
	if valid != 1 {
		return nil, ErrCoverImportReviewState
	}
	sqlQuery := `SELECT d.id,d.title,COALESCE(d.auteur,''),bm25(defta_fts,5.0,1.0,2.0,0.5,0.5),EXISTS(SELECT 1 FROM book_covers b WHERE b.book_id=d.id AND b.library_id=d.library_id AND b.active=1 AND b.status='READY') FROM defta_fts JOIN defta d ON d.id=defta_fts.rowid WHERE defta_fts MATCH ? AND d.library_id=? AND d.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM cover_import_review_suggestions s WHERE s.job_id=? AND s.book_id=d.id AND s.rejected=1) ORDER BY bm25(defta_fts,5.0,1.0,2.0,0.5,0.5),d.id LIMIT 3`
	args := []any{strings.Join(tokens, " OR "), job.LibraryID, job.ID}
	isbn := strings.ReplaceAll(strings.ReplaceAll(query, "-", ""), " ", "")
	numeric := len(isbn) == 13
	for _, c := range isbn {
		if c < '0' || c > '9' {
			numeric = false
		}
	}
	if numeric {
		// ISBNs available from prior OCR attachments, not an invented catalogue field.
		sqlQuery = `SELECT DISTINCT d.id,d.title,COALESCE(d.auteur,''),0,EXISTS(SELECT 1 FROM book_covers b WHERE b.book_id=d.id AND b.library_id=d.library_id AND b.active=1 AND b.status='READY') FROM defta d JOIN cover_import_jobs j ON j.target_book_id=d.id AND j.library_id=d.library_id AND j.status='READY' JOIN cover_import_ocr_results o ON o.job_id=j.id WHERE o.isbn13=? AND d.library_id=? AND d.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM cover_import_review_suggestions s WHERE s.job_id=? AND s.book_id=d.id AND s.rejected=1) ORDER BY d.id LIMIT 3`
		args = []any{isbn, job.LibraryID, job.ID}
	}
	rows, err := tx.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	result := []CoverImportReviewCandidate{}
	for rows.Next() {
		var c CoverImportReviewCandidate
		if err = rows.Scan(&c.BookID, &c.Title, &c.Author, &c.Score, &c.HasActiveCover); err != nil {
			rows.Close()
			return nil, err
		}
		c.Rank = len(result) + 1
		c.Origin = "MANUAL"
		if numeric {
			c.ISBN13 = isbn
		}
		result = append(result, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, c := range result {
		if _, err = tx.ExecContext(ctx, `INSERT INTO cover_import_review_suggestions(job_id,book_id,origin,actor_user_id,updated_at) VALUES(?,?,'MANUAL',?,?) ON CONFLICT(job_id,book_id) DO NOTHING`, job.ID, c.BookID, actorID, now); err != nil {
			return nil, err
		}
	}
	return result, tx.Commit()
}

func (r *CoverImportRepository) DismissReviewCandidate(ctx context.Context, job CoverImportReviewJob, bookID int, actorID, actorRole, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var valid int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cover_import_jobs j JOIN defta d ON d.id=? AND d.library_id=j.library_id AND d.deleted_at IS NULL WHERE j.id=? AND j.library_id=? AND j.status='REVIEW_REQUIRED' AND j.nsfw_decision='SAFE' AND (EXISTS(SELECT 1 FROM cover_import_candidate_matches c WHERE c.job_id=j.id AND c.book_id=d.id) OR EXISTS(SELECT 1 FROM cover_import_review_suggestions s WHERE s.job_id=j.id AND s.book_id=d.id))`, bookID, job.ID, job.LibraryID).Scan(&valid); err != nil {
		return err
	}
	if valid != 1 {
		return ErrCoverImportCandidateNotFound
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO cover_import_review_suggestions(job_id,book_id,origin,rejected,actor_user_id,updated_at) VALUES(?,?,'OCR',1,?,?) ON CONFLICT(job_id,book_id) DO UPDATE SET rejected=1,actor_user_id=excluded.actor_user_id,updated_at=excluded.updated_at WHERE rejected=0`, job.ID, bookID, actorID, now)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	values, _ := json.Marshal(map[string]any{"bookId": bookID, "libraryId": job.LibraryID, "actorRole": actorRole, "correlationId": auditID, "nsfwPolicyVersion": job.NSFWPolicyVersion})
	if _, err = tx.ExecContext(ctx, `INSERT INTO audit_logs(id,actor_user_id,action,resource_type,resource_id,new_values,success,created_at) VALUES(?,?,'DISMISS_COVER_IMPORT_CANDIDATE','COVER_IMPORT_JOB',?,?,1,?)`, auditID, actorID, job.ID, string(values), now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *CoverImportRepository) ReviewCandidateAllowed(ctx context.Context, job CoverImportReviewJob, bookID int) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM defta d WHERE d.id=? AND d.library_id=? AND d.deleted_at IS NULL AND NOT EXISTS(SELECT 1 FROM cover_import_review_suggestions s WHERE s.job_id=? AND s.book_id=d.id AND s.rejected=1) AND (EXISTS(SELECT 1 FROM cover_import_candidate_matches c WHERE c.job_id=? AND c.book_id=d.id) OR EXISTS(SELECT 1 FROM cover_import_review_suggestions s WHERE s.job_id=? AND s.book_id=d.id AND s.origin='MANUAL'))`, bookID, job.LibraryID, job.ID, job.ID, job.ID).Scan(&count)
	return count == 1, err
}
