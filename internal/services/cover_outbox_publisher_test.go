package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"defta-librairie/internal/repositories"
)

type coverOutboxStoreStub struct {
	events        []repositories.CoverOutboxEvent
	claimErr      error
	publishedID   string
	failedID      string
	failedAt      time.Time
	failedMessage string
	markErr       error
}

func (s *coverOutboxStoreStub) ClaimNext(_ context.Context, _ string, _, _ time.Time) (repositories.CoverOutboxEvent, error) {
	if s.claimErr != nil {
		return repositories.CoverOutboxEvent{}, s.claimErr
	}
	if len(s.events) == 0 {
		return repositories.CoverOutboxEvent{}, repositories.ErrCoverOutboxEmpty
	}
	event := s.events[0]
	s.events = s.events[1:]
	return event, nil
}

func (s *coverOutboxStoreStub) MarkPublished(_ context.Context, eventID, _ string, _ time.Time) error {
	s.publishedID = eventID
	return s.markErr
}

func (s *coverOutboxStoreStub) MarkFailed(_ context.Context, eventID, _ string, availableAt time.Time, lastError string) error {
	s.failedID = eventID
	s.failedAt = availableAt
	s.failedMessage = lastError
	return s.markErr
}

type coverMessagePublisherStub struct {
	subject string
	eventID string
	payload string
	err     error
}

func (p *coverMessagePublisherStub) Publish(_ context.Context, subject, eventID string, payload []byte) error {
	p.subject = subject
	p.eventID = eventID
	p.payload = string(payload)
	return p.err
}

func newCoverOutboxPublisherForTest(t *testing.T, store CoverOutboxStore, messages CoverMessagePublisher, now time.Time) *CoverOutboxPublisher {
	t.Helper()
	publisher, err := NewCoverOutboxPublisher(store, messages, "publisher-1", "book.covers.process.v1")
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	publisher.now = func() time.Time { return now }
	return publisher
}

func TestCoverOutboxPublisherPublishesAndAcknowledgesBatch(t *testing.T) {
	now := time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC)
	store := &coverOutboxStoreStub{events: []repositories.CoverOutboxEvent{
		{EventID: "event-1", Payload: `{"coverId":"cover-1"}`},
		{EventID: "event-2", Payload: `{"coverId":"cover-2"}`},
	}}
	messages := &coverMessagePublisherStub{}
	publisher := newCoverOutboxPublisherForTest(t, store, messages, now)

	count, err := publisher.PublishAvailable(context.Background(), 2)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if count != 2 || store.publishedID != "event-2" {
		t.Fatalf("count=%d published=%q", count, store.publishedID)
	}
	if messages.subject != "book.covers.process.v1" ||
		messages.eventID != "event-2" ||
		messages.payload != `{"coverId":"cover-2"}` {
		t.Fatalf("message=%+v", messages)
	}
}

func TestCoverOutboxPublisherReschedulesFailedMessage(t *testing.T) {
	now := time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC)
	store := &coverOutboxStoreStub{events: []repositories.CoverOutboxEvent{
		{EventID: "event-3", Payload: "{}", Attempts: 2},
	}}
	messages := &coverMessagePublisherStub{err: errors.New("  nats\\n unavailable  ")}
	publisher := newCoverOutboxPublisherForTest(t, store, messages, now)

	count, err := publisher.PublishAvailable(context.Background(), 10)
	if count != 0 || !errors.Is(err, ErrCoverEventPublish) {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if store.failedID != "event-3" ||
		!store.failedAt.Equal(now.Add(20*time.Second)) ||
		store.failedMessage != "nats unavailable" {
		t.Fatalf("failed id=%q at=%v message=%q", store.failedID, store.failedAt, store.failedMessage)
	}
	if store.publishedID != "" {
		t.Fatalf("unexpected acknowledgement: %q", store.publishedID)
	}
}

func TestCoverOutboxPublisherStopsCleanlyWhenEmptyOrCancelled(t *testing.T) {
	now := time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC)
	store := &coverOutboxStoreStub{}
	messages := &coverMessagePublisherStub{}
	publisher := newCoverOutboxPublisherForTest(t, store, messages, now)

	count, err := publisher.PublishAvailable(context.Background(), 10)
	if err != nil || count != 0 {
		t.Fatalf("empty count=%d err=%v", count, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	count, err = publisher.PublishAvailable(ctx, 10)
	if count != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled count=%d err=%v", count, err)
	}
}

func TestCoverOutboxPublisherDoesNotAcknowledgeWhenTransportFails(t *testing.T) {
	now := time.Date(2026, 9, 15, 18, 0, 0, 0, time.UTC)
	store := &coverOutboxStoreStub{
		events: []repositories.CoverOutboxEvent{{EventID: "event-4", Payload: "{}"}},
		markErr: repositories.ErrCoverOutboxLeaseLost,
	}
	messages := &coverMessagePublisherStub{err: errors.New("offline")}
	publisher := newCoverOutboxPublisherForTest(t, store, messages, now)

	count, err := publisher.PublishAvailable(context.Background(), 1)
	if count != 0 ||
		!errors.Is(err, ErrCoverEventPublish) ||
		!errors.Is(err, repositories.ErrCoverOutboxLeaseLost) {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if store.publishedID != "" {
		t.Fatalf("unexpected acknowledgement: %q", store.publishedID)
	}
}
