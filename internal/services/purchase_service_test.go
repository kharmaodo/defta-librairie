package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestPurchaseDraftLifecycleIsolationAndAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "purchases.db")+"?_foreign_keys=on")
	if err != nil { t.Fatalf("open database: %v", err) }
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT);
		CREATE TABLE defta(id INTEGER PRIMARY KEY,title TEXT,library_id TEXT,deleted_at TEXT);
		CREATE TABLE suppliers(id TEXT PRIMARY KEY,library_id TEXT,name TEXT,status TEXT,UNIQUE(id,library_id));
		CREATE TABLE purchases(id TEXT PRIMARY KEY,library_id TEXT,supplier_id TEXT,reference TEXT,status TEXT,
			total_amount REAL,version INTEGER,created_by TEXT,received_by TEXT,cancelled_by TEXT,created_at TEXT,
			updated_at TEXT,received_at TEXT,cancelled_at TEXT);
		CREATE TABLE purchase_lines(id TEXT PRIMARY KEY,purchase_id TEXT,book_id INTEGER,title_snapshot TEXT,
			quantity INTEGER,unit_cost REAL,line_total REAL,created_at TEXT);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES ('owner-1'),('owner-2');
		INSERT INTO libraries VALUES ('library-1','ACTIVE'),('library-2','ACTIVE');
		INSERT INTO defta VALUES (1,'Livre un','library-1',NULL),(2,'Livre deux','library-1',NULL),(3,'Livre tiers','library-2',NULL);
		INSERT INTO suppliers VALUES ('supplier-1','library-1','Fournisseur','ACTIVE'),
			('supplier-2','library-2','Autre','ACTIVE'),('supplier-off','library-1','Inactif','DISABLED');
	`)
	if err != nil { t.Fatalf("create schema: %v", err) }

	service := NewPurchaseService(repositories.NewPurchaseRepository(db))
	ownerOne := &auth.Claims{Role:models.RoleOwnerLibrary, LibraryID:"library-1"}; ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role:models.RoleOwnerLibrary, LibraryID:"library-2"}; ownerTwo.Subject = "owner-2"

	purchase, err := service.Create(context.Background(), ownerOne, models.PurchaseInput{
		SupplierID:"supplier-1", Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:2,UnitCost:1500},{BookID:2,Quantity:1,UnitCost:750}},
	})
	if err != nil { t.Fatalf("create purchase: %v", err) }
	if purchase.Status != models.PurchaseStatusDraft || purchase.TotalAmount != 3750 || purchase.Version != 1 || len(purchase.Lines) != 2 {
		t.Fatalf("unexpected purchase: %+v", purchase)
	}
	if _, err = service.Find(context.Background(), ownerTwo, purchase.ID); !errors.Is(err, repositories.ErrPurchaseNotFound) {
		t.Fatalf("cross-library purchase must be hidden: %v", err)
	}
	if _, err = service.Create(context.Background(), ownerOne, models.PurchaseInput{
		SupplierID:"supplier-2", Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1,UnitCost:1}},
	}); !errors.Is(err, repositories.ErrPurchaseSupplier) { t.Fatalf("cross-library supplier: %v", err) }
	if _, err = service.Create(context.Background(), ownerOne, models.PurchaseInput{
		SupplierID:"supplier-off", Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1,UnitCost:1}},
	}); !errors.Is(err, repositories.ErrPurchaseSupplier) { t.Fatalf("disabled supplier: %v", err) }

	purchase, err = service.Update(context.Background(), ownerOne, purchase.ID, models.PurchaseInput{
		SupplierID:"supplier-1", Version:1, Lines:[]models.PurchaseLineInput{{BookID:2,Quantity:4,UnitCost:500}},
	})
	if err != nil || purchase.TotalAmount != 2000 || purchase.Version != 2 || len(purchase.Lines) != 1 {
		t.Fatalf("update purchase=%+v err=%v", purchase, err)
	}
	if _, err = service.Update(context.Background(), ownerOne, purchase.ID, models.PurchaseInput{
		SupplierID:"supplier-1", Version:1, Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1,UnitCost:1}},
	}); !errors.Is(err, repositories.ErrPurchaseConflict) { t.Fatalf("stale update: %v", err) }
	if err = service.Delete(context.Background(), ownerOne, purchase.ID, 1); !errors.Is(err, repositories.ErrPurchaseConflict) {
		t.Fatalf("stale delete: %v", err)
	}
	if err = service.Delete(context.Background(), ownerOne, purchase.ID, 2); err != nil { t.Fatalf("delete purchase: %v", err) }
	var lines, audits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM purchase_lines WHERE purchase_id=?`, purchase.ID).Scan(&lines)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='PURCHASE'`).Scan(&audits)
	if lines != 0 || audits != 3 { t.Fatalf("purchase lines=%d audits=%d", lines, audits) }
}

func TestPurchaseValidation(t *testing.T) {
	cases := []models.PurchaseInput{
		{SupplierID:"",Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1,UnitCost:1}}},
		{SupplierID:"supplier",Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:0,UnitCost:1}}},
		{SupplierID:"supplier",Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1,UnitCost:-1}}},
		{SupplierID:"supplier",Lines:[]models.PurchaseLineInput{{BookID:1,Quantity:1},{BookID:1,Quantity:2}}},
	}
	for index, input := range cases {
		if err := validatePurchaseInput(input, false); !errors.Is(err, ErrInvalidPurchase) {
			t.Fatalf("case %d should be invalid: %v", index, err)
		}
	}
}
