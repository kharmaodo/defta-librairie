package covers

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioProcessingAPI interface {
	Open(
		ctx context.Context,
		bucketName string,
		objectName string,
	) (io.ReadCloser, error)

	PutObject(
		ctx context.Context,
		bucketName string,
		objectName string,
		reader io.Reader,
		objectSize int64,
		opts minio.PutObjectOptions,
	) (minio.UploadInfo, error)

	RemoveObject(
		ctx context.Context,
		bucketName string,
		objectName string,
		opts minio.RemoveObjectOptions,
	) error
}

type minioProcessingClient struct {
	client *minio.Client
}

func (c *minioProcessingClient) Open(
	ctx context.Context,
	bucketName string,
	objectName string,
) (io.ReadCloser, error) {
	object, err := c.client.GetObject(
		ctx,
		bucketName,
		objectName,
		minio.GetObjectOptions{},
	)
	if err != nil {
		return nil, err
	}
	// GetObject is lazy. Stat prevents a missing remote object being sent as
	// an empty 200 response, which otherwise becomes a broken blob URL.
	if _, err = object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (c *minioProcessingClient) PutObject(
	ctx context.Context,
	bucketName string,
	objectName string,
	reader io.Reader,
	objectSize int64,
	opts minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	return c.client.PutObject(
		ctx,
		bucketName,
		objectName,
		reader,
		objectSize,
		opts,
	)
}

func (c *minioProcessingClient) RemoveObject(
	ctx context.Context,
	bucketName string,
	objectName string,
	opts minio.RemoveObjectOptions,
) error {
	return c.client.RemoveObject(ctx, bucketName, objectName, opts)
}

type MinIOProcessingStore struct {
	client minioProcessingAPI
	bucket string
}

func NewMinIOProcessingStore(
	endpoint string,
	accessKey string,
	secretKey string,
	bucket string,
	useSSL bool,
) (*MinIOProcessingStore, error) {
	endpoint = strings.TrimSpace(endpoint)
	accessKey = strings.TrimSpace(accessKey)
	secretKey = strings.TrimSpace(secretKey)
	bucket = strings.TrimSpace(bucket)

	if endpoint == "" || accessKey == "" ||
		secretKey == "" || bucket == "" {
		return nil, ErrInvalidStoreConfig
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidStoreConfig, err)
	}

	return newMinIOProcessingStore(
		&minioProcessingClient{client: client},
		bucket,
	)
}

func newMinIOProcessingStore(
	client minioProcessingAPI,
	bucket string,
) (*MinIOProcessingStore, error) {
	bucket = strings.TrimSpace(bucket)
	if client == nil || bucket == "" {
		return nil, ErrInvalidStoreConfig
	}
	return &MinIOProcessingStore{
		client: client,
		bucket: bucket,
	}, nil
}

func (s *MinIOProcessingStore) OpenSource(
	ctx context.Context,
	key string,
) (io.ReadCloser, error) {
	if ctx == nil || !validObjectKey(key) {
		return nil, ErrInvalidObjectKey
	}

	reader, err := s.client.Open(ctx, s.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("%w: open cover source: %v", ErrStoreUnavailable, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("%w: empty cover source reader", ErrStoreUnavailable)
	}
	return reader, nil
}

func (s *MinIOProcessingStore) OpenVariant(
	ctx context.Context,
	key string,
) (io.ReadCloser, error) {
	if ctx == nil || !validGeneratedObjectKey(key) {
		return nil, ErrInvalidObjectKey
	}

	reader, err := s.client.Open(ctx, s.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("%w: open processed cover: %v", ErrStoreUnavailable, err)
	}
	if reader == nil {
		return nil, fmt.Errorf("%w: empty processed cover reader", ErrStoreUnavailable)
	}
	return reader, nil
}

func (s *MinIOProcessingStore) PutVariant(
	ctx context.Context,
	key string,
	body io.Reader,
	size int64,
	contentType string,
) error {
	if ctx == nil || body == nil || size < 1 ||
		!validProcessingObjectKey(key, contentType) {
		return ErrInvalidObjectKey
	}

	kind := "book-cover-variant"
	if strings.HasPrefix(key, "masters/") {
		kind = "book-cover-master"
	}

	_, err := s.client.PutObject(
		ctx,
		s.bucket,
		key,
		body,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"defta-kind": kind,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("%w: put processed cover: %v", ErrStoreUnavailable, err)
	}
	return nil
}

func (s *MinIOProcessingStore) DeleteObject(
	ctx context.Context,
	key string,
) error {
	if ctx == nil || (!validObjectKey(key) && !validGeneratedObjectKey(key)) {
		return ErrInvalidObjectKey
	}

	err := s.client.RemoveObject(
		ctx,
		s.bucket,
		key,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("%w: delete cover object: %v", ErrStoreUnavailable, err)
	}
	return nil
}

func (s *MinIOProcessingStore) DeleteVariant(
	ctx context.Context,
	key string,
) error {
	if ctx == nil || !validGeneratedObjectKey(key) {
		return ErrInvalidObjectKey
	}

	err := s.client.RemoveObject(
		ctx,
		s.bucket,
		key,
		minio.RemoveObjectOptions{},
	)
	if err != nil {
		return fmt.Errorf("%w: delete processed cover: %v", ErrStoreUnavailable, err)
	}
	return nil
}

var (
	_ SourceReader  = (*MinIOProcessingStore)(nil)
	_ VariantReader = (*MinIOProcessingStore)(nil)
	_ VariantStore  = (*MinIOProcessingStore)(nil)
	_ ObjectDeleter = (*MinIOProcessingStore)(nil)
)
