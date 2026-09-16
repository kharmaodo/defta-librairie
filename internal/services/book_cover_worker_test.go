package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
)

type coverProcessingStoreStub struct {
	claim         repositories.CoverProcessingClaim
	claimErr      error
	releasedCode  string
	failedCode    string
	completed     repositories.ProcessedCover
	completeErr   error
}

func (s *coverProcessingStoreStub) Claim(
	context.Context, string, int, string, string, string, time.Time, time.Time,
) (repositories.CoverProcessingClaim, error) {
	return s.claim, s.claimErr
}

func (s *coverProcessingStoreStub) Release(
	_ context.Context, _, _, errorCode string, _ time.Time,
) error {
	s.releasedCode = errorCode
	return nil
}

func (s *coverProcessingStoreStub) MarkFailed(
	_ context.Context, _, errorCode string, _ time.Time,
) error {
	s.failedCode = errorCode
	return nil
}

func (s *coverProcessingStoreStub) Complete(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	processed repositories.ProcessedCover,
	_ time.Time,
) error {
	s.completed = processed
	return s.completeErr
}

type coverVariantProcessorStub struct {
	result repositories.ProcessedCover
	err    error
	calls  int
}

func (p *coverVariantProcessorStub) Process(
	context.Context, covers.ProcessingEvent,
) (repositories.ProcessedCover, error) {
	p.calls++
	return p.result, p.err
}

func completeCoverResult() repositories.ProcessedCover {
	return repositories.ProcessedCover{
		MasterObjectKey:    "masters/library/1/cover.jpg",
		LargeJPEGObjectKey: "variants/library/1/large.jpg",
		LargeWebPObjectKey: "variants/library/1/large.webp",
		ThumbJPEGObjectKey: "variants/library/1/thumb.jpg",
		ThumbWebPObjectKey: "variants/library/1/thumb.webp",
	}
}

func coverWorkerEvent() covers.ProcessingEvent {
	return covers.ProcessingEvent{
		SchemaVersion: 1,
		EventID: "event-1",
		CoverID: "cover-1",
		BookID: 1,
		LibraryID: "library-1",
		SourceObjectKey: "sources/library-1/1/cover-1.png",
		Attempt: 1,
	}
}

func TestBookCoverWorkerCompletesClaimedCover(t *testing.T) {
	store := &coverProcessingStoreStub{claim: repositories.CoverProcessingClaimed}
	processor := &coverVariantProcessorStub{result: completeCoverResult()}
	worker, err := NewBookCoverWorker(store, processor, "worker-1")
	if err != nil {
		t.Fatalf("new worker: %v", err)
	}
	if err = worker.Handle(context.Background(), coverWorkerEvent()); err != nil {
		t.Fatalf("handle: %v", err)
	}
	if processor.calls != 1 || store.completed.MasterObjectKey == "" ||
		store.releasedCode != "" {
		t.Fatalf(
			"calls=%d completed=%+v released=%q",
			processor.calls,
			store.completed,
			store.releasedCode,
		)
	}
}

func TestBookCoverWorkerTreatsReadyAndTerminalCoversIdempotently(t *testing.T) {
	processor := &coverVariantProcessorStub{}
	store := &coverProcessingStoreStub{
		claim: repositories.CoverProcessingAlreadyReady,
	}
	worker, _ := NewBookCoverWorker(store, processor, "worker-1")
	if err := worker.Handle(context.Background(), coverWorkerEvent()); err != nil {
		t.Fatalf("ready handle: %v", err)
	}
	if processor.calls != 0 {
		t.Fatalf("ready processor calls=%d", processor.calls)
	}

	store.claim = 0
	store.claimErr = repositories.ErrCoverProcessingTerminal
	err := worker.Handle(context.Background(), coverWorkerEvent())
	if !errors.Is(err, covers.ErrPermanentCoverProcessing) {
		t.Fatalf("terminal err=%v", err)
	}
}

func TestBookCoverWorkerReleasesTransientFailures(t *testing.T) {
	store := &coverProcessingStoreStub{claim: repositories.CoverProcessingClaimed}
	processor := &coverVariantProcessorStub{err: errors.New("resize unavailable")}
	worker, _ := NewBookCoverWorker(store, processor, "worker-1")

	err := worker.Handle(context.Background(), coverWorkerEvent())
	if err == nil || store.releasedCode != "PROCESSING_RETRY" {
		t.Fatalf("err=%v released=%q", err, store.releasedCode)
	}

	store.releasedCode = ""
	processor.err = nil
	processor.result = completeCoverResult()
	store.completeErr = errors.New("database unavailable")
	err = worker.Handle(context.Background(), coverWorkerEvent())
	if err == nil || store.releasedCode != "COMPLETION_RETRY" {
		t.Fatalf("completion err=%v released=%q", err, store.releasedCode)
	}
}

func TestBookCoverWorkerMarksExhaustedDeliveryFailed(t *testing.T) {
	store := &coverProcessingStoreStub{}
	processor := &coverVariantProcessorStub{}
	worker, _ := NewBookCoverWorker(store, processor, "worker-1")

	if err := worker.MarkFailed(
		context.Background(), coverWorkerEvent(), errors.New("exhausted"),
	); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if store.failedCode != "MAX_DELIVERIES" {
		t.Fatalf("failed code=%q", store.failedCode)
	}
}
