package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"github.com/golang-jwt/jwt/v5"
)

type fakeHoldStore struct{ calls int }

func (f *fakeHoldStore) SetLegalHold(context.Context, string, string, string, time.Time, time.Time) error {
	f.calls++
	return nil
}
func (f *fakeHoldStore) ReleaseLegalHold(context.Context, string, string, time.Time) error {
	f.calls++
	return nil
}

func TestCoverImportLegalHoldRequiresRoot(t *testing.T) {
	store := &fakeHoldStore{}
	service := NewCoverImportRetentionService(store)
	expires := time.Now().Add(time.Hour)
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, RegisteredClaims: jwt.RegisteredClaims{Subject: "owner"}}
	if err := service.Hold(context.Background(), owner, "job", "reason", expires); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("owner hold: %v", err)
	}
	if err := service.Release(context.Background(), owner, "job"); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("owner release: %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("owner reached store: %d", store.calls)
	}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	if err := service.Hold(context.Background(), root, "job", "reason", expires); err != nil {
		t.Fatal(err)
	}
	if err := service.Release(context.Background(), root, "job"); err != nil {
		t.Fatal(err)
	}
	if store.calls != 2 {
		t.Fatalf("root store calls: %d", store.calls)
	}
}
