package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
)

type CoverProcessingStore interface {
	Claim(
		ctx context.Context,
		coverID string,
		bookID int,
		libraryID string,
		sourceObjectKey string,
		workerID string,
		now time.Time,
		leaseUntil time.Time,
	) (repositories.CoverProcessingClaim, error)
	Release(ctx context.Context, coverID, workerID, errorCode string, now time.Time) error
	MarkFailed(ctx context.Context, coverID, errorCode string, now time.Time) error
	Complete(
		ctx context.Context,
		coverID string,
		libraryID string,
		workerID string,
		processed repositories.ProcessedCover,
		now time.Time,
	) error
}

type CoverVariantProcessor interface {
	Process(ctx context.Context, event covers.ProcessingEvent) (repositories.ProcessedCover, error)
}

type BookCoverWorker struct {
	store         CoverProcessingStore
	processor     CoverVariantProcessor
	workerID      string
	leaseDuration time.Duration
	now           func() time.Time
}

func NewBookCoverWorker(
	store CoverProcessingStore,
	processor CoverVariantProcessor,
	workerID string,
) (*BookCoverWorker, error) {
	if store == nil || processor == nil || workerID == "" {
		return nil, fmt.Errorf("invalid book cover worker configuration")
	}
	return &BookCoverWorker{
		store: store, processor: processor, workerID: workerID,
		leaseDuration: 2 * time.Minute,
		now:           time.Now,
	}, nil
}

func (w *BookCoverWorker) Handle(
	ctx context.Context,
	event covers.ProcessingEvent,
) error {
	now := w.now().UTC()
	claim, err := w.store.Claim(
		ctx,
		event.CoverID,
		event.BookID,
		event.LibraryID,
		event.SourceObjectKey,
		w.workerID,
		now,
		now.Add(w.leaseDuration),
	)
	if err != nil {
		if errors.Is(err, repositories.ErrCoverProcessingNotFound) ||
			errors.Is(err, repositories.ErrCoverProcessingTerminal) {
			return fmt.Errorf("%w: %v", covers.ErrPermanentCoverProcessing, err)
		}
		return err
	}
	if claim == repositories.CoverProcessingAlreadyReady {
		return nil
	}

	processed, processErr := w.processor.Process(ctx, event)
	if processErr != nil {
		releaseErr := w.store.Release(
			ctx, event.CoverID, w.workerID, "PROCESSING_RETRY", w.now().UTC(),
		)
		if releaseErr != nil {
			return errors.Join(processErr, fmt.Errorf("release cover processing: %w", releaseErr))
		}
		return processErr
	}
	if err = w.store.Complete(
		ctx,
		event.CoverID,
		event.LibraryID,
		w.workerID,
		processed,
		w.now().UTC(),
	); err != nil {
		releaseErr := w.store.Release(
			ctx, event.CoverID, w.workerID, "COMPLETION_RETRY", w.now().UTC(),
		)
		if releaseErr != nil {
			return errors.Join(err, fmt.Errorf("release failed completion: %w", releaseErr))
		}
		return err
	}
	return nil
}

func (w *BookCoverWorker) MarkFailed(
	ctx context.Context,
	event covers.ProcessingEvent,
	_ error,
) error {
	err := w.store.MarkFailed(
		ctx, event.CoverID, "MAX_DELIVERIES", w.now().UTC(),
	)
	if errors.Is(err, repositories.ErrCoverProcessingTerminal) {
		return nil
	}
	return err
}
