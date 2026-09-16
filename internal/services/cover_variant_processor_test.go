package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"defta-librairie/internal/covers"
)

type trackedCoverSource struct {
	*bytes.Reader
	closed bool
}

func (s *trackedCoverSource) Close() error {
	s.closed = true
	return nil
}

type coverSourceReaderStub struct {
	source *trackedCoverSource
	err    error
}

func (s *coverSourceReaderStub) OpenSource(
	context.Context,
	string,
) (io.ReadCloser, error) {
	return s.source, s.err
}

type coverTransformerStub struct {
	result covers.GeneratedVariants
	err    error
}

func (s *coverTransformerStub) Process(
	context.Context,
	io.Reader,
) (covers.GeneratedVariants, error) {
	return s.result, s.err
}

type coverVariantStoreStub struct {
	putCalls    []string
	deleteCalls []string
	failAt      int
}

func (s *coverVariantStoreStub) PutVariant(
	_ context.Context,
	key string,
	_ io.Reader,
	_ int64,
	_ string,
) error {
	s.putCalls = append(s.putCalls, key)
	if s.failAt > 0 && len(s.putCalls) == s.failAt {
		return errors.New("minio write failed")
	}
	return nil
}

func (s *coverVariantStoreStub) DeleteVariant(
	_ context.Context,
	key string,
) error {
	s.deleteCalls = append(s.deleteCalls, key)
	return nil
}

func generatedCoverFixture() covers.GeneratedVariants {
	return covers.GeneratedVariants{
		MasterJPEG: []byte("master"),
		LargeJPEG:  []byte("large-jpeg"),
		LargeWebP:  []byte("large-webp"),
		ThumbJPEG:  []byte("thumb-jpeg"),
		ThumbWebP:  []byte("thumb-webp"),
	}
}

func TestCoverVariantProcessorStoresEveryVariant(t *testing.T) {
	source := &trackedCoverSource{Reader: bytes.NewReader([]byte("source"))}
	reader := &coverSourceReaderStub{source: source}
	store := &coverVariantStoreStub{}
	transformer := &coverTransformerStub{result: generatedCoverFixture()}

	processor, err := NewStoredCoverVariantProcessor(reader, store, transformer)
	if err != nil {
		t.Fatalf("new processor: %v", err)
	}

	processed, err := processor.Process(context.Background(), coverWorkerEvent())
	if err != nil {
		t.Fatalf("process: %v", err)
	}

	expected := []string{
		"masters/library-1/1/cover-1/master.jpg",
		"variants/library-1/1/cover-1/large.jpg",
		"variants/library-1/1/cover-1/large.webp",
		"variants/library-1/1/cover-1/thumb.jpg",
		"variants/library-1/1/cover-1/thumb.webp",
	}
	if !reflect.DeepEqual(store.putCalls, expected) {
		t.Fatalf("put calls=%v expected=%v", store.putCalls, expected)
	}
	if len(store.deleteCalls) != 0 {
		t.Fatalf("unexpected deletes=%v", store.deleteCalls)
	}
	if !source.closed {
		t.Fatal("source was not closed")
	}
	if processed.MasterObjectKey != expected[0] ||
		processed.ThumbWebPObjectKey != expected[4] {
		t.Fatalf("processed=%+v", processed)
	}
}

func TestCoverVariantProcessorCompensatesPartialWrite(t *testing.T) {
	source := &trackedCoverSource{Reader: bytes.NewReader([]byte("source"))}
	reader := &coverSourceReaderStub{source: source}
	store := &coverVariantStoreStub{failAt: 3}
	transformer := &coverTransformerStub{result: generatedCoverFixture()}

	processor, _ := NewStoredCoverVariantProcessor(reader, store, transformer)
	_, err := processor.Process(context.Background(), coverWorkerEvent())
	if err == nil {
		t.Fatal("expected processing error")
	}

	expectedDeletes := []string{
		"variants/library-1/1/cover-1/large.jpg",
		"masters/library-1/1/cover-1/master.jpg",
	}
	if !reflect.DeepEqual(store.deleteCalls, expectedDeletes) {
		t.Fatalf("delete calls=%v expected=%v", store.deleteCalls, expectedDeletes)
	}
	if !source.closed {
		t.Fatal("source was not closed")
	}
}

func TestCoverVariantProcessorDoesNotWriteAfterTransformationFailure(t *testing.T) {
	source := &trackedCoverSource{Reader: bytes.NewReader([]byte("source"))}
	reader := &coverSourceReaderStub{source: source}
	store := &coverVariantStoreStub{}
	transformer := &coverTransformerStub{err: errors.New("invalid image")}

	processor, _ := NewStoredCoverVariantProcessor(reader, store, transformer)
	_, err := processor.Process(context.Background(), coverWorkerEvent())
	if err == nil {
		t.Fatal("expected transformation error")
	}
	if len(store.putCalls) != 0 || len(store.deleteCalls) != 0 {
		t.Fatalf("puts=%v deletes=%v", store.putCalls, store.deleteCalls)
	}
	if !source.closed {
		t.Fatal("source was not closed")
	}
}
