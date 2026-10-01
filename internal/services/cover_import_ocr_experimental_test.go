package services

import (
	"bytes"
	"context"
	"database/sql"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type ocrTestSource struct{ data []byte }

func (s ocrTestSource) OpenSource(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

type detailedTestOCR struct {
	calls    int
	failures int
	err      error
	cancel   context.CancelFunc
}

func (s *detailedTestOCR) Run(context.Context, []byte, string) (string, error) {
	return "", errors.New("unexpected legacy fallback")
}
func (s *detailedTestOCR) Extract(ctx context.Context, _ []byte, _ string, correlation string) (ocr.Result, error) {
	s.calls++
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		return ocr.Result{}, ctx.Err()
	}
	if s.calls <= s.failures {
		return ocr.Result{}, s.err
	}
	confidence := 0.7
	return ocr.Result{Engine: "tesseract-experimental", EngineVersion: "tesseract 5.3.0", PolicyVersion: "ocr-local-v1", Language: "ara", TextRaw: "الْكتاب", TextNormalized: "الكتاب", Confidence: &confidence, PSM: 11, Preprocessing: "original"}, nil
}

type legacyTestOCR struct{ calls int }

func (s *legacyTestOCR) Run(context.Context, []byte, string) (string, error) {
	s.calls++
	return "عنوان   محلي", nil
}

func ocrFixture(t *testing.T, runner ocr.Runner) (*CoverImportOCRService, *sql.DB, string) {
	t.Helper()
	imports, claims, db := importErrorFixture(t, &importErrorStore{})
	if _, err := imports.Create(context.Background(), claims, "", "ocr-fixture", []CoverImportFile{importPNGFile(t)}); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := db.QueryRow(`SELECT id FROM cover_import_jobs`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE cover_import_jobs SET status='OCR_PENDING',nsfw_decision='SAFE'`); err != nil {
		t.Fatal(err)
	}
	service := NewCoverImportOCRService(repositories.NewCoverImportRepository(db), ocrTestSource{coverPNG(t)}, runner, "tesseract", "ara").WithExperimentalRetryPolicy(time.Second, 2)
	return service, db, id
}
func ocrStatus(t *testing.T, db *sql.DB) string {
	t.Helper()
	var status string
	if err := db.QueryRow(`SELECT status FROM cover_import_jobs`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}
func assertOCRPersistedOnce(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, query := range []string{`SELECT COUNT(*) FROM cover_import_ocr_results`, `SELECT COUNT(*) FROM cover_import_outbox WHERE event_type='cover.imports.match.v1'`, `SELECT COUNT(*) FROM audit_logs WHERE action='COMPLETE_COVER_IMPORT_OCR'`} {
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatalf("not once: %d %v", count, err)
		}
	}
}

func TestExperimentalOCRTransientRetryAndDuplicateDelivery(t *testing.T) {
	runner := &detailedTestOCR{failures: 1, err: ocr.ErrBusy}
	service, db, id := ocrFixture(t, runner)
	ctx := context.Background()
	if err := service.Process(ctx, id); !errors.Is(err, ocr.ErrBusy) || errors.Is(err, ErrCoverImportOCRFailed) {
		t.Fatalf("premature ACK/terminal failure: %v", err)
	}
	if ocrStatus(t, db) != "OCR_PENDING" {
		t.Fatal("transient job cannot retry")
	}
	if err := service.Process(ctx, id); err != nil {
		t.Fatal(err)
	}
	if ocrStatus(t, db) != "MATCHING" {
		t.Fatal("not matching")
	}
	if err := service.Process(ctx, id); !errors.Is(err, repositories.ErrCoverImportJobNotFound) {
		t.Fatalf("duplicate: %v", err)
	}
	if runner.calls != 2 {
		t.Fatal("duplicate re-extracted")
	}
	assertOCRPersistedOnce(t, db)
}
func TestExperimentalOCRRetryBudgetAndPermanentFailure(t *testing.T) {
	for _, failure := range []error{ocr.ErrTimeout, ocr.ErrUnavailable, ocr.ErrInvalidResponse, ocr.ErrInvalidInput} {
		t.Run(failure.Error(), func(t *testing.T) {
			runner := &detailedTestOCR{failures: 10, err: failure}
			service, db, id := ocrFixture(t, runner)
			err := service.Process(context.Background(), id)
			if failure != ocr.ErrInvalidInput {
				if errors.Is(err, ErrCoverImportOCRFailed) {
					t.Fatal("first transient terminal")
				}
				err = service.Process(context.Background(), id)
			}
			if !errors.Is(err, ErrCoverImportOCRFailed) || ocrStatus(t, db) != "FAILED" {
				t.Fatalf("budget not terminal: %v", err)
			}
			var audits, results int
			_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='COMPLETE_COVER_IMPORT_OCR' AND success=0`).Scan(&audits)
			_ = db.QueryRow(`SELECT COUNT(*) FROM cover_import_ocr_results`).Scan(&results)
			if audits != 1 || results != 0 {
				t.Fatal("bad failure persistence")
			}
		})
	}
}
func TestExperimentalOCRPersistenceRollbackRequeues(t *testing.T) {
	runner := &detailedTestOCR{}
	service, db, id := ocrFixture(t, runner)
	if _, err := db.Exec(`CREATE TRIGGER fail_match BEFORE INSERT ON cover_import_outbox WHEN NEW.event_type='cover.imports.match.v1' BEGIN SELECT RAISE(ABORT,'simulated outbox failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.Process(context.Background(), id); err == nil || errors.Is(err, ErrCoverImportOCRFailed) {
		t.Fatal("failed transaction ACKed")
	}
	if ocrStatus(t, db) != "OCR_PENDING" {
		t.Fatal("failed transaction stuck")
	}
	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM cover_import_ocr_results`).Scan(&count)
	if count != 0 {
		t.Fatal("partial result committed")
	}
	if _, err := db.Exec(`DROP TRIGGER fail_match`); err != nil {
		t.Fatal(err)
	}
	if err := service.Process(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	assertOCRPersistedOnce(t, db)
}
func TestExperimentalOCRCancellationAndLocalRollback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &detailedTestOCR{cancel: cancel}
	service, db, id := ocrFixture(t, runner)
	if err := service.Process(ctx, id); !errors.Is(err, context.Canceled) || errors.Is(err, ErrCoverImportOCRFailed) {
		t.Fatalf("cancelled decision persisted: %v", err)
	}
	if ocrStatus(t, db) != "OCR_PENDING" {
		t.Fatal("cancelled job cannot resume")
	}
	local := &legacyTestOCR{}
	service.runner = local
	if err := service.Process(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	assertOCRPersistedOnce(t, db)
	var engine, raw, normalized string
	var confidence sql.NullFloat64
	if err := db.QueryRow(`SELECT engine,text_raw,text_normalized,confidence FROM cover_import_ocr_results`).Scan(&engine, &raw, &normalized, &confidence); err != nil {
		t.Fatal(err)
	}
	if local.calls != 1 || engine != "tesseract" || raw != "عنوان   محلي" || normalized != "عنوان محلي" || confidence.Valid {
		t.Fatal("local rollback changed legacy contract")
	}
}
func TestExperimentalOCRCrashRecovery(t *testing.T) {
	runner := &detailedTestOCR{}
	service, db, id := ocrFixture(t, runner)
	repo := repositories.NewCoverImportRepository(db)
	now := time.Now().UTC()
	if _, err := repo.ClaimExperimentalOCR(context.Background(), id, "crashed", now.Add(-2*time.Minute).Format(time.RFC3339Nano), now.Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := service.Process(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	assertOCRPersistedOnce(t, db)
}

func TestExperimentalOCRLostClaimCannotCauseEarlyACK(t *testing.T) {
	if err := experimentalClaimError(repositories.ErrCoverImportJobState); !errors.Is(err, repositories.ErrCoverImportOCRLeaseHeld) || errors.Is(err, repositories.ErrCoverImportJobState) {
		t.Fatal("lost claim would be ACKed before another worker commits")
	}
}

// Exercise the production HTTP runner through the durable service transaction.
func TestExperimentalOCRHTTPToDurableResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-Id") == "" {
			t.Error("missing correlation")
		}
		if err := r.ParseMultipartForm(10 * 1024 * 1024); err != nil {
			t.Error(err)
			w.WriteHeader(422)
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, _, err := r.FormFile("image")
		if err != nil {
			t.Error(err)
			w.WriteHeader(422)
			return
		}
		defer f.Close()
		if data, err := io.ReadAll(f); err != nil || !bytes.Equal(data, coverPNG(t)) {
			t.Error("source changed")
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"schemaVersion":1,"engine":"tesseract-experimental","engineVersion":"tesseract 5.3.0","policyVersion":"ocr-local-v1","language":"ara","textRaw":"الْكتاب","textNormalized":"الكتاب","confidence":null,"psm":6,"preprocessing":"grayscale-autocontrast"}`)
	}))
	defer server.Close()
	runner, err := ocr.NewHTTP(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	service, db, id := ocrFixture(t, runner)
	if err := service.Process(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	assertOCRPersistedOnce(t, db)
	var confidence sql.NullFloat64
	var policy, preprocessing, normalized string
	var psm int
	if err := db.QueryRow(`SELECT confidence,policy_version,psm,preprocessing,text_normalized FROM cover_import_ocr_results`).Scan(&confidence, &policy, &psm, &preprocessing, &normalized); err != nil {
		t.Fatal(err)
	}
	if confidence.Valid || policy != "ocr-local-v1" || psm != 6 || preprocessing != "grayscale-autocontrast" || normalized != "الكتاب" {
		t.Fatal("HTTP metadata not preserved")
	}
}

type deadlineOCRSource struct{}

func (deadlineOCRSource) OpenSource(ctx context.Context, _ string) (io.ReadCloser, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}
func TestExperimentalOCRDeadlineIncludesSourceRead(t *testing.T) {
	runner := &detailedTestOCR{}
	service, db, id := ocrFixture(t, runner)
	service.sources = deadlineOCRSource{}
	service.WithExperimentalRetryPolicy(10*time.Millisecond, 2)
	start := time.Now()
	if err := service.Process(context.Background(), id); !errors.Is(err, ocr.ErrUnavailable) {
		t.Fatalf("source deadline: %v", err)
	}
	if time.Since(start) > time.Second || runner.calls != 0 || ocrStatus(t, db) != "OCR_PENDING" {
		t.Fatal("source timeout did not release claim before inference")
	}
}
