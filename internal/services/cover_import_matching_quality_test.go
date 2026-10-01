package services

import (
	"context"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMatchingQualityOffOnAndDuplicateDelivery(t *testing.T) {
	for _, quality := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[quality], func(t *testing.T) {
			runner := &detailedTestOCR{}
			ocrService, db, id := ocrFixture(t, runner)
			if err := ocrService.Process(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1,'library-a','صحيح البخاري');UPDATE cover_import_ocr_results SET text_raw=? WHERE job_id=?`, strings.Repeat("ضجيج مجهول ", 20)+"صحيح البخاري", id); err != nil {
				t.Fatal(err)
			}
			service := NewCoverImportMatchingService(repositories.NewCoverImportRepository(db)).WithQualityMatching(quality)
			if err := service.Process(context.Background(), id, "foreign"); !errors.Is(err, repositories.ErrCoverImportJobNotFound) {
				t.Fatal("foreign scope accepted", err)
			}
			if err := service.Process(context.Background(), id, "library-a"); err != nil {
				t.Fatal(err)
			}
			if err := service.Process(context.Background(), id, "library-a"); !IsMatchingAlreadyProcessed(err) {
				t.Fatal("duplicate", err)
			}
			var count int
			db.QueryRow(`SELECT COUNT(*) FROM cover_import_candidate_matches WHERE job_id=?`, id).Scan(&count)
			want := 0
			if quality {
				want = 1
			}
			if count != want {
				t.Fatalf("count=%d want=%d", count, want)
			}
			var audit string
			var audits int
			db.QueryRow(`SELECT COUNT(*),new_values FROM audit_logs WHERE resource_id=? AND action='COMPLETE_COVER_IMPORT_MATCHING'`, id).Scan(&audits, &audit)
			if audits != 1 || strings.Contains(audit, "البخاري") {
				t.Fatal("audit duplicate/text", audit)
			}
			var metadata map[string]any
			if err := json.Unmarshal([]byte(audit), &metadata); err != nil {
				t.Fatal(err)
			}
			policy := "v1"
			if quality {
				policy = "v2"
			}
			if metadata["policyVersion"] != policy {
				t.Fatal(metadata)
			}
			if quality {
				var explanation string
				db.QueryRow(`SELECT explanation_json FROM cover_import_candidate_matches WHERE job_id=?`, id).Scan(&explanation)
				if !strings.Contains(explanation, "scoped_fts5_bm25") {
					t.Fatal(explanation)
				}
			}
		})
	}
}

func TestMatchingQualityPersistenceFailureRollsBack(t *testing.T) {
	ocrService, db, id := ocrFixture(t, &detailedTestOCR{})
	if err := ocrService.Process(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1,'library-a','الكتاب');CREATE TRIGGER matching_audit_failure BEFORE INSERT ON audit_logs WHEN new.action='COMPLETE_COVER_IMPORT_MATCHING' BEGIN SELECT RAISE(ABORT,'simulated'); END;`)
	service := NewCoverImportMatchingService(repositories.NewCoverImportRepository(db)).WithQualityMatching(true)
	if err := service.Process(context.Background(), id, "library-a"); err == nil {
		t.Fatal("failure ignored")
	}
	if ocrStatus(t, db) != "MATCHING" {
		t.Fatal("state committed before audit")
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM cover_import_candidate_matches`).Scan(&count)
	if count != 0 {
		t.Fatal("partial candidates")
	}
	db.Exec(`DROP TRIGGER matching_audit_failure`)
	if err := service.Process(context.Background(), id, "library-a"); err != nil {
		t.Fatal(err)
	}
	if ocrStatus(t, db) != "REVIEW_REQUIRED" {
		t.Fatal("human review skipped")
	}
}
