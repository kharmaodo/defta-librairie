package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterRejectsAndResets(t *testing.T) {
	limiter, err := NewRateLimiter(2, time.Minute)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := limiter.Limit(next)

	for attempt, expected := range []int{http.StatusNoContent, http.StatusNoContent, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != expected {
			t.Fatalf("attempt %d: expected %d, got %d", attempt+1, expected, response.Code)
		}
		if expected == http.StatusTooManyRequests && response.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}

	now = now.Add(time.Minute)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected reset limiter, got %d", response.Code)
	}
}
