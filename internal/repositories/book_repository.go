package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
	"strings"
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
		if taxonomyErr := r.loadBookCategories(ctx, &book); taxonomyErr != nil {
			return nil, 0, taxonomyErr
		}
		if tagErr := r.loadBookTags(ctx, &book); tagErr != nil {
			return nil, 0, tagErr
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
				if taxonomyErr := r.loadBookCategories(ctx, &book); taxonomyErr != nil {
			return nil, 0, taxonomyErr
		}
		if tagErr := r.loadBookTags(ctx, &book); tagErr != nil {
			return nil, 0, tagErr
		}
		books = append(books, book)
			}
			return books, total, rows.Err()
		}
	}
	return r.searchLike(ctx, libraryID, query, offset, limit)
}

func (r *BookRepository) SearchByTag(ctx context.Context, libraryID, tagID, query string, offset, limit int) ([]models.Book, int, error) {
	where := " WHERE d.deleted_at IS NULL AND bt.tag_id=?"
	args := []interface{}{tagID}
	if libraryID != "" {
		where += " AND d.library_id=?"
		args = append(args, libraryID)
	}
	if query = strings.TrimSpace(query); query != "" {
		pattern := "%" + query + "%"
		where += " AND (d.title LIKE ? OR d.auteur LIKE ? OR d.editeur LIKE ? OR d.tags LIKE ? OR d.categorie LIKE ?)"
		args = append(args, pattern, pattern, pattern, pattern, pattern)
	}
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM defta d JOIN book_tags bt ON bt.book_id=d.id"+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tagged books: %w", err)
	}
	queryArgs := append(append([]interface{}{}, args...), limit, offset)
	rows, err := r.db.QueryContext(ctx, taggedBookSelect+where+" ORDER BY d.id DESC LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("search tagged books: %w", err)
	}
	defer rows.Close()
	books := make([]models.Book, 0)
	for rows.Next() {
		book, scanErr := scanManagedBook(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan tagged book: %w", scanErr)
		}
		if err = r.loadBookCategories(ctx, &book); err != nil {
			return nil, 0, err
		}
		if err = r.loadBookTags(ctx, &book); err != nil {
			return nil, 0, err
		}
		books = append(books, book)
	}
	return books, total, rows.Err()
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
		if taxonomyErr := r.loadBookCategories(ctx, &book); taxonomyErr != nil {
			return nil, 0, taxonomyErr
		}
		if tagErr := r.loadBookTags(ctx, &book); tagErr != nil {
			return nil, 0, tagErr
		}
		books = append(books, book)
	}
	return books, total, rows.Err()
}

func (r *BookRepository) loadBookCategories(ctx context.Context, book *models.Book) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT category_id FROM book_categories
		WHERE book_id=?
		ORDER BY is_primary DESC, category_id
	`, book.ID)
	if err != nil {
		return fmt.Errorf("list book categories: %w", err)
	}
	defer rows.Close()
	book.CategoryIDs = make([]int, 0)
	for rows.Next() {
		var categoryID int
		if err = rows.Scan(&categoryID); err != nil {
			return fmt.Errorf("scan book category: %w", err)
		}
		book.CategoryIDs = append(book.CategoryIDs, categoryID)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate book categories: %w", err)
	}
	return nil
}

func (r *BookRepository) loadBookTags(ctx context.Context, book *models.Book) error {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tag_id FROM book_tags
		WHERE book_id=?
		ORDER BY tag_id
	`, book.ID)
	if err != nil {
		return fmt.Errorf("list book tags: %w", err)
	}
	defer rows.Close()
	book.TagIDs = make([]string, 0)
	for rows.Next() {
		var tagID string
		if err = rows.Scan(&tagID); err != nil {
			return fmt.Errorf("scan book tag: %w", err)
		}
		book.TagIDs = append(book.TagIDs, tagID)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("iterate book tags: %w", err)
	}
	return nil
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
	if err = r.loadBookCategories(ctx, &book); err != nil {
		return models.Book{}, err
	}
	if err = r.loadBookTags(ctx, &book); err != nil {
		return models.Book{}, err
	}
	return book, nil
}

func (r *BookRepository) FindLibraryID(ctx context.Context, id int) (string, error) {
	var libraryID string
	err := r.db.QueryRowContext(ctx, `SELECT library_id FROM defta WHERE id=?`, id).Scan(&libraryID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrBookNotFound
	}
	if err != nil {
		return "", fmt.Errorf("find book library: %w", err)
	}
	return libraryID, nil
}

func (r *BookRepository) Create(ctx context.Context, book models.BookInput, actorID, auditID, newValues, now string) (models.Book, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Book{}, fmt.Errorf("begin book creation: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO defta(title, auteur, editeur, publisher_id, price, volume, status, tags, categorie, coverUrl,
		                  library_id, created_at, updated_at, version)
		VALUES (?, NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''),
		        NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, 1)
	`, book.Title, book.Auteur, book.Editeur, book.PublisherID, book.Price, book.Volume, book.Status, book.Tags,
		book.Categorie, book.CoverURL, book.LibraryID, now, now)
	if err != nil {
		return models.Book{}, fmt.Errorf("insert book: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return models.Book{}, fmt.Errorf("read book id: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO book_inventory(book_id, library_id, quantity, low_stock_threshold, version, updated_at)
		VALUES (?, ?, 0, COALESCE((SELECT default_low_stock_threshold FROM library_settings WHERE library_id=?),5), 1, ?)
	`, id, book.LibraryID, book.LibraryID, now); err != nil {
		return models.Book{}, fmt.Errorf("initialize book inventory: %w", err)
	}
	if book.CategoryIDs != nil {
		if err = replaceBookCategories(ctx, tx, id, book.CategoryIDs, book.PrimaryCategoryID, now); err != nil {
			return models.Book{}, fmt.Errorf("set book categories: %w", err)
		}
	}
	if book.TagIDs != nil {
		if err = replaceBookTags(ctx, tx, id, book.TagIDs, now); err != nil {
			return models.Book{}, fmt.Errorf("set book tags: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'CREATE_BOOK', 'BOOK', ?, ?, 1, ?)
	`, auditID, actorID, id, newValues, now); err != nil {
		return models.Book{}, fmt.Errorf("audit book creation: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Book{}, fmt.Errorf("commit book creation: %w", err)
	}
	return r.Find(ctx, int(id), book.LibraryID)
}

func (r *BookRepository) Update(ctx context.Context, id int, book models.BookInput, actorID, auditID, oldValues, newValues, now string) (models.Book, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Book{}, fmt.Errorf("begin book update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE defta SET title=?, auteur=NULLIF(?, ''), editeur=NULLIF(?, ''),
		                 publisher_id=CASE WHEN ? IS NULL THEN publisher_id ELSE ? END,
		                 price=?, volume=?,
		                 status=NULLIF(?, ''), tags=NULLIF(?, ''), categorie=NULLIF(?, ''),
		                 coverUrl=NULLIF(?, ''), updated_at=?, version=version+1
		WHERE id=? AND library_id=? AND deleted_at IS NULL AND version=?
	`, book.Title, book.Auteur, book.Editeur, book.PublisherID, book.PublisherID, book.Price, book.Volume,
		book.Status, book.Tags, book.Categorie, book.CoverURL, now, id, book.LibraryID, book.Version)
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
	if book.CategoryIDs != nil {
		if err = replaceBookCategories(ctx, tx, int64(id), book.CategoryIDs, book.PrimaryCategoryID, now); err != nil {
			return models.Book{}, fmt.Errorf("replace book categories: %w", err)
		}
	}
	if book.TagIDs != nil {
		if err = replaceBookTags(ctx, tx, int64(id), book.TagIDs, now); err != nil {
			return models.Book{}, fmt.Errorf("replace book tags: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, old_values, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_BOOK', 'BOOK', ?, ?, ?, 1, ?)
	`, auditID, actorID, id, oldValues, newValues, now); err != nil {
		return models.Book{}, fmt.Errorf("audit book update: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return models.Book{}, fmt.Errorf("commit book update: %w", err)
	}
	return r.Find(ctx, id, book.LibraryID)
}

func (r *BookRepository) Delete(ctx context.Context, id int, libraryID, actorID, auditID, oldValues, now string) error {
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
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, old_values, new_values, success, created_at)
		VALUES (?, ?, 'DELETE_BOOK', 'BOOK', ?, ?, '{"deleted":true}', 1, ?)
	`, auditID, actorID, id, oldValues, now); err != nil {
		return fmt.Errorf("audit book deletion: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit book deletion: %w", err)
	}
	return nil
}

func replaceBookCategories(ctx context.Context, tx *sql.Tx, bookID int64, categoryIDs []int, primaryID *int, now string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM book_categories WHERE book_id=?", bookID); err != nil {
		return err
	}
	for _, categoryID := range categoryIDs {
		primary := 0
		if primaryID != nil && categoryID == *primaryID {
			primary = 1
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO book_categories(book_id, category_id, is_primary, created_at) VALUES (?, ?, ?, ?)", bookID, categoryID, primary, now); err != nil {
			return err
		}
	}
	return nil
}

func replaceBookTags(ctx context.Context, tx *sql.Tx, bookID int64, tagIDs []string, now string) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM book_tags WHERE book_id=?", bookID); err != nil {
		return err
	}
	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO book_tags(book_id, tag_id, created_at) VALUES (?, ?, ?)",
			bookID, tagID, now,
		); err != nil {
			return err
		}
	}
	return nil
}

func (r *BookRepository) History(ctx context.Context, id int, offset, limit int) ([]models.AuditLog, int, error) {
	resourceID := fmt.Sprintf("%d", id)
	var total int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM audit_logs WHERE resource_type='BOOK' AND resource_id=?
	`, resourceID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count book history: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, COALESCE(a.actor_user_id, ''), COALESCE(u.username, ''), a.action,
		       a.resource_type, COALESCE(a.resource_id, ''), COALESCE(a.old_values, ''),
		       COALESCE(a.new_values, ''), COALESCE(a.ip_address, ''), a.success, a.created_at
		FROM audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id
		WHERE a.resource_type='BOOK' AND a.resource_id=?
		ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?
	`, resourceID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list book history: %w", err)
	}
	defer rows.Close()
	logs := make([]models.AuditLog, 0)
	for rows.Next() {
		var log models.AuditLog
		if err = rows.Scan(&log.ID, &log.ActorUserID, &log.ActorUsername, &log.Action,
			&log.ResourceType, &log.ResourceID, &log.OldValues, &log.NewValues,
			&log.IPAddress, &log.Success, &log.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan book history: %w", err)
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

const bookSelect = `
	SELECT id, title, auteur, editeur, COALESCE(price, 0), COALESCE(volume, 0),
	       status, tags, categorie, publisher_id,
	       (SELECT category_id FROM book_categories WHERE book_id=defta.id AND is_primary=1), coverUrl,
	       EXISTS(SELECT 1 FROM book_covers c WHERE c.book_id=defta.id AND c.active=1 AND c.status='READY'),
	       library_id, COALESCE(created_at, ''), COALESCE(updated_at, ''), version
	FROM defta`

const taggedBookSelect = `
	SELECT d.id, d.title, d.auteur, d.editeur, COALESCE(d.price, 0), COALESCE(d.volume, 0),
	       d.status, d.tags, d.categorie, d.publisher_id,
	       (SELECT category_id FROM book_categories WHERE book_id=d.id AND is_primary=1), d.coverUrl,
	       EXISTS(SELECT 1 FROM book_covers c WHERE c.book_id=d.id AND c.active=1 AND c.status='READY'),
	       d.library_id, COALESCE(d.created_at, ''), COALESCE(d.updated_at, ''), d.version
	FROM defta d JOIN book_tags bt ON bt.book_id=d.id`

const managedBookSearchSelect = `
	SELECT d.id, d.title, d.auteur, d.editeur, COALESCE(d.price, 0), COALESCE(d.volume, 0),
	       d.status, d.tags, d.categorie, d.publisher_id,
	       (SELECT category_id FROM book_categories WHERE book_id=d.id AND is_primary=1), d.coverUrl,
	       EXISTS(SELECT 1 FROM book_covers c WHERE c.book_id=d.id AND c.active=1 AND c.status='READY'),
	       d.library_id, COALESCE(d.created_at, ''), COALESCE(d.updated_at, ''), d.version,
	       defta_fts.rank
	FROM defta_fts JOIN defta d ON defta_fts.rowid=d.id`

func scanManagedBook(row rowScanner) (models.Book, error) {
	var book models.Book
	var hasActiveCover int
	err := row.Scan(&book.ID, &book.Title, &book.Auteur, &book.Editeur, &book.Price, &book.Volume,
		&book.Status, &book.Tags, &book.Categorie, &book.PublisherID, &book.PrimaryCategoryID, &book.CoverURL, &hasActiveCover, &book.LibraryID,
		&book.CreatedAt, &book.UpdatedAt, &book.Version)
	book.HasActiveCover = hasActiveCover == 1
	return book, err
}

func scanManagedSearchBook(row rowScanner) (models.Book, error) {
	var book models.Book
	var hasActiveCover int
	err := row.Scan(&book.ID, &book.Title, &book.Auteur, &book.Editeur, &book.Price, &book.Volume,
		&book.Status, &book.Tags, &book.Categorie, &book.PublisherID, &book.PrimaryCategoryID,
		&book.CoverURL, &hasActiveCover, &book.LibraryID, &book.CreatedAt, &book.UpdatedAt, &book.Version, &book.Score)
	book.HasActiveCover = hasActiveCover == 1
	return book, err
}
