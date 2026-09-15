package covers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var ErrInvalidStoreConfig = errors.New("invalid cover object store configuration")

type minioAPI interface {
	PutObject(
		ctx context.Context,
		bucketName, objectName string,
		reader io.Reader,
		objectSize int64,
		opts minio.PutObjectOptions,
	) (minio.UploadInfo, error)
	RemoveObject(
		ctx context.Context,
		bucketName, objectName string,
		opts minio.RemoveObjectOptions,
	) error
}

type MinIOStore struct {
	client minioAPI
	bucket string
}

func NewMinIOStore(endpoint, accessKey, secretKey, bucket string, useSSL bool) (*MinIOStore, error) {
	endpoint = strings.TrimSpace(endpoint)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)
	bucket = strings.TrimSpace(bucket)
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		return nil, ErrInvalidStoreConfig
	}
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStoreConfig, err)
	}
	return newMinIOStore(client, bucket)
}

func newMinIOStore(client minioAPI, bucket string) (*MinIOStore, error) {
	if client == nil || strings.TrimSpace(bucket) == "" {
		return nil, ErrInvalidStoreConfig
	}
	return &MinIOStore{client: client, bucket: strings.TrimSpace(bucket)}, nil
}

func (s *MinIOStore) Put(
	ctx context.Context,
	key string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	if ctx == nil || body == nil || size < 1 || !validObjectKey(key) ||
		(contentType != "image/jpeg" && contentType != "image/png") {
		return ErrInvalidObjectKey
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
			"defta-kind": "book-cover-source",
		},
	})
	if err != nil {
		return fmt.Errorf("put cover source: %w", err)
	}
	return nil
}

func (s *MinIOStore) Delete(ctx context.Context, key string) error {
	if ctx == nil || !validObjectKey(key) {
		return ErrInvalidObjectKey
	}
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete cover source: %w", err)
	}
	return nil
}

func validObjectKey(key string) bool {
	parts := strings.Split(key, "/")
	if len(parts) != 4 || parts[0] != "sources" {
		return false
	}
	if !safeKeySegment.MatchString(parts[1]) || !safeKeySegment.MatchString(parts[2]) {
		return false
	}
	fileParts := strings.Split(parts[3], ".")
	return len(fileParts) == 2 && safeKeySegment.MatchString(fileParts[0]) &&
		(fileParts[1] == "jpg" || fileParts[1] == "png")
}
