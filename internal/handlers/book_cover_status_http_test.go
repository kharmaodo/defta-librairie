package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCoverStatusService struct {
	status services.BookCoverStatus
	err    error
	bookID int
	calls  int
}

func (f *fakeCoverStatusService) Status(_ context.Context, _ *auth.Claims, id int) (services.BookCoverStatus, error) {
	f.bookID, f.calls = id, f.calls+1
	return f.status, f.err
}
func (f *fakeCoverStatusService) Retry(_ context.Context, _ *auth.Claims, id int) (services.BookCoverStatus, error) {
	f.bookID, f.calls = id, f.calls+1
	return f.status, f.err
}

func TestCoverStatusHTTPReturnsPublicState(t *testing.T) {
	fake := &fakeCoverStatusService{status: services.BookCoverStatus{
		ID: "cover-id", Status: "FAILED", ErrorCode: "decode_failed",
		CanRetry: true, UpdatedAt: "2026-09-16T12:00:00Z",
	}}
	handler := &BookCoverHandler{enabled: true, statusService: fake}
	request := httptest.NewRequest(http.MethodGet, "/api/manage/books/42/cover/status", nil)
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()
	handler.Status(response, request)
	if response.Code != http.StatusOK || fake.bookID != 42 || fake.calls != 1 ||
		!strings.Contains(response.Body.String(), `"canRetry":true`) ||
		strings.Contains(response.Body.String(), "sourceObjectKey") {
		t.Fatalf("status=%d body=%s fake=%+v", response.Code, response.Body.String(), fake)
	}
}

func TestCoverRetryHTTPAcceptedAndConflicts(t *testing.T) {
	fake := &fakeCoverStatusService{status: services.BookCoverStatus{ID: "cover-id", Status: "PENDING", UpdatedAt: "2026-09-16T12:00:00Z"}}
	handler := &BookCoverHandler{enabled: true, statusService: fake}
	request := httptest.NewRequest(http.MethodPost, "/api/manage/books/42/cover/retry", nil)
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()
	handler.Retry(response, request)
	if response.Code != http.StatusAccepted || response.Header().Get("Location") != "/api/manage/books/42/cover/status" {
		t.Fatalf("status=%d location=%q", response.Code, response.Header().Get("Location"))
	}
	fake.err = repositories.ErrBookCoverNotRetryable
	response = httptest.NewRecorder()
	handler.Retry(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "cover_not_retryable") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCoverStatusHTTPMissing(t *testing.T) {
	fake := &fakeCoverStatusService{err: repositories.ErrBookCoverNotFound}
	handler := &BookCoverHandler{enabled: true, statusService: fake}
	request := httptest.NewRequest(http.MethodGet, "/api/manage/books/42/cover/status", nil)
	request.SetPathValue("id", "42")
	response := httptest.NewRecorder()
	handler.Status(response, request)
	if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), "cover_not_found") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
