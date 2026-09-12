package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"math"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCustomerReturnLifecycleIsolationStockAndAudit(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		stockCost, saleCost, expectedCost sql.NullFloat64
	}{
		{"weighted", sql.NullFloat64{Float64: 2000, Valid: true}, sql.NullFloat64{Float64: 1100, Valid: true}, sql.NullFloat64{Float64: 1800, Valid: true}},
		{"unknown sale", sql.NullFloat64{Float64: 2000, Valid: true}, sql.NullFloat64{}, sql.NullFloat64{}},
		{"unknown stock", sql.NullFloat64{}, sql.NullFloat64{Float64: 1100, Valid: true}, sql.NullFloat64{}},
		{"known zero", sql.NullFloat64{Valid: true}, sql.NullFloat64{Valid: true}, sql.NullFloat64{Valid: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testCustomerReturnValuation(t, tc.stockCost, tc.saleCost, tc.expectedCost)
		})
	}
}

func testCustomerReturnValuation(t *testing.T, stockCost, saleCost, expectedCost sql.NullFloat64) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "returns.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY);
		CREATE TABLE customers(id TEXT PRIMARY KEY);
		CREATE TABLE defta(id INTEGER PRIMARY KEY,title TEXT);
		CREATE TABLE sales(id TEXT PRIMARY KEY,library_id TEXT,customer_id TEXT,status TEXT);
		CREATE TABLE sale_lines(id TEXT PRIMARY KEY,sale_id TEXT,book_id INTEGER,quantity INTEGER,unit_price REAL);
		CREATE TABLE customer_returns(id TEXT PRIMARY KEY,library_id TEXT,sale_id TEXT,customer_id TEXT,reference TEXT,
			reason TEXT,status TEXT,resolution TEXT,total_amount REAL DEFAULT 0,version INTEGER,created_by TEXT,
			completed_by TEXT,cancelled_by TEXT,created_at TEXT,updated_at TEXT,completed_at TEXT,cancelled_at TEXT);
		CREATE TABLE customer_return_lines(id TEXT PRIMARY KEY,return_id TEXT,sale_line_id TEXT,book_id INTEGER,
			quantity INTEGER,unit_price REAL,line_total REAL,created_at TEXT,UNIQUE(return_id,sale_line_id));
		CREATE TABLE book_inventory(book_id INTEGER,library_id TEXT,quantity INTEGER,version INTEGER,updated_at TEXT,
			PRIMARY KEY(book_id,library_id));
		CREATE TABLE inventory_movements(id TEXT PRIMARY KEY,book_id INTEGER,library_id TEXT,actor_user_id TEXT,
			movement_type TEXT,quantity_delta INTEGER,quantity_before INTEGER,quantity_after INTEGER,reason TEXT,created_at TEXT);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,
			old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		CREATE TRIGGER return_line_total_insert AFTER INSERT ON customer_return_lines BEGIN
			UPDATE customer_returns SET total_amount=(SELECT SUM(line_total) FROM customer_return_lines WHERE return_id=NEW.return_id)
			WHERE id=NEW.return_id; END;
		CREATE TRIGGER return_line_total_delete AFTER DELETE ON customer_return_lines BEGIN
			UPDATE customer_returns SET total_amount=COALESCE((SELECT SUM(line_total) FROM customer_return_lines WHERE return_id=OLD.return_id),0)
			WHERE id=OLD.return_id; END;
		CREATE TRIGGER return_complete_guard BEFORE UPDATE OF status ON customer_returns
		WHEN NEW.status='COMPLETED' AND EXISTS (
			SELECT 1 FROM customer_return_lines current_line JOIN sale_lines sold ON sold.id=current_line.sale_line_id
			WHERE current_line.return_id=NEW.id AND current_line.quantity+COALESCE((SELECT SUM(previous_line.quantity)
			FROM customer_return_lines previous_line JOIN customer_returns previous_return ON previous_return.id=previous_line.return_id
			WHERE previous_line.sale_line_id=current_line.sale_line_id AND previous_return.status='COMPLETED' AND previous_return.id<>NEW.id),0)>sold.quantity)
		BEGIN SELECT RAISE(ABORT,'customer return quantity exceeds sold quantity'); END;
		INSERT INTO users VALUES('owner-1'),('owner-2');
		INSERT INTO libraries VALUES('library-1'),('library-2');
		INSERT INTO customers VALUES('customer-1'); INSERT INTO defta VALUES(1,'Livre test');
		INSERT INTO sales VALUES('sale-1','library-1','customer-1','CONFIRMED');
		INSERT INTO sale_lines VALUES('line-1','sale-1',1,5,1000);
		INSERT INTO book_inventory VALUES(1,'library-1',7,1,'now');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	if _, err = db.Exec("ALTER TABLE book_inventory ADD COLUMN average_unit_cost REAL; ALTER TABLE sale_lines ADD COLUMN unit_cost_snapshot REAL"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE book_inventory SET average_unit_cost=?", stockCost); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE sale_lines SET unit_cost_snapshot=?", saleCost); err != nil {
		t.Fatal(err)
	}
	service := NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	input := models.CustomerReturnInput{SaleID: "sale-1", Reason: "Livre endommagé", Resolution: models.CustomerReturnResolutionRefund,
		Lines: []models.CustomerReturnLineInput{{SaleLineID: "line-1", Quantity: 2}}}
	value, err := service.Create(context.Background(), ownerOne, input)
	if err != nil || value.TotalAmount != 2000 || value.Status != models.CustomerReturnStatusDraft || value.Version != 1 {
		t.Fatalf("create return=%+v err=%v", value, err)
	}
	if _, err = service.Find(context.Background(), ownerTwo, value.ID); !errors.Is(err, repositories.ErrCustomerReturnNotFound) {
		t.Fatalf("cross-library return must be hidden: %v", err)
	}
	// Fail after the inventory update: quantity, valuation and status must all roll back.
	if _, err = db.Exec("CREATE TRIGGER fail_return_audit BEFORE INSERT ON audit_logs WHEN NEW.action='UPDATE_INVENTORY' BEGIN SELECT RAISE(ABORT,'test audit failure'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Complete(context.Background(), ownerOne, value.ID, value.Version); err == nil {
		t.Fatal("expected audit failure")
	}
	var rolledCost sql.NullFloat64
	var rolledQuantity, rolledVersion int
	if err = db.QueryRow("SELECT quantity,version,average_unit_cost FROM book_inventory WHERE book_id=1").Scan(&rolledQuantity, &rolledVersion, &rolledCost); err != nil {
		t.Fatal(err)
	}
	if rolledQuantity != 7 || rolledVersion != 1 || rolledCost != stockCost {
		t.Fatalf("rollback quantity=%d version=%d cost=%+v", rolledQuantity, rolledVersion, rolledCost)
	}
	stored, err := service.Find(context.Background(), ownerOne, value.ID)
	if err != nil || stored.Status != models.CustomerReturnStatusDraft || stored.Version != 1 {
		t.Fatalf("rollback return=%+v err=%v", stored, err)
	}
	if _, err = db.Exec("DROP TRIGGER fail_return_audit"); err != nil {
		t.Fatal(err)
	}
	value, err = service.Complete(context.Background(), ownerOne, value.ID, value.Version)
	if err != nil || value.Status != models.CustomerReturnStatusCompleted || value.Version != 2 {
		t.Fatalf("complete=%+v err=%v", value, err)
	}
	var quantity, movements, audits int
	_ = db.QueryRow(`SELECT quantity FROM book_inventory WHERE book_id=1 AND library_id='library-1'`).Scan(&quantity)
	_ = db.QueryRow(`SELECT COUNT(*) FROM inventory_movements WHERE reason LIKE 'Retour client %'`).Scan(&movements)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action IN ('CREATE_CUSTOMER_RETURN','COMPLETE_CUSTOMER_RETURN','UPDATE_INVENTORY')`).Scan(&audits)
	if quantity != 9 || movements != 1 || audits != 3 {
		t.Fatalf("quantity=%d movements=%d audits=%d", quantity, movements, audits)
	}

	var actualCost sql.NullFloat64
	if err = db.QueryRow("SELECT average_unit_cost FROM book_inventory WHERE book_id=1").Scan(&actualCost); err != nil || actualCost.Valid != expectedCost.Valid || (actualCost.Valid && math.Abs(actualCost.Float64-expectedCost.Float64) > 1e-8) {
		t.Fatalf("cost=%+v expected=%+v err=%v", actualCost, expectedCost, err)
	}
	if _, err = service.Complete(context.Background(), ownerOne, value.ID, value.Version); !errors.Is(err, repositories.ErrCustomerReturnState) {
		t.Fatalf("repeated completion: %v", err)
	}
	var frozen sql.NullFloat64
	if err = db.QueryRow("SELECT unit_cost_snapshot FROM sale_lines WHERE id='line-1'").Scan(&frozen); err != nil || frozen != saleCost {
		t.Fatalf("sale cost changed=%+v err=%v", frozen, err)
	}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root-1"
	rootDraft := input
	rootDraft.Lines = []models.CustomerReturnLineInput{{SaleLineID: "line-1", Quantity: 1}}
	rootReturn, err := service.Create(context.Background(), ownerOne, rootDraft)
	if err != nil {
		t.Fatalf("create root completion draft: %v", err)
	}
	rootReturn, err = service.Complete(context.Background(), root, rootReturn.ID, rootReturn.Version)
	if err != nil || rootReturn.Status != models.CustomerReturnStatusCompleted || rootReturn.LibraryID != "library-1" {
		t.Fatalf("root completion=%+v err=%v", rootReturn, err)
	}
	excess := input
	excess.Lines = []models.CustomerReturnLineInput{{SaleLineID: "line-1", Quantity: 4}}
	second, err := service.Create(context.Background(), ownerOne, excess)
	if err != nil {
		t.Fatalf("create excess draft: %v", err)
	}
	if _, err = service.Complete(context.Background(), ownerOne, second.ID, second.Version); !errors.Is(err, repositories.ErrCustomerReturnQuantity) {
		t.Fatalf("excess return: %v", err)
	}
	second, err = service.Cancel(context.Background(), ownerOne, second.ID, second.Version)
	if err != nil || second.Status != models.CustomerReturnStatusCancelled {
		t.Fatalf("cancel=%+v err=%v", second, err)
	}
}

func TestCustomerReturnValidation(t *testing.T) {
	valid := models.CustomerReturnInput{SaleID: "sale", Reason: "Motif valide", Resolution: models.CustomerReturnResolutionRefund, Lines: []models.CustomerReturnLineInput{{SaleLineID: "line", Quantity: 1}}}
	if err := validateCustomerReturn(valid, false); err != nil {
		t.Fatalf("valid return: %v", err)
	}
	invalid := valid
	invalid.Lines = append(invalid.Lines, invalid.Lines[0])
	if !errors.Is(validateCustomerReturn(invalid, false), ErrInvalidCustomerReturn) {
		t.Fatal("duplicate sale line should fail")
	}
}
