package services

import (
	"context"
	"defta-librairie/internal/repositories"
	"errors"
	"testing"
	"time"
)

type cleanupStoreStub struct {
	jobs        []repositories.CoverCleanupJob
	claimIndex  int
	completedID int64
	failedID    int64
	availableAt time.Time
	lastError   string
}

func (s *cleanupStoreStub) ClaimNext(
	_ context.Context,
	_ string,
	_ time.Time,
	_ time.Time,
) (repositories.CoverCleanupJob, error) {
	if s.claimIndex >= len(s.jobs) {
		return repositories.CoverCleanupJob{}, repositories.ErrCoverCleanupEmpty
	}
	job := s.jobs[s.claimIndex]
	s.claimIndex++
	return job, nil
}

func (s *cleanupStoreStub) MarkCompleted(
	_ context.Context,
	jobID int64,
	_ string,
	_ time.Time,
) error {
	s.completedID = jobID
	return nil
}

func (s *cleanupStoreStub) MarkFailed(
	_ context.Context,
	jobID int64,
	_ string,
	availableAt time.Time,
	lastError string,
) error {
	s.failedID = jobID
	s.availableAt = availableAt
	s.lastError = lastError
	return nil
}

type cleanupObjectStoreStub struct {
	keys []string
	err  error
}

func (s *cleanupObjectStoreStub) DeleteObject(
	_ context.Context,
	key string,
) error {
	s.keys = append(s.keys, key)
	return s.err
}

func TestCoverCleanupServiceDeletesAndAcknowledgesAvailableJobs(t *testing.T) {
	store := &cleanupStoreStub{jobs: []repositories.CoverCleanupJob{
		{ID: 1, ObjectKey: "sources/library-1/7/cover-old.jpg"},
		{ID: 2, ObjectKey: "variants/library-1/7/cover-old/thumb.webp"},
	}}
	objects := &cleanupObjectStoreStub{}
	service, err := NewCoverCleanupService(store, objects, "cleanup-worker")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	cleaned, err := service.CleanAvailable(context.Background(), 10)
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if cleaned != 2 || store.completedID != 2 || len(objects.keys) != 2 {
		t.Fatalf(
			"cleaned=%d completed=%d keys=%v",
			cleaned,
			store.completedID,
			objects.keys,
		)
	}
}

func TestCoverCleanupServiceReschedulesStoreFailure(t *testing.T) {
	store := &cleanupStoreStub{jobs: []repositories.CoverCleanupJob{{
		ID: 3, ObjectKey: "variants/library-1/7/cover-old/large.webp", Attempts: 1,
	}}}
	objects := &cleanupObjectStoreStub{err: errors.New("minio\n unavailable")}
	service, _ := NewCoverCleanupService(store, objects, "cleanup-worker")
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	cleaned, err := service.CleanAvailable(context.Background(), 10)
	if err == nil {
		t.Fatal("expected cleanup failure")
	}
	if cleaned != 0 || store.failedID != 3 {
		t.Fatalf("cleaned=%d failed=%d", cleaned, store.failedID)
	}
	if !store.availableAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("available=%v", store.availableAt)
	}
	if store.lastError != "minio unavailable" {
		t.Fatalf("last error=%q", store.lastError)
	}
}
