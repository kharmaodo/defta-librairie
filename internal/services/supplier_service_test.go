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

func TestSupplierLifecycleIsolationAndAudit(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "suppliers.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY, status TEXT NOT NULL);
		CREATE TABLE suppliers(
			id TEXT PRIMARY KEY, library_id TEXT NOT NULL, name TEXT NOT NULL, normalized_name TEXT NOT NULL, contact_name TEXT,
			phone TEXT, email TEXT, address TEXT, status TEXT NOT NULL, version INTEGER NOT NULL,
			created_by TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
			UNIQUE(library_id, normalized_name), FOREIGN KEY(library_id) REFERENCES libraries(id)
		);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY,actor_user_id TEXT,action TEXT,resource_type TEXT,
			resource_id TEXT,old_values TEXT,new_values TEXT,success INTEGER,created_at TEXT);
		INSERT INTO users VALUES ('root'),('owner-1'),('owner-2');
		INSERT INTO libraries VALUES ('library-1','ACTIVE'),('library-2','ACTIVE');
	`)
	if err != nil {
		t.Fatalf("create schema: %v", err)
	}

	service := NewSupplierService(repositories.NewSupplierRepository(db))
	ownerOne := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}
	ownerOne.Subject = "owner-1"
	ownerTwo := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}
	ownerTwo.Subject = "owner-2"
	root := &auth.Claims{Role: models.RoleSuperAdminRoot}
	root.Subject = "root"

	supplier, err := service.Create(context.Background(), ownerOne, models.SupplierInput{
		Name: "  Éditions   Defta ", ContactName: "Moussa", Email: "CONTACT@EXAMPLE.COM",
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	if supplier.LibraryID != "library-1" || supplier.Name != "Éditions Defta" || supplier.Email != "contact@example.com" || supplier.Version != 1 {
		t.Fatalf("unexpected supplier: %+v", supplier)
	}
	if _, err = service.Create(context.Background(), ownerOne, models.SupplierInput{Name: "éditions defta"}); !errors.Is(err, repositories.ErrSupplierConflict) {
		t.Fatalf("duplicate supplier: %v", err)
	}
	if _, err = service.Find(context.Background(), ownerTwo, supplier.ID); !errors.Is(err, repositories.ErrSupplierNotFound) {
		t.Fatalf("cross-library supplier must be hidden: %v", err)
	}

	supplier, err = service.Update(context.Background(), ownerOne, supplier.ID, models.SupplierInput{
		Name: "Éditions Defta Sénégal", ContactName: "Moussa Diop", Email: "contact@example.com", Version: 1,
	})
	if err != nil || supplier.Version != 2 {
		t.Fatalf("update supplier=%+v err=%v", supplier, err)
	}
	if _, err = service.Update(context.Background(), ownerOne, supplier.ID, models.SupplierInput{Name: "Ancienne version", Version: 1}); !errors.Is(err, repositories.ErrSupplierVersion) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	if err = service.Disable(context.Background(), ownerOne, supplier.ID, 2); err != nil {
		t.Fatalf("disable supplier: %v", err)
	}
	if err = service.Reactivate(context.Background(), ownerOne, supplier.ID, 3); err != nil {
		t.Fatalf("reactivate supplier: %v", err)
	}

	all, total, err := service.List(context.Background(), root, "", "Defta", "", 0, 30)
	if err != nil || total != 1 || len(all) != 1 {
		t.Fatalf("root list total=%d suppliers=%+v err=%v", total, all, err)
	}
	ownerTwoList, total, err := service.List(context.Background(), ownerTwo, "", "", "", 0, 30)
	if err != nil || total != 0 || len(ownerTwoList) != 0 {
		t.Fatalf("owner isolation total=%d suppliers=%+v err=%v", total, ownerTwoList, err)
	}
	var audits int
	if err = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE resource_type='SUPPLIER'`).Scan(&audits); err != nil || audits != 4 {
		t.Fatalf("supplier audits=%d err=%v", audits, err)
	}
}

func TestSupplierValidation(t *testing.T) {
	invalid := []models.SupplierInput{{Name: "x"}, {Name: "Valid", Email: "not-an-email"}, {Name: "Valid", Version: 0}}
	if err := normalizeSupplier(&invalid[0], false); !errors.Is(err, ErrInvalidSupplier) {
		t.Fatalf("short name: %v", err)
	}
	if err := normalizeSupplier(&invalid[1], false); !errors.Is(err, ErrInvalidSupplier) {
		t.Fatalf("email: %v", err)
	}
	if err := normalizeSupplier(&invalid[2], true); !errors.Is(err, ErrInvalidSupplier) {
		t.Fatalf("version: %v", err)
	}
}
