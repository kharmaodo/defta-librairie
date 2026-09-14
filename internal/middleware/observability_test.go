package middleware

import (
	"bytes"
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestHTTPObservabilityPrivacyAndStatus(t *testing.T) {
	var log bytes.Buffer
	o := NewHTTPObservability(slog.New(slog.NewJSONHandler(&log, nil)))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/items/{id}", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(201); w.Write([]byte("ok")) })
	handler := SecureHTTP(o.Wrap(mux))
	req := httptest.NewRequest("POST", "/api/items/private-id?token=secret-query", strings.NewReader("secret-body"))
	req.Header.Set("Authorization", "Bearer secret-header")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	var event map[string]interface{}
	if err := json.Unmarshal(log.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"private-id", "secret-query", "secret-body", "secret-header"} {
		if strings.Contains(log.String(), secret) {
			t.Fatalf("log leaked %s", secret)
		}
	}
	if event["route"] != "POST /api/items/{id}" || event["status"] != float64(201) || event["request_id"] != w.Header().Get("X-Request-ID") {
		t.Fatalf("event=%v", event)
	}
	s := o.Snapshot()
	if s.Completed != 1 || s.InFlight != 0 || len(s.Routes) != 1 || s.Routes[0].Bytes != 2 || s.Routes[0].Requests != 1 {
		t.Fatalf("snapshot=%+v", s)
	}
	for _, path := range []string{"/unknown-one", "/unknown-two"} {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", path, nil))
	}
	s = o.Snapshot()
	if len(s.Routes) != 2 {
		t.Fatalf("unbounded unmatched labels: %+v", s)
	}
}
func TestHTTPObservabilityConcurrencyAndPanic(t *testing.T) {
	o := NewHTTPObservability(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	h := o.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
			o.Snapshot()
		}()
	}
	wg.Wait()
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic must propagate")
			}
		}()
		o.Wrap(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("test") })).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
	}()
	s := o.Snapshot()
	if s.Completed != 51 || s.InFlight != 0 {
		t.Fatalf("snapshot=%+v", s)
	}
}
func TestMetricsRootGuard(t *testing.T) {
	o := NewHTTPObservability(slog.New(slog.NewJSONHandler(io.Discard, nil)))
	h := RequirePasswordChanged(RequireRoles(http.HandlerFunc(o.Metrics), models.RoleSuperAdminRoot))
	for _, tc := range []struct {
		claims *auth.Claims
		code   int
	}{{nil, 401}, {&auth.Claims{Role: models.RoleOwnerLibrary}, 403}, {&auth.Claims{Role: models.RoleSuperAdminRoot, PasswordChangeRequired: true}, 403}, {&auth.Claims{Role: models.RoleSuperAdminRoot}, 200}} {
		req := httptest.NewRequest("GET", "/api/admin/metrics", nil)
		if tc.claims != nil {
			req = req.WithContext(auth.ContextWithClaims(context.Background(), tc.claims))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != tc.code {
			t.Fatalf("status=%d want=%d", w.Code, tc.code)
		}
		if tc.code == 200 && w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("metrics cached")
		}
	}
}
