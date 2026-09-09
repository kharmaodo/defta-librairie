package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"testing"
)

func TestBusinessAlertsScopeAndCategories(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "alerts.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialAcceptanceFixture + businessAlertFixture); err != nil {
		t.Fatal(err)
	}
	s := NewBusinessAlertService(repositories.NewBusinessAlertRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "accept-library"}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	for _, tc := range []struct {
		name, library, kind  string
		claims               *auth.Claims
		offset, limit, total int
		wantErr              error
	}{
		{"all", "", "", owner, 0, 10, 4, nil},
		{"low", "", "LOW_STOCK", owner, 0, 10, 1, nil},
		{"out", "", "OUT_OF_STOCK", owner, 0, 10, 1, nil},
		{"draft", "", "DRAFT_PURCHASE", owner, 0, 10, 1, nil},
		{"disabled", "", "DISABLED_SUPPLIER", owner, 0, 10, 1, nil},
		{"pagination", "", "", owner, 1, 1, 4, nil},
		{"empty page", "", "", owner, 9, 10, 4, nil},
		{"empty library", "absent", "", root, 0, 10, 0, nil},
		{"cross library", "other", "", owner, 0, 10, 0, ErrBookForbidden},
		{"root explicit", "accept-library", "", root, 0, 10, 4, nil},
		{"root missing", "", "", root, 0, 10, 0, ErrInvalidBusinessAlert},
		{"anonymous", "", "", nil, 0, 10, 0, ErrBookForbidden},
		{"bad category", "", "SENT", owner, 0, 10, 0, ErrInvalidBusinessAlert},
		{"bad offset", "", "", owner, -1, 10, 0, ErrInvalidBusinessAlert},
		{"bad limit", "", "", owner, 0, 101, 0, ErrInvalidBusinessAlert},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.List(ctx, tc.claims, tc.library, tc.kind, tc.offset, tc.limit)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v want=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			count := tc.total - tc.offset
			if count < 0 {
				count = 0
			}
			if count > tc.limit {
				count = tc.limit
			}
			if got.Total != tc.total || got.Results == nil || len(got.Results) != count {
				t.Fatalf("result=%+v", got)
			}
			if tc.name == "all" {
				for i, k := range []string{"OUT_OF_STOCK", "LOW_STOCK", "DRAFT_PURCHASE", "DISABLED_SUPPLIER"} {
					if got.Results[i].Kind != k {
						t.Fatalf("order=%+v", got.Results)
					}
				}
			}
			if tc.name == "pagination" && got.Results[0].Kind != "LOW_STOCK" {
				t.Fatalf("page=%+v", got)
			}
		})
	}
}

const businessAlertFixture = `
UPDATE book_inventory SET quantity=0 WHERE book_id=1;
INSERT INTO defta(id,title,price,library_id) VALUES(2,'Low',100,'accept-library'),(3,'Deleted',100,'accept-library');
INSERT INTO book_inventory(book_id,library_id,quantity,low_stock_threshold,updated_at) VALUES(2,'accept-library',2,2,'now'),(3,'accept-library',0,2,'now');
UPDATE defta SET deleted_at='2026-09-01T00:00:00Z' WHERE id=3;
INSERT INTO suppliers(id,library_id,name,status,created_by,created_at,updated_at) VALUES('s','accept-library','Disabled supplier','DISABLED','accept-owner','now','now');
INSERT INTO purchases(id,library_id,supplier_id,reference,status,created_by,created_at,updated_at) VALUES('p','accept-library','s','Draft','DRAFT','accept-owner','now','now'),('done','accept-library','s','Received','RECEIVED','accept-owner','now','now'),('cancel','accept-library','s','Cancelled','CANCELLED','accept-owner','now','now');
INSERT INTO users(id,username,password_hash,role,created_at,updated_at) VALUES('other','other','unused','OWNER_LIBRARY','now','now');
INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at) VALUES('other','Other','other','now','now');
INSERT INTO suppliers(id,library_id,name,status,created_by,created_at,updated_at) VALUES('other','other','Other supplier','DISABLED','other','now','now');
`
