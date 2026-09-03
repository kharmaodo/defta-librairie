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
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSaleDraftLifecycleAndIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "sales.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY);
		CREATE TABLE defta(id INTEGER PRIMARY KEY,title TEXT NOT NULL,price REAL NOT NULL,library_id TEXT,deleted_at TEXT);
		CREATE TABLE sales(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,reference TEXT NOT NULL,customer_name TEXT,
			status TEXT NOT NULL,total_amount REAL NOT NULL,version INTEGER NOT NULL,created_by TEXT NOT NULL,
			confirmed_by TEXT,cancelled_by TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
			confirmed_at TEXT,cancelled_at TEXT,UNIQUE(library_id,reference));
		CREATE TABLE sale_lines(id TEXT PRIMARY KEY,sale_id TEXT NOT NULL,book_id INTEGER NOT NULL,title_snapshot TEXT NOT NULL,
			quantity INTEGER NOT NULL,unit_price REAL NOT NULL,line_total REAL NOT NULL,created_at TEXT NOT NULL,UNIQUE(sale_id,book_id));
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES('owner-1'),('owner-2');
		INSERT INTO libraries VALUES('library-1'),('library-2');
		INSERT INTO defta VALUES(1,'Livre un',2500,'library-1',NULL),(2,'Livre deux',3000,'library-1',NULL),
			(3,'Livre tiers',1000,'library-2',NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}
	service := NewSaleService(repositories.NewSaleRepository(db))
	service.now = func() time.Time { return time.Date(2026, 9, 3, 16, 0, 0, 0, time.UTC) }
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	owner.Subject = "owner-1"
	other := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	other.Subject = "owner-2"

	sale, err := service.Create(context.Background(), owner, models.SaleInput{
		CustomerName: "Client", Lines: []models.SaleLineInput{{BookID: 1, Quantity: 2}, {BookID: 2, Quantity: 1}},
	})
	if err != nil || sale.Status != models.SaleStatusDraft || sale.TotalAmount != 8000 || len(sale.Lines) != 2 {
		t.Fatalf("create sale=%+v err=%v", sale, err)
	}
	if _, err = service.Find(context.Background(), other, sale.ID); !errors.Is(err, repositories.ErrSaleNotFound) {
		t.Fatalf("cross-library find=%v", err)
	}
	sale, err = service.Update(context.Background(), owner, sale.ID, models.SaleInput{
		CustomerName: "Client modifié", Version: 1, Lines: []models.SaleLineInput{{BookID: 2, Quantity: 2}},
	})
	if err != nil || sale.Version != 2 || sale.TotalAmount != 6000 || len(sale.Lines) != 1 {
		t.Fatalf("update sale=%+v err=%v", sale, err)
	}
	if _, err = service.Update(context.Background(), owner, sale.ID, models.SaleInput{
		Version: 1, Lines: []models.SaleLineInput{{BookID: 1, Quantity: 1}},
	}); !errors.Is(err, repositories.ErrSaleConflict) {
		t.Fatalf("stale update=%v", err)
	}
	if _, err = service.Create(context.Background(), owner, models.SaleInput{
		Lines: []models.SaleLineInput{{BookID: 3, Quantity: 1}},
	}); !errors.Is(err, repositories.ErrSaleBook) {
		t.Fatalf("cross-library book=%v", err)
	}
	sales, total, err := service.List(context.Background(), owner, "", models.SaleFilter{Status: models.SaleStatusDraft}, 0, 30)
	if err != nil || total != 1 || len(sales) != 1 {
		t.Fatalf("list sales=%+v total=%d err=%v", sales, total, err)
	}
	var audits int
	_ = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action IN ('CREATE_SALE','UPDATE_SALE')").Scan(&audits)
	if audits != 2 {
		t.Fatalf("sale audits=%d", audits)
	}
}
