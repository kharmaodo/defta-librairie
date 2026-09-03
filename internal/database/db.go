package database

import (
	"context"
	"database/sql"
	"defta-librairie/internal/migrations"
	"defta-librairie/internal/models"
	"fmt"
	"io"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var DB *sql.DB

func Init(path string) error {
	seeded, err := ensureDatabaseFile(path)
	if err != nil {
		return err
	}
	if seeded {
		log.Printf("Catalogue initial copié vers → %s", path)
	}
	DB, err = sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000&mode=rwc")
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}
	if err = migrations.Run(context.Background(), DB); err != nil {
		return fmt.Errorf("database migrations: %w", err)
	}

	log.Printf("Base SQLite connectée → %s", path)
	return nil
}

func ensureDatabaseFile(path string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("inspect SQLite database: %w", err)
	}
	seedPath := filepath.Join(filepath.Dir(path), "catalogue.seed.db")
	seed, err := os.Open(seedPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("open catalogue seed: %w", err)
	}
	defer seed.Close()
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return false, fmt.Errorf("create SQLite directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".defta-seed-*.db")
	if err != nil {
		return false, fmt.Errorf("create temporary SQLite database: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = io.Copy(temporary, seed)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, fmt.Errorf("copy catalogue seed: %w", err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return false, fmt.Errorf("install catalogue seed: %w", err)
	}
	return true, nil
}

func Close() {
	if DB != nil {
		DB.Close()
		log.Println("Connexion SQLite fermée")
	}
}

// SearchBooks retourne des livres paginés
// - si query vide → tous les livres (de la table defta)
// - sinon → recherche FTS5 avec rank
// SearchBooks recherche des livres avec ou sans FTS5
func SearchBooks(query string, offset, limit int) ([]models.Book, int, error) {
	query = strings.TrimSpace(query)
	var total int
	var rows *sql.Rows
	var err error

	// ────────────────────────────────────────────────
	// 1. Cas sans recherche → tous les livres
	// ────────────────────────────────────────────────
	if query == "" {
		err = DB.QueryRow("SELECT COUNT(*) FROM defta WHERE deleted_at IS NULL").Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("count all failed: %w", err)
		}

		rows, err = DB.Query(`
            SELECT id, title, auteur, editeur, price, volume,
                   status, tags, categorie, coverUrl
            FROM defta
            WHERE deleted_at IS NULL
            ORDER BY id DESC
            LIMIT ? OFFSET ?
        `, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("query all failed: %w", err)
		}
		defer rows.Close()

		books, err := scanBooks(rows, false)
		return books, total, err
	}

	// ────────────────────────────────────────────────
	// 2. Tentative FTS5 (si activé)
	// ────────────────────────────────────────────────
	ftsQuery := query // on peut améliorer plus tard (ex: query + "*")

	err = DB.QueryRow(`
        SELECT COUNT(*)
        FROM defta_fts fts
        JOIN defta d ON fts.rowid = d.id
        WHERE defta_fts MATCH ? AND d.deleted_at IS NULL
    `, ftsQuery).Scan(&total)

	if err == nil {
		// FTS5 est disponible → on utilise le rank
		log.Printf("FTS5 activé → recherche avec MATCH '%s'", ftsQuery)

		rows, err = DB.Query(`
            SELECT
                d.id, d.title, d.auteur, d.editeur, d.price, d.volume,
                d.status, d.tags, d.categorie, d.coverUrl,
                rank
            FROM defta_fts fts
            JOIN defta d ON fts.rowid = d.id
            WHERE defta_fts MATCH ? AND d.deleted_at IS NULL
            ORDER BY fts.rank
            LIMIT ? OFFSET ?
        `, ftsQuery, limit, offset)

		if err == nil {
			defer rows.Close()
			books, scanErr := scanBooks(rows, true)
			return books, total, scanErr
		}
	}

	// ────────────────────────────────────────────────
	// 3. Fallback LIKE si FTS5 échoue
	// ────────────────────────────────────────────────
	log.Printf("FTS5 non disponible ou erreur → fallback LIKE : %v", err)

	likePattern := "%" + query + "%"

	err = DB.QueryRow(`
        SELECT COUNT(*)
        FROM defta
        WHERE deleted_at IS NULL
          AND (title LIKE ?
           OR auteur LIKE ?
           OR editeur LIKE ?)
    `, likePattern, likePattern, likePattern).Scan(&total)

	if err != nil {
		return nil, 0, fmt.Errorf("count fallback failed: %w", err)
	}

	rows, err = DB.Query(`
        SELECT
            id, title, auteur, editeur, price, volume,
            status, tags, categorie, coverUrl
        FROM defta
        WHERE deleted_at IS NULL
          AND (title LIKE ?
           OR auteur LIKE ?
           OR editeur LIKE ?)
        ORDER BY id DESC
        LIMIT ? OFFSET ?
    `, likePattern, likePattern, likePattern, limit, offset)

	if err != nil {
		return nil, 0, fmt.Errorf("query fallback failed: %w", err)
	}
	defer rows.Close()

	books, scanErr := scanBooks(rows, false)
	return books, total, scanErr
}

// scanBooks factorise le scan (avec ou sans rank)
func scanBooks(rows *sql.Rows, withRank bool) ([]models.Book, error) {
	var books []models.Book

	for rows.Next() {
		var b models.Book
		var score sql.NullFloat64

		args := []interface{}{
			&b.ID, &b.Title, &b.Auteur, &b.Editeur, &b.Price,
			&b.Volume, &b.Status, &b.Tags, &b.Categorie, &b.CoverURL,
		}

		if withRank {
			args = append(args, &score)
		}

		err := rows.Scan(args...)
		if err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		if withRank && score.Valid {
			b.Score = score
		}

		books = append(books, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return books, nil
}
