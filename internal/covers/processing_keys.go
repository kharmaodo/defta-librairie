package covers

import "strings"

const (
	CoverJPEGContentType = "image/jpeg"
	CoverWebPContentType = "image/webp"
)

func validProcessingObjectKey(key, contentType string) bool {
	parts := strings.Split(key, "/")
	if len(parts) != 5 {
		return false
	}

	prefix := parts[0]
	libraryID := parts[1]
	bookID := parts[2]
	coverID := parts[3]
	filename := parts[4]

	if prefix != "masters" && prefix != "variants" {
		return false
	}
	if !safeKeySegment.MatchString(libraryID) ||
		!safeKeySegment.MatchString(bookID) ||
		!safeKeySegment.MatchString(coverID) {
		return false
	}

	switch {
	case prefix == "masters":
		return filename == "master.jpg" &&
			contentType == CoverJPEGContentType

	case filename == "large.jpg" || filename == "thumb.jpg":
		return contentType == CoverJPEGContentType

	case filename == "large.webp" || filename == "thumb.webp":
		return contentType == CoverWebPContentType

	default:
		return false
	}
}

func validGeneratedObjectKey(key string) bool {
	return validProcessingObjectKey(key, contentTypeForGeneratedKey(key))
}

func contentTypeForGeneratedKey(key string) string {
	switch {
	case strings.HasSuffix(key, ".jpg"):
		return CoverJPEGContentType
	case strings.HasSuffix(key, ".webp"):
		return CoverWebPContentType
	default:
		return ""
	}
}
