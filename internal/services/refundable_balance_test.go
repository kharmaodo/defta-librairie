package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"testing"
)

func TestRefundableBalanceAcrossReturns(t *testing.T) {
	db, owner := paymentCycleDB(t)
	ctx := context.Background()
	returns := NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))
	payments := NewPaymentService(repositories.NewPaymentRepository(db))
	settlements := NewReturnSettlementService(repositories.NewReturnSettlementRepository(db))
	check := func(expected float64) {
		t.Helper()
		b, err := settlements.Balance(ctx, owner, "cycle-return")
		if err != nil || b.RefundableAmount == nil || *b.RefundableAmount != expected {
			t.Fatalf("balance=%+v expected=%v err=%v", b, expected, err)
		}
	}
	check(0) // Drafts have no refund capacity.
	if _, err := returns.Complete(ctx, owner, "cycle-return", 1); err != nil {
		t.Fatal(err)
	}
	check(0) // No recorded payment.
	if _, err := payments.Create(ctx, owner, "cycle-sale", models.PaymentInput{CashRegisterID: "cycle-register", Method: models.PaymentMethodCash, Amount: 100}); err != nil {
		t.Fatal(err)
	}
	check(100) // Return total is 200, payment is 100.
	first, err := settlements.Create(ctx, owner, "cycle-return", models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCash, Amount: 60})
	if err != nil {
		t.Fatal(err)
	}
	check(40)
	if _, err = db.Exec(`INSERT INTO customer_returns(id,library_id,sale_id,reference,reason,resolution,created_by,created_at,updated_at)
        VALUES('second-return','cycle-library','cycle-sale','RC-SECOND','Second return','REFUND','cycle-owner','now','now');
        INSERT INTO customer_return_lines(id,return_id,sale_line_id,book_id,quantity,unit_price,line_total,created_at)
        VALUES('second-line','second-return','cycle-line',1,1,200,200,'now');`); err != nil {
		t.Fatal(err)
	}
	if _, err = returns.Complete(ctx, owner, "second-return", 1); err != nil {
		t.Fatal(err)
	}
	second, err := settlements.Create(ctx, owner, "second-return", models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCard, Amount: 30})
	if err != nil {
		t.Fatal(err)
	}
	check(10)
	if _, err = settlements.Void(ctx, owner, second.ID, models.ReturnSettlementVoidInput{Version: 1, Reason: "Correction saisie"}); err != nil {
		t.Fatal(err)
	}
	check(40)
	if _, err = settlements.Void(ctx, owner, first.ID, models.ReturnSettlementVoidInput{Version: 1, Reason: "Correction saisie"}); err != nil {
		t.Fatal(err)
	}
	check(100)
	if _, err = payments.Create(ctx, owner, "cycle-sale", models.PaymentInput{CashRegisterID: "cycle-register", Method: models.PaymentMethodCash, Amount: 200}); err != nil {
		t.Fatal(err)
	}
	check(200) // Capacity capped by this return, not 300 received.
	if _, err = settlements.Create(ctx, owner, "cycle-return", models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCash, Amount: 200}); err != nil {
		t.Fatal(err)
	}
	check(0)
	other := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "other"}
	if _, err = settlements.Balance(ctx, other, "cycle-return"); !errors.Is(err, repositories.ErrReturnSettlementReturnNotFound) {
		t.Fatalf("cross-library balance: %v", err)
	}
	if _, err = db.Exec("UPDATE customer_returns SET resolution='CREDIT_NOTE' WHERE id='second-return'"); err != nil {
		t.Fatal(err)
	}
	b, err := settlements.Balance(ctx, owner, "second-return")
	if err != nil || b.RefundableAmount != nil || b.RemainingAmount != 200 {
		t.Fatalf("credit balance=%+v err=%v", b, err)
	}
}
