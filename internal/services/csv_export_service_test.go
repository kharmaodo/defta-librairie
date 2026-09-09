package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"path/filepath"
	"testing"
)

func TestCSVExportScopeAndFilters(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "exports.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialAcceptanceFixture + businessAlertFixture + customerHistoryFixture); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO audit_logs(id,actor_user_id,action,resource_type,success,created_at) VALUES('mine','accept-owner','EXPORT_TEST','BOOK',1,'2026-09-01T00:00:00Z'),('theirs','other-owner','EXPORT_TEST','BOOK',1,'2026-09-01T00:00:00Z');`); err != nil {
		t.Fatal(err)
	}
	s := NewCSVExportService(repositories.NewCSVExportRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "accept-library", RegisteredClaims: jwt.RegisteredClaims{Subject: "accept-owner"}}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot, RegisteredClaims: jwt.RegisteredClaims{Subject: "root"}}
	for _, tc := range []struct {
		name, kind string
		claims     *auth.Claims
		filter     repositories.CSVExportFilter
		count      int
		wantErr    error
	}{
		{"stocks", "stocks", owner, repositories.CSVExportFilter{}, 2, nil},
		{"stock status", "stocks", owner, repositories.CSVExportFilter{Status: "LOW_STOCK"}, 1, nil},
		{"sales", "sales", owner, repositories.CSVExportFilter{}, 5, nil},
		{"sales confirmed", "sales", owner, repositories.CSVExportFilter{Status: "CONFIRMED"}, 3, nil},
		{"purchases", "purchases", owner, repositories.CSVExportFilter{}, 3, nil},
		{"suppliers", "suppliers", owner, repositories.CSVExportFilter{}, 1, nil},
		{"literal search", "suppliers", owner, repositories.CSVExportFilter{Search: "%"}, 0, nil},
		{"owner audit", "audit", owner, repositories.CSVExportFilter{Action: "EXPORT_TEST"}, 1, nil},
		{"root audit", "audit", root, repositories.CSVExportFilter{Action: "EXPORT_TEST"}, 2, nil},
		{"root library", "suppliers", root, repositories.CSVExportFilter{LibraryID: "other"}, 1, nil},
		{"cross library", "stocks", owner, repositories.CSVExportFilter{LibraryID: "other"}, 0, ErrBookForbidden},
		{"root missing scope", "stocks", root, repositories.CSVExportFilter{}, 0, ErrInvalidBook},
		{"anonymous", "audit", nil, repositories.CSVExportFilter{}, 0, ErrBookForbidden},
		{"bad status", "sales", owner, repositories.CSVExportFilter{Status: "ACTIVE"}, 0, repositories.ErrInvalidExport},
		{"bad dates", "sales", owner, repositories.CSVExportFilter{From: "bad"}, 0, repositories.ErrInvalidExport},
		{"equal dates", "sales", owner, repositories.CSVExportFilter{From: "2026-09-01T00:00:00Z", To: "2026-09-01T00:00:00Z"}, 0, repositories.ErrInvalidExport},
		{"exclusive end", "sales", owner, repositories.CSVExportFilter{To: "2026-09-01T00:00:00Z"}, 0, nil},
		{"audit library rejected", "audit", root, repositories.CSVExportFilter{LibraryID: "other"}, 0, repositories.ErrInvalidExport},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Read(ctx, tc.claims, tc.kind, tc.filter)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if err == nil && (len(got.Rows) != tc.count || got.Rows == nil || len(got.Header) == 0) {
				t.Fatalf("rows=%d want=%d", len(got.Rows), tc.count)
			}
		})
	}
	if _, err = db.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10001) INSERT INTO audit_logs(id,actor_user_id,action,resource_type,success,created_at) SELECT 'bulk-'||x,'accept-owner','BULK','BOOK',1,'2026-09-01T00:00:00Z' FROM n`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(ctx, owner, "audit", repositories.CSVExportFilter{Action: "BULK"}); !errors.Is(err, repositories.ErrExportTooLarge) {
		t.Fatalf("limit: %v", err)
	}
	if _, err = db.Exec("DELETE FROM audit_logs WHERE id='bulk-10001'"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Read(ctx, owner, "audit", repositories.CSVExportFilter{Action: "BULK"}); err != nil || len(got.Rows) != 10000 {
		t.Fatalf("boundary rows=%d err=%v", len(got.Rows), err)
	}
}
