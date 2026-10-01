package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/ocr"
	"errors"
	"strings"
	"testing"
	"time"
)

func experimentalRepo(t *testing.T) (*CoverImportRepository, *sql.DB) {
	db := openCoverImportModerationDB(t)
	_, err := db.Exec(`ALTER TABLE cover_import_jobs ADD COLUMN ocr_claim_token TEXT;ALTER TABLE cover_import_jobs ADD COLUMN ocr_claim_until TEXT;ALTER TABLE cover_import_jobs ADD COLUMN ocr_attempts INTEGER NOT NULL DEFAULT 0;ALTER TABLE cover_import_ocr_results ADD COLUMN policy_version TEXT;ALTER TABLE cover_import_ocr_results ADD COLUMN psm INTEGER;ALTER TABLE cover_import_ocr_results ADD COLUMN preprocessing TEXT;`)
	if err != nil {
		t.Fatal(err)
	}
	seedModerationJob(t, db, "job-experimental")
	if _, err = db.Exec(`UPDATE cover_import_jobs SET status='OCR_PENDING' WHERE id='job-experimental'`); err != nil {
		t.Fatal(err)
	}
	return NewCoverImportRepository(db), db
}
func experimentalResult() ocr.Result {
	confidence := 0.82
	return ocr.Result{Engine: "tesseract-experimental", EngineVersion: "tesseract 5.3.0", PolicyVersion: "ocr-local-v1", Language: "ara", TextRaw: "PRIVATE OCR", TextNormalized: "PRIVATE OCR", Confidence: &confidence, PSM: 11, Preprocessing: "original"}
}
func TestExperimentalOCRLeaseFencingAndAtomicCompletion(t *testing.T) {
	repo, db := experimentalRepo(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	old, err := repo.ClaimExperimentalOCR(ctx, "job-experimental", "old", now.Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.ClaimExperimentalOCR(ctx, old.ID, "other", now.Format(time.RFC3339Nano), now.Add(time.Minute).Format(time.RFC3339Nano)); !errors.Is(err, ErrCoverImportOCRLeaseHeld) {
		t.Fatalf("stole live claim: %v", err)
	}
	later := now.Add(2 * time.Minute).Format(time.RFC3339Nano)
	job, err := repo.ClaimExperimentalOCR(ctx, old.ID, "new", later, now.Add(3*time.Minute).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
	if job.Attempts != 2 {
		t.Fatal("attempts not durable")
	}
	if err = repo.CompleteExperimentalOCR(ctx, old, experimentalResult(), "event-old", "audit-old", later); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatal("stale result accepted")
	}
	if err = repo.FailExperimentalOCR(ctx, old, "OCR_TIMEOUT", "failed-old", later); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatal("stale failure accepted")
	}
	if err = repo.RetryExperimentalOCR(ctx, old, "OCR_BUSY", later); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatal("stale retry accepted")
	}
	wrong := job
	wrong.LibraryID = "other-library"
	if err = repo.CompleteExperimentalOCR(ctx, wrong, experimentalResult(), "event-other", "audit-other", later); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatal("cross-library completion accepted")
	}
	if err = repo.CompleteExperimentalOCR(ctx, job, experimentalResult(), "event-new", "audit-new", later); err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteExperimentalOCR(ctx, job, experimentalResult(), "event-double", "audit-double", later); !errors.Is(err, ErrCoverImportJobState) {
		t.Fatal("duplicate completion accepted")
	}
	for _, table := range []string{"cover_import_ocr_results", "cover_import_outbox", "audit_logs"} {
		var count int
		if err = db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil || count != 1 {
			t.Fatalf("%s: %d %v", table, count, err)
		}
	}
	var version, policy, metadata string
	var confidence float64
	var psm int
	if err = db.QueryRow(`SELECT engine_version,policy_version,confidence,psm FROM cover_import_ocr_results`).Scan(&version, &policy, &confidence, &psm); err != nil {
		t.Fatal(err)
	}
	if version != "tesseract 5.3.0" || policy != "ocr-local-v1" || confidence != 0.82 || psm != 11 {
		t.Fatal("metadata not persisted")
	}
	if err = db.QueryRow(`SELECT new_values FROM audit_logs`).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(metadata, "PRIVATE OCR") || !strings.Contains(metadata, "library-1") {
		t.Fatal("private OCR in audit or missing scope")
	}
}
func TestExperimentalOCRRollbackAndNullableConfidence(t *testing.T) {
	repo, db := experimentalRepo(t)
	ctx := context.Background()
	now := "2026-10-01T00:00:00Z"
	until := "2026-10-01T00:01:00Z"
	job, err := repo.ClaimExperimentalOCR(ctx, "job-experimental", "token", now, until)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RecoverExpiredExperimentalOCR(ctx, job.ID, now); !errors.Is(err, ErrCoverImportOCRLeaseHeld) {
		t.Fatal("rollback stole live claim")
	}
	if err = repo.RecoverExpiredExperimentalOCR(ctx, job.ID, "2026-10-01T00:02:00Z"); err != nil {
		t.Fatal(err)
	}
	job, err = repo.ClaimExperimentalOCR(ctx, job.ID, "token-2", "2026-10-01T00:02:00Z", "2026-10-01T00:03:00Z")
	if err != nil {
		t.Fatal(err)
	}
	result := experimentalResult()
	result.Confidence = nil
	if err = repo.CompleteExperimentalOCR(ctx, job, result, "event", "audit", "2026-10-01T00:02:05Z"); err != nil {
		t.Fatal(err)
	}
	var confidence sql.NullFloat64
	if err = db.QueryRow(`SELECT confidence FROM cover_import_ocr_results`).Scan(&confidence); err != nil || confidence.Valid {
		t.Fatal("missing confidence must stay NULL")
	}
}

func TestExperimentalOCRNeverStealsUnleasedLocalWork(t *testing.T) {
	repo, db := experimentalRepo(t)
	if _, err := db.Exec(`UPDATE cover_import_jobs SET status='OCR_PROCESSING'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ClaimExperimentalOCR(context.Background(), "job-experimental", "new", "2026-10-01T00:00:00Z", "2026-10-01T00:01:00Z"); !errors.Is(err, ErrCoverImportOCRLeaseHeld) {
		t.Fatal("experimental engine stole local work")
	}
}
