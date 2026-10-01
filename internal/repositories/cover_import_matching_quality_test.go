package repositories

import (
	"context"
	"defta-librairie/internal/ocr"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestQualityMatchingAnnotatedBenchmark(t *testing.T) {
	db := matchingDB(t)
	_, err := db.Exec(`INSERT INTO defta(id,library_id,title,auteur) VALUES
 (1,'a','الْبِدَايَة والنهاية','ابن كثير'),
 (2,'a','تفسير القرآن العظيم','ابن كثير'),
 (3,'a','صحيح البخاري','محمد البخاري'),
 (4,'b','تفسير القرآن العظيم','ابن كثير'),
 (5,'a','العذب الفائض شرح عمدة الفارض',''),
 (6,'a','كتاب محذوف',''),
 (7,'a','تفسير الحديث',''),(8,'a','تفسير الأحلام',''),
 (9,'a','تفسير المعاني',''),(10,'a','تفسير اللغة',''),
 (11,'a','تفسير السيرة',''),(12,'a','تفسير الأخبار','');UPDATE defta SET deleted_at='yesterday' WHERE id=6;`)
	if err != nil {
		t.Fatal(err)
	}
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	cases := []struct {
		name, text string
		want       int64
	}{
		{"title", "تفسير القرآن العظيم", 2},
		{"after-noise", strings.Repeat("ضجيج مجهول ", 20) + "صحيح البخاري", 3},
		{"diacritics", "البداية والنهاية", 1},
		{"one-edit", "تفسير القرآم العظيم", 2},
		{"one-edit-only-evidence", "البدايه", 1},
		{"reported-title", "العذب الفائض شرح عمدة الفارض", 5},
		{"unknown", "مجهول تماما", 0},
		{"deleted", "كتاب محذوف", 0},
	}
	baselineHits, baselineAt5, newHits, at5, total := 0, 0, 0, 0, 0
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := repo.SearchQualityCoverImportCandidates(ctx, "a", c.text)
			if err != nil {
				t.Fatal(err)
			}
			if c.want == 0 {
				if len(got) != 0 {
					t.Fatalf("false candidates: %v", got)
				}
				return
			}
			total++
			if len(got) > 0 && got[0].BookID == c.want {
				newHits++
			} else {
				t.Fatalf("top1: %v expected %d", got, c.want)
			}
			for _, candidate := range got {
				if candidate.BookID == 4 || candidate.BookID == 6 {
					t.Fatal("foreign/deleted candidate")
				}
				if candidate.BookID == c.want {
					at5++
				}
			}
			words := strings.Fields(ocr.NormalizeForMatching(c.text))
			if len(words) > 12 {
				words = words[:12]
			}
			// Legacy path normalizes and truncates before searching the global index.
			old, err := repo.SearchCoverImportCandidates(ctx, "a", legacyQualityQuery(strings.Join(words, " ")))
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range old {
				if candidate.BookID == c.want {
					baselineAt5++
				}
			}
			if len(old) > 0 && old[0].BookID == c.want {
				baselineHits++
			}
		})
	}
	t.Logf("annotated synthetic corpus: positives=%d baseline recall@1=%d/%d recall@5=%d/%d v2 recall@1=%d/%d recall@5=%d/%d; negative cases=2", total, baselineHits, total, baselineAt5, total, newHits, total, at5, total)
}

func TestQualityMatchingUnaffectedByForeignCatalogue(t *testing.T) {
	db := matchingDB(t)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1,'a','البداية والنهاية'),(2,'a','تفسير القرآن')`)
	before, err := repo.SearchQualityCoverImportCandidates(ctx, "a", "البدايه والنهاية")
	if err != nil {
		t.Fatal(err)
	}
	// A foreign near-word must not make local correction ambiguous, or alter BM25.
	db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(3,'b','البدايا والنهاية'),(4,'b','البداية البداية البداية البداية')`)
	after, err := repo.SearchQualityCoverImportCandidates(ctx, "a", "البدايه والنهاية")
	if err != nil {
		t.Fatal(err)
	}
	if len(before) == 0 || !reflect.DeepEqual(before, after) {
		t.Fatalf("foreign influence: %v -> %v", before, after)
	}
	if _, err = repo.SearchQualityCoverImportCandidates(ctx, "", "text"); !errors.Is(err, ErrInvalidCoverImport) {
		t.Fatal(err)
	}
}

func TestQualityMatchingBoundsAndCleanup(t *testing.T) {
	db := matchingDB(t)
	db.SetMaxOpenConns(1)
	repo := NewCoverImportRepository(db)
	ctx := context.Background()
	db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1,'a','كتاب البداية')`)
	for i := 0; i < 3; i++ {
		if _, err := repo.SearchQualityCoverImportCandidates(ctx, "a", "البداية"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM sqlite_temp_master WHERE name LIKE 'cover_matching_%'`).Scan(&count)
	if count != 0 {
		t.Fatal("temporary index leaked")
	}
	db.Exec(`UPDATE defta SET title=?`, strings.Repeat("x", 65537))
	if _, err := repo.SearchQualityCoverImportCandidates(ctx, "a", "البداية"); !errors.Is(err, ErrMatchingCatalogueLimit) {
		t.Fatal("partial catalogue used", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.SearchQualityCoverImportCandidates(ctx, "a", "البداية"); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func legacyQualityQuery(s string) string {
	return strings.ReplaceAll(ocr.FTSQuery(ocr.NormalizeForMatching(s)), " AND ", " OR ")
}

func TestQualityMatchingAmbiguousCorrectionAndFTSOperators(t *testing.T) {
	db := matchingDB(t)
	repo := NewCoverImportRepository(db)
	if _, err := db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1,'a','البداية'),(2,'a','البدايا'),(3,'b','NEAR OR NOT SELECT')`); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"البدايه", "NEAR OR NOT SELECT * \""} {
		got, err := repo.SearchQualityCoverImportCandidates(context.Background(), "a", text)
		if err != nil || len(got) != 0 {
			t.Fatalf("ambiguous/foreign operator evidence: %v %v", got, err)
		}
	}
}

func TestQualityMatchingRepresentativeCatalogueBudget(t *testing.T) {
	db := matchingDB(t)
	repo := NewCoverImportRepository(db)
	transaction, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	statement, err := transaction.Prepare(`INSERT INTO defta(id,library_id,title,auteur) VALUES(?,'a',?,'مؤلف تجريبي')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 1000; i++ {
		if _, err := statement.Exec(i, fmt.Sprintf("كتاب تجريبي رقم %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	statement.Close()
	if err := transaction.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO defta(id,library_id,title) VALUES(1001,'a','صحيح البخاري')`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	got, err := repo.SearchQualityCoverImportCandidates(context.Background(), "a", strings.Repeat("ضجيج مجهول ", 100)+"صحيح البخاري")
	if err != nil || len(got) != 1 || got[0].BookID != 1001 {
		t.Fatalf("representative search: %v %v", got, err)
	}
	t.Logf("1001 synthetic books: search elapsed=%s (indicative local measurement)", time.Since(started))
}
