package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var (
	ErrCoverBookNotFound   = errors.New("cover book not found")
	ErrActiveCoverNotFound = errors.New("active book cover not found")
)

type PendingCover struct {
	ID                string
	BookID            int
	LibraryID         string
	SourceObjectKey   string
	SourceContentType string
	SourceFormat      string
	SourceWidth       int
	SourceHeight      int
	SourceSize        int64
}

type CoverRepository struct {
	db *sql.DB
}

func NewCoverRepository(db *sql.DB) *CoverRepository {
	return &CoverRepository{db: db}
}

func (r *CoverRepository) CreatePending(
	ctx context.Context,
	cover PendingCover,
	eventID, payload, actorID, auditID, now string,
) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin pending cover: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM defta
		WHERE id=? AND library_id=? AND deleted_at IS NULL
	`, cover.BookID, cover.LibraryID).Scan(&exists); err != nil {
		return fmt.Errorf("check cover book: %w", err)
	}
	if exists != 1 {
		return ErrCoverBookNotFound
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO book_covers(
			id, book_id, library_id, status, source_object_key,
			source_content_type, source_format, source_width, source_height,
			source_size, active, created_at, updated_at
		) VALUES (?, ?, ?, 'PENDING', ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, cover.ID, cover.BookID, cover.LibraryID, cover.SourceObjectKey,
		cover.SourceContentType, cover.SourceFormat, cover.SourceWidth,
		cover.SourceHeight, cover.SourceSize, now, now); err != nil {
		return fmt.Errorf("insert pending cover: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO cover_processing_outbox(
			event_id, cover_id, book_id, library_id, event_type,
			schema_version, payload, attempts, available_at, created_at
		) VALUES (?, ?, ?, ?, 'book.covers.process.v1', 1, ?, 0, ?, ?)
	`, eventID, cover.ID, cover.BookID, cover.LibraryID, payload, now, now); err != nil {
		return fmt.Errorf("insert cover outbox: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO audit_logs(
			id, actor_user_id, action, resource_type, resource_id,
			new_values, success, created_at
		) VALUES (?, ?, 'UPLOAD_BOOK_COVER', 'BOOK', ?, ?, 1, ?)
	`, auditID, actorID, cover.BookID, payload, now); err != nil {
		return fmt.Errorf("audit pending cover: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit pending cover: %w", err)
	}
	return nil
}


type ActiveCoverVariant struct {
	CoverID    string
	ObjectKey  string
	ContentType string
	UpdatedAt  string
}

func (r *CoverRepository) ActiveVariant(
	ctx context.Context,
	bookID int,
	libraryID string,
	variant string,
	format string,
) (ActiveCoverVariant, error) {
	if bookID < 1 || libraryID == "" {
		return ActiveCoverVariant{}, ErrActiveCoverNotFound
	}

	column := ""
	contentType := ""
	switch variant + "/" + format {
	case "master/jpeg":
		column, contentType = "master_object_key", "image/jpeg"
	case "large/jpeg":
		column, contentType = "large_jpeg_object_key", "image/jpeg"
	case "large/webp":
		column, contentType = "large_webp_object_key", "image/webp"
	case "thumb/jpeg":
		column, contentType = "thumb_jpeg_object_key", "image/jpeg"
	case "thumb/webp":
		column, contentType = "thumb_webp_object_key", "image/webp"
	default:
		return ActiveCoverVariant{}, ErrActiveCoverNotFound
	}

	var result ActiveCoverVariant
	query := `
		SELECT c.id, c.` + column + `, c.updated_at
		FROM book_covers c
		JOIN defta b ON b.id = c.book_id
		WHERE c.book_id = ?
		  AND c.library_id = ?
		  AND c.status = 'READY'
		  AND c.active = 1
		  AND c.` + column + ` IS NOT NULL
		  AND b.deleted_at IS NULL
	`
	err := r.db.QueryRowContext(ctx, query, bookID, libraryID).Scan(
		&result.CoverID,
		&result.ObjectKey,
		&result.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ActiveCoverVariant{}, ErrActiveCoverNotFound
	}
	if err != nil {
		return ActiveCoverVariant{}, fmt.Errorf("find active cover variant: %w", err)
	}
	result.ContentType = contentType
	return result, nil
}
