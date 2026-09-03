package auth

import (
	"defta-librairie/internal/models"
	"errors"
	"testing"
	"time"
)

func TestTokenManagerIssueAndParse(t *testing.T) {
	manager, err := NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", 15*time.Minute)
	if err != nil {
		t.Fatalf("new token manager: %v", err)
	}
	manager.now = func() time.Time { return time.Unix(1700000000, 0) }

	raw, _, err := manager.IssueForSession(models.User{ID: "user-1", Role: models.RoleOwnerLibrary, LibraryID: "library-1", MustChangePassword: true}, "session-1")
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := manager.Parse(raw)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-1" || claims.Role != models.RoleOwnerLibrary || claims.LibraryID != "library-1" || claims.SessionID != "session-1" || !claims.PasswordChangeRequired {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestTokenManagerRejectsWrongAudienceAndWeakSecret(t *testing.T) {
	if _, err := NewTokenManager("short", "issuer", "audience", time.Minute); err == nil {
		t.Fatal("expected weak secret error")
	}
	issuer, _ := NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	parser, _ := NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "other", time.Minute)
	raw, _, _ := issuer.Issue(models.User{ID: "user-1", Role: models.RoleSuperAdminRoot})
	if _, err := parser.Parse(raw); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
