package covers

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

type coverEventHandlerFunc func(context.Context, ProcessingEvent) error

func (f coverEventHandlerFunc) Handle(ctx context.Context, event ProcessingEvent) error {
	return f(ctx, event)
}

type exhaustingCoverHandler struct {
	attempts int
	failures int
}

func (h *exhaustingCoverHandler) Handle(context.Context, ProcessingEvent) error {
	h.attempts++
	return errors.New("persistent image processor failure")
}

func (h *exhaustingCoverHandler) MarkFailed(
	_ context.Context, _ ProcessingEvent, cause error,
) error {
	if cause == nil {
		return errors.New("missing exhaustion cause")
	}
	h.failures++
	return nil
}

func TestJetStreamConsumerIntegration(t *testing.T) {
	if os.Getenv("NATS_INTEGRATION") != "1" {
		t.Skip("set NATS_INTEGRATION=1 to test Docker JetStream")
	}
	url := envOrDefault("NATS_URL", nats.DefaultURL)
	user := os.Getenv("NATS_USER")
	password := os.Getenv("NATS_PASSWORD")
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	streamName := "COVER_CONSUMER_" + suffix
	subject := "book.covers.consumer." + suffix
	durable := "cover-worker-" + suffix

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	publisher, err := NewJetStreamPublisher(
		ctx, url, user, password, streamName, subject,
	)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	t.Cleanup(func() {
		_ = publisher.jetStream.DeleteStream(streamName)
		_ = publisher.Close()
	})

	consumer, err := NewJetStreamConsumer(
		ctx, url, user, password, streamName, subject, durable, 2,
	)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	consumer.retryDelay = 50 * time.Millisecond
	t.Cleanup(func() { _ = consumer.Close() })

	eventID := "event-consumer-1"
	payload := []byte(`{
		"schemaVersion":1,
		"eventId":"event-consumer-1",
		"coverId":"cover-consumer-1",
		"bookId":9,
		"libraryId":"library-consumer",
		"sourceObjectKey":"sources/library-consumer/9/cover-consumer-1.png",
		"attempt":1
	}`)
	if err = publisher.Publish(ctx, subject, eventID, payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	attempts := 0
	handler := coverEventHandlerFunc(func(_ context.Context, event ProcessingEvent) error {
		attempts++
		if event.EventID != eventID {
			t.Fatalf("event id=%q", event.EventID)
		}
		if attempts == 1 {
			return errors.New("temporary image processor failure")
		}
		return nil
	})

	handled, err := consumer.FetchAndHandle(ctx, 1, 2*time.Second, handler)
	if err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if handled != 0 || attempts != 1 {
		t.Fatalf("first handled=%d attempts=%d", handled, attempts)
	}

	time.Sleep(75 * time.Millisecond)
	handled, err = consumer.FetchAndHandle(ctx, 1, 2*time.Second, handler)
	if err != nil {
		t.Fatalf("retry fetch: %v", err)
	}
	if handled != 1 || attempts != 2 {
		t.Fatalf("retry handled=%d attempts=%d", handled, attempts)
	}

	handled, err = consumer.FetchAndHandle(ctx, 1, 200*time.Millisecond, handler)
	if err != nil {
		t.Fatalf("post-ack fetch: %v", err)
	}
	if handled != 0 || attempts != 2 {
		t.Fatalf("post-ack handled=%d attempts=%d", handled, attempts)
	}

	exhaustedPayload := []byte(`{
		"schemaVersion":1,
		"eventId":"event-consumer-2",
		"coverId":"cover-consumer-2",
		"bookId":10,
		"libraryId":"library-consumer",
		"sourceObjectKey":"sources/library-consumer/10/cover-consumer-2.png",
		"attempt":1
	}`)
	if err = publisher.Publish(
		ctx, subject, "event-consumer-2", exhaustedPayload,
	); err != nil {
		t.Fatalf("publish exhausted event: %v", err)
	}
	exhausting := &exhaustingCoverHandler{}
	handled, err = consumer.FetchAndHandle(ctx, 1, 2*time.Second, exhausting)
	if err != nil || handled != 0 || exhausting.attempts != 1 {
		t.Fatalf(
			"first exhaustion handled=%d attempts=%d err=%v",
			handled, exhausting.attempts, err,
		)
	}
	time.Sleep(75 * time.Millisecond)
	handled, err = consumer.FetchAndHandle(ctx, 1, 2*time.Second, exhausting)
	if err != nil || handled != 0 ||
		exhausting.attempts != 2 || exhausting.failures != 1 {
		t.Fatalf(
			"final exhaustion handled=%d attempts=%d failures=%d err=%v",
			handled, exhausting.attempts, exhausting.failures, err,
		)
	}
	handled, err = consumer.FetchAndHandle(ctx, 1, 200*time.Millisecond, exhausting)
	if err != nil || handled != 0 || exhausting.attempts != 2 {
		t.Fatalf(
			"post-term handled=%d attempts=%d err=%v",
			handled, exhausting.attempts, err,
		)
	}
}
