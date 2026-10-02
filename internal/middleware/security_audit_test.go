package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestCSRFCookieAndOriginEnforcement(t *testing.T) {
	h := CSRF(false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), []byte(strings.Repeat("k", 32)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "http://example.test/login", nil))
	cookie := rr.Result().Cookies()[0]
	if cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" || cookie.Path != "/" {
		t.Fatal(cookie)
	}
	for _, tc := range []struct {
		name, token, origin, site string
		cookieSession             bool
		want                      int
	}{
		{"missing token", "", "", "", true, 403},
		{"valid", cookie.Value, "http://example.test", "same-origin", true, 204},
		{"forged", "fake.signature", "", "", true, 403},
		{"cross origin", cookie.Value, "http://evil.test", "", true, 403},
		{"metadata", cookie.Value, "", "cross-site", true, 403},
		{"bearer has no ambient authority", "", "", "", false, 204},
		{"bearer cross origin rejected", "", "http://evil.test", "", false, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://example.test/api/auth/refresh", nil)
			r.AddCookie(cookie)
			if tc.cookieSession {
				r.Header.Set("X-Defta-Session", "cookie")
			}
			r.Header.Set("X-Defta-CSRF", tc.token)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
	// Copying the same attacker-chosen value to cookie and header must not work.
	r := httptest.NewRequest("POST", "http://example.test/api/auth/login", nil)
	r.AddCookie(&http.Cookie{Name: "defta_csrf", Value: "fake.signature"})
	r.Header.Set("X-Defta-Session", "cookie")
	r.Header.Set("X-Defta-CSRF", "fake.signature")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("unsigned cookie accepted")
	}
}

func TestStaticFilesRejectDirectoriesAndUnintendedAssets(t *testing.T) {
	h := StaticFiles(fstest.MapFS{"js/app.js": {Data: []byte("ok")}, "js/app.js.map": {Data: []byte("source")}, ".env": {Data: []byte("secret")}, "config.json": {Data: []byte("secret")}})
	for _, tc := range []struct {
		path string
		want int
	}{{"/js/app.js", 200}, {"/", 404}, {"/js/", 404}, {"/.env", 404}, {"/js/app.js.map", 404}, {"/config.json", 404}, {"/../.env", 404}} {
		r := httptest.NewRequest("GET", tc.path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s: %d", tc.path, w.Code)
		}
	}
}

func TestProductionHTTPSRejectsSpoofedProxyAndUsesCanonicalOrigin(t *testing.T) {
	h := ProductionHTTPS("https://books.example", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	for _, tc := range []struct {
		remote, proto string
		want          int
	}{{"203.0.113.1:555", "https", 308}, {"127.0.0.1:555", "", 308}, {"127.0.0.1:555", "https", 204}} {
		r := httptest.NewRequest("GET", "http://attacker.example/admin?q=test", nil)
		r.RemoteAddr = tc.remote
		r.Header.Set("X-Forwarded-Proto", tc.proto)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatal(w.Code)
		}
		if tc.want == 308 && w.Header().Get("Location") != "https://books.example/admin?q=test" {
			t.Fatal(w.Header())
		}
		if tc.want == 204 && !strings.Contains(w.Header().Get("Strict-Transport-Security"), "includeSubDomains") {
			t.Fatal(w.Header())
		}
	}
}

func TestCSPHasNoInlineOrRemoteFontExceptions(t *testing.T) {
	w := httptest.NewRecorder()
	SecureHTTP(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	policy := w.Header().Get("Content-Security-Policy")
	if strings.Contains(policy, "unsafe-inline") || strings.Contains(policy, "google") || !strings.Contains(policy, "form-action 'self'") {
		t.Fatal(policy)
	}
}
