//go:build fts5

package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSearchBooksUsesFTS5AndKeepsTotal(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "catalogue.db")

	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = DB.Close() })

	schema := `
		CREATE TABLE defta (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			auteur TEXT,
			editeur TEXT,
			price REAL NOT NULL DEFAULT 0,
			volume INTEGER NOT NULL DEFAULT 0,
			status TEXT,
			tags TEXT,
			categorie TEXT,
			coverUrl TEXT,
			deleted_at TEXT
		);
		CREATE VIRTUAL TABLE defta_fts USING fts5(
			title, editeur, auteur, tags, categorie,
			content='defta', content_rowid='id'
		);
		CREATE TRIGGER defta_ai AFTER INSERT ON defta BEGIN
			INSERT INTO defta_fts(rowid, title, editeur, auteur, tags, categorie)
			VALUES (new.id, new.title, new.editeur, new.auteur, new.tags, new.categorie);
		END;
	`
	if _, err = DB.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	books := []string{"ديوان طرفة", "ديوان النابغة", "شرح أسماء الله الحسنى"}
	for _, title := range books {
		if _, err = DB.Exec(`INSERT INTO defta(title, auteur) VALUES (?, ?)`, title, "Anonyme"); err != nil {
			t.Fatalf("insert %q: %v", title, err)
		}
	}

	results, total, err := SearchBooks("ديوان", 0, 1)
	if err != nil {
		t.Fatalf("search books: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(results) != 1 {
		t.Fatalf("expected one paginated result, got %d", len(results))
	}
	if !results[0].Score.Valid {
		t.Fatal("expected an FTS5 relevance score")
	}
}
