package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidBookTags = errors.New("invalid book tags")

type BookTagRepository struct {
	db *sql.DB
}

func NewBookTagRepository(db *sql.DB) *BookTagRepository {
	return &BookTagRepository{db: db}
}

func (r *BookTagRepository) ValidateSelection(ctx context.Context, libraryID string, tagIDs []string) error {
	seen := make(map[string]struct{}, len(tagIDs))
	for _, tagID := range tagIDs {
		tagID = strings.TrimSpace(tagID)
		if tagID == "" {
			return ErrInvalidBookTags
		}
		if _, exists := seen[tagID]; exists {
			return ErrInvalidBookTags
		}
		seen[tagID] = struct{}{}
	}
	if len(tagIDs) == 0 {
		return nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(tagIDs)), ",")
	args := make([]interface{}, 0, len(tagIDs)+1)
	args = append(args, libraryID)
	for _, tagID := range tagIDs {
		args = append(args, strings.TrimSpace(tagID))
	}
	var count int
	if err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM library_tags WHERE library_id=? AND id IN ("+placeholders+")",
		args...,
	).Scan(&count); err != nil {
		return fmt.Errorf("validate book tags: %w", err)
	}
	if count != len(tagIDs) {
		return ErrInvalidBookTags
	}
	return nil
}

func (r *BookTagRepository) Tags(ctx context.Context, bookID int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT tag_id FROM book_tags
		WHERE book_id=?
		ORDER BY tag_id
	`, bookID)
	if err != nil {
		return nil, fmt.Errorf("list book tags: %w", err)
	}
	defer rows.Close()
	tagIDs := make([]string, 0)
	for rows.Next() {
		var tagID string
		if err = rows.Scan(&tagID); err != nil {
			return nil, fmt.Errorf("scan book tag: %w", err)
		}
		tagIDs = append(tagIDs, tagID)
	}
	return tagIDs, rows.Err()
}
