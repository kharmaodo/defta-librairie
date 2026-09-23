package services

import (
	"context"
	"crypto/sha256"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
	"encoding/hex"
	"io"
)

type ActiveBookCover struct {
	Body        io.ReadCloser
	ContentType string
	ETag        string
}

type BookCoverReadService struct {
	books      *BookService
	repository *repositories.CoverRepository
	reader     covers.VariantReader
}

func NewBookCoverReadService(
	books *BookService,
	repository *repositories.CoverRepository,
	reader covers.VariantReader,
) *BookCoverReadService {
	return &BookCoverReadService{
		books: books, repository: repository, reader: reader,
	}
}

func (s *BookCoverReadService) Open(
	ctx context.Context,
	claims *auth.Claims,
	bookID int,
	variant string,
	format string,
) (ActiveBookCover, error) {
	if s.books == nil || s.repository == nil || s.reader == nil {
		return ActiveBookCover{}, ErrCoversDisabled
	}

	book, err := s.books.Find(ctx, claims, bookID)
	if err != nil {
		return ActiveBookCover{}, err
	}

	return s.openVariant(ctx, bookID, book.LibraryID, variant, format)
}

// OpenPublic exposes only the active, processed representation of a book that
// is already visible in the public catalogue. Source objects remain private.
func (s *BookCoverReadService) OpenPublic(ctx context.Context, bookID int, variant, format string) (ActiveBookCover, error) {
	if s.repository == nil || s.reader == nil {
		return ActiveBookCover{}, ErrCoversDisabled
	}
	book, err := s.repository.Find(ctx, bookID, "")
	if err != nil {
		return ActiveBookCover{}, err
	}
	return s.openVariant(ctx, bookID, book.LibraryID, variant, format)
}

func (s *BookCoverReadService) openVariant(ctx context.Context, bookID int, libraryID, variant, format string) (ActiveBookCover, error) {
	stored, err := s.repository.ActiveVariant(ctx, bookID, libraryID, variant, format)
	if err != nil {
		return ActiveBookCover{}, err
	}
	body, err := s.reader.OpenVariant(ctx, stored.ObjectKey)
	if err != nil {
		return ActiveBookCover{}, err
	}
	digest := sha256.Sum256([]byte(stored.CoverID + "\x00" + stored.ObjectKey + "\x00" + stored.UpdatedAt))
	return ActiveBookCover{Body: body, ContentType: stored.ContentType, ETag: "\"" + hex.EncodeToString(digest[:]) + "\""}, nil
}
