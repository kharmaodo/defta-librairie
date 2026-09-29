package services

import (
	"context"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
)

type coverImportHoldStore interface {
	SetLegalHold(context.Context, string, string, string, time.Time, time.Time) error
	ReleaseLegalHold(context.Context, string, string, time.Time) error
}

type CoverImportRetentionService struct {
	store coverImportHoldStore
	now   func() time.Time
}

func NewCoverImportRetentionService(store coverImportHoldStore) *CoverImportRetentionService {
	return &CoverImportRetentionService{store: store, now: time.Now}
}

func (s *CoverImportRetentionService) Hold(ctx context.Context, claims *auth.Claims, jobID, reason string, expiresAt time.Time) error {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot || claims.Subject == "" {
		return ErrBookForbidden
	}
	if s == nil || s.store == nil {
		return ErrCoversDisabled
	}
	if expiresAt.IsZero() {
		return repositories.ErrCoverImportHoldInvalid
	}
	return s.store.SetLegalHold(ctx, jobID, claims.Subject, reason, expiresAt, s.now().UTC())
}

func (s *CoverImportRetentionService) Release(ctx context.Context, claims *auth.Claims, jobID string) error {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot || claims.Subject == "" {
		return ErrBookForbidden
	}
	if s == nil || s.store == nil {
		return ErrCoversDisabled
	}
	return s.store.ReleaseLegalHold(ctx, jobID, claims.Subject, s.now().UTC())
}
