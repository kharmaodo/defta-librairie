package services

import (
	"context"
	"database/sql"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCashRegisterLifecycleIsolationAndAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "cash-registers.db")+"?_foreign_keys=on")
	if err != nil { t.Fatalf("open database: %v", err) }
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY,status TEXT NOT NULL);
		CREATE TABLE cash_registers(
			id TEXT PRIMARY KEY,library_id TEXT NOT NULL,name TEXT NOT NULL,normalized_name TEXT NOT NULL,
			status TEXT NOT NULL,version INTEGER NOT NULL,created_by TEXT NOT NULL,
			created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(library_id,normalized_name),
			FOREIGN KEY(library_id) REFERENCES libraries(id));
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES ('root'),('owner-1'),('owner-2');
		INSERT INTO libraries VALUES ('library-1','ACTIVE'),('library-2','ACTIVE');`)
	if err != nil { t.Fatalf("create schema: %v", err) }

	service := NewCashRegisterService(repositories.NewCashRegisterRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}; ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}; ownerTwo.Subject = "owner-2"
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}; root.Subject = "root"

	register, err := service.Create(context.Background(), ownerOne, models.CashRegisterInput{Name: "  Caisse   principale "})
	if err != nil || register.LibraryID != "library-1" || register.Name != "Caisse principale" || register.Version != 1 {
		t.Fatalf("create register=%+v err=%v", register, err)
	}
	if _, err = service.Create(context.Background(), ownerOne, models.CashRegisterInput{Name: "CAISSE PRINCIPALE"});
		!errors.Is(err, repositories.ErrCashRegisterConflict) { t.Fatalf("expected duplicate conflict, got %v", err) }
	if _, err = service.Find(context.Background(), ownerTwo, register.ID); !errors.Is(err, repositories.ErrCashRegisterNotFound) {
		t.Fatalf("cross-library register must be hidden: %v", err)
	}
	register, err = service.Update(context.Background(), ownerOne, register.ID,
		models.CashRegisterInput{Name: "Caisse comptoir", Version: 1})
	if err != nil || register.Version != 2 { t.Fatalf("update register=%+v err=%v", register, err) }
	if _, err = service.Update(context.Background(), ownerOne, register.ID,
		models.CashRegisterInput{Name: "Ancienne", Version: 1}); !errors.Is(err, repositories.ErrCashRegisterVersion) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if err = service.Disable(context.Background(), ownerOne, register.ID, 2); err != nil { t.Fatalf("disable: %v", err) }
	if err = service.Reactivate(context.Background(), ownerOne, register.ID, 3); err != nil { t.Fatalf("reactivate: %v", err) }
	results, total, err := service.List(context.Background(), root, "library-1",
		models.CashRegisterFilter{Query: "comptoir", Status: models.CashRegisterStatusActive}, 0, 30)
	if err != nil || total != 1 || len(results) != 1 { t.Fatalf("list total=%d results=%+v err=%v", total, results, err) }
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='CASH_REGISTER'`).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("cash register audits=%d err=%v", audits, err)
	}
}

func TestCashRegisterValidation(t *testing.T) {
	if _, _, err := normalizeCashRegisterName("x"); !errors.Is(err, ErrInvalidCashRegister) { t.Fatalf("short name: %v", err) }
	if _, _, err := normalizeCashRegisterName(strings.Repeat("x", 81)); !errors.Is(err, ErrInvalidCashRegister) { t.Fatalf("long name: %v", err) }
}
