package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
)

var (
	ErrBookNotFound       = errors.New("book not found")
	ErrBookConflict       = errors.New("book was modified by another request")
	ErrLibraryUnavailable = errors.New("library not found or disabled")
)

type BookRepository struct{ db *sql.DB }

func NewBookRepository(db *sql.DB) *BookRepository { return &BookRepository{db: db} }

func (r *BookRepository) LibraryActive(ctx context.Context, libraryID string) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM libraries WHERE id=? AND status='ACTIVE'`, libraryID).Scan(&count); err != nil {
		return false, fmt.Errorf("check library: %w", err)
	}
	return count == 1, nil
}

func (r *BookRepository) List(ctx context.Context, libraryID string, offset, limit int) ([]models.Book, int, error) {
	where := ` WHERE deleted_at IS NULL`
	args := make([]interface{}, 0, 3)
	if libraryID != "" {
		where += ` AND library_id=?`
		args = append(args, libraryID)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM defta`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count managed books: %w", err)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, bookSelect+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list managed books: %w", err)
	}
	defer rows.Close()
	books := make([]models.Book, 0)
	for rows.Next() {
		book, scanErr := scanManagedBook(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan managed book: %w", scanErr)
		}
		books = append(books, book)
	}
	return books, total, rows.Err()
}

func (r *BookRepository) Search(ctx context.Context, libraryID, query string, offset, limit int) ([]models.Book, int, error) {
	where := " WHERE defta_fts MATCH ? AND d.deleted_at IS NULL"
	args := []interface{}{query}
	if libraryID != "" {
		where += " AND d.library_id=?"
		args = append(args, libraryID)
	}
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM defta_fts JOIN defta d ON defta_fts.rowid=d.id
	`+where, args...).Scan(&total)
	if err == nil {
		queryArgs := append(append([]interface{}{}, args...), limit, offset)
		rows, queryErr := r.db.QueryContext(ctx, managedBookSearchSelect+where+` ORDER BY defta_fts.rank LIMIT ? OFFSET ?`, queryArgs...)
		if queryErr == nil {
			defer rows.Close()
			books := make([]models.Book, 0)
			for rows.Next() {
				book, scanErr := scanManagedSearchBook(rows)
				if scanErr != nil {
					return nil, 0, fmt.Errorf("scan managed FTS book: %w", scanErr)
				}
				books = append(books, book)
			}
			return books, total, rows.Err()
		}
	}
	return r.searchLike(ctx, libraryID, query, offset, limit)
}

func (r *BookRepository) searchLike(ctx context.Context, libraryID, query string, offset, limit int) ([]models.Book, int, error) {
	pattern := "%" + query + "%"
	where := ` WHERE deleted_at IS NULL AND (
		title LIKE ? OR auteur LIKE ? OR editeur LIKE ? OR tags LIKE ? OR categorie LIKE ?)`
	args := []interface{}{pattern, pattern, pattern, pattern, pattern}
	if libraryID != "" {
		where += " AND library_id=?"
		args = append(args, libraryID)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM defta"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count managed LIKE books: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, bookSelect+where+" ORDER BY id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search managed LIKE books: %w", err)
	}
	defer rows.Close()
	books := make([]models.Book, 0)
	for rows.Next() {
		book, scanErr := scanManagedBook(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan managed LIKE book: %w", scanErr)
		}
		books = append(books, book)
	}
	return books, total, rows.Err()
}

func (r *BookRepository) Find(ctx context.Context, id int, libraryID string) (models.Book, error) {
	query := bookSelect + ` WHERE id=? AND deleted_at IS NULL`
	args := []interface{}{id}
	if libraryID != "" {
		query += ` AND library_id=?`
		args = append(args, libraryID)
	}
	book, err := scanManagedBook(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return models.Book{}, ErrBookNotFound
	}
	if err != nil {
		return models.Book{}, fmt.Errorf("find managed book: %w", err)
	}
	return book, nil
}

func (r *BookRepository) Create(ctx context.Context, book models.BookInput, actorID, auditID, now string) (models.Book, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Book{}, fmt.Errorf("begin book creation: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO defta(title, auteur, editeur, price, volume, status, tags, categorie, coverUrl,
		                  library_id, created_at, updated_at, version)
		VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, NULLIF(?, ''), NULLIF(?, ''),
		        NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, 1)
	`, book.Title, book.Auteur, book.Editeur, book.Price, book.Volume, book.Status, book.Tags,
		book.Categorie, book.CoverURL, book.LibraryID, now, now)
	if err != nil {
		return models.Book{}, fmt.Errorf("insert book: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Book{}, fmt.Errorf("read book id: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'CREATE_BOOK', 'BOOK', ?, '{"created":true}', 1, ?)
	`, auditID, actorID, id, now); err != nil {
		return models.Book{}, fmt.Errorf("audit book creation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Book{}, fmt.Errorf("commit book creation: %w", err)
	}
	return r.Find(ctx, int(id), book.LibraryID)
}

func (r *BookRepository) Update(ctx context.Context, id int, book models.BookInput, actorID, auditID, now string) (models.Book, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Book{}, fmt.Errorf("begin book update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE defta SET title=?, auteur=NULLIF(?, ''), editeur=NULLIF(?, ''), price=?, volume=?,
		                 status=NULLIF(?, ''), tags=NULLIF(?, ''), categorie=NULLIF(?, ''),
		                 coverUrl=NULLIF(?, ''), updated_at=?, version=version+1
		WHERE id=? AND library_id=? AND deleted_at IS NULL AND version=?
	`, book.Title, book.Auteur, book.Editeur, book.Price, book.Volume, book.Status, book.Tags,
		book.Categorie, book.CoverURL, now, id, book.LibraryID, book.Version)
	if err != nil {
		return models.Book{}, fmt.Errorf("update book: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return models.Book{}, fmt.Errorf("updated book rows: %w", err)
	}
	if rows != 1 {
		return models.Book{}, ErrBookConflict
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_BOOK', 'BOOK', ?, '{"updated":true}', 1, ?)
	`, auditID, actorID, id, now); err != nil {
		return models.Book{}, fmt.Errorf("audit book update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Book{}, fmt.Errorf("commit book update: %w", err)
	}
	return r.Find(ctx, id, book.LibraryID)
}

func (r *BookRepository) Delete(ctx context.Context, id int, libraryID, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin book deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE defta SET deleted_at=?, updated_at=?, version=version+1
		WHERE id=? AND library_id=? AND deleted_at IS NULL
	`, now, now, id, libraryID)
	if err != nil {
		return fmt.Errorf("delete book: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return ErrBookNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'DELETE_BOOK', 'BOOK', ?, '{"deleted":true}', 1, ?)
	`, auditID, actorID, id, now); err != nil {
		return fmt.Errorf("audit book deletion: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit book deletion: %w", err)
	}
	return nil
}

const bookSelect = `
	SELECT id, title, auteur, editeur, COALESCE(price, 0), COALESCE(volume, 0),
	       status, tags, categorie, coverUrl,
	       library_id, COALESCE(created_at, ''), COALESCE(updated_at, ''), version
	FROM defta`

const managedBookSearchSelect = `
	SELECT d.id, d.title, d.auteur, d.editeur, COALESCE(d.price, 0), COALESCE(d.volume, 0),
	       d.status, d.tags, d.categorie, d.coverUrl,
	       d.library_id, COALESCE(d.created_at, ''), COALESCE(d.updated_at, ''), d.version,
	       defta_fts.rank
	FROM defta_fts JOIN defta d ON defta_fts.rowid=d.id`

func scanManagedBook(row rowScanner) (models.Book, error) {
	var book models.Book
	err := row.Scan(&book.ID, &book.Title, &book.Auteur, &book.Editeur, &book.Price, &book.Volume,
		&book.Status, &book.Tags, &book.Categorie, &book.CoverURL, &book.LibraryID,
		&book.CreatedAt, &book.UpdatedAt, &book.Version)
	return book, err
}

func scanManagedSearchBook(row rowScanner) (models.Book, error) {
	var book models.Book
	err := row.Scan(&book.ID, &book.Title, &book.Auteur, &book.Editeur, &book.Price, &book.Volume,
		&book.Status, &book.Tags, &book.Categorie, &book.CoverURL, &book.LibraryID,
		&book.CreatedAt, &book.UpdatedAt, &book.Version, &book.Score)
	return book, err
}
