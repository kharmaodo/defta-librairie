package covers

import "testing"

func TestValidProcessingObjectKey(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		contentType string
		want        bool
	}{
		{
			name:        "master jpeg",
			key:         "masters/library-1/42/cover-1/master.jpg",
			contentType: CoverJPEGContentType,
			want:        true,
		},
		{
			name:        "large jpeg",
			key:         "variants/library-1/42/cover-1/large.jpg",
			contentType: CoverJPEGContentType,
			want:        true,
		},
		{
			name:        "large webp",
			key:         "variants/library-1/42/cover-1/large.webp",
			contentType: CoverWebPContentType,
			want:        true,
		},
		{
			name:        "thumb jpeg",
			key:         "variants/library-1/42/cover-1/thumb.jpg",
			contentType: CoverJPEGContentType,
			want:        true,
		},
		{
			name:        "thumb webp",
			key:         "variants/library-1/42/cover-1/thumb.webp",
			contentType: CoverWebPContentType,
			want:        true,
		},
		{
			name:        "master webp forbidden",
			key:         "masters/library-1/42/cover-1/master.jpg",
			contentType: CoverWebPContentType,
		},
		{
			name:        "extension mismatch",
			key:         "variants/library-1/42/cover-1/large.webp",
			contentType: CoverJPEGContentType,
		},
		{
			name:        "unknown variant",
			key:         "variants/library-1/42/cover-1/list.webp",
			contentType: CoverWebPContentType,
		},
		{
			name:        "foreign prefix",
			key:         "other/library-1/42/cover-1/large.webp",
			contentType: CoverWebPContentType,
		},
		{
			name:        "traversal",
			key:         "variants/library-1/42/../large.webp",
			contentType: CoverWebPContentType,
		},
		{
			name:        "nested path",
			key:         "variants/library-1/42/cover-1/nested/large.webp",
			contentType: CoverWebPContentType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validProcessingObjectKey(test.key, test.contentType); got != test.want {
				t.Fatalf("valid=%v expected=%v", got, test.want)
			}
		})
	}
}

func TestContentTypeForGeneratedKey(t *testing.T) {
	tests := map[string]string{
		"variants/library-1/42/cover-1/large.jpg":  CoverJPEGContentType,
		"variants/library-1/42/cover-1/large.webp": CoverWebPContentType,
		"variants/library-1/42/cover-1/large.png":  "",
	}

	for key, expected := range tests {
		if got := contentTypeForGeneratedKey(key); got != expected {
			t.Fatalf("key=%q contentType=%q expected=%q", key, got, expected)
		}
	}
}
