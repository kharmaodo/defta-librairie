package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"errors"
	"strings"
	"time"
)

var ErrInvalidCustomerHistory = errors.New("invalid customer history filters")

func (s *CustomerService) SaleHistory(ctx context.Context, claims *auth.Claims, id string, filter models.CustomerHistoryFilter, offset, limit int) ([]models.CustomerSaleSummary, int, error) {
	customer, err := s.Find(ctx, claims, id)
	if err != nil {
		return nil, 0, err
	}
	filter.Status = models.SaleStatus(strings.ToUpper(strings.TrimSpace(string(filter.Status))))
	if (filter.Status != "" && filter.Status != models.SaleStatusConfirmed && filter.Status != models.SaleStatusCancelled && filter.Status != models.SaleStatusDraft) || offset < 0 || limit < 1 || limit > 100 {
		return nil, 0, ErrInvalidCustomerHistory
	}
	var from, to time.Time
	for _, pair := range []struct {
		value  *string
		parsed *time.Time
	}{{&filter.From, &from}, {&filter.To, &to}} {
		*pair.value = strings.TrimSpace(*pair.value)
		if *pair.value == "" {
			continue
		}
		parsed, e := time.Parse(time.RFC3339Nano, *pair.value)
		if e != nil || parsed.IsZero() {
			return nil, 0, ErrInvalidCustomerHistory
		}
		*pair.parsed = parsed
		*pair.value = parsed.UTC().Format(time.RFC3339Nano)
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return nil, 0, ErrInvalidCustomerHistory
	}
	return s.repository.SaleHistory(ctx, customer.ID, customer.LibraryID, filter, offset, limit)
}
