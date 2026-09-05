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

func TestPaymentLifecycleBalanceIsolationAndAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "payments.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY);
		CREATE TABLE sales(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,status TEXT NOT NULL,total_amount REAL NOT NULL);
		CREATE TABLE cash_registers(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,name TEXT,normalized_name TEXT,
			status TEXT NOT NULL,version INTEGER,created_by TEXT,created_at TEXT,updated_at TEXT,UNIQUE(id,library_id));
		CREATE TABLE payments(id TEXT PRIMARY KEY,library_id TEXT NOT NULL,sale_id TEXT NOT NULL,cash_register_id TEXT NOT NULL,
			method TEXT NOT NULL,amount REAL NOT NULL,external_reference TEXT,notes TEXT,status TEXT NOT NULL,version INTEGER NOT NULL,
			recorded_by TEXT NOT NULL,voided_by TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,voided_at TEXT,
			UNIQUE(library_id,method,external_reference));
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		CREATE TRIGGER payment_context BEFORE INSERT ON payments WHEN NOT EXISTS(
			SELECT 1 FROM sales s JOIN cash_registers c ON c.id=NEW.cash_register_id AND c.library_id=NEW.library_id
			AND c.status='ACTIVE' WHERE s.id=NEW.sale_id AND s.library_id=NEW.library_id AND s.status='CONFIRMED')
			BEGIN SELECT RAISE(ABORT,'payment sale or cash register is unavailable'); END;
		CREATE TRIGGER payment_overpay BEFORE INSERT ON payments WHEN COALESCE((SELECT SUM(amount) FROM payments
			WHERE sale_id=NEW.sale_id AND status='RECORDED'),0)+NEW.amount>(SELECT total_amount FROM sales WHERE id=NEW.sale_id)
			BEGIN SELECT RAISE(ABORT,'payment exceeds sale remaining amount'); END;
		CREATE VIEW sale_payment_balances AS SELECT s.id sale_id,s.library_id,s.total_amount,
			COALESCE(SUM(CASE WHEN p.status='RECORDED' THEN p.amount ELSE 0 END),0) paid_amount,
			s.total_amount-COALESCE(SUM(CASE WHEN p.status='RECORDED' THEN p.amount ELSE 0 END),0) remaining_amount,
			CASE WHEN COALESCE(SUM(CASE WHEN p.status='RECORDED' THEN p.amount ELSE 0 END),0)=0 THEN 'UNPAID'
			WHEN COALESCE(SUM(CASE WHEN p.status='RECORDED' THEN p.amount ELSE 0 END),0)<s.total_amount
			THEN 'PARTIALLY_PAID' ELSE 'PAID' END payment_status FROM sales s LEFT JOIN payments p ON p.sale_id=s.id
			GROUP BY s.id,s.library_id,s.total_amount;
		INSERT INTO users VALUES ('owner-1'),('owner-2');
		INSERT INTO libraries VALUES ('library-1'),('library-2');
		INSERT INTO sales VALUES ('sale-1','library-1','CONFIRMED',10000),('sale-2','library-2','CONFIRMED',5000);
		INSERT INTO cash_registers VALUES ('register-1','library-1','Principale','principale','ACTIVE',1,'owner-1','now','now');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewPaymentService(repositories.NewPaymentRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	payment, err := service.Create(context.Background(), ownerOne, "sale-1", models.PaymentInput{
		CashRegisterID: "register-1", Method: models.PaymentMethodCash, Amount: 4000,
	})
	if err != nil || payment.Version != 1 {
		t.Fatalf("create payment=%+v err=%v", payment, err)
	}
	balance, err := service.Balance(context.Background(), ownerOne, "sale-1")
	if err != nil || balance.PaidAmount != 4000 || balance.RemainingAmount != 6000 || balance.PaymentStatus != "PARTIALLY_PAID" {
		t.Fatalf("partial balance=%+v err=%v", balance, err)
	}
	if _, err = service.Create(context.Background(), ownerOne, "sale-1", models.PaymentInput{
		CashRegisterID: "register-1", Method: models.PaymentMethodCash, Amount: 6001,
	}); !errors.Is(err, repositories.ErrPaymentOverpaid) {
		t.Fatalf("expected overpayment, got %v", err)
	}
	if _, err = service.Balance(context.Background(), ownerTwo, "sale-1"); !errors.Is(err, repositories.ErrPaymentSaleNotFound) {
		t.Fatalf("cross-library sale must be hidden: %v", err)
	}
	payment, err = service.Void(context.Background(), ownerOne, payment.ID,
		models.PaymentVoidInput{Version: 1, Reason: "Erreur de saisie"})
	if err != nil || payment.Status != models.PaymentStatusVoided || payment.Version != 2 {
		t.Fatalf("void payment=%+v err=%v", payment, err)
	}
	balance, err = service.Balance(context.Background(), ownerOne, "sale-1")
	if err != nil || balance.PaidAmount != 0 || balance.RemainingAmount != 10000 || balance.PaymentStatus != "UNPAID" {
		t.Fatalf("voided balance=%+v err=%v", balance, err)
	}
	if _, err = service.Void(context.Background(), ownerOne, payment.ID,
		models.PaymentVoidInput{Version: 2, Reason: "Encore"}); !errors.Is(err, repositories.ErrPaymentState) {
		t.Fatalf("expected state conflict, got %v", err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='PAYMENT'`).Scan(&audits); err != nil || audits != 2 {
		t.Fatalf("payment audits=%d err=%v", audits, err)
	}
}

func TestPaymentValidation(t *testing.T) {
	if validPaymentMethod(models.PaymentMethod("CHEQUE"), false) {
		t.Fatal("invalid payment method accepted")
	}
	if validPaymentStatus(models.PaymentStatus("DELETED"), false) {
		t.Fatal("invalid payment status accepted")
	}
}
