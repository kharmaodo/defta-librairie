package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"net/http/httptest"
	"testing"
)

type alertHTTPReader struct{ calls int }

func (r *alertHTTPReader) List(_ context.Context, _ string, _ string, offset, limit int) (repositories.BusinessAlerts, error) {
	r.calls++
	return repositories.BusinessAlerts{Results: []repositories.BusinessAlert{}, Offset: offset, Limit: limit}, nil
}
func TestBusinessAlertsHTTPValidation(t *testing.T) {
	for _, tc := range []struct {
		query     string
		anonymous bool
		status    int
	}{
		{"", false, 200}, {"?limit=0", false, 400}, {"?limit=abc", false, 400}, {"?kind=bad", false, 400}, {"?kind=LOW_STOCK&kind=OUT_OF_STOCK", false, 400}, {"?unexpected=1", false, 400}, {"?libraryId=other", false, 403}, {"", true, 401},
	} {
		reader := &alertHTTPReader{}
		h := NewBusinessAlertHandler(services.NewBusinessAlertService(reader))
		req := httptest.NewRequest("GET", "/api/manage/alerts"+tc.query, nil)
		if !tc.anonymous {
			req = req.WithContext(auth.ContextWithClaims(req.Context(), &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "own"}))
		}
		w := httptest.NewRecorder()
		h.List(w, req)
		if w.Code != tc.status || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("query=%s status=%d body=%s", tc.query, w.Code, w.Body.String())
		}
		if tc.status != 200 && reader.calls != 0 {
			t.Fatal("invalid request reached repository")
		}
	}
}
