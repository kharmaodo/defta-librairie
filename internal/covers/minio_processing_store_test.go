package covers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
)

type processingMinIOStub struct {
	reader         io.ReadCloser
	openErr        error
	putErr         error
	removeErr      error
	openedKey      string
	putKey         string
	putContentType string
	putKind        string
	putBody        []byte
	removedKey     string
}

func (s *processingMinIOStub) Open(
	_ context.Context,
	_ string,
	objectName string,
) (io.ReadCloser, error) {
	s.openedKey = objectName
	return s.reader, s.openErr
}

func (s *processingMinIOStub) PutObject(
	_ context.Context,
	_ string,
	objectName string,
	reader io.Reader,
	_ int64,
	opts minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	s.putKey = objectName
	s.putContentType = opts.ContentType
	s.putKind = opts.UserMetadata["defta-kind"]
	s.putBody, _ = io.ReadAll(reader)
	return minio.UploadInfo{}, s.putErr
}

func (s *processingMinIOStub) RemoveObject(
	_ context.Context,
	_ string,
	objectName string,
	_ minio.RemoveObjectOptions,
) error {
	s.removedKey = objectName
	return s.removeErr
}

func TestMinIOProcessingStoreReadsSource(t *testing.T) {
	client := &processingMinIOStub{
		reader: io.NopCloser(bytes.NewReader([]byte("source"))),
	}
	store, err := newMinIOProcessingStore(client, "book-covers")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	reader, err := store.OpenSource(
		context.Background(),
		"sources/library-1/42/cover-1.jpg",
	)
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(body) != "source" ||
		client.openedKey != "sources/library-1/42/cover-1.jpg" {
		t.Fatalf("body=%q key=%q", body, client.openedKey)
	}
}

func TestMinIOProcessingStoreWritesGeneratedObjects(t *testing.T) {
	client := &processingMinIOStub{}
	store, _ := newMinIOProcessingStore(client, "book-covers")

	tests := []struct {
		key         string
		contentType string
		kind        string
	}{
		{
			key:         "masters/library-1/42/cover-1/master.jpg",
			contentType: CoverJPEGContentType,
			kind:        "book-cover-master",
		},
		{
			key:         "variants/library-1/42/cover-1/large.webp",
			contentType: CoverWebPContentType,
			kind:        "book-cover-variant",
		},
	}

	for _, test := range tests {
		body := []byte("generated")
		err := store.PutVariant(
			context.Background(),
			test.key,
			bytes.NewReader(body),
			int64(len(body)),
			test.contentType,
		)
		if err != nil {
			t.Fatalf("put %s: %v", test.key, err)
		}
		if client.putKey != test.key ||
			client.putContentType != test.contentType ||
			client.putKind != test.kind ||
			!bytes.Equal(client.putBody, body) {
			t.Fatalf(
				"key=%q type=%q kind=%q body=%q",
				client.putKey,
				client.putContentType,
				client.putKind,
				client.putBody,
			)
		}
	}
}

func TestMinIOProcessingStoreDeletesGeneratedObject(t *testing.T) {
	client := &processingMinIOStub{}
	store, _ := newMinIOProcessingStore(client, "book-covers")
	key := "variants/library-1/42/cover-1/thumb.webp"

	if err := store.DeleteVariant(context.Background(), key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if client.removedKey != key {
		t.Fatalf("removed=%q expected=%q", client.removedKey, key)
	}
}

func TestMinIOProcessingStoreRejectsUnsafeOperations(t *testing.T) {
	client := &processingMinIOStub{}
	store, _ := newMinIOProcessingStore(client, "book-covers")

	if _, err := store.OpenSource(
		context.Background(),
		"variants/library-1/42/cover-1/large.webp",
	); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("open err=%v", err)
	}

	if err := store.PutVariant(
		context.Background(),
		"variants/library-1/42/../large.webp",
		bytes.NewReader([]byte("x")),
		1,
		CoverWebPContentType,
	); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("put err=%v", err)
	}

	if err := store.DeleteVariant(
		context.Background(),
		"sources/library-1/42/cover-1.jpg",
	); !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("delete err=%v", err)
	}
}

func TestMinIOProcessingStoreWrapsClientFailures(t *testing.T) {
	failure := errors.New("minio unavailable")
	client := &processingMinIOStub{
		openErr:   failure,
		putErr:    failure,
		removeErr: failure,
	}
	store, _ := newMinIOProcessingStore(client, "book-covers")

	_, err := store.OpenSource(
		context.Background(),
		"sources/library-1/42/cover-1.jpg",
	)
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("open err=%v", err)
	}

	err = store.PutVariant(
		context.Background(),
		"variants/library-1/42/cover-1/large.webp",
		bytes.NewReader([]byte("x")),
		1,
		CoverWebPContentType,
	)
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("put err=%v", err)
	}

	err = store.DeleteVariant(
		context.Background(),
		"variants/library-1/42/cover-1/large.webp",
	)
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("delete err=%v", err)
	}
}
