package services

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"errors"
	"fmt"
	"io"
	"time"
)

type reviewSourceStore interface {
	CopySource(context.Context, string, string, int64, string) error
	DeleteObject(context.Context, string) error
	OpenSource(context.Context, string) (io.ReadCloser, error)
}

func (s *CoverImportReviewService) OpenSource(ctx context.Context, claims *auth.Claims, jobID, requestedLibrary string) (io.ReadCloser, string, error) {
	if s == nil || s.store == nil {
		return nil, "", ErrCoversDisabled
	}
	job, err := s.Job(ctx, claims, jobID, requestedLibrary)
	if err != nil {
		return nil, "", err
	}
	if job.Status != "REVIEW_REQUIRED" || job.NSFWDecision != "SAFE" {
		if claims == nil || claims.Role != models.RoleSuperAdminRoot || job.Status != "QUARANTINED" || job.NSFWDecision != "REVIEW" {
			return nil, "", repositories.ErrCoverImportReviewState
		}
	}
	reader, err := s.store.OpenSource(ctx, job.SourceKey)
	if err != nil {
		return nil, "", err
	}
	return reader, job.ContentType, nil
}

type CoverImportReviewService struct {
	books      *BookService
	repository *repositories.CoverImportRepository
	store      reviewSourceStore
	newID      func() (string, error)
	now        func() time.Time
}

func NewCoverImportReviewService(books *BookService, repository *repositories.CoverImportRepository, store reviewSourceStore) *CoverImportReviewService {
	return &CoverImportReviewService{books: books, repository: repository, store: store, newID: identity.NewID, now: time.Now}
}

func (s *CoverImportReviewService) Job(ctx context.Context, claims *auth.Claims, jobID, requestedLibrary string) (repositories.CoverImportReviewJob, error) {
	if s == nil || s.repository == nil || s.books == nil {
		return repositories.CoverImportReviewJob{}, ErrCoversDisabled
	}
	libraryID, err := resolveBookScope(claims, requestedLibrary, true)
	if err != nil {
		return repositories.CoverImportReviewJob{}, err
	}
	if err = s.books.ensureOwnerLibraryActive(ctx, claims, libraryID); err != nil {
		return repositories.CoverImportReviewJob{}, err
	}
	return s.repository.ReviewJob(ctx, jobID, libraryID)
}

func (s *CoverImportReviewService) Decide(ctx context.Context, claims *auth.Claims, jobID, requestedLibrary, action, reason string, bookID int) (repositories.CoverImportReviewJob, error) {
	if s == nil || s.repository == nil || s.books == nil || s.store == nil || s.newID == nil {
		return repositories.CoverImportReviewJob{}, ErrCoversDisabled
	}
	if action != "ACCEPT" && action != "REJECT" {
		return repositories.CoverImportReviewJob{}, ErrInvalidBook
	}
	if (action == "ACCEPT" && bookID < 1) || (action == "REJECT" && bookID != 0) {
		return repositories.CoverImportReviewJob{}, ErrInvalidBook
	}
	job, err := s.Job(ctx, claims, jobID, requestedLibrary)
	if err != nil {
		return job, err
	}
	if job.Status != "REVIEW_REQUIRED" || job.NSFWDecision != "SAFE" {
		return job, repositories.ErrCoverImportReviewState
	}
	if action == "ACCEPT" {
		found := false
		for _, candidate := range job.Candidates {
			if candidate.BookID == bookID {
				found = true
				break
			}
		}
		if !found {
			return job, repositories.ErrCoverImportCandidateNotFound
		}
	}
	auditID, err := s.newID()
	if err != nil {
		return job, err
	}
	var cover repositories.PendingCover
	eventID, target := "", ""
	if action == "ACCEPT" {
		cover.ID, err = s.newID()
		if err != nil {
			return job, err
		}
		eventID, err = s.newID()
		if err != nil {
			return job, err
		}
		extension := "jpg"
		if job.Format == "png" {
			extension = "png"
		}
		target, err = covers.SourceObjectKey(job.LibraryID, bookID, cover.ID, extension)
		if err != nil {
			return job, err
		}
		cover = repositories.PendingCover{ID: cover.ID, BookID: bookID, LibraryID: job.LibraryID, SourceObjectKey: target, SourceContentType: job.ContentType, SourceFormat: job.Format, SourceWidth: job.Width, SourceHeight: job.Height, SourceSize: job.Size}
		if err = s.store.CopySource(ctx, job.SourceKey, target, job.Size, job.ContentType); err != nil {
			return job, err
		}
	}
	err = s.repository.DecideReview(ctx, job, claims.Subject, string(claims.Role), action, reason, bookID, cover, eventID, auditID, s.now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		if target != "" {
			if deleteErr := s.store.DeleteObject(ctx, target); deleteErr != nil {
				return job, errors.Join(err, fmt.Errorf("compensate review source: %w", deleteErr))
			}
		}
		return job, err
	}
	job.Status = "REJECTED"
	if action == "ACCEPT" {
		job.Status = "READY"
	}
	return job, nil
}

func (s *CoverImportReviewService) DecideQuarantine(ctx context.Context, claims *auth.Claims, jobID, requestedLibrary, decision string) (repositories.CoverImportReviewJob, error) {
	if claims == nil || claims.Role != models.RoleSuperAdminRoot {
		return repositories.CoverImportReviewJob{}, ErrBookForbidden
	}
	if decision != "APPROVE" && decision != "REJECT" {
		return repositories.CoverImportReviewJob{}, ErrInvalidBook
	}
	job, err := s.Job(ctx, claims, jobID, requestedLibrary)
	if err != nil {
		return job, err
	}
	if job.Status != "QUARANTINED" || job.NSFWDecision != "REVIEW" {
		return job, repositories.ErrCoverImportReviewState
	}
	auditID, err := s.newID()
	if err != nil {
		return job, err
	}
	eventID := ""
	if decision == "APPROVE" {
		eventID, err = s.newID()
		if err != nil {
			return job, err
		}
	}
	if err = s.repository.DecideQuarantine(ctx, jobID, job.LibraryID, claims.Subject, string(claims.Role), decision, eventID, auditID, s.now().UTC().Format(time.RFC3339Nano)); err != nil {
		return job, err
	}
	job.Status = "REJECTED"
	if decision == "APPROVE" {
		job.Status = "OCR_PENDING"
		job.NSFWDecision = "SAFE"
	}
	return job, nil
}
