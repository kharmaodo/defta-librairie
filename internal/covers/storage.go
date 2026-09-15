package covers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

var (
	ErrInvalidObjectKey = errors.New("invalid cover object key")
	ErrStoreUnavailable = errors.New("cover object store unavailable")
)

var safeKeySegment = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type IDGenerator func() (string, error)

type SourceUploader struct {
	store     ObjectStore
	validator Validator
	newID     IDGenerator
}

type StoredSource struct {
	CoverID     string
	ObjectKey   string
	ContentType string
	Format      string
	Width       int
	Height      int
	Size        int64
}

func NewSourceUploader(store ObjectStore, validator Validator, newID IDGenerator) (*SourceUploader, error) {
	if store == nil || newID == nil {
		return nil, ErrStoreUnavailable
	}
	return &SourceUploader{store: store, validator: validator, newID: newID}, nil
}

func (u *SourceUploader) Upload(
	ctx context.Context,
	libraryID string,
	bookID int,
	declaredContentType string,
	body io.Reader,
) (StoredSource, error) {
	image, err := u.validator.Validate(body, declaredContentType)
	if err != nil {
		return StoredSource{}, err
	}
	coverID, err := u.newID()
	if err != nil {
		return StoredSource{}, fmt.Errorf("generate cover id: %w", err)
	}
	key, err := SourceObjectKey(libraryID, bookID, coverID, image.Extension)
	if err != nil {
		return StoredSource{}, err
	}
	if err = u.store.Put(ctx, key, bytes.NewReader(image.Data), int64(len(image.Data)), image.ContentType); err != nil {
		return StoredSource{}, fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return StoredSource{
		CoverID:     coverID,
		ObjectKey:   key,
		ContentType: image.ContentType,
		Format:      image.Format,
		Width:       image.Width,
		Height:      image.Height,
		Size:        int64(len(image.Data)),
	}, nil
}

func (u *SourceUploader) Discard(ctx context.Context, source StoredSource) error {
	if source.ObjectKey == "" {
		return ErrInvalidObjectKey
	}
	if err := u.store.Delete(ctx, source.ObjectKey); err != nil {
		return fmt.Errorf("%w: %v", ErrStoreUnavailable, err)
	}
	return nil
}

func SourceObjectKey(libraryID string, bookID int, coverID, extension string) (string, error) {
	if !safeKeySegment.MatchString(libraryID) || bookID < 1 ||
		!safeKeySegment.MatchString(coverID) ||
		(extension != "jpg" && extension != "png") {
		return "", ErrInvalidObjectKey
	}
	return "sources/" + libraryID + "/" + strconv.Itoa(bookID) + "/" + coverID + "." + extension, nil
}
