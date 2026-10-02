package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
)

// CSRF protects browser cookie session endpoints. Other write APIs require an
// explicit bearer token and never authenticate using ambient cookies.
// A signed, host-only random double-submit cookie is reinforced with same-origin and
// Fetch Metadata checks. It grants no authentication or business permission.
func CSRF(secure bool, next http.Handler, signingKey []byte) http.Handler {
	valid := func(token string) bool {
		parts := strings.Split(token, ".")
		if len(parts) != 2 || len(parts[0]) != 43 {
			return false
		}
		mac := hmac.New(sha256.New, signingKey)
		mac.Write([]byte(parts[0]))
		expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		return subtle.ConstantTimeCompare([]byte(expected), []byte(parts[1])) == 1
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := "defta_csrf"
		if secure {
			name = "__Host-defta_csrf"
		}
		safe := r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions
		if !safe {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				csrfError(w)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				expectedScheme := "http"
				if secure || r.TLS != nil {
					expectedScheme = "https"
				}
				parsed, err := url.Parse(origin)
				if err != nil || parsed.User != nil || parsed.Host != r.Host || parsed.Scheme != expectedScheme || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
					csrfError(w)
					return
				}
			}
			if strings.EqualFold(r.Header.Get("X-Defta-Session"), "cookie") {
				cookie, err := r.Cookie(name)
				token := r.Header.Get("X-Defta-CSRF")
				if err != nil || !valid(token) || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(token)) != 1 {
					csrfError(w)
					return
				}
			}
		}
		if safe && (r.URL.Path == "/login" || r.URL.Path == "/admin" || r.URL.Path == "/admin/cover-imports") {
			cookie, err := r.Cookie(name)
			if err != nil || !valid(cookie.Value) {
				random := make([]byte, 32)
				if _, err := rand.Read(random); err != nil {
					http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
					return
				}
				nonce := base64.RawURLEncoding.EncodeToString(random)
				mac := hmac.New(sha256.New, signingKey)
				mac.Write([]byte(nonce))
				token := nonce + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
				// #nosec G124 -- Production uses Secure and __Host prefix; CSRF cookie intentionally readable for double-submit header, not an authentication cookie.
				http.SetCookie(w, &http.Cookie{Name: name, Value: token, Path: "/", Secure: secure, SameSite: http.SameSiteStrictMode})
			}
		}
		next.ServeHTTP(w, r)
	})
}

func csrfError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"csrf_failed","message":"Request origin or CSRF token rejected"}`))
}
