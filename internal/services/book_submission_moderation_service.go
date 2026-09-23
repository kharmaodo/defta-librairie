package services

import (
	"context"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/moderation"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"io"
	"time"
)

var (
	ErrInvalidModerationWorker = errors.New("invalid book submission moderation worker")
	ErrBookSubmissionModerationFailed = errors.New("book submission moderation failed")
)

type submissionSourceReader interface {
	OpenSource(ctx context.Context, key string) (io.ReadCloser, error)
}

type BookSubmissionModerationService struct {
	repository *repositories.BookSubmissionRepository
	sources    submissionSourceReader
	client     moderation.Client
	newID      func() (string, error)
	now        func() time.Time
}

func NewBookSubmissionModerationService(
	repository *repositories.BookSubmissionRepository,
	sources submissionSourceReader,
	client moderation.Client,
) *BookSubmissionModerationService {
	return &BookSubmissionModerationService{
		repository: repository,
		sources:    sources,
		client:     client,
		newID:      identity.NewID,
		now:        time.Now,
	}
}

func (s *BookSubmissionModerationService) Process(ctx context.Context, submissionID string) (int, error) {
	if s == nil || s.repository == nil || s.sources == nil || s.client == nil || s.newID == nil ||
		submissionID == "" {
		return 0, ErrInvalidModerationWorker
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	submission, err := s.repository.ClaimForModeration(ctx, submissionID, now)
	if err != nil {
		return 0, err
	}
	source, err := s.sources.OpenSource(ctx, submission.SourceObjectKey)
	if err != nil {
		return 0, s.fail(ctx, submission.ID, "SOURCE_UNAVAILABLE", err)
	}
	defer source.Close()
	image, err := io.ReadAll(io.LimitReader(source, submission.SourceSize+1))
	if err != nil || int64(len(image)) != submission.SourceSize {
		if err == nil {
			err = errors.New("unexpected quarantined source size")
		}
		return 0, s.fail(ctx, submission.ID, "SOURCE_INVALID", err)
	}
	result, err := s.client.Moderate(ctx, submission.SourceContentType, image)
	if err != nil {
		return 0, s.fail(ctx, submission.ID, "MODERATOR_UNAVAILABLE", err)
	}
	decisionAuditID, err := s.newID()
	if err != nil {
		return 0, s.fail(ctx, submission.ID, "WORKER_ID_FAILURE", err)
	}
	bookAuditID := ""
	decisionCode := "MODEL_REVIEW"
	if result.Class == "SAFE" {
		decisionCode = "MODEL_SAFE"
		bookAuditID, err = s.newID()
		if err != nil {
			return 0, s.fail(ctx, submission.ID, "WORKER_ID_FAILURE", err)
		}
	} else if result.Class == "UNSAFE" {
		decisionCode = "MODEL_UNSAFE"
	}
	createdBookID, err := s.repository.CompleteModeration(
		ctx, submission, result.Class, result.Score, result.ModelVersion,
		decisionCode, decisionAuditID, bookAuditID, s.now().UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, err
	}
	return createdBookID, nil
}

func (s *BookSubmissionModerationService) fail(ctx context.Context, submissionID, code string, cause error) error {
	auditID, err := s.newID()
	if err == nil {
		err = s.repository.FailModeration(ctx, submissionID, code, auditID, s.now().UTC().Format(time.RFC3339Nano))
	}
	if err != nil {
		return errors.Join(fmt.Errorf("moderate book submission: %w", cause), err)
	}
	return fmt.Errorf("%w: %w", ErrBookSubmissionModerationFailed, cause)
}

var _ submissionSourceReader = (*covers.MinIOProcessingStore)(nil)
