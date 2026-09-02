package middleware

import (
	"defta-librairie/internal/identity"
	"net/http"
)

func SecureHTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if requestID, err := identity.NewID(); err == nil {
			w.Header().Set("X-Request-ID", requestID)
		}
		if len(r.URL.Path) >= len("/api/auth/") && r.URL.Path[:len("/api/auth/")] == "/api/auth/" {
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}
