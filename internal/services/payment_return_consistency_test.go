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

func paymentCycleDB(t *testing.T) (*sql.DB, *auth.Claims) {
	t.Helper()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "payments.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if err = migrations.Run(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialCycleFixture + paymentCycleFixture); err != nil {
		t.Fatal(err)
	}
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "cycle-library"}
	owner.Subject = "cycle-owner"
	return db, owner
}

func TestPaidSaleCancellationRollback(t *testing.T) {
	db, owner := paymentCycleDB(t)
	ctx := context.Background()
	payments := NewPaymentService(repositories.NewPaymentRepository(db))
	sales := NewSaleService(repositories.NewSaleRepository(db))
	p, err := payments.Create(ctx, owner, "cycle-sale", models.PaymentInput{CashRegisterID: "cycle-register", Method: models.PaymentMethodCash, Amount: 100})
	if err != nil {
		t.Fatal(err)
	}
	before := readCycleState(t, db)
	if _, err = sales.Cancel(ctx, owner, "cycle-sale", 1); !errors.Is(err, repositories.ErrSaleRecordedPayments) {
		t.Fatalf("paid sale cancelled: %v", err)
	}
	if after := readCycleState(t, db); after != before {
		t.Fatal("rejected cancellation changed stock, costs or audit")
	}
	if _, err = payments.Void(ctx, owner, p.ID, models.PaymentVoidInput{Version: p.Version, Reason: "Correction saisie"}); err != nil {
		t.Fatal(err)
	}
	if _, err = sales.Cancel(ctx, owner, "cycle-sale", 1); err != nil {
		t.Fatal(err)
	}
}

func TestRefundLimitedByRecordedPayments(t *testing.T) {
	for _, method := range []models.ReturnSettlementMethod{models.ReturnSettlementMethodCash, models.ReturnSettlementMethodMobileMoney, models.ReturnSettlementMethodCard} {
		t.Run(string(method), func(t *testing.T) {
			db, owner := paymentCycleDB(t)
			ctx := context.Background()
			returns := NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))
			payments := NewPaymentService(repositories.NewPaymentRepository(db))
			settlements := NewReturnSettlementService(repositories.NewReturnSettlementRepository(db))
			if _, err := returns.Complete(ctx, owner, "cycle-return", 1); err != nil {
				t.Fatal(err)
			}
			before := readCycleState(t, db)
			input := models.ReturnSettlementInput{Method: method, Amount: 60}
			if _, err := settlements.Create(ctx, owner, "cycle-return", input); !errors.Is(err, repositories.ErrReturnSettlementUnfunded) {
				t.Fatalf("unpaid refund: %v", err)
			}
			if readCycleState(t, db) != before {
				t.Fatal("rejected refund left audit")
			}
			p, err := payments.Create(ctx, owner, "cycle-sale", models.PaymentInput{CashRegisterID: "cycle-register", Method: models.PaymentMethodCash, Amount: 100})
			if err != nil {
				t.Fatal(err)
			}
			refund, err := settlements.Create(ctx, owner, "cycle-return", input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = settlements.Create(ctx, owner, "cycle-return", input); !errors.Is(err, repositories.ErrReturnSettlementUnfunded) {
				t.Fatalf("cumulative refund: %v", err)
			}
			if _, err = db.Exec(`INSERT INTO customer_returns(id,library_id,sale_id,reference,reason,resolution,created_by,created_at,updated_at)
                VALUES('second-return','cycle-library','cycle-sale','RC-SECOND','Second return','REFUND','cycle-owner','now','now');
                INSERT INTO customer_return_lines(id,return_id,sale_line_id,book_id,quantity,unit_price,line_total,created_at)
                VALUES('second-line','second-return','cycle-line',1,1,200,200,'now');`); err != nil {
				t.Fatal(err)
			}
			if _, err = returns.Complete(ctx, owner, "second-return", 1); err != nil {
				t.Fatal(err)
			}
			if _, err = settlements.Create(ctx, owner, "second-return", models.ReturnSettlementInput{Method: method, Amount: 41}); !errors.Is(err, repositories.ErrReturnSettlementUnfunded) {
				t.Fatalf("refund across multiple returns: %v", err)
			}
			before = readCycleState(t, db)
			if _, err = payments.Void(ctx, owner, p.ID, models.PaymentVoidInput{Version: p.Version, Reason: "Correction saisie"}); !errors.Is(err, repositories.ErrPaymentRefundConflict) {
				t.Fatalf("payment covering refund voided: %v", err)
			}
			if readCycleState(t, db) != before {
				t.Fatal("rejected payment void left audit")
			}
			balance, err := payments.Balance(ctx, owner, "cycle-sale")
			if err != nil || balance.PaidAmount != 100 {
				t.Fatalf("balance=%+v err=%v", balance, err)
			}
			if _, err = settlements.Void(ctx, owner, refund.ID, models.ReturnSettlementVoidInput{Version: refund.Version, Reason: "Correction saisie"}); err != nil {
				t.Fatal(err)
			}
			if _, err = payments.Void(ctx, owner, p.ID, models.PaymentVoidInput{Version: p.Version, Reason: "Correction saisie"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

const paymentCycleFixture = `
INSERT INTO cash_registers(id,library_id,name,normalized_name,created_by,created_at,updated_at)
VALUES('cycle-register','cycle-library','Caisse test','caisse test','cycle-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
`
