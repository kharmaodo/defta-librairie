package covers

import (
	"errors"
	"testing"
)

func TestNewProcessingObjectKeys(t *testing.T) {
	keys, err := NewProcessingObjectKeys("library-1", 42, "cover-1")
	if err != nil {
		t.Fatalf("new keys: %v", err)
	}

	expected := ProcessingObjectKeys{
		MasterJPEG: "masters/library-1/42/cover-1/master.jpg",
		LargeJPEG:  "variants/library-1/42/cover-1/large.jpg",
		LargeWebP:  "variants/library-1/42/cover-1/large.webp",
		ThumbJPEG:  "variants/library-1/42/cover-1/thumb.jpg",
		ThumbWebP:  "variants/library-1/42/cover-1/thumb.webp",
	}

	if keys != expected {
		t.Fatalf("keys=%+v expected=%+v", keys, expected)
	}
}

func TestNewProcessingObjectKeysRejectsUnsafeIdentifiers(t *testing.T) {
	tests := []struct {
		name      string
		libraryID string
		bookID    int
		coverID   string
	}{
		{name: "empty library", bookID: 1, coverID: "cover-1"},
		{name: "library traversal", libraryID: "../library", bookID: 1, coverID: "cover-1"},
		{name: "invalid book", libraryID: "library-1", bookID: 0, coverID: "cover-1"},
		{name: "empty cover", libraryID: "library-1", bookID: 1},
		{name: "cover traversal", libraryID: "library-1", bookID: 1, coverID: "../cover"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewProcessingObjectKeys(
				test.libraryID,
				test.bookID,
				test.coverID,
			)
			if !errors.Is(err, ErrInvalidObjectKey) {
				t.Fatalf("err=%v expected=%v", err, ErrInvalidObjectKey)
			}
		})
	}
}
