package covers

import (
	"testing"

	"github.com/nats-io/nats.go"
)

func TestCoverJetStreamMessageCarriesDeduplicationID(t *testing.T) {
	payload := []byte(`{"coverId":"cover-1"}`)
	message := newCoverJetStreamMessage("book.covers.process.v1", "event-1", payload)

	payload[0] = 'X'
	if message.Subject != "book.covers.process.v1" ||
		message.Header.Get(nats.MsgIdHdr) != "event-1" ||
		message.Header.Get("Content-Type") != "application/json" ||
		string(message.Data) != `{"coverId":"cover-1"}` {
		t.Fatalf("unexpected message: subject=%q headers=%v data=%q", message.Subject, message.Header, message.Data)
	}
}

func TestValidateCoverStreamRequiresSubjectAndFileStorage(t *testing.T) {
	valid := nats.StreamConfig{
		Subjects: []string{"book.covers.process.v1"},
		Storage:  nats.FileStorage,
	}
	if err := validateCoverStream(valid, "book.covers.process.v1"); err != nil {
		t.Fatalf("valid stream: %v", err)
	}

	memory := valid
	memory.Storage = nats.MemoryStorage
	if err := validateCoverStream(memory, "book.covers.process.v1"); err == nil {
		t.Fatal("expected memory storage rejection")
	}
	if err := validateCoverStream(valid, "book.covers.other.v1"); err == nil {
		t.Fatal("expected missing subject rejection")
	}
}
