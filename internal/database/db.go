package database

import (
    "database/sql"
    "fmt"
    "log"
    "strings"
	"defta-librairie/internal/models"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(path string) error {
	var err error
	DB, err = sql.Open("sqlite3", path+"?_journal_mode=WAL&mode=rwc")
	if err != nil {
		return err
	}

	if err = DB.Ping(); err != nil {
		return err
	}

	log.Printf("Base SQLite connectée → %s", path)
	return nil
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
        err = DB.QueryRow("SELECT COUNT(*) FROM defta").Scan(&total)
        if err != nil {
            return nil, 0, fmt.Errorf("count all failed: %w", err)
        }

        rows, err = DB.Query(`
            SELECT id, title, auteur, editeur, price, volume,
                   status, tags, categorie, coverUrl
            FROM defta
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
        FROM defta_fts
        WHERE defta_fts MATCH ?
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
            WHERE defta_fts MATCH ?
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
        WHERE title LIKE ?
           OR auteur LIKE ?
           OR editeur LIKE ?
    `, likePattern, likePattern, likePattern).Scan(&total)

    if err != nil {
        return nil, 0, fmt.Errorf("count fallback failed: %w", err)
    }

    rows, err = DB.Query(`
        SELECT
            id, title, auteur, editeur, price, volume,
            status, tags, categorie, coverUrl
        FROM defta
        WHERE title LIKE ?
           OR auteur LIKE ?
           OR editeur LIKE ?
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
