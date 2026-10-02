package middleware

import (
	"net"
	"net/http"
)

// ProductionHTTPS accepts forwarded HTTPS only from the loopback reverse
// proxy. Production configuration also binds the backend to loopback.
func ProductionHTTPS(origin string, next http.Handler) http.Handler {
	if origin == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		secure := r.TLS != nil || (err == nil && ip != nil && ip.IsLoopback() && r.Header.Get("X-Forwarded-Proto") == "https")
		if !secure {
			// #nosec G710 -- Canonical HTTPS origin validated at startup; request Host never controls destination.
			http.Redirect(w, r, origin+r.URL.RequestURI(), http.StatusPermanentRedirect)
			return
		}
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}
