package repositories

import (
	"context"
	"database/sql"
	"defta-librairie/internal/models"
	"errors"
	"fmt"
)

var (
	ErrTagNotFound = errors.New("tag not found")
	ErrTagConflict = errors.New("tag already exists")
)

type TagRepository struct{ db *sql.DB }

func NewTagRepository(db *sql.DB) *TagRepository { return &TagRepository{db: db} }

func (r *TagRepository) List(ctx context.Context, libraryID string) ([]models.LibraryTag, error) {
	query := `SELECT id, library_id, name, created_at, updated_at FROM library_tags`
	args := make([]interface{}, 0, 1)
	if libraryID != "" {
		query += ` WHERE library_id=?`
		args = append(args, libraryID)
	}
	query += ` ORDER BY normalized_name`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	tags := make([]models.LibraryTag, 0)
	for rows.Next() {
		var tag models.LibraryTag
		if err = rows.Scan(&tag.ID, &tag.LibraryID, &tag.Name, &tag.CreatedAt, &tag.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (r *TagRepository) Find(ctx context.Context, id string) (models.LibraryTag, error) {
	var tag models.LibraryTag
	err := r.db.QueryRowContext(ctx, `
		SELECT id, library_id, name, created_at, updated_at FROM library_tags WHERE id=?
	`, id).Scan(&tag.ID, &tag.LibraryID, &tag.Name, &tag.CreatedAt, &tag.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return models.LibraryTag{}, ErrTagNotFound
	}
	if err != nil {
		return models.LibraryTag{}, fmt.Errorf("find tag: %w", err)
	}
	return tag, nil
}

func (r *TagRepository) Create(ctx context.Context, tag models.LibraryTag, normalizedName, actorID, auditID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag creation: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO library_tags(id, library_id, name, normalized_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, tag.ID, tag.LibraryID, tag.Name, normalizedName, tag.CreatedAt, tag.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrTagConflict
	}
	if err != nil {
		return fmt.Errorf("insert tag: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, new_values, success, created_at)
		VALUES (?, ?, 'CREATE_LIBRARY_TAG', 'TAG', ?, ?, 1, ?)
	`, auditID, actorID, tag.ID, tag.Name, tag.CreatedAt); err != nil {
		return fmt.Errorf("audit tag creation: %w", err)
	}
	return tx.Commit()
}

func (r *TagRepository) Update(ctx context.Context, tag models.LibraryTag, normalizedName, oldName, actorID, auditID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag update: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE library_tags SET name=?, normalized_name=?, updated_at=? WHERE id=? AND library_id=?
	`, tag.Name, normalizedName, tag.UpdatedAt, tag.ID, tag.LibraryID)
	if isUniqueViolation(err) {
		return ErrTagConflict
	}
	if err != nil {
		return fmt.Errorf("update tag: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrTagNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, old_values, new_values, success, created_at)
		VALUES (?, ?, 'UPDATE_LIBRARY_TAG', 'TAG', ?, ?, ?, 1, ?)
	`, auditID, actorID, tag.ID, oldName, tag.Name, tag.UpdatedAt); err != nil {
		return fmt.Errorf("audit tag update: %w", err)
	}
	return tx.Commit()
}

func (r *TagRepository) Delete(ctx context.Context, tag models.LibraryTag, actorID, auditID, now string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tag deletion: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM library_tags WHERE id=? AND library_id=?`, tag.ID, tag.LibraryID)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return ErrTagNotFound
	}
	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(id, actor_user_id, action, resource_type, resource_id, old_values, success, created_at)
		VALUES (?, ?, 'DELETE_LIBRARY_TAG', 'TAG', ?, ?, 1, ?)
	`, auditID, actorID, tag.ID, tag.Name, now); err != nil {
		return fmt.Errorf("audit tag deletion: %w", err)
	}
	return tx.Commit()
}

func (r *TagRepository) LibraryActive(ctx context.Context, id string) (bool, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `SELECT status FROM libraries WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrLibraryUnavailable
	}
	if err != nil {
		return false, fmt.Errorf("find tag library: %w", err)
	}
	return status == string(models.LibraryStatusActive), nil
}
