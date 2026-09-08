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

func TestReturnSettlementLifecycleBalanceIsolationAndAudit(t *testing.T){
	db,err:=sql.Open("sqlite3",filepath.Join(t.TempDir(),"settlements.db")+"?_foreign_keys=on");if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = db.Close()})
	_,err=db.Exec(`CREATE TABLE customer_returns(id TEXT PRIMARY KEY,library_id TEXT,resolution TEXT,status TEXT,total_amount REAL,UNIQUE(id,library_id));
		CREATE TABLE return_settlements(id TEXT PRIMARY KEY,library_id TEXT,return_id TEXT,method TEXT,amount REAL,external_reference TEXT,notes TEXT,status TEXT,version INTEGER,issued_by TEXT,voided_by TEXT,created_at TEXT,updated_at TEXT,voided_at TEXT,UNIQUE(library_id,method,external_reference));
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		CREATE TRIGGER settlement_context BEFORE INSERT ON return_settlements WHEN NOT EXISTS(SELECT 1 FROM customer_returns r WHERE r.id=NEW.return_id AND r.library_id=NEW.library_id AND r.status='COMPLETED' AND ((r.resolution='CREDIT_NOTE' AND NEW.method='CREDIT_NOTE') OR (r.resolution='REFUND' AND NEW.method<>'CREDIT_NOTE'))) BEGIN SELECT RAISE(ABORT,'customer return settlement is unavailable');END;
		CREATE TRIGGER settlement_amount BEFORE INSERT ON return_settlements WHEN COALESCE((SELECT SUM(amount) FROM return_settlements WHERE return_id=NEW.return_id AND status='ISSUED'),0)+NEW.amount>(SELECT total_amount FROM customer_returns WHERE id=NEW.return_id) BEGIN SELECT RAISE(ABORT,'customer return settlement exceeds return amount');END;
		CREATE VIEW customer_return_balances AS SELECT r.id return_id,r.library_id,r.total_amount,COALESCE(SUM(CASE WHEN s.status='ISSUED' THEN s.amount ELSE 0 END),0) settled_amount,r.total_amount-COALESCE(SUM(CASE WHEN s.status='ISSUED' THEN s.amount ELSE 0 END),0) remaining_amount,CASE WHEN COALESCE(SUM(CASE WHEN s.status='ISSUED' THEN s.amount ELSE 0 END),0)=0 THEN 'PENDING' WHEN COALESCE(SUM(CASE WHEN s.status='ISSUED' THEN s.amount ELSE 0 END),0)<r.total_amount THEN 'PARTIALLY_SETTLED' ELSE 'SETTLED' END settlement_status FROM customer_returns r LEFT JOIN return_settlements s ON s.return_id=r.id GROUP BY r.id,r.library_id,r.total_amount;
		INSERT INTO customer_returns VALUES('return-1','library-1','REFUND','COMPLETED',3000);INSERT INTO customer_returns VALUES('return-2','library-2','REFUND','COMPLETED',1000);INSERT INTO customer_returns VALUES('credit-1','library-1','CREDIT_NOTE','COMPLETED',500);`);if err!=nil{t.Fatal(err)}
	service:=NewReturnSettlementService(repositories.NewReturnSettlementRepository(db));owner:=&auth.Claims{Role:models.RoleOwnerLibrary,LibraryID:"library-1"};owner.Subject="owner-1";other:=&auth.Claims{Role:models.RoleOwnerLibrary,LibraryID:"library-2"};other.Subject="owner-2"
	first,err:=service.Create(context.Background(),owner,"return-1",models.ReturnSettlementInput{Method:models.ReturnSettlementMethodCash,Amount:1000});if err!=nil{t.Fatalf("create: %v",err)}
	balance,err:=service.Balance(context.Background(),owner,"return-1");if err!=nil||balance.RemainingAmount!=2000||balance.SettlementStatus!="PARTIALLY_SETTLED"{t.Fatalf("balance=%+v err=%v",balance,err)}
	if _,err=service.Balance(context.Background(),other,"return-1");!errors.Is(err,repositories.ErrReturnSettlementReturnNotFound){t.Fatalf("cross-library balance: %v",err)}
	if _,err=service.Create(context.Background(),owner,"return-1",models.ReturnSettlementInput{Method:models.ReturnSettlementMethodCash,Amount:2500});!errors.Is(err,repositories.ErrReturnSettlementOverpaid){t.Fatalf("over settlement: %v",err)}
	if _,err=service.Create(context.Background(),owner,"credit-1",models.ReturnSettlementInput{Method:models.ReturnSettlementMethodCash,Amount:500});!errors.Is(err,repositories.ErrReturnSettlementUnavailable){t.Fatalf("wrong resolution: %v",err)}
	first,err=service.Void(context.Background(),owner,first.ID,models.ReturnSettlementVoidInput{Version:first.Version,Reason:"Erreur de saisie"});if err!=nil||first.Status!=models.ReturnSettlementStatusVoided{t.Fatalf("void=%+v err=%v",first,err)}
	balance,err=service.Balance(context.Background(),owner,"return-1");if err!=nil||balance.RemainingAmount!=3000||balance.SettlementStatus!="PENDING"{t.Fatalf("void balance=%+v err=%v",balance,err)}
	var audits int;_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action IN('ISSUE_RETURN_SETTLEMENT','VOID_RETURN_SETTLEMENT')`).Scan(&audits);if audits!=2{t.Fatalf("audits=%d",audits)}
}
