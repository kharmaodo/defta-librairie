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

func TestCommercialCyclePreventsDoubleRestock(t *testing.T) {
	for _, completeFirst := range []bool{true, false} {
		name := "cancel_then_complete"
		if completeFirst {
			name = "complete_then_cancel"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cycle.db")+"?_foreign_keys=on")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			if err = migrations.Run(ctx, db); err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(commercialCycleFixture); err != nil {
				t.Fatal(err)
			}
			owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "cycle-library"}
			owner.Subject = "cycle-owner"
			sales := NewSaleService(repositories.NewSaleRepository(db))
			returns := NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))
			if completeFirst {
				if _, err = returns.Complete(ctx, owner, "cycle-return", 1); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err = sales.Cancel(ctx, owner, "cycle-sale", 1); err != nil {
					t.Fatal(err)
				}
			}
			before := readCycleState(t, db)
			if completeFirst {
				_, err = sales.Cancel(ctx, owner, "cycle-sale", 1)
				if !errors.Is(err, repositories.ErrSaleCompletedReturns) {
					t.Fatalf("expected completed return conflict, got %v", err)
				}
				if before.quantity != 4 {
					t.Fatalf("return stock=%d", before.quantity)
				}
			} else {
				_, err = returns.Complete(ctx, owner, "cycle-return", 1)
				if !errors.Is(err, repositories.ErrCustomerReturnSale) {
					t.Fatalf("expected unavailable sale, got %v", err)
				}
				if before.quantity != 5 {
					t.Fatalf("cancelled stock=%d", before.quantity)
				}
			}
			if after := readCycleState(t, db); after != before {
				t.Fatalf("failed transition changed state: before=%+v after=%+v", before, after)
			}
			if !completeFirst {
				// The now unusable draft can still be cancelled without another restock.
				if _, err = returns.Cancel(ctx, owner, "cycle-return", 1); err != nil {
					t.Fatal(err)
				}
				if after := readCycleState(t, db); after.quantity != 5 || after.movements != before.movements {
					t.Fatal("draft cancellation changed stock")
				}
			}
		})
	}
}

type cycleState struct {
	quantity, inventoryVersion, saleVersion, returnVersion, movements, audits int
	cost                                                                      sql.NullFloat64
	saleStatus, returnStatus, inventoryUpdated, saleUpdated, returnUpdated    string
}

func readCycleState(t *testing.T, db *sql.DB) cycleState {
	t.Helper()
	var s cycleState
	err := db.QueryRow(`SELECT i.quantity,i.version,i.average_unit_cost,s.status,s.version,r.status,r.version,
        i.updated_at,s.updated_at,r.updated_at,
        (SELECT COUNT(*) FROM inventory_movements),(SELECT COUNT(*) FROM audit_logs)
        FROM book_inventory i CROSS JOIN sales s CROSS JOIN customer_returns r
        WHERE i.book_id=1 AND s.id='cycle-sale' AND r.id='cycle-return'`).Scan(
		&s.quantity, &s.inventoryVersion, &s.cost, &s.saleStatus, &s.saleVersion, &s.returnStatus, &s.returnVersion,
		&s.inventoryUpdated, &s.saleUpdated, &s.returnUpdated, &s.movements, &s.audits)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

const commercialCycleFixture = `
INSERT INTO users(id,username,password_hash,role,created_at,updated_at)
VALUES('cycle-owner','cycle-owner','unused','OWNER_LIBRARY','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at)
VALUES('cycle-library','Cycle','cycle-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO defta(id,title,library_id) VALUES(1,'Cycle book','cycle-library');
INSERT INTO book_inventory(book_id,library_id,quantity,average_unit_cost,version,updated_at)
VALUES(1,'cycle-library',3,100,1,'2026-09-01T00:00:00Z');
INSERT INTO sales(id,library_id,reference,status,total_amount,created_by,created_at,updated_at,confirmed_at)
VALUES('cycle-sale','cycle-library','V-CYCLE','CONFIRMED',400,'cycle-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO sale_lines(id,sale_id,book_id,title_snapshot,quantity,unit_price,line_total,created_at,unit_cost_snapshot)
VALUES('cycle-line','cycle-sale',1,'Cycle book',2,200,400,'2026-09-01T00:00:00Z',100);
INSERT INTO customer_returns(id,library_id,sale_id,reference,reason,resolution,created_by,created_at,updated_at)
VALUES('cycle-return','cycle-library','cycle-sale','RC-CYCLE','Test return','REFUND','cycle-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO customer_return_lines(id,return_id,sale_line_id,book_id,quantity,unit_price,line_total,created_at)
VALUES('cycle-return-line','cycle-return','cycle-line',1,1,200,200,'2026-09-01T00:00:00Z');
`
