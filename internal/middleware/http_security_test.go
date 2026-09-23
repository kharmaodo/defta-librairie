package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecureHTTPAddsHeaders(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	response := httptest.NewRecorder()
	SecureHTTP(next).ServeHTTP(response, request)

	for _, header := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "X-Request-ID"} {
		if response.Header().Get(header) == "" {
			t.Fatalf("missing %s", header)
		}
	}
	if !strings.Contains(response.Header().Get("Content-Security-Policy"), "img-src 'self' data: blob:") {
		t.Fatalf("CSP does not allow authenticated image previews: %q", response.Header().Get("Content-Security-Policy"))
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store, got %q", response.Header().Get("Cache-Control"))
	}
}
