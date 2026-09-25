package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
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


func TestRateLimiterEnforcesConcurrentLimitPerClient(t *testing.T) {
	const (
		limit    = 8
		attempts = 64
	)
	limiter, err := NewRateLimiter(limit, time.Minute)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	now := time.Date(2026, 9, 25, 11, 30, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := limiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	results := make(chan *httptest.ResponseRecorder, attempts)
	var group sync.WaitGroup
	for range attempts {
		group.Add(1)
		go func() {
			defer group.Done()
			request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
			request.RemoteAddr = "192.0.2.10:1234"
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			results <- response
		}()
	}
	group.Wait()
	close(results)

	allowed := 0
	rejected := 0
	for response := range results {
		switch response.Code {
		case http.StatusNoContent:
			allowed++
		case http.StatusTooManyRequests:
			rejected++
			if got := response.Header().Get("Retry-After"); got != "61" {
				t.Errorf("unexpected Retry-After %q", got)
			}
		default:
			t.Errorf("unexpected status %d", response.Code)
		}
	}
	if allowed != limit || rejected != attempts-limit {
		t.Fatalf("allowed=%d rejected=%d, want %d/%d", allowed, rejected, limit, attempts-limit)
	}
}

func TestRateLimiterSeparatesClients(t *testing.T) {
	limiter, err := NewRateLimiter(1, time.Minute)
	if err != nil {
		t.Fatalf("new limiter: %v", err)
	}
	handler := limiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, address := range []string{"192.0.2.10:1234", "198.51.100.10:1234"} {
		request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		request.RemoteAddr = address
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s: expected allowed request, got %d", address, response.Code)
		}
	}
}
