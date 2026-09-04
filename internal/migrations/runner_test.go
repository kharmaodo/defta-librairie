package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestRunMigratesLegacyCatalogueAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE defta (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			price INTEGER,
			title TEXT NOT NULL,
			editeur TEXT,
			tags INTEGER,
			categorie INTEGER,
			status TEXT,
			volume INTEGER DEFAULT 1,
			auteur TEXT,
			coverUrl TEXT
		);
		CREATE VIRTUAL TABLE defta_fts USING fts5(
			title, editeur, auteur, tags, categorie,
			content='defta', content_rowid='id'
		);
		CREATE TRIGGER defta_ai AFTER INSERT ON defta BEGIN
			INSERT INTO defta_fts(rowid, title, editeur, auteur, tags, categorie)
			VALUES (new.id, new.title, new.editeur, new.auteur, new.tags, new.categorie);
		END;
		INSERT INTO defta(title) VALUES ('كتاب أول'), ('كتاب ثان');
	`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err = Run(context.Background(), db); err != nil {
			t.Fatalf("migration run %d: %v", i+1, err)
		}
	}

	var migrationsCount int
	if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationsCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationsCount != 15 {
		t.Fatalf("expected 15 migrations, got %d", migrationsCount)
	}

	var assignedBooks int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM defta
		WHERE library_id = '00000000-0000-0000-0000-000000000001'
		  AND created_at IS NOT NULL
		  AND updated_at IS NOT NULL
		  AND version = 1
	`).Scan(&assignedBooks); err != nil {
		t.Fatalf("count assigned books: %v", err)
	}
	if assignedBooks != 2 {
		t.Fatalf("expected 2 assigned legacy books, got %d", assignedBooks)
	}
	var initializedInventory int
	if err = db.QueryRow(`SELECT COUNT(*) FROM book_inventory WHERE quantity=0 AND version=1`).Scan(&initializedInventory); err != nil {
		t.Fatalf("count initialized inventory: %v", err)
	}
	if initializedInventory != 2 {
		t.Fatalf("expected inventory for 2 legacy books, got %d", initializedInventory)
	}

	var salesTables int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name IN ('sales', 'sale_lines')
	`).Scan(&salesTables); err != nil {
		t.Fatalf("inspect sales tables: %v", err)
	}
	if salesTables != 2 {
		t.Fatalf("expected sales tables, got %d", salesTables)
	}

	var purchaseTables int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name IN ('suppliers', 'purchases', 'purchase_lines')
	`).Scan(&purchaseTables); err != nil {
		t.Fatalf("inspect supplier and purchase tables: %v", err)
	}
	if purchaseTables != 3 {
		t.Fatalf("expected supplier and purchase tables, got %d", purchaseTables)
	}

	var customerTables int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='table' AND name='customers'
	`).Scan(&customerTables); err != nil {
		t.Fatalf("inspect customer table: %v", err)
	}
	if customerTables != 1 {
		t.Fatalf("expected customer table, got %d", customerTables)
	}
	var customerIDColumns int
	if err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sales') WHERE name='customer_id'`).Scan(&customerIDColumns); err != nil {
		t.Fatalf("inspect sale customer link: %v", err)
	}
	if customerIDColumns != 1 {
		t.Fatalf("expected customer_id on sales, got %d", customerIDColumns)
	}

	if _, err = db.Exec("UPDATE defta SET library_id = 'missing-library' WHERE id = 1"); err == nil {
		t.Fatal("expected foreign key violation for an unknown library")
	}
}

func TestRunInitializesFreshDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fresh.db")
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err = Run(context.Background(), db); err != nil {
		t.Fatalf("migrate fresh database: %v", err)
	}

	var catalogueTables int
	if err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type IN ('table', 'view') AND name IN ('defta', 'defta_fts')
	`).Scan(&catalogueTables); err != nil {
		t.Fatalf("inspect fresh database: %v", err)
	}
	if catalogueTables != 2 {
		t.Fatalf("expected catalogue and FTS tables, got %d", catalogueTables)
	}

	if _, err = db.Exec(`INSERT INTO defta(title, tags) VALUES ('Livre FTS neuf', 'test')`); err != nil {
		t.Fatalf("insert fresh book: %v", err)
	}
	var matches int
	if err = db.QueryRow(`SELECT COUNT(*) FROM defta_fts WHERE defta_fts MATCH 'neuf'`).Scan(&matches); err != nil {
		t.Fatalf("search fresh catalogue: %v", err)
	}
	if matches != 1 {
		t.Fatalf("expected one FTS match, got %d", matches)
	}
}
