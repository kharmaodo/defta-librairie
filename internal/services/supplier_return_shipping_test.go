package services

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"

	_ "github.com/mattn/go-sqlite3"
)

func TestSupplierReturnShipInsufficientStockRollsBack(t *testing.T) {
	for _, multiLine := range []bool{false, true} {
		name := "single_line"
		if multiLine { name = "rollback_after_first_stock_exit" }
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "shipping.db")+"?_foreign_keys=on")
			if err != nil { t.Fatal(err) }
			t.Cleanup(func() { _ = db.Close() })
			db.SetMaxOpenConns(1)
			if err = migrations.Run(ctx, db); err != nil { t.Fatalf("migrate: %v", err) }
			_, err = db.Exec(`
				INSERT INTO users(id,username,password_hash,role,created_at,updated_at)
				VALUES('ship-owner','ship-owner','unused-test-hash','OWNER_LIBRARY','before','before');
				INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at)
				VALUES('ship-library','Shipping test','ship-owner','before','before');
				INSERT INTO defta(id,title,library_id) VALUES
				(1,'First book','ship-library'),(2,'Insufficient book','ship-library');
				INSERT INTO book_inventory(book_id,library_id,quantity,version,updated_at)
				VALUES(1,'ship-library',5,1,'before'),(2,'ship-library',1,1,'before');
				INSERT INTO suppliers(id,library_id,name,normalized_name,created_by,created_at,updated_at)
				VALUES('ship-supplier','ship-library','Shipping supplier','shipping supplier','ship-owner','before','before');
				INSERT INTO purchases(id,library_id,supplier_id,reference,status,total_amount,created_by,created_at,updated_at)
				VALUES('ship-purchase','ship-library','ship-supplier','P-SHIP','RECEIVED',10000,'ship-owner','before','before');
				INSERT INTO purchase_lines(id,purchase_id,book_id,title_snapshot,quantity,unit_cost,line_total,created_at)
				VALUES('ship-line-1','ship-purchase',1,'First book',5,1000,5000,'before'),
				('ship-line-2','ship-purchase',2,'Insufficient book',5,1000,5000,'before');
			`)
			if err != nil { t.Fatalf("fixture: %v", err) }
			service := NewSupplierReturnService(repositories.NewSupplierReturnRepository(db))
			owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "ship-library"}
			owner.Subject = "ship-owner"
			lines := []models.SupplierReturnLineInput{{PurchaseLineID: "ship-line-2", Quantity: 2}}
			if multiLine { lines = append(lines, models.SupplierReturnLineInput{PurchaseLineID: "ship-line-1", Quantity: 2}) }
			value, err := service.Create(ctx, owner, models.SupplierReturnInput{
				PurchaseID: "ship-purchase", Reason: "Stock rollback test", Lines: lines,
			})
			if err != nil { t.Fatalf("create draft: %v", err) }
			var auditsBefore int
			if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&auditsBefore); err != nil { t.Fatal(err) }
			_, err = service.Ship(ctx, owner, value.ID, value.Version)
			if !errors.Is(err, repositories.ErrSupplierReturnStock) { t.Fatalf("expected insufficient stock, got %v", err) }
			after, err := service.Find(ctx, owner, value.ID)
			if err != nil { t.Fatal(err) }
			if after.Status != models.SupplierReturnStatusDraft || after.Version != value.Version || after.UpdatedAt != value.UpdatedAt || after.ShippedAt != "" || after.ShippedBy != "" || len(after.Lines) != len(lines) {
				t.Fatalf("draft changed after failure: %+v", after)
			}
			for book, expected := range map[int]int{1: 5, 2: 1} {
				var quantity, version int
				var updated string
				if err = db.QueryRow(`SELECT quantity,version,updated_at FROM book_inventory WHERE book_id=?`, book).Scan(&quantity,&version,&updated); err != nil { t.Fatal(err) }
				if quantity != expected || version != 1 || updated != "before" { t.Fatalf("book %d mutated: quantity=%d version=%d updated=%s",book,quantity,version,updated) }
			}
			var movements, audits int
			if err = db.QueryRow(`SELECT COUNT(*) FROM inventory_movements`).Scan(&movements); err != nil { t.Fatal(err) }
			if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&audits); err != nil { t.Fatal(err) }
			if movements != 0 || audits != auditsBefore { t.Fatalf("partial writes: movements=%d audits=%d expected=%d",movements,audits,auditsBefore) }
		})
	}
}
