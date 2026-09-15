package covers

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"defta-librairie/internal/identity"
)

func TestMinIOStoreIntegration(t *testing.T) {
	if os.Getenv("MINIO_INTEGRATION") != "1" {
		t.Skip("set MINIO_INTEGRATION=1 to run against the Docker MinIO service")
	}
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" || endpoint == "minio:9000" {
		endpoint = "127.0.0.1:9000"
	}
	store, err := NewMinIOStore(
		endpoint,
		os.Getenv("MINIO_ACCESS_KEY"),
		os.Getenv("MINIO_SECRET_KEY"),
		os.Getenv("MINIO_BUCKET_COVERS"),
		false,
	)
	if err != nil {
		t.Fatalf("new MinIO store: %v", err)
	}
	coverID, err := identity.NewID()
	if err != nil {
		t.Fatalf("new id: %v", err)
	}
	key, err := SourceObjectKey("integration-library", 1, coverID, "png")
	if err != nil {
		t.Fatalf("source key: %v", err)
	}
	data := encodedImage(t, "png", 4, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err = store.Put(ctx, key, bytes.NewReader(data), int64(len(data)), "image/png"); err != nil {
		t.Fatalf("put integration object: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		if cleanupErr := store.Delete(cleanupCtx, key); cleanupErr != nil {
			t.Errorf("delete integration object: %v", cleanupErr)
		}
	})
}
