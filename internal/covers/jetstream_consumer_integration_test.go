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
		ctx, url, user, password, streamName, subject, durable, 5,
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
}
