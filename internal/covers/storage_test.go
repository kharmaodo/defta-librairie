package covers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type memoryStore struct {
	key         string
	contentType string
	data        []byte
	deleteKey   string
	putErr      error
	deleteErr   error
}

func (s *memoryStore) Put(_ context.Context, key string, body io.Reader, _ int64, contentType string) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.key, s.contentType, s.data = key, contentType, data
	return nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.deleteKey = key
	return s.deleteErr
}

func TestSourceUploaderStoresValidatedContent(t *testing.T) {
	store := &memoryStore{}
	uploader, err := NewSourceUploader(
		store,
		NewValidator(1024*1024, 100),
		func() (string, error) { return "cover-id", nil },
	)
	if err != nil {
		t.Fatalf("new uploader: %v", err)
	}
	data := encodedImage(t, "png", 4, 5)
	source, err := uploader.Upload(
		context.Background(),
		"library-id",
		42,
		"image/png",
		bytes.NewReader(data),
	)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if source.ObjectKey != "sources/library-id/42/cover-id.png" {
		t.Fatalf("object key=%q", source.ObjectKey)
	}
	if store.key != source.ObjectKey || store.contentType != "image/png" ||
		!bytes.Equal(store.data, data) {
		t.Fatalf("unexpected stored object: %+v", store)
	}
	if err = uploader.Discard(context.Background(), source); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if store.deleteKey != source.ObjectKey {
		t.Fatalf("deleted key=%q", store.deleteKey)
	}
}

func TestSourceUploaderDoesNotStoreInvalidImage(t *testing.T) {
	store := &memoryStore{}
	uploader, _ := NewSourceUploader(
		store,
		NewValidator(100, 100),
		func() (string, error) { return "cover-id", nil },
	)
	_, err := uploader.Upload(
		context.Background(),
		"library-id",
		42,
		"image/png",
		bytes.NewReader([]byte("not an image")),
	)
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("err=%v", err)
	}
	if store.key != "" {
		t.Fatalf("invalid image was stored as %q", store.key)
	}
}

func TestSourceUploaderWrapsStoreFailure(t *testing.T) {
	store := &memoryStore{putErr: errors.New("offline")}
	uploader, _ := NewSourceUploader(
		store,
		NewValidator(1024*1024, 100),
		func() (string, error) { return "cover-id", nil },
	)
	_, err := uploader.Upload(
		context.Background(),
		"library-id",
		42,
		"image/jpeg",
		bytes.NewReader(encodedImage(t, "jpeg", 4, 5)),
	)
	if !errors.Is(err, ErrStoreUnavailable) {
		t.Fatalf("err=%v", err)
	}
}

func TestSourceObjectKeyRejectsUnsafeSegments(t *testing.T) {
	for _, test := range []struct {
		libraryID string
		bookID    int
		coverID   string
		extension string
	}{
		{libraryID: "../other", bookID: 1, coverID: "cover", extension: "jpg"},
		{libraryID: "library", bookID: 0, coverID: "cover", extension: "jpg"},
		{libraryID: "library", bookID: 1, coverID: "../cover", extension: "jpg"},
		{libraryID: "library", bookID: 1, coverID: "cover", extension: "gif"},
	} {
		if _, err := SourceObjectKey(test.libraryID, test.bookID, test.coverID, test.extension); !errors.Is(err, ErrInvalidObjectKey) {
			t.Fatalf("expected invalid key for %+v, got %v", test, err)
		}
	}
}
