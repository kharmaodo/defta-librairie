package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"strings"
	"time"
)

var ErrInvalidStatistics = errors.New("valid RFC3339 from/to and explicit root libraryId required")

type statisticsReader interface {
	Summary(context.Context, string, time.Time, time.Time) (repositories.CommercialStatistics, error)
}

type CommercialStatisticsService struct{ repository statisticsReader }

func NewCommercialStatisticsService(repository statisticsReader) *CommercialStatisticsService {
	return &CommercialStatisticsService{repository: repository}
}

func (s *CommercialStatisticsService) Summary(ctx context.Context, claims *auth.Claims, library, from, to string) (repositories.CommercialStatistics, error) {
	var empty repositories.CommercialStatistics
	if claims == nil {
		return empty, ErrBookForbidden
	}
	library = strings.TrimSpace(library)
	if claims.Role == models.RoleSuperAdminRoot && library == "" {
		return empty, ErrInvalidStatistics
	}
	scope, err := resolveBookScope(claims, library, true)
	if err != nil {
		return empty, err
	}
	start, startErr := time.Parse(time.RFC3339Nano, from)
	end, endErr := time.Parse(time.RFC3339Nano, to)
	if startErr != nil || endErr != nil || start.IsZero() || end.IsZero() || !start.Before(end) {
		return empty, ErrInvalidStatistics
	}
	return s.repository.Summary(ctx, scope, start, end)
}
