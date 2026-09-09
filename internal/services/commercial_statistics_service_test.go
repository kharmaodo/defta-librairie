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

type statisticsStub struct {
	calls   int
	library string
}

func (r *statisticsStub) Summary(_ context.Context, library string, _, _ time.Time) (repositories.CommercialStatistics, error) {
	r.calls++
	r.library = library
	return repositories.CommercialStatistics{}, nil
}

func TestCommercialStatisticsAuthorizationAndDates(t *testing.T) {
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "own"}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	for _, tc := range []struct {
		name                     string
		claims                   *auth.Claims
		library, from, to, scope string
		want                     error
	}{
		{"owner", owner, "", "2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z", "own", nil},
		{"cross library", owner, "other", "2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z", "", ErrBookForbidden},
		{"root", root, "other", "2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z", "other", nil},
		{"root scope missing", root, "", "2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z", "", ErrInvalidStatistics},
		{"anonymous", nil, "", "", "", "", ErrBookForbidden},
		{"missing dates", owner, "", "", "", "", ErrInvalidStatistics},
		{"reversed", owner, "", "2026-09-02T00:00:00Z", "2026-09-01T00:00:00Z", "", ErrInvalidStatistics},
		{"equal", owner, "", "2026-09-01T00:00:00Z", "2026-09-01T00:00:00Z", "", ErrInvalidStatistics},
		{"invalid role", &auth.Claims{}, "", "", "", "", ErrBookForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &statisticsStub{}
			_, err := NewCommercialStatisticsService(r).Summary(context.Background(), tc.claims, tc.library, tc.from, tc.to)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
			if tc.want != nil && r.calls != 0 {
				t.Fatal("unauthorized or invalid request reached repository")
			}
			if tc.want == nil && (r.calls != 1 || r.library != tc.scope) {
				t.Fatalf("scope=%s calls=%d", r.library, r.calls)
			}
		})
	}
}
