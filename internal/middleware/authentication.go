package middleware

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type AccessSessionValidator interface {
	IsActive(context.Context, string, string, models.UserRole, string, time.Time) (bool, error)
}

func Authenticate(tokens *auth.TokenManager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeUnauthorized(w)
			return
		}
		claims, err := tokens.Parse(parts[1])
		if err != nil {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.ContextWithClaims(r.Context(), claims)))
	})
}

func AuthenticateSession(tokens *auth.TokenManager, sessions AccessSessionValidator, next http.Handler) http.Handler {
	return Authenticate(tokens, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims.SessionID == "" {
			writeUnauthorized(w)
			return
		}
		active, err := sessions.IsActive(r.Context(), claims.SessionID, claims.Subject,
			claims.Role, claims.LibraryID, time.Now().UTC())
		if err != nil {
			writeSessionUnavailable(w)
			return
		}
		if !active {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", `Bearer realm="defta-librairie"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": "Authentication required"})
}

func writeSessionUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "session_validation_failed", "message": "Session validation unavailable"})
}
