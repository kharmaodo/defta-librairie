package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"regexp"
	"strings"
)

var ErrInvalidAuditFilter = errors.New("invalid audit filter")

var auditActionPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

type AuditService struct{ repository *repositories.AuditRepository }

func NewAuditService(repository *repositories.AuditRepository) *AuditService {
	return &AuditService{repository: repository}
}

func (s *AuditService) List(ctx context.Context, claims *auth.Claims, action string, success *bool, offset, limit int) ([]models.AuditLog, int, error) {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot && claims.Role != models.RoleOwnerLibrary {
		return nil, 0, ErrInvalidAuditFilter
	}
	action = strings.ToUpper(strings.TrimSpace(action))
	if action != "" && !auditActionPattern.MatchString(action) {
		return nil, 0, ErrInvalidAuditFilter
	}
	actorID := ""
	if claims.Role == models.RoleOwnerLibrary {
		actorID = claims.Subject
	}
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	return s.repository.List(ctx, actorID, action, success, offset, limit)
}
