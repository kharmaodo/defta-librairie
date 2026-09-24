package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestBookTagMigrationSplitsAndReusesLibraryTags(t *testing.T) {
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "book-tags.db")+"?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`
		CREATE TABLE library_tags (
			id TEXT PRIMARY KEY,
			library_id TEXT NOT NULL,
			name TEXT NOT NULL,
			normalized_name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			UNIQUE(library_id, normalized_name)
		);
		CREATE TABLE defta (
			id INTEGER PRIMARY KEY,
			library_id TEXT NOT NULL,
			tags TEXT,
			created_at TEXT,
			updated_at TEXT
		);
		INSERT INTO library_tags(id, library_id, name, normalized_name, created_at, updated_at)
		VALUES ('known-fiqh', 'library-1', 'Fiqh', 'fiqh', 'now', 'now');
		INSERT INTO defta(id, library_id, tags, created_at, updated_at)
		VALUES (1, 'library-1', 'Fiqh, Arabic, Fiqh', 'now', 'now');
	`); err != nil {
		t.Fatalf("seed legacy tags: %v", err)
	}

	migration, err := os.ReadFile("sql/031_add_book_tag_relations.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if _, err = db.Exec(string(migration)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}

	var tagCount, relationCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM library_tags WHERE library_id='library-1'`).Scan(&tagCount); err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if err = db.QueryRow(`SELECT COUNT(*) FROM book_tags WHERE book_id=1`).Scan(&relationCount); err != nil {
		t.Fatalf("count relations: %v", err)
	}
	if tagCount != 2 || relationCount != 2 {
		t.Fatalf("tags=%d relations=%d, want 2 and 2", tagCount, relationCount)
	}
	var fiqhTagID string
	if err = db.QueryRow(`
		SELECT tag_id FROM book_tags
		WHERE book_id=1 AND tag_id='known-fiqh'
	`).Scan(&fiqhTagID); err != nil {
		t.Fatalf("existing tag must be reused: %v", err)
	}
}
