package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"encoding/json"
	"fmt"
	"time"
)

type BookCoverStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
	CanRetry  bool   `json:"canRetry"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *BookCoverService) Status(
	ctx context.Context,
	claims *auth.Claims,
	bookID int,
) (BookCoverStatus, error) {
	if !s.enabled || s.books == nil || s.covers == nil {
		return BookCoverStatus{}, ErrCoversDisabled
	}
	book, err := s.books.Find(ctx, claims, bookID)
	if err != nil {
		return BookCoverStatus{}, err
	}
	status, err := s.covers.LatestStatus(ctx, book.ID, book.LibraryID)
	if err != nil {
		return BookCoverStatus{}, err
	}
	return publicBookCoverStatus(status), nil
}

func (s *BookCoverService) Retry(
	ctx context.Context,
	claims *auth.Claims,
	bookID int,
) (BookCoverStatus, error) {
	if !s.enabled || s.books == nil || s.covers == nil || s.newID == nil || s.now == nil {
		return BookCoverStatus{}, ErrCoversDisabled
	}
	book, err := s.books.Find(ctx, claims, bookID)
	if err != nil {
		return BookCoverStatus{}, err
	}
	current, err := s.covers.LatestStatus(ctx, book.ID, book.LibraryID)
	if err != nil {
		return BookCoverStatus{}, err
	}
	if current.Status != "FAILED" || !current.SourceRetained {
		return BookCoverStatus{}, repositories.ErrBookCoverNotRetryable
	}

	eventID, err := s.newID()
	if err != nil {
		return BookCoverStatus{}, coverRetryPersistenceError(err)
	}
	auditID, err := s.newID()
	if err != nil {
		return BookCoverStatus{}, coverRetryPersistenceError(err)
	}
	payload, err := json.Marshal(map[string]interface{}{
		"schemaVersion":   1,
		"eventId":         eventID,
		"coverId":         current.ID,
		"bookId":          book.ID,
		"libraryId":       book.LibraryID,
		"sourceObjectKey": current.SourceObjectKey,
		"attempt":         1,
	})
	if err != nil {
		return BookCoverStatus{}, coverRetryPersistenceError(err)
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	retried, err := s.covers.RetryFailed(
		ctx,
		book.ID,
		book.LibraryID,
		eventID,
		string(payload),
		claims.Subject,
		auditID,
		now,
	)
	if err != nil {
		return BookCoverStatus{}, err
	}
	return publicBookCoverStatus(retried), nil
}

func publicBookCoverStatus(status repositories.BookCoverStatus) BookCoverStatus {
	return BookCoverStatus{
		ID:        status.ID,
		Status:    status.Status,
		ErrorCode: status.ErrorCode,
		CanRetry:  status.Status == "FAILED" && status.SourceRetained,
		UpdatedAt: status.UpdatedAt,
	}
}

func coverRetryPersistenceError(err error) error {
	return fmt.Errorf("%w: %v", ErrCoverPersistence, err)
}
