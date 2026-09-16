package covers

import (
	"context"
	"io"
	"strconv"
)

const MaxProcessingSourceSize int64 = 5 * 1024 * 1024

type SourceReader interface {
	OpenSource(ctx context.Context, key string) (io.ReadCloser, error)
}

type VariantReader interface {
	OpenVariant(ctx context.Context, key string) (io.ReadCloser, error)
}

type ObjectDeleter interface {
	DeleteObject(ctx context.Context, key string) error
}

type VariantStore interface {
	PutVariant(
		ctx context.Context,
		key string,
		body io.Reader,
		size int64,
		contentType string,
	) error
	DeleteVariant(ctx context.Context, key string) error
}

type ProcessingObjectKeys struct {
	MasterJPEG string
	LargeJPEG  string
	LargeWebP  string
	ThumbJPEG  string
	ThumbWebP  string
}

func NewProcessingObjectKeys(
	libraryID string,
	bookID int,
	coverID string,
) (ProcessingObjectKeys, error) {
	if !safeKeySegment.MatchString(libraryID) ||
		bookID < 1 ||
		!safeKeySegment.MatchString(coverID) {
		return ProcessingObjectKeys{}, ErrInvalidObjectKey
	}

	base := libraryID + "/" + strconv.Itoa(bookID) + "/" + coverID

	return ProcessingObjectKeys{
		MasterJPEG: "masters/" + base + "/master.jpg",
		LargeJPEG:  "variants/" + base + "/large.jpg",
		LargeWebP:  "variants/" + base + "/large.webp",
		ThumbJPEG:  "variants/" + base + "/thumb.jpg",
		ThumbWebP:  "variants/" + base + "/thumb.webp",
	}, nil
}
