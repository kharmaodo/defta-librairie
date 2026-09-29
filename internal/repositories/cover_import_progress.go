package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"fmt"
)

// listCoverImportJobs returns only the status metadata needed by the review UI.
// Object keys, image bytes and OCR text never enter the list response.
func (r *CoverImportRepository) listCoverImportJobs(ctx context.Context, libraryID, importID string) ([]models.CoverImportJob, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,import_id,library_id,status,source_content_type,source_format,
		source_width,source_height,source_size,nsfw_decision,decision_code,failure_code,target_book_id,
		expires_at,created_at,updated_at FROM cover_import_jobs
		WHERE library_id=? AND import_id=? ORDER BY created_at,id`, libraryID, importID)
	if err != nil {
		return nil, fmt.Errorf("list cover import jobs: %w", err)
	}
	defer rows.Close()
	jobs := []models.CoverImportJob{}
	for rows.Next() {
		var job models.CoverImportJob
		var decision, code, failure sql.NullString
		var target sql.NullInt64
		if err = rows.Scan(&job.ID, &job.ImportID, &job.LibraryID, &job.Status, &job.ContentType, &job.Format,
			&job.Width, &job.Height, &job.Size, &decision, &code, &failure, &target,
			&job.ExpiresAt, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cover import job: %w", err)
		}
		job.NSFWDecision, job.DecisionCode, job.FailureCode = decision.String, code.String, failure.String
		if target.Valid {
			id := int(target.Int64)
			job.TargetBookID = &id
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func updateCoverImportSummary(item *models.CoverImport) {
	if len(item.Jobs) == 0 {
		return
	}
	item.AcceptedFiles, item.RejectedFiles = 0, 0
	active := false
	latest := item.UpdatedAt
	for _, job := range item.Jobs {
		switch job.Status {
		case "READY":
			item.AcceptedFiles++
		case "REJECTED", "FAILED", "CANCELLED":
			item.RejectedFiles++
		default:
			active = true
		}
		if job.UpdatedAt > latest {
			latest = job.UpdatedAt
		}
	}
	item.UpdatedAt = latest
	if active {
		item.Status = "PROCESSING"
		item.CompletedAt = nil
	} else {
		item.Status = "COMPLETED"
		item.CompletedAt = &latest
	}
}

func countActiveCoverImports(ctx context.Context, tx *sql.Tx, libraryID string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM cover_imports i WHERE i.library_id=? AND EXISTS (
		SELECT 1 FROM cover_import_jobs j WHERE j.import_id=i.id
		AND j.status NOT IN ('READY','REJECTED','FAILED','CANCELLED')
	)`, libraryID).Scan(&count)
	return count, err
}
