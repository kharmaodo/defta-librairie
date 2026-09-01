package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"testing"
	"time"
)

type loginStoreStub struct {
	user models.User
	findErr error
	failed int
	succeeded int
	unknown int
}

func (s *loginStoreStub) FindByUsername(context.Context, string) (models.User, error) { return s.user, s.findErr }
func (s *loginStoreStub) RecordFailedLogin(context.Context, string, string, string, string, string) error { s.failed++; return nil }
func (s *loginStoreStub) RecordSuccessfulLogin(context.Context, string, string, string, string) error { s.succeeded++; return nil }
func (s *loginStoreStub) RecordUnknownLogin(context.Context, string, string, string) error { s.unknown++; return nil }

func TestLoginServiceSuccessAndInvalidPassword(t *testing.T) {
	hash, err := auth.HashPassword("Correct-Horse-2026")
	if err != nil { t.Fatalf("hash password: %v", err) }
	store := &loginStoreStub{user: models.User{
		ID: "user-1", Username: "owner", PasswordHash: hash,
		Role: models.RoleOwnerLibrary, Status: models.UserStatusActive, LibraryID: "library-1",
	}}
	tokens, _ := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	service, err := NewLoginService(store, tokens)
	if err != nil { t.Fatalf("new login service: %v", err) }

	result, err := service.Login(context.Background(), "owner", "Correct-Horse-2026", "127.0.0.1")
	if err != nil || result.AccessToken == "" || store.succeeded != 1 {
		t.Fatalf("successful login: token=%v succeeded=%d err=%v", result.AccessToken != "", store.succeeded, err)
	}
	_, err = service.Login(context.Background(), "owner", "Wrong-Password-2026", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) || store.failed != 1 {
		t.Fatalf("invalid login: failed=%d err=%v", store.failed, err)
	}
}

func TestLoginServiceUnknownUserUsesGenericError(t *testing.T) {
	store := &loginStoreStub{findErr: repositories.ErrUserNotFound}
	tokens, _ := auth.NewTokenManager("0123456789abcdef0123456789abcdef", "issuer", "audience", time.Minute)
	service, _ := NewLoginService(store, tokens)
	_, err := service.Login(context.Background(), "missing", "Any-Password-2026", "127.0.0.1")
	if !errors.Is(err, ErrInvalidCredentials) || store.unknown != 1 {
		t.Fatalf("unknown login: audits=%d err=%v", store.unknown, err)
	}
}
