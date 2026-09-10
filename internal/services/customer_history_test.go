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
	"reflect"
	"testing"
)

func TestCustomerHistoryScopeFiltersAndPagination(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "history.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialAcceptanceFixture + customerHistoryFixture); err != nil {
		t.Fatal(err)
	}
	service := NewCustomerService(repositories.NewCustomerRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "accept-library"}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	for _, tc := range []struct {
		name, id             string
		claims               *auth.Claims
		filter               models.CustomerHistoryFilter
		offset, limit, total int
		want                 []string
		err                  error
	}{
		{"disabled customer", "history-client", owner, models.CustomerHistoryFilter{}, 0, 30, 3, []string{"c", "b", "a"}, nil},
		{"first page", "history-client", owner, models.CustomerHistoryFilter{}, 0, 1, 3, []string{"c"}, nil},
		{"stable second page", "history-client", owner, models.CustomerHistoryFilter{}, 1, 1, 3, []string{"b"}, nil},
		{"beyond page", "history-client", owner, models.CustomerHistoryFilter{}, 9, 1, 3, []string{}, nil},
		{"timezone and exclusive end", "history-client", owner, models.CustomerHistoryFilter{From: "2026-09-01T00:00:00Z", To: "2026-09-02T00:00:00Z"}, 0, 30, 1, []string{"a"}, nil},
		{"cancelled", "history-client", owner, models.CustomerHistoryFilter{Status: models.SaleStatusCancelled}, 0, 30, 1, []string{"b"}, nil},
		{"draft explicit", "history-client", owner, models.CustomerHistoryFilter{Status: models.SaleStatusDraft, From: "2026-09-03T00:00:00Z"}, 0, 30, 1, []string{"d"}, nil},
		{"empty client", "empty-client", owner, models.CustomerHistoryFilter{}, 0, 30, 0, []string{}, nil},
		{"other client forbidden", "other-client", owner, models.CustomerHistoryFilter{}, 0, 30, 0, nil, repositories.ErrCustomerNotFound},
		{"root other client", "other-client", root, models.CustomerHistoryFilter{}, 0, 30, 1, []string{"other"}, nil},
		{"missing", "missing", owner, models.CustomerHistoryFilter{}, 0, 30, 0, nil, repositories.ErrCustomerNotFound},
		{"anonymous", "history-client", nil, models.CustomerHistoryFilter{}, 0, 30, 0, nil, ErrInvalidBook},
		{"invalid date", "history-client", owner, models.CustomerHistoryFilter{From: "bad"}, 0, 30, 0, nil, ErrInvalidCustomerHistory},
		{"inverted interval", "history-client", owner, models.CustomerHistoryFilter{From: "2026-09-03T00:00:00Z", To: "2026-09-01T00:00:00Z"}, 0, 30, 0, nil, ErrInvalidCustomerHistory},
		{"invalid status", "history-client", owner, models.CustomerHistoryFilter{Status: "PAID"}, 0, 30, 0, nil, ErrInvalidCustomerHistory},
		{"negative offset", "history-client", owner, models.CustomerHistoryFilter{}, -1, 30, 0, nil, ErrInvalidCustomerHistory},
		{"excess limit", "history-client", owner, models.CustomerHistoryFilter{}, 0, 101, 0, nil, ErrInvalidCustomerHistory},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := service.SaleHistory(ctx, tc.claims, tc.id, tc.filter, tc.offset, tc.limit)
			if !errors.Is(err, tc.err) {
				t.Fatalf("err=%v want=%v", err, tc.err)
			}
			if tc.err != nil {
				return
			}
			ids := []string{}
			for _, row := range rows {
				ids = append(ids, row.ID)
			}
			if total != tc.total || !reflect.DeepEqual(ids, tc.want) || rows == nil {
				t.Fatalf("ids=%v total=%d want=%v/%d", ids, total, tc.want, tc.total)
			}
		})
	}
}

const customerHistoryFixture = `
INSERT INTO users(id,username,password_hash,role,created_at,updated_at) VALUES('other-owner','other-owner','unused','OWNER_LIBRARY','now','now');
INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at) VALUES('other-library','Other','other-owner','now','now');
INSERT INTO customers(id,library_id,reference,name,created_by,created_at,updated_at) VALUES
 ('history-client','accept-library','C-H','Same name','accept-owner','now','now'),
 ('empty-client','accept-library','C-E','Same name','accept-owner','now','now'),
 ('other-client','other-library','C-O','Other client','other-owner','now','now');
INSERT INTO sales(id,library_id,customer_id,customer_name,reference,status,total_amount,created_by,created_at,updated_at,confirmed_at,cancelled_at) VALUES
 ('a','accept-library','history-client','Same name','A','CONFIRMED',100,'accept-owner','2026-09-01T00:00:00Z','now','2026-09-02T01:00:00+02:00',NULL),
 ('b','accept-library','history-client','Same name','B','CANCELLED',200,'accept-owner','2026-09-01T00:00:00Z','now','2026-09-02T00:00:00Z','2026-09-03T00:00:00Z'),
 ('c','accept-library','history-client','Same name','C','CONFIRMED',300,'accept-owner','2026-09-01T00:00:00Z','now','2026-09-02T00:00:00Z',NULL),
 ('d','accept-library','history-client','Same name','D','DRAFT',400,'accept-owner','2026-09-03T00:00:00Z','now',NULL,NULL),
 ('guest','accept-library',NULL,'Same name','G','CONFIRMED',999,'accept-owner','now','now','2026-09-04T00:00:00Z',NULL),
 ('other','other-library','other-client','Other client','O','CONFIRMED',500,'other-owner','now','now','2026-09-01T00:00:00Z',NULL);
UPDATE customers SET status='DISABLED' WHERE id='history-client';
`
