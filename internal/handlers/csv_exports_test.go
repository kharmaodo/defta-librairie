package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/csv"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type csvHTTPReader struct{ err error }

func (r csvHTTPReader) Read(context.Context, string, repositories.CSVExportFilter) (repositories.CSVExportTable, error) {
	return repositories.CSVExportTable{Header: []string{"title", "value"}, Rows: [][]string{{"كتاب; \"test\"\nligne", "  =SUM(1,2)"}}}, r.err
}
func TestCSVDownloadRoundTripAndErrors(t *testing.T) {
	for _, tc := range []struct {
		query   string
		repoErr error
		status  int
	}{{"", nil, 200}, {"?status=DRAFT&status=CONFIRMED", nil, 400}, {"?bad=1", nil, 400}, {"?from=%zz", nil, 400}, {"", repositories.ErrExportTooLarge, 422}, {"", errors.New("db unavailable"), 500}} {
		h := NewCSVExportHandler(services.NewCSVExportService(csvHTTPReader{err: tc.repoErr}))
		req := httptest.NewRequest("GET", "/api/manage/exports/sales"+tc.query, nil)
		req.SetPathValue("kind", "sales")
		req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "own", RegisteredClaims: jwt.RegisteredClaims{Subject: "owner"}}))
		w := httptest.NewRecorder()
		h.Download(w, req)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		if tc.status != 200 {
			if strings.Contains(w.Header().Get("Content-Type"), "text/csv") {
				t.Fatal("error returned as CSV")
			}
			continue
		}
		if !strings.HasPrefix(w.Body.String(), "\xEF\xBB\xBF") || w.Header().Get("Content-Disposition") != `attachment; filename="sales.csv"` {
			t.Fatal("download headers/BOM")
		}
		reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\xEF\xBB\xBF")))
		reader.Comma = ';'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(rows, [][]string{{"title", "value"}, {"كتاب; \"test\"\nligne", "'  =SUM(1,2)"}}) {
			t.Fatalf("CSV=%q", rows)
		}
	}
}
func TestSafeCSVCell(t *testing.T) {
	for _, s := range []string{"=1+1", " +cmd", "-1", "@SUM(A1)", "\ttext", "\n=1", "\uFEFF=1"} {
		if safeCSVCell(s) != "'"+s {
			t.Fatalf("unsafe %q", s)
		}
	}
	for _, s := range []string{"كتاب", "normal;value", "42", ""} {
		if safeCSVCell(s) != s {
			t.Fatalf("altered %q", s)
		}
	}
}
