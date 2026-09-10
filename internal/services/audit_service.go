package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"regexp"
	"strings"
	"time"
)

var ErrInvalidAuditFilter = errors.New("invalid audit filter")

var auditActionPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
var auditResourcePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,31}$`)

type AuditService struct{ repository *repositories.AuditRepository }

func NewAuditService(repository *repositories.AuditRepository) *AuditService {
	return &AuditService{repository: repository}
}

func (s *AuditService) List(ctx context.Context, claims *auth.Claims, filter models.AuditFilter, offset, limit int) ([]models.AuditLog, int, error) {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot && claims.Role != models.RoleOwnerLibrary {
		return nil, 0, ErrInvalidAuditFilter
	}
	filter.Action = strings.ToUpper(strings.TrimSpace(filter.Action))
	filter.ResourceType = strings.ToUpper(strings.TrimSpace(filter.ResourceType))
	filter.ResourceID = strings.TrimSpace(filter.ResourceID)
	filter.ActorUsername = strings.TrimSpace(filter.ActorUsername)
	filter.From = strings.TrimSpace(filter.From)
	filter.To = strings.TrimSpace(filter.To)
	if filter.Action != "" && !auditActionPattern.MatchString(filter.Action) {
		return nil, 0, ErrInvalidAuditFilter
	}
	if filter.ResourceType != "" && !auditResourcePattern.MatchString(filter.ResourceType) {
		return nil, 0, ErrInvalidAuditFilter
	}
	if len([]rune(filter.ResourceID)) > 100 || len([]rune(filter.ActorUsername)) > 64 {
		return nil, 0, ErrInvalidAuditFilter
	}
	var fromTime, toTime time.Time
	if filter.From != "" {
		parsed, err := time.Parse(time.RFC3339, filter.From)
		if err != nil {
			return nil, 0, ErrInvalidAuditFilter
		}
		fromTime = parsed
		filter.From = fromTime.UTC().Format(time.RFC3339Nano)
	}
	if filter.To != "" {
		parsed, err := time.Parse(time.RFC3339, filter.To)
		if err != nil {
			return nil, 0, ErrInvalidAuditFilter
		}
		toTime = parsed
		filter.To = toTime.UTC().Format(time.RFC3339Nano)
	}
	if !fromTime.IsZero() && !toTime.IsZero() && fromTime.After(toTime) {
		return nil, 0, ErrInvalidAuditFilter
	}
	actorID := ""
	if claims.Role == models.RoleOwnerLibrary {
		actorID = claims.Subject
		if filter.ActorUsername != "" {
			return nil, 0, ErrInvalidAuditFilter
		}
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
	return s.repository.List(ctx, actorID, filter, offset, limit)
}
