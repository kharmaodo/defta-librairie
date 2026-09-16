package covers

import (
	"errors"
	"testing"
)

func TestDecodeProcessingEvent(t *testing.T) {
	payload := []byte(`{
		"schemaVersion":1,
		"eventId":"event-1",
		"coverId":"cover-1",
		"bookId":7,
		"libraryId":"library-1",
		"sourceObjectKey":"sources/library-1/7/cover-1.png",
		"attempt":1
	}`)
	event, err := DecodeProcessingEvent(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if event.EventID != "event-1" || event.CoverID != "cover-1" ||
		event.BookID != 7 || event.Attempt != 1 {
		t.Fatalf("unexpected event: %+v", event)
	}
}

func TestDecodeProcessingEventRejectsInvalidContracts(t *testing.T) {
	cases := map[string]string{
		"invalid JSON": `{`,
		"unknown field": `{"schemaVersion":1,"unknown":true}`,
		"wrong version": `{"schemaVersion":2,"eventId":"e","coverId":"c","bookId":1,"libraryId":"l","sourceObjectKey":"s","attempt":1}`,
		"missing identity": `{"schemaVersion":1,"eventId":"","coverId":"c","bookId":1,"libraryId":"l","sourceObjectKey":"s","attempt":1}`,
		"trailing data": `{"schemaVersion":1,"eventId":"e","coverId":"c","bookId":1,"libraryId":"l","sourceObjectKey":"s","attempt":1} {}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeProcessingEvent([]byte(payload))
			if !errors.Is(err, ErrInvalidCoverMessage) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
