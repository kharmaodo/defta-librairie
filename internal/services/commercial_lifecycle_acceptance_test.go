package services

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
)

// Uses real services/repositories/migrations, with no production database or HTTP server.
func TestCommercialLifecycleAcceptance(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "acceptance.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = migrations.Run(ctx, db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(commercialAcceptanceFixture); err != nil {
		t.Fatal(err)
	}
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "accept-library"}
	owner.Subject = "accept-owner"
	sales := NewSaleService(repositories.NewSaleRepository(db))
	payments := NewPaymentService(repositories.NewPaymentRepository(db))
	returns := NewCustomerReturnService(repositories.NewCustomerReturnRepository(db))
	settlements := NewReturnSettlementService(repositories.NewReturnSettlementRepository(db))
	statistics := NewCommercialStatisticsService(repositories.NewCommercialStatisticsRepository(db))
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	sales.now = clock
	payments.now = clock
	returns.now = clock
	settlements.now = clock

	checkStock := func(quantity int) {
		t.Helper()
		var q int
		var cost float64
		if err := db.QueryRow("SELECT quantity,average_unit_cost FROM book_inventory WHERE book_id=1").Scan(&q, &cost); err != nil {
			t.Fatal(err)
		}
		if q != quantity || math.IsNaN(cost) || math.IsInf(cost, 0) || math.Abs(cost-1000) > 1e-9 {
			t.Fatalf("stock=%d cost=%v, expected quantity=%d cost=1000", q, cost, quantity)
		}
	}
	checkStock(10)
	sale, err := sales.Create(ctx, owner, models.SaleInput{CustomerName: "Client recette", Lines: []models.SaleLineInput{{BookID: 1, Quantity: 4}}})
	if err != nil {
		t.Fatal(err)
	}
	if sale.Status != models.SaleStatusDraft || sale.TotalAmount != 6000 {
		t.Fatalf("draft=%+v", sale)
	}
	checkStock(10)
	sale, err = sales.Confirm(ctx, owner, sale.ID, sale.Version)
	if err != nil {
		t.Fatal(err)
	}
	checkStock(6)
	var frozen float64
	if err = db.QueryRow("SELECT unit_cost_snapshot FROM sale_lines WHERE sale_id=?", sale.ID).Scan(&frozen); err != nil || frozen != 1000 {
		t.Fatalf("frozen=%v err=%v", frozen, err)
	}
	paymentInput := models.PaymentInput{CashRegisterID: "accept-register", Method: models.PaymentMethodCash, Amount: 1000}
	if _, err = payments.Create(ctx, owner, sale.ID, paymentInput); err != nil {
		t.Fatal(err)
	}
	balance, err := payments.Balance(ctx, owner, sale.ID)
	if err != nil || balance.PaidAmount != 1000 || balance.RemainingAmount != 5000 {
		t.Fatalf("partial payment=%+v err=%v", balance, err)
	}
	checkStock(6)

	now = now.AddDate(0, 0, 1)
	ret, err := returns.Create(ctx, owner, models.CustomerReturnInput{SaleID: sale.ID, Reason: "Retour recette", Resolution: models.CustomerReturnResolutionRefund, Lines: []models.CustomerReturnLineInput{{SaleLineID: sale.Lines[0].ID, Quantity: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	checkStock(6)
	ret, err = returns.Complete(ctx, owner, ret.ID, ret.Version)
	if err != nil || ret.TotalAmount != 1500 {
		t.Fatalf("return=%+v err=%v", ret, err)
	}
	checkStock(7)
	checkReturn := func(settled, remaining, refundable float64) {
		t.Helper()
		b, e := settlements.Balance(ctx, owner, ret.ID)
		if e != nil || b.SettledAmount != settled || b.RemainingAmount != remaining || b.RefundableAmount == nil || *b.RefundableAmount != refundable {
			t.Fatalf("return balance=%+v err=%v", b, e)
		}
	}
	checkReturn(0, 1500, 1000)
	if _, err = settlements.Create(ctx, owner, ret.ID, models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCash, Amount: 700}); err != nil {
		t.Fatal(err)
	}
	checkReturn(700, 800, 300)
	paymentInput.Amount = 5000
	if _, err = payments.Create(ctx, owner, sale.ID, paymentInput); err != nil {
		t.Fatal(err)
	}
	checkReturn(700, 800, 800)
	if _, err = settlements.Create(ctx, owner, ret.ID, models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCash, Amount: 800}); err != nil {
		t.Fatal(err)
	}
	checkReturn(1500, 0, 0)
	checkStock(7)
	balance, err = payments.Balance(ctx, owner, sale.ID)
	if err != nil || balance.PaidAmount != 6000 || balance.RemainingAmount != 0 {
		t.Fatalf("gross payment=%+v err=%v", balance, err)
	}
	var netCash float64
	if err = db.QueryRow(`SELECT (SELECT SUM(amount) FROM payments WHERE status='RECORDED')-(SELECT SUM(amount) FROM return_settlements WHERE status='ISSUED' AND method<>'CREDIT_NOTE')`).Scan(&netCash); err != nil || netCash != 4500 {
		t.Fatalf("net cash=%v err=%v", netCash, err)
	}

	for _, tc := range []struct {
		from, to                           string
		gross, returned, net, cost, margin float64
	}{
		{"2026-09-01T00:00:00Z", "2026-09-02T00:00:00Z", 6000, 0, 6000, 4000, 2000},
		{"2026-09-02T00:00:00Z", "2026-09-03T00:00:00Z", 0, 1500, -1500, -1000, -500},
		{"2026-09-01T00:00:00Z", "2026-09-03T00:00:00Z", 6000, 1500, 4500, 3000, 1500},
	} {
		got, e := statistics.Summary(ctx, owner, "", tc.from, tc.to)
		if e != nil {
			t.Fatal(e)
		}
		if got.GrossSales != tc.gross || got.CustomerReturns != tc.returned || got.NetSales != tc.net || got.KnownCost != tc.cost || got.UnknownCostEvents != 0 || got.NetMargin == nil || *got.NetMargin != tc.margin {
			t.Fatalf("period %s: %+v", tc.from, got)
		}
	}
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	got, err := statistics.Summary(ctx, root, "accept-library", "2026-09-01T00:00:00Z", "2026-09-03T00:00:00Z")
	if err != nil || got.NetSales != 4500 {
		t.Fatalf("root scope: %+v %v", got, err)
	}
	if _, err = statistics.Summary(ctx, owner, "other-library", "2026-09-01T00:00:00Z", "2026-09-03T00:00:00Z"); !errors.Is(err, ErrBookForbidden) {
		t.Fatalf("cross-library statistics: %v", err)
	}

	var beforeAudits int
	if err = db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&beforeAudits); err != nil {
		t.Fatal(err)
	}
	if _, err = returns.Complete(ctx, owner, ret.ID, ret.Version); !errors.Is(err, repositories.ErrCustomerReturnState) {
		t.Fatalf("repeated return: %v", err)
	}
	if _, err = sales.Cancel(ctx, owner, sale.ID, sale.Version); !errors.Is(err, repositories.ErrSaleCompletedReturns) && !errors.Is(err, repositories.ErrSaleRecordedPayments) {
		t.Fatalf("cancellation must fail: %v", err)
	}
	if _, err = settlements.Create(ctx, owner, ret.ID, models.ReturnSettlementInput{Method: models.ReturnSettlementMethodCash, Amount: 1}); !errors.Is(err, repositories.ErrReturnSettlementOverpaid) {
		t.Fatalf("excess refund: %v", err)
	}
	checkStock(7)
	checkReturn(1500, 0, 0)
	var afterAudits, movements, delta int
	if err = db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&afterAudits); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT COUNT(*),COALESCE(SUM(quantity_delta),0) FROM inventory_movements").Scan(&movements, &delta); err != nil {
		t.Fatal(err)
	}
	if afterAudits != beforeAudits || movements != 2 || delta != -3 {
		t.Fatalf("side effects: audits %d/%d movements=%d delta=%d", beforeAudits, afterAudits, movements, delta)
	}
}

const commercialAcceptanceFixture = `
INSERT INTO users(id,username,password_hash,role,created_at,updated_at)
VALUES('accept-owner','accept-owner','unused','OWNER_LIBRARY','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO libraries(id,name,owner_user_id,created_at,updated_at)
VALUES('accept-library','Recette','accept-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
INSERT INTO defta(id,title,price,library_id) VALUES(1,'Livre recette',1500,'accept-library');
INSERT INTO book_inventory(book_id,library_id,quantity,average_unit_cost,version,updated_at)
VALUES(1,'accept-library',10,1000,1,'2026-09-01T00:00:00Z');
INSERT INTO cash_registers(id,library_id,name,normalized_name,created_by,created_at,updated_at)
VALUES('accept-register','accept-library','Caisse recette','caisse recette','accept-owner','2026-09-01T00:00:00Z','2026-09-01T00:00:00Z');
`
