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
		CREATE TABLE customers(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,name TEXT NOT NULL,status TEXT NOT NULL);
		CREATE TABLE defta(id INTEGER PRIMARY KEY,title TEXT NOT NULL,price REAL NOT NULL,library_id TEXT,deleted_at TEXT);
		CREATE TABLE sales(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,reference TEXT NOT NULL,customer_id TEXT,customer_name TEXT,
			status TEXT NOT NULL,total_amount REAL NOT NULL,version INTEGER NOT NULL,created_by TEXT NOT NULL,
			confirmed_by TEXT,cancelled_by TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
			confirmed_at TEXT,cancelled_at TEXT,UNIQUE(library_id,reference));
		CREATE TABLE sale_lines(id TEXT PRIMARY KEY,sale_id TEXT NOT NULL,book_id INTEGER NOT NULL,title_snapshot TEXT NOT NULL,
			quantity INTEGER NOT NULL,unit_price REAL NOT NULL,line_total REAL NOT NULL,created_at TEXT NOT NULL,UNIQUE(sale_id,book_id));
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		CREATE TABLE book_inventory(book_id INTEGER PRIMARY KEY,library_id TEXT NOT NULL,quantity INTEGER NOT NULL,
			low_stock_threshold INTEGER NOT NULL,version INTEGER NOT NULL,updated_at TEXT NOT NULL);
		CREATE TABLE inventory_movements(id TEXT PRIMARY KEY,book_id INTEGER,library_id TEXT,actor_user_id TEXT,
			movement_type TEXT,quantity_delta INTEGER,quantity_before INTEGER,quantity_after INTEGER,reason TEXT,created_at TEXT);
		INSERT INTO users VALUES('owner-1'),('owner-2');
		INSERT INTO libraries VALUES('library-1'),('library-2');
		INSERT INTO customers VALUES('customer-1','library-1','Aïssatou Diop','ACTIVE'),
			('customer-2','library-2','Client tiers','ACTIVE'),('customer-disabled','library-1','Client désactivé','DISABLED');
		INSERT INTO defta VALUES(1,'Livre un',2500,'library-1',NULL),(2,'Livre deux',3000,'library-1',NULL),
			(3,'Livre tiers',1000,'library-2',NULL);
		INSERT INTO book_inventory VALUES(1,'library-1',10,5,1,'now'),(2,'library-1',10,5,1,'now'),
			(3,'library-2',10,5,1,'now');
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
		CustomerID: "customer-1", CustomerName: "Nom ignoré",
		Lines: []models.SaleLineInput{{BookID: 1, Quantity: 2}, {BookID: 2, Quantity: 1}},
	})
	if err != nil || sale.Status != models.SaleStatusDraft || sale.TotalAmount != 8000 || len(sale.Lines) != 2 ||
		sale.CustomerID != "customer-1" || sale.CustomerName != "Aïssatou Diop" {
		t.Fatalf("create sale=%+v err=%v", sale, err)
	}
	if _, err = service.Find(context.Background(), other, sale.ID); !errors.Is(err, repositories.ErrSaleNotFound) {
		t.Fatalf("cross-library find=%v", err)
	}
	sale, err = service.Update(context.Background(), owner, sale.ID, models.SaleInput{
		CustomerID: "customer-1", Version: 1, Lines: []models.SaleLineInput{{BookID: 2, Quantity: 2}},
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
	if _, err = service.Create(context.Background(), owner, models.SaleInput{
		CustomerID: "customer-2", Lines: []models.SaleLineInput{{BookID: 1, Quantity: 1}},
	}); !errors.Is(err, repositories.ErrSaleCustomer) {
		t.Fatalf("cross-library customer=%v", err)
	}
	if _, err = service.Create(context.Background(), owner, models.SaleInput{
		CustomerID: "customer-disabled", Lines: []models.SaleLineInput{{BookID: 1, Quantity: 1}},
	}); !errors.Is(err, repositories.ErrSaleCustomer) {
		t.Fatalf("disabled customer=%v", err)
	}
	sales, total, err := service.List(context.Background(), owner, "", models.SaleFilter{Status: models.SaleStatusDraft}, 0, 30)
	if err != nil || total != 1 || len(sales) != 1 || len(sales[0].Lines) != 1 {
		t.Fatalf("list sales=%+v total=%d err=%v", sales, total, err)
	}
	var audits int
	_ = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action IN ('CREATE_SALE','UPDATE_SALE')").Scan(&audits)
	if audits != 2 {
		t.Fatalf("sale audits=%d", audits)
	}
	deletable, err := service.Create(context.Background(), owner, models.SaleInput{
		Lines: []models.SaleLineInput{{BookID: 1, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Delete(context.Background(), other, deletable.ID); !errors.Is(err, repositories.ErrSaleNotFound) {
		t.Fatalf("cross-library delete=%v", err)
	}
	if err = service.Delete(context.Background(), owner, deletable.ID); err != nil {
		t.Fatalf("delete draft=%v", err)
	}
	if _, err = service.Find(context.Background(), owner, deletable.ID); !errors.Is(err, repositories.ErrSaleNotFound) {
		t.Fatalf("find deleted draft=%v", err)
	}
	var deletedLines, deleteAudits int
	_ = db.QueryRow("SELECT COUNT(*) FROM sale_lines WHERE sale_id=?", deletable.ID).Scan(&deletedLines)
	_ = db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE action='DELETE_SALE' AND resource_id=?", deletable.ID).Scan(&deleteAudits)
	if deletedLines != 0 || deleteAudits != 1 {
		t.Fatalf("delete draft lines=%d audits=%d", deletedLines, deleteAudits)
	}

	sale, err = service.Confirm(context.Background(), owner, sale.ID, sale.Version)
	if err != nil || sale.Status != models.SaleStatusConfirmed || sale.Version != 3 {
		t.Fatalf("confirm sale=%+v err=%v", sale, err)
	}
	var stock int
	_ = db.QueryRow("SELECT quantity FROM book_inventory WHERE book_id=2").Scan(&stock)
	if stock != 8 {
		t.Fatalf("stock after confirmation=%d", stock)
	}
	if _, err = service.Confirm(context.Background(), owner, sale.ID, sale.Version); !errors.Is(err, repositories.ErrSaleState) {
		t.Fatalf("repeated confirmation=%v", err)
	}
	if err = service.Delete(context.Background(), owner, sale.ID); !errors.Is(err, repositories.ErrSaleState) {
		t.Fatalf("delete confirmed sale=%v", err)
	}
	sale, err = service.Cancel(context.Background(), owner, sale.ID, sale.Version)
	if err != nil || sale.Status != models.SaleStatusCancelled || sale.Version != 4 {
		t.Fatalf("cancel sale=%+v err=%v", sale, err)
	}
	_ = db.QueryRow("SELECT quantity FROM book_inventory WHERE book_id=2").Scan(&stock)
	if stock != 10 {
		t.Fatalf("stock after cancellation=%d", stock)
	}
	var movements int
	_ = db.QueryRow("SELECT COUNT(*) FROM inventory_movements WHERE book_id=2").Scan(&movements)
	if movements != 2 {
		t.Fatalf("sale inventory movements=%d", movements)
	}
}

func TestSaleConfirmationRollsBackOnInsufficientStock(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "sales-stock.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY); CREATE TABLE libraries(id TEXT PRIMARY KEY);
		CREATE TABLE defta(id INTEGER PRIMARY KEY,title TEXT,price REAL,library_id TEXT,deleted_at TEXT);
		CREATE TABLE sales(id TEXT PRIMARY KEY,library_id TEXT,reference TEXT,customer_id TEXT,customer_name TEXT,status TEXT,total_amount REAL,
			version INTEGER,created_by TEXT,confirmed_by TEXT,cancelled_by TEXT,created_at TEXT,updated_at TEXT,confirmed_at TEXT,cancelled_at TEXT);
		CREATE TABLE sale_lines(id TEXT PRIMARY KEY,sale_id TEXT,book_id INTEGER,title_snapshot TEXT,quantity INTEGER,
			unit_price REAL,line_total REAL,created_at TEXT,UNIQUE(sale_id,book_id));
		CREATE TABLE book_inventory(book_id INTEGER PRIMARY KEY,library_id TEXT,quantity INTEGER,low_stock_threshold INTEGER,
			version INTEGER,updated_at TEXT);
		CREATE TABLE inventory_movements(id TEXT PRIMARY KEY,book_id INTEGER,library_id TEXT,actor_user_id TEXT,
			movement_type TEXT,quantity_delta INTEGER,quantity_before INTEGER,quantity_after INTEGER,reason TEXT,created_at TEXT);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,
			new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES('owner-1'); INSERT INTO libraries VALUES('library-1');
		INSERT INTO defta VALUES(1,'Disponible',1000,'library-1',NULL),(2,'Épuisé',2000,'library-1',NULL);
		INSERT INTO book_inventory VALUES(1,'library-1',5,2,1,'now'),(2,'library-1',0,2,1,'now');
	`)
	if err != nil {
		t.Fatal(err)
	}
	service := NewSaleService(repositories.NewSaleRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	owner.Subject = "owner-1"
	sale, err := service.Create(context.Background(), owner, models.SaleInput{Lines: []models.SaleLineInput{
		{BookID: 1, Quantity: 1}, {BookID: 2, Quantity: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Confirm(context.Background(), owner, sale.ID, sale.Version); !errors.Is(err, repositories.ErrInsufficientStock) {
		t.Fatalf("insufficient stock=%v", err)
	}
	var firstStock, movements int
	_ = db.QueryRow("SELECT quantity FROM book_inventory WHERE book_id=1").Scan(&firstStock)
	_ = db.QueryRow("SELECT COUNT(*) FROM inventory_movements").Scan(&movements)
	if firstStock != 5 || movements != 0 {
		t.Fatalf("partial write stock=%d movements=%d", firstStock, movements)
	}
	stored, err := service.Find(context.Background(), owner, sale.ID)
	if err != nil || stored.Status != models.SaleStatusDraft || stored.Version != 1 {
		t.Fatalf("sale changed after rollback=%+v err=%v", stored, err)
	}
}
