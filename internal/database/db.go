package database

import (
	"fmt"
	"database/sql"
	"log"
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
func SearchBooks(query string, offset, limit int) ([]models.Book, int, error) {
    var rows *sql.Rows
    var err error
    var total int

    isFTS := query != ""

    if !isFTS {
        // Liste complète (sans rank)
        err = DB.QueryRow("SELECT COUNT(*) FROM defta").Scan(&total)
        if err != nil {
            return nil, 0, fmt.Errorf("count failed: %w", err)
        }

        rows, err = DB.Query(`
            SELECT 
                id, title, auteur, editeur, price, volume, 
                status, tags, categorie, coverUrl
            FROM defta
            ORDER BY id DESC
            LIMIT ? OFFSET ?
        `, limit, offset)
    } else {
        // Recherche FTS5 (avec rank)
        err = DB.QueryRow(`
            SELECT COUNT(*)
            FROM defta_fts
            WHERE defta_fts MATCH ?
        `, query).Scan(&total)
        if err != nil {
            return nil, 0, fmt.Errorf("fts count failed: %w", err)
        }

        rows, err = DB.Query(`
            SELECT 
                d.id, d.title, d.auteur, d.editeur, d.price, d.volume, 
                d.status, d.tags, d.categorie, d.coverUrl,
                rank
            FROM defta_fts fts
            JOIN defta d ON fts.rowid = d.id
            WHERE fts MATCH ?
            ORDER BY rank
            LIMIT ? OFFSET ?
        `, query, limit, offset)
    }

    if err != nil {
        return nil, 0, fmt.Errorf("query failed: %w", err)
    }
    defer rows.Close()

    var books []models.Book

    for rows.Next() {
        var b models.Book
        var score sql.NullFloat64

        if isFTS {
            // 11 colonnes (avec rank)
            err = rows.Scan(
                &b.ID, &b.Title, &b.Auteur, &b.Editeur, &b.Price,
                &b.Volume, &b.Status, &b.Tags, &b.Categorie, &b.CoverURL,
                &score,
            )
        } else {
            // 10 colonnes (sans rank)
            err = rows.Scan(
                &b.ID, &b.Title, &b.Auteur, &b.Editeur, &b.Price,
                &b.Volume, &b.Status, &b.Tags, &b.Categorie, &b.CoverURL,
            )
        }

        if err != nil {
            return nil, 0, fmt.Errorf("scan failed: %w", err)
        }

        if score.Valid {
            b.Score = score
        }

        books = append(books, b)
    }

    if err = rows.Err(); err != nil {
        return nil, 0, fmt.Errorf("rows error: %w", err)
    }

    return books, total, nil
}