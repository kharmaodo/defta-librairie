package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrCoversDisabled   = errors.New("book cover uploads are disabled")
	ErrCoverPersistence = errors.New("book cover metadata could not be saved")
)

type BookCoverService struct {
	enabled  bool
	books    *BookService
	covers   *repositories.CoverRepository
	uploader *covers.SourceUploader
	newID    covers.IDGenerator
	now      func() time.Time
}

type PendingBookCover struct {
	ID          string `json:"id"`
	BookID      int    `json:"bookId"`
	LibraryID   string `json:"libraryId"`
	Status      string `json:"status"`
	ContentType string `json:"contentType"`
	Format      string `json:"format"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Size        int64  `json:"size"`
}

func NewBookCoverService(
	enabled bool,
	books *BookService,
	repository *repositories.CoverRepository,
	uploader *covers.SourceUploader,
) *BookCoverService {
	return &BookCoverService{
		enabled: enabled, books: books, covers: repository, uploader: uploader,
		newID: identity.NewID, now: time.Now,
	}
}

func (s *BookCoverService) Upload(
	ctx context.Context,
	claims *auth.Claims,
	bookID int,
	declaredContentType string,
	body io.Reader,
) (PendingBookCover, error) {
	if !s.enabled {
		return PendingBookCover{}, ErrCoversDisabled
	}
	if s.books == nil || s.covers == nil || s.uploader == nil || s.newID == nil {
		return PendingBookCover{}, ErrCoversDisabled
	}
	book, err := s.books.Find(ctx, claims, bookID)
	if err != nil {
		return PendingBookCover{}, err
	}

	source, err := s.uploader.Upload(ctx, book.LibraryID, bookID, declaredContentType, body)
	if err != nil {
		return PendingBookCover{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return PendingBookCover{}, s.discardAfterFailure(ctx, source, err)
	}
	auditID, err := s.newID()
	if err != nil {
		return PendingBookCover{}, s.discardAfterFailure(ctx, source, err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"schemaVersion":   1,
		"eventId":         eventID,
		"coverId":         source.CoverID,
		"bookId":          bookID,
		"libraryId":       book.LibraryID,
		"sourceObjectKey": source.ObjectKey,
		"attempt":         1,
	})
	if err != nil {
		return PendingBookCover{}, s.discardAfterFailure(ctx, source, err)
	}
	pending := repositories.PendingCover{
		ID: source.CoverID, BookID: bookID, LibraryID: book.LibraryID,
		SourceObjectKey: source.ObjectKey, SourceContentType: source.ContentType,
		SourceFormat: source.Format, SourceWidth: source.Width,
		SourceHeight: source.Height, SourceSize: source.Size,
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	if err = s.covers.CreatePending(
		ctx, pending, eventID, string(payload), claims.Subject, auditID, now,
	); err != nil {
		return PendingBookCover{}, s.discardAfterFailure(ctx, source, err)
	}
	return PendingBookCover{
		ID: source.CoverID, BookID: bookID, LibraryID: book.LibraryID,
		Status: "PENDING", ContentType: source.ContentType, Format: source.Format,
		Width: source.Width, Height: source.Height, Size: source.Size,
	}, nil
}

func (s *BookCoverService) discardAfterFailure(
	ctx context.Context,
	source covers.StoredSource,
	cause error,
) error {
	if err := s.uploader.Discard(ctx, source); err != nil {
		return errors.Join(
			fmt.Errorf("%w: %v", ErrCoverPersistence, cause),
			fmt.Errorf("cover source compensation failed: %w", err),
		)
	}
	return fmt.Errorf("%w: %v", ErrCoverPersistence, cause)
}
