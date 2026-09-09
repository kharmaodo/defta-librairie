package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"strings"
	"time"
)

type csvExportReader interface {
	Read(context.Context, string, repositories.CSVExportFilter) (repositories.CSVExportTable, error)
}
type CSVExportService struct{ repository csvExportReader }

func NewCSVExportService(r csvExportReader) *CSVExportService {
	return &CSVExportService{repository: r}
}
func (s *CSVExportService) Read(ctx context.Context, c *auth.Claims, kind string, f repositories.CSVExportFilter) (repositories.CSVExportTable, error) {
	var empty repositories.CSVExportTable
	if c == nil || (c.Role != models.RoleSuperAdminRoot && c.Role != models.RoleOwnerLibrary) {
		return empty, ErrBookForbidden
	}
	f.ActorID = ""
	if kind == "audit" {
		if f.LibraryID != "" || f.Status != "" {
			return empty, repositories.ErrInvalidExport
		}
		if c.Role == models.RoleOwnerLibrary {
			if c.Subject == "" {
				return empty, ErrBookForbidden
			}
			f.ActorID = c.Subject
		}
	} else {
		scope, err := resolveBookScope(c, strings.TrimSpace(f.LibraryID), true)
		if err != nil {
			return empty, err
		}
		f.LibraryID = scope
		if f.Action != "" || f.ResourceType != "" {
			return empty, repositories.ErrInvalidExport
		}
	}
	allowed := map[string]string{"stocks": "|OUT_OF_STOCK|LOW_STOCK|IN_STOCK|", "sales": "|DRAFT|CONFIRMED|CANCELLED|", "purchases": "|DRAFT|RECEIVED|CANCELLED|", "suppliers": "|ACTIVE|DISABLED|", "audit": "|"}
	statuses, ok := allowed[kind]
	if !ok {
		return empty, repositories.ErrInvalidExport
	}
	if f.Status != "" && !strings.Contains(statuses, "|"+f.Status+"|") {
		return empty, repositories.ErrInvalidExport
	}
	if len(f.Search) > 256 || len(f.Action) > 64 || len(f.ResourceType) > 32 {
		return empty, repositories.ErrInvalidExport
	}
	var from, to time.Time
	for _, p := range []struct {
		value  *string
		parsed *time.Time
	}{{&f.From, &from}, {&f.To, &to}} {
		if *p.value != "" {
			v, err := time.Parse(time.RFC3339Nano, *p.value)
			if err != nil || v.IsZero() {
				return empty, repositories.ErrInvalidExport
			}
			*p.parsed = v
			*p.value = v.UTC().Format(time.RFC3339Nano)
		}
	}
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return empty, repositories.ErrInvalidExport
	}
	return s.repository.Read(ctx, kind, f)
}
