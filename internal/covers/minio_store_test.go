package covers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
)

type fakeMinIO struct {
	bucket      string
	key         string
	size        int64
	contentType string
	metadata    map[string]string
	data        []byte
	removed     string
	putErr      error
	removeErr   error
}

func (f *fakeMinIO) PutObject(
	_ context.Context,
	bucket, key string,
	reader io.Reader,
	size int64,
	options minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	if f.putErr != nil {
		return minio.UploadInfo{}, f.putErr
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	f.bucket, f.key, f.size = bucket, key, size
	f.contentType, f.metadata, f.data = options.ContentType, options.UserMetadata, data
	return minio.UploadInfo{Bucket: bucket, Key: key, Size: size}, nil
}

func (f *fakeMinIO) RemoveObject(
	_ context.Context,
	bucket, key string,
	_ minio.RemoveObjectOptions,
) error {
	f.bucket, f.removed = bucket, key
	return f.removeErr
}

func TestMinIOStorePutAndDelete(t *testing.T) {
	client := &fakeMinIO{}
	store, err := newMinIOStore(client, "book-covers")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	body := []byte("image")
	key := "sources/library-id/42/cover-id.jpg"
	if err = store.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)), "image/jpeg"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if client.bucket != "book-covers" || client.key != key ||
		client.contentType != "image/jpeg" || client.metadata["defta-kind"] != "book-cover-source" ||
		!bytes.Equal(client.data, body) {
		t.Fatalf("unexpected put: %+v", client)
	}
	if err = store.Delete(context.Background(), key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if client.removed != key {
		t.Fatalf("removed=%q", client.removed)
	}
}

func TestMinIOStoreRejectsInvalidOperations(t *testing.T) {
	store, _ := newMinIOStore(&fakeMinIO{}, "book-covers")
	for _, test := range []struct {
		name        string
		key         string
		size        int64
		contentType string
	}{
		{name: "foreign prefix", key: "other/library/42/cover.jpg", size: 1, contentType: "image/jpeg"},
		{name: "traversal", key: "sources/../42/cover.jpg", size: 1, contentType: "image/jpeg"},
		{name: "nested filename", key: "sources/library/42/a.b.jpg", size: 1, contentType: "image/jpeg"},
		{name: "empty", key: "sources/library/42/cover.jpg", size: 0, contentType: "image/jpeg"},
		{name: "mime", key: "sources/library/42/cover.jpg", size: 1, contentType: "image/gif"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := store.Put(
				context.Background(),
				test.key,
				bytes.NewReader([]byte("x")),
				test.size,
				test.contentType,
			)
			if !errors.Is(err, ErrInvalidObjectKey) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestMinIOStoreWrapsClientFailures(t *testing.T) {
	client := &fakeMinIO{putErr: errors.New("offline")}
	store, _ := newMinIOStore(client, "book-covers")
	err := store.Put(
		context.Background(),
		"sources/library/42/cover.jpg",
		bytes.NewReader([]byte("x")),
		1,
		"image/jpeg",
	)
	if err == nil || !errors.Is(err, client.putErr) {
		t.Fatalf("err=%v", err)
	}

	client.putErr = nil
	client.removeErr = errors.New("offline")
	err = store.Delete(context.Background(), "sources/library/42/cover.jpg")
	if err == nil || !errors.Is(err, client.removeErr) {
		t.Fatalf("err=%v", err)
	}
}

func TestNewMinIOStoreValidatesConfiguration(t *testing.T) {
	for _, test := range []struct {
		endpoint string
		access   string
		secret   string
		bucket   string
	}{
		{access: "access", secret: "secret", bucket: "bucket"},
		{endpoint: "localhost:9000", secret: "secret", bucket: "bucket"},
		{endpoint: "localhost:9000", access: "access", bucket: "bucket"},
		{endpoint: "localhost:9000", access: "access", secret: "secret"},
	} {
		if _, err := NewMinIOStore(test.endpoint, test.access, test.secret, test.bucket, false); !errors.Is(err, ErrInvalidStoreConfig) {
			t.Fatalf("expected invalid configuration for %+v, got %v", test, err)
		}
	}
}
