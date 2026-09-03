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

func TestInventoryMovementsAndIsolation(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "inventory.db")+"?_foreign_keys=on")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE users(id TEXT PRIMARY KEY);
		CREATE TABLE libraries(id TEXT PRIMARY KEY);
		CREATE TABLE defta(id INTEGER PRIMARY KEY, library_id TEXT, deleted_at TEXT);
		CREATE TABLE book_inventory(book_id INTEGER PRIMARY KEY, library_id TEXT NOT NULL, quantity INTEGER NOT NULL,
			low_stock_threshold INTEGER NOT NULL, version INTEGER NOT NULL, updated_at TEXT NOT NULL);
		CREATE TABLE inventory_movements(id TEXT PRIMARY KEY, book_id INTEGER, library_id TEXT, actor_user_id TEXT,
			movement_type TEXT, quantity_delta INTEGER, quantity_before INTEGER, quantity_after INTEGER, reason TEXT, created_at TEXT);
		CREATE TABLE audit_logs(id TEXT PRIMARY KEY, actor_user_id TEXT, action TEXT, resource_type TEXT,
			resource_id TEXT, new_values TEXT, success INTEGER, created_at TEXT);
		INSERT INTO users VALUES('owner-1'),('owner-2'); INSERT INTO libraries VALUES('library-1'),('library-2');
		INSERT INTO defta VALUES(1,'library-1',NULL); INSERT INTO book_inventory VALUES(1,'library-1',0,5,1,'now');
	`)
	if err != nil { t.Fatal(err) }
	service := NewInventoryService(repositories.NewInventoryRepository(db))
	owner := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}; owner.Subject = "owner-1"
	other := &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-2"}; other.Subject = "owner-2"
	stock, err := service.Move(context.Background(), owner, 1, models.InventoryMovementEntry, 10, 1, "réception")
	if err != nil || stock.Quantity != 10 || stock.Version != 2 { t.Fatalf("entry=%+v err=%v", stock, err) }
	stock, err = service.Move(context.Background(), owner, 1, models.InventoryMovementExit, 3, 2, "vente")
	if err != nil || stock.Quantity != 7 || stock.Version != 3 { t.Fatalf("exit=%+v err=%v", stock, err) }
	if _, err = service.Move(context.Background(), owner, 1, models.InventoryMovementExit, 8, 3, "vente"); !errors.Is(err, repositories.ErrInsufficientStock) { t.Fatalf("insufficient=%v", err) }
	if _, err = service.Adjust(context.Background(), owner, 1, 12, 2, "inventaire"); !errors.Is(err, repositories.ErrInventoryConflict) { t.Fatalf("conflict=%v", err) }
	if _, err = service.Find(context.Background(), other, 1); !errors.Is(err, repositories.ErrInventoryNotFound) { t.Fatalf("cross-library=%v", err) }
	var movements, audits int
	_ = db.QueryRow(`SELECT COUNT(*) FROM inventory_movements`).Scan(&movements)
	_ = db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action='UPDATE_INVENTORY'`).Scan(&audits)
	if movements != 2 || audits != 2 { t.Fatalf("movements=%d audits=%d", movements, audits) }
}
