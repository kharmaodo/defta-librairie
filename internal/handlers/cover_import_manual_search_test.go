package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type manualReviewStub struct {
	coverImportReviewer
	calls int
	err   error
}

func (s *manualReviewStub) SearchCandidates(context.Context, *auth.Claims, string, string, string) ([]repositories.CoverImportReviewCandidate, error) {
	s.calls++
	return []repositories.CoverImportReviewCandidate{}, s.err
}
func (s *manualReviewStub) DismissCandidate(context.Context, *auth.Claims, string, string, int) error {
	s.calls++
	return s.err
}
func TestManualCoverReviewHTTPValidation(t *testing.T) {
	for _, method := range []string{"search", "dismiss"} {
		t.Run(method, func(t *testing.T) {
			stub := &manualReviewStub{}
			h := NewCoverImportReviewHandler(stub, true)
			call := h.SearchCandidates
			body := `{"query":"كتاب","libraryId":"lib"}`
			if method == "dismiss" {
				call = h.DismissCandidate
				body = `{"bookId":1,"libraryId":"lib"}`
			}
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			r.SetPathValue("id", "job")
			w := httptest.NewRecorder()
			call(w, r)
			if w.Code != 200 || stub.calls != 1 {
				t.Fatalf("nominal %d calls=%d", w.Code, stub.calls)
			}
			w = httptest.NewRecorder()
			call(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{`)))
			if w.Code != 400 || stub.calls != 1 {
				t.Fatalf("invalid %d calls=%d", w.Code, stub.calls)
			}
			stub.err = repositories.ErrCoverImportReviewState
			w = httptest.NewRecorder()
			call(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code != 409 {
				t.Fatalf("state %d", w.Code)
			}
			stub.err = errors.New("backend private detail")
			w = httptest.NewRecorder()
			call(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
			if w.Code != 503 || strings.Contains(w.Body.String(), "private detail") {
				t.Fatalf("private backend error %d %s", w.Code, w.Body.String())
			}
		})
	}
}
