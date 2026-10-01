package services

import (
	"context"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
	"errors"
	"strings"
	"time"
)

type CoverImportMatchingService struct {
	quality    bool
	repository *repositories.CoverImportRepository
	newID      func() (string, error)
	now        func() time.Time
}

func NewCoverImportMatchingService(repository *repositories.CoverImportRepository) *CoverImportMatchingService {
	return &CoverImportMatchingService{repository: repository, newID: identity.NewID, now: time.Now}
}

func (s *CoverImportMatchingService) WithQualityMatching(enabled bool) *CoverImportMatchingService {
	s.quality = enabled
	return s
}

// Process never updates a book. It only records candidates for human review.
func (s *CoverImportMatchingService) Process(ctx context.Context, jobID, libraryID string) error {
	if s == nil || s.repository == nil || jobID == "" || libraryID == "" {
		return repositories.ErrInvalidCoverImport
	}
	job, err := s.repository.MatchingJob(ctx, jobID, libraryID)
	if err != nil {
		return err
	}
	var candidates []repositories.CoverImportCandidate
	if s.quality {
		candidates, err = s.repository.SearchQualityCoverImportCandidates(ctx, job.LibraryID, job.Text)
	} else {
		tokens := strings.Fields(ocr.NormalizeForMatching(job.Text))
		if len(tokens) > 12 {
			tokens = tokens[:12]
		}
		for i, token := range tokens {
			runes := []rune(token)
			if len(runes) > 64 {
				tokens[i] = string(runes[:64])
			}
		}
		query := strings.ReplaceAll(ocr.FTSQuery(strings.Join(tokens, " ")), " AND ", " OR ")
		candidates, err = s.repository.SearchCoverImportCandidates(ctx, job.LibraryID, query)
	}
	if err != nil {
		return err
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	if s.quality {
		return s.repository.CompleteQualityMatching(ctx, job, candidates, auditID, s.now().UTC().Format(time.RFC3339Nano))
	}
	return s.repository.CompleteMatching(ctx, job, candidates, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func (s *CoverImportMatchingService) Fail(ctx context.Context, jobID, libraryID string) error {
	if s == nil || s.repository == nil {
		return repositories.ErrInvalidCoverImport
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	return s.repository.FailMatching(ctx, jobID, libraryID, auditID, s.now().UTC().Format(time.RFC3339Nano))
}

func IsMatchingAlreadyProcessed(err error) bool {
	return errors.Is(err, repositories.ErrCoverImportJobNotFound) || errors.Is(err, repositories.ErrCoverImportJobState)
}
