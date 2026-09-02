package middleware

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type sessionValidatorStub struct {
	active bool
	err    error
}

func (s sessionValidatorStub) IsActive(context.Context, string, string, models.UserRole, string, time.Time) (bool, error) {
	return s.active, s.err
}

func TestAuthenticate(t *testing.T) {
	tokens, err := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", 15*time.Minute)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	raw, _, err := tokens.Issue(models.User{ID: "root-id", Role: models.RoleSuperAdminRoot})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	protected := Authenticate(tokens, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims.Subject != "root-id" {
			t.Fatal("claims missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}

func TestAuthenticateSessionRejectsRevokedSession(t *testing.T) {
	tokens, _ := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	raw, _, err := tokens.IssueForSession(models.User{ID: "root-id", Role: models.RoleSuperAdminRoot}, "session-id")
	if err != nil {
		t.Fatalf("issue session token: %v", err)
	}
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("revoked session must not reach handler")
	})
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	AuthenticateSession(tokens, sessionValidatorStub{active: false}, next).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestAuthenticateSessionAllowsActiveSession(t *testing.T) {
	tokens, _ := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	raw, _, _ := tokens.IssueForSession(models.User{ID: "root-id", Role: models.RoleSuperAdminRoot}, "session-id")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	AuthenticateSession(tokens, sessionValidatorStub{active: true}, next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
}

func TestAuthenticateRejectsMissingAndMalformedTokens(t *testing.T) {
	tokens, _ := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	protected := Authenticate(tokens, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler must not be called")
	}))
	for _, header := range []string{"", "Basic value", "Bearer invalid"} {
		request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		request.Header.Set("Authorization", header)
		response := httptest.NewRecorder()
		protected.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("header %q: expected 401, got %d", header, response.Code)
		}
	}
}
