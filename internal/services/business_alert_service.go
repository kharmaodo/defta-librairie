package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"errors"
	"strings"
)

var ErrInvalidBusinessAlert = errors.New("invalid business alert filters")

type businessAlertReader interface {
	List(context.Context, string, string, int, int) (repositories.BusinessAlerts, error)
}
type BusinessAlertService struct{ repository businessAlertReader }

func NewBusinessAlertService(r businessAlertReader) *BusinessAlertService {
	return &BusinessAlertService{repository: r}
}
func (s *BusinessAlertService) List(ctx context.Context, claims *auth.Claims, library, kind string, offset, limit int) (repositories.BusinessAlerts, error) {
	var empty repositories.BusinessAlerts
	if claims == nil {
		return empty, ErrBookForbidden
	}
	scope, err := resolveBookScope(claims, strings.TrimSpace(library), true)
	if errors.Is(err, ErrInvalidBook) {
		return empty, ErrInvalidBusinessAlert
	}
	if err != nil {
		return empty, err
	}
	switch kind {
	case "", "OUT_OF_STOCK", "LOW_STOCK", "DRAFT_PURCHASE", "DISABLED_SUPPLIER":
	default:
		return empty, ErrInvalidBusinessAlert
	}
	if offset < 0 || limit < 1 || limit > 100 {
		return empty, ErrInvalidBusinessAlert
	}
	return s.repository.List(ctx, scope, kind, offset, limit)
}
