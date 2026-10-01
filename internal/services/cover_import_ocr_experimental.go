package services

import (
	"context"
	"defta-librairie/internal/ocr"
	"defta-librairie/internal/repositories"
	"errors"
	"io"
	"time"
)

func (s *CoverImportOCRService) WithExperimentalRetryPolicy(timeout time.Duration, attempts int) *CoverImportOCRService {
	if timeout > 0 && timeout <= 180*time.Second {
		s.experimentalLease = timeout + 30*time.Second
	}
	if attempts > 0 {
		s.experimentalAttempts = attempts
	}
	return s
}

func (s *CoverImportOCRService) processExperimental(ctx context.Context, jobID string, runner ocr.DetailedRunner) error {
	token, err := s.newID()
	if err != nil {
		return err
	}
	started := s.now().UTC()
	job, err := s.repository.ClaimExperimentalOCR(ctx, jobID, token, started.Format(time.RFC3339Nano), started.Add(s.experimentalLease).Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if job.Attempts > s.experimentalAttempts {
		return s.experimentalFailure(ctx, job, "OCR_RETRIES_EXHAUSTED", false, ocr.ErrUnavailable)
	}
	execution, cancel := context.WithTimeout(ctx, s.experimentalLease-30*time.Second)
	defer cancel()
	source, err := s.sources.OpenSource(execution, job.SourceObjectKey)
	if err != nil {
		return s.experimentalFailure(ctx, job, "SOURCE_UNAVAILABLE", true, ocr.ErrUnavailable)
	}
	defer source.Close()
	if job.SourceSize < 1 || job.SourceSize > 10*1024*1024 {
		return s.experimentalFailure(ctx, job, "SOURCE_INVALID", false, ocr.ErrInvalidInput)
	}
	image, err := io.ReadAll(io.LimitReader(source, job.SourceSize+1))
	if err != nil {
		return s.experimentalFailure(ctx, job, "SOURCE_UNAVAILABLE", true, ocr.ErrUnavailable)
	}
	if int64(len(image)) != job.SourceSize {
		return s.experimentalFailure(ctx, job, "SOURCE_INVALID", false, ocr.ErrInvalidInput)
	}
	result, err := runner.Extract(execution, image, job.SourceContentType, job.ID)
	if err != nil {
		if ctx.Err() == nil && execution.Err() != nil {
			err = ocr.ErrTimeout
		}
		code := "OCR_UNAVAILABLE"
		retry := true
		switch {
		case errors.Is(err, ocr.ErrInvalidInput):
			code = "SOURCE_INVALID"
			retry = false
		case errors.Is(err, ocr.ErrBusy):
			code = "OCR_BUSY"
		case errors.Is(err, ocr.ErrTimeout):
			code = "OCR_TIMEOUT"
		case errors.Is(err, ocr.ErrInvalidResponse):
			code = "OCR_INVALID_RESPONSE"
		}
		return s.experimentalFailure(ctx, job, code, retry, err)
	}
	eventID, err := s.newID()
	if err != nil {
		return s.experimentalFailure(ctx, job, "WORKER_ID_FAILURE", true, err)
	}
	auditID, err := s.newID()
	if err != nil {
		return s.experimentalFailure(ctx, job, "WORKER_ID_FAILURE", true, err)
	}
	if err = s.repository.CompleteExperimentalOCR(ctx, job, result, eventID, auditID, s.now().UTC().Format(time.RFC3339Nano)); err != nil {
		if errors.Is(err, repositories.ErrCoverImportJobState) {
			return repositories.ErrCoverImportOCRLeaseHeld
		}
		// A failed transaction must not leave an otherwise retryable delivery stuck.
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		reset := s.repository.RetryExperimentalOCR(cleanup, job, "OCR_PERSISTENCE_FAILED", s.now().UTC().Format(time.RFC3339Nano))
		return errors.Join(err, experimentalClaimError(reset))
	}
	return nil
}

func (s *CoverImportOCRService) experimentalFailure(ctx context.Context, job repositories.ExperimentalOCRJob, code string, retry bool, cause error) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	// Shutdown/cancellation never commits a terminal decision for an unfinished call.
	if ctx.Err() != nil || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return errors.Join(cause, experimentalClaimError(s.repository.RetryExperimentalOCR(cleanup, job, code, s.now().UTC().Format(time.RFC3339Nano))))
	}
	if retry && job.Attempts < s.experimentalAttempts {
		return errors.Join(cause, experimentalClaimError(s.repository.RetryExperimentalOCR(cleanup, job, code, s.now().UTC().Format(time.RFC3339Nano))))
	}
	auditID, err := s.newID()
	if err != nil {
		return err
	}
	if err = s.repository.FailExperimentalOCR(cleanup, job, code, auditID, s.now().UTC().Format(time.RFC3339Nano)); err != nil {
		return experimentalClaimError(err)
	}
	return errors.Join(ErrCoverImportOCRFailed, cause)
}

func experimentalClaimError(err error) error {
	if errors.Is(err, repositories.ErrCoverImportJobState) {
		return repositories.ErrCoverImportOCRLeaseHeld
	}
	return err
}
