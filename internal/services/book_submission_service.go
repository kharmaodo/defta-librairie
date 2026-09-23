package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type BookSubmissionService struct {
	enabled    bool
	books      *BookService
	repository *repositories.BookSubmissionRepository
	uploader   *covers.SourceUploader
	newID      covers.IDGenerator
	now        func() time.Time
}

type PendingBookSubmission struct {
	ID string
	LibraryID string
	Status string
	ExpiresAt string
}

func NewBookSubmissionService(enabled bool, books *BookService, repository *repositories.BookSubmissionRepository, uploader *covers.SourceUploader) *BookSubmissionService {
	return &BookSubmissionService{enabled: enabled, books: books, repository: repository, uploader: uploader, newID: identity.NewID, now: time.Now}
}

func (s *BookSubmissionService) Submit(ctx context.Context, claims *auth.Claims, input models.BookInput, declaredContentType string, body io.Reader) (PendingBookSubmission, error) {
	if !s.enabled || s.books == nil || s.repository == nil || s.uploader == nil || s.newID == nil {
		return PendingBookSubmission{}, ErrCoversDisabled
	}
	libraryID, err := resolveBookScope(claims, input.LibraryID, true)
	if err != nil {
		return PendingBookSubmission{}, err
	}
	if err = s.books.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil {
		return PendingBookSubmission{}, err
	}
	input.LibraryID = libraryID
	normalizeBook(&input)
	if err = validateBook(input, false); err != nil {
		return PendingBookSubmission{}, err
	}
	submissionID, err := s.newID()
	if err != nil {
		return PendingBookSubmission{}, err
	}
	source, err := s.uploader.UploadSubmission(ctx, libraryID, submissionID, declaredContentType, body)
	if err != nil {
		return PendingBookSubmission{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return PendingBookSubmission{}, s.discardSubmissionSource(ctx, source, err)
	}
	auditID, err := s.newID()
	if err != nil {
		return PendingBookSubmission{}, s.discardSubmissionSource(ctx, source, err)
	}
	now := s.now().UTC()
	expiresAt := now.Add(24 * time.Hour).Format(time.RFC3339Nano)
	payload, err := json.Marshal(map[string]interface{}{
		"schemaVersion": 1, "eventId": eventID, "submissionId": submissionID,
		"libraryId": libraryID, "sourceObjectKey": source.ObjectKey, "attempt": 1,
	})
	if err != nil {
		return PendingBookSubmission{}, s.discardSubmissionSource(ctx, source, err)
	}
	err = s.repository.CreatePending(ctx, repositories.PendingBookSubmission{
		ID: submissionID, LibraryID: libraryID, ActorUserID: claims.Subject, Book: input,
		SourceObjectKey: source.ObjectKey, SourceContentType: source.ContentType,
		SourceFormat: source.Format, SourceWidth: source.Width, SourceHeight: source.Height,
		SourceSize: source.Size, ExpiresAt: expiresAt,
	}, eventID, string(payload), auditID, now.Format(time.RFC3339Nano))
	if err != nil {
		return PendingBookSubmission{}, s.discardSubmissionSource(ctx, source, err)
	}
	return PendingBookSubmission{ID: submissionID, LibraryID: libraryID, Status: "PENDING_SCAN", ExpiresAt: expiresAt}, nil
}

func (s *BookSubmissionService) discardSubmissionSource(ctx context.Context, source covers.StoredSource, cause error) error {
	if err := s.uploader.Discard(ctx, source); err != nil {
		return errors.Join(fmt.Errorf("%w: %v", ErrCoverPersistence, cause), fmt.Errorf("submission source compensation failed: %w", err))
	}
	return fmt.Errorf("%w: %v", ErrCoverPersistence, cause)
}
