package services

import (
	"context"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"strings"
	"time"
)

type CoverCleanupStore interface {
	ClaimNext(
		ctx context.Context,
		workerID string,
		now time.Time,
		leaseUntil time.Time,
	) (repositories.CoverCleanupJob, error)
	MarkCompleted(ctx context.Context, jobID int64, workerID string, completedAt time.Time) error
	MarkFailed(
		ctx context.Context,
		jobID int64,
		workerID string,
		availableAt time.Time,
		lastError string,
	) error
}

type CoverCleanupService struct {
	store         CoverCleanupStore
	objects       covers.ObjectDeleter
	workerID      string
	leaseDuration time.Duration
	baseRetry     time.Duration
	maxRetry      time.Duration
	now           func() time.Time
}

func NewCoverCleanupService(
	store CoverCleanupStore,
	objects covers.ObjectDeleter,
	workerID string,
) (*CoverCleanupService, error) {
	if store == nil || objects == nil || workerID == "" {
		return nil, fmt.Errorf("invalid cover cleanup configuration")
	}
	return &CoverCleanupService{
		store: store, objects: objects, workerID: workerID,
		leaseDuration: time.Minute,
		baseRetry:     30 * time.Second,
		maxRetry:      time.Hour,
		now:           time.Now,
	}, nil
}

func (s *CoverCleanupService) CleanAvailable(
	ctx context.Context,
	limit int,
) (int, error) {
	if limit < 1 {
		return 0, nil
	}

	cleaned := 0
	for cleaned < limit {
		if err := ctx.Err(); err != nil {
			return cleaned, err
		}
		now := s.now().UTC()
		job, err := s.store.ClaimNext(
			ctx,
			s.workerID,
			now,
			now.Add(s.leaseDuration),
		)
		if errors.Is(err, repositories.ErrCoverCleanupEmpty) {
			return cleaned, nil
		}
		if err != nil {
			return cleaned, fmt.Errorf("claim cover cleanup: %w", err)
		}

		if err = s.objects.DeleteObject(ctx, job.ObjectKey); err != nil {
			recordErr := s.store.MarkFailed(
				ctx,
				job.ID,
				s.workerID,
				now.Add(s.retryDelay(job.Attempts)),
				coverCleanupError(err),
			)
			if recordErr != nil {
				return cleaned, errors.Join(
					fmt.Errorf("delete cover object: %w", err),
					fmt.Errorf("record cleanup failure: %w", recordErr),
				)
			}
			return cleaned, fmt.Errorf("delete cover object: %w", err)
		}

		if err = s.store.MarkCompleted(
			ctx,
			job.ID,
			s.workerID,
			now,
		); err != nil {
			return cleaned, fmt.Errorf("acknowledge cover cleanup: %w", err)
		}
		cleaned++
	}
	return cleaned, nil
}

func (s *CoverCleanupService) retryDelay(previousAttempts int) time.Duration {
	if previousAttempts < 0 {
		previousAttempts = 0
	}
	delay := s.baseRetry
	for index := 0; index < previousAttempts && delay < s.maxRetry; index++ {
		if delay > s.maxRetry/2 {
			return s.maxRetry
		}
		delay *= 2
	}
	if delay > s.maxRetry {
		return s.maxRetry
	}
	return delay
}

func coverCleanupError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.NewReplacer(
		"\n", " ",
		"\r", " ",
		"\t", " ",
	).Replace(err.Error())
	message = strings.Join(strings.Fields(message), " ")
	const maxRunes = 512
	runes := []rune(message)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return message
}
