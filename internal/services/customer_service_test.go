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

func TestCustomerLifecycleIsolationAndAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "customers.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY, status TEXT NOT NULL);
		CREATE TABLE customers(
			id TEXT PRIMARY KEY,library_id TEXT NOT NULL,reference TEXT NOT NULL,name TEXT NOT NULL,
			phone TEXT,email TEXT,address TEXT,notes TEXT,status TEXT NOT NULL,version INTEGER NOT NULL,
			created_by TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,
			UNIQUE(library_id,reference),FOREIGN KEY(library_id) REFERENCES libraries(id)
		);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES ('root'),('owner-1'),('owner-2');
		INSERT INTO libraries VALUES ('library-1','ACTIVE'),('library-2','ACTIVE');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewCustomerService(repositories.NewCustomerRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root"

	customer, err := service.Create(context.Background(), ownerOne, models.CustomerInput{
		Name: "  Aïssatou   Diop ", Phone: "+221 77 000 00 00", Email: "AISSA@EXAMPLE.COM",
	})
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	if customer.LibraryID != "library-1" || customer.Name != "Aïssatou Diop" ||
		customer.Email != "aissa@example.com" || customer.Version != 1 || customer.Reference == "" {
		t.Fatalf("unexpected customer: %+v", customer)
	}
	if _, err = service.Create(context.Background(), ownerOne, models.CustomerInput{Name: "Aïssatou Diop"}); err != nil {
		t.Fatalf("customers may share a name: %v", err)
	}
	if _, err = service.Find(context.Background(), ownerTwo, customer.ID); !errors.Is(err, repositories.ErrCustomerNotFound) {
		t.Fatalf("cross-library customer must be hidden: %v", err)
	}

	customer, err = service.Update(context.Background(), ownerOne, customer.ID, models.CustomerInput{
		Name: "Aïssatou Fall", Phone: "+221 77 000 00 00", Email: "aissa@example.com", Notes: "Cliente fidèle", Version: 1,
	})
	if err != nil || customer.Version != 2 {
		t.Fatalf("update customer=%+v err=%v", customer, err)
	}
	if _, err = service.Update(context.Background(), ownerOne, customer.ID,
		models.CustomerInput{Name: "Ancienne version", Version: 1}); !errors.Is(err, repositories.ErrCustomerVersion) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if err = service.Disable(context.Background(), ownerOne, customer.ID, 2); err != nil {
		t.Fatalf("disable customer: %v", err)
	}
	if err = service.Reactivate(context.Background(), ownerOne, customer.ID, 3); err != nil {
		t.Fatalf("reactivate customer: %v", err)
	}

	results, total, err := service.List(context.Background(), root, "library-1",
		models.CustomerFilter{Query: "Fall", Status: models.CustomerStatusActive}, 0, 30)
	if err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("root customer list total=%d results=%+v err=%v", total, results, err)
	}
	ownerTwoList, total, err := service.List(context.Background(), ownerTwo, "", models.CustomerFilter{}, 0, 30)
	if err != nil || total != 0 || len(ownerTwoList) != 0 {
		t.Fatalf("owner isolation total=%d customers=%+v err=%v", total, ownerTwoList, err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='CUSTOMER'`).Scan(&audits); err != nil || audits != 5 {
		t.Fatalf("customer audits=%d err=%v", audits, err)
	}
}

func TestCustomerValidation(t *testing.T) {
	invalid := []models.CustomerInput{{Name: "x"}, {Name: "Valid", Email: "not-an-email"}, {Name: "Valid", Version: 0}}
	if err := normalizeCustomer(&invalid[0], false); !errors.Is(err, ErrInvalidCustomer) {
		t.Fatalf("short name: %v", err)
	}
	if err := normalizeCustomer(&invalid[1], false); !errors.Is(err, ErrInvalidCustomer) {
		t.Fatalf("email: %v", err)
	}
	if err := normalizeCustomer(&invalid[2], true); !errors.Is(err, ErrInvalidCustomer) {
		t.Fatalf("version: %v", err)
	}
}
