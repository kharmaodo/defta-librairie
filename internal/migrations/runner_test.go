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
	if migrationsCount != 5 {
		t.Fatalf("expected 5 migrations, got %d", migrationsCount)
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

	if _, err = db.Exec("UPDATE defta SET library_id = 'missing-library' WHERE id = 1"); err == nil {
		t.Fatal("expected foreign key violation for an unknown library")
	}
}
