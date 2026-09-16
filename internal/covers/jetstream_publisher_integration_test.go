package covers

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestJetStreamPublisherIntegration(t *testing.T) {
	if os.Getenv("NATS_INTEGRATION") != "1" {
		t.Skip("set NATS_INTEGRATION=1 to test Docker JetStream")
	}
	url := envOrDefault("NATS_URL", nats.DefaultURL)
	user := os.Getenv("NATS_USER")
	password := os.Getenv("NATS_PASSWORD")
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	streamName := "COVER_TEST_" + suffix
	subject := "book.covers.test." + suffix

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	publisher, err := NewJetStreamPublisher(ctx, url, user, password, streamName, subject)
	if err != nil {
		t.Fatalf("new publisher: %v", err)
	}
	t.Cleanup(func() {
		_ = publisher.jetStream.DeleteStream(streamName)
		_ = publisher.Close()
	})

	if err = publisher.Publish(ctx, subject, "event-integration-1", []byte(`{"schemaVersion":1}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}
	message, err := publisher.jetStream.GetMsg(streamName, 1, nats.Context(ctx))
	if err != nil {
		t.Fatalf("read stored message: %v", err)
	}
	if message.Header.Get(nats.MsgIdHdr) != "event-integration-1" ||
		string(message.Data) != `{"schemaVersion":1}` {
		t.Fatalf("unexpected stored message: headers=%v data=%q", message.Header, message.Data)
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
