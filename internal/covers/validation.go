package covers

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
)

const (
	DefaultMaxBytes  int64  = 5 * 1024 * 1024
	DefaultMaxPixels uint64 = 24_000_000
)

var (
	ErrEmpty               = errors.New("cover is empty")
	ErrTooLarge            = errors.New("cover exceeds size limit")
	ErrUnsupportedFormat   = errors.New("cover format is not supported")
	ErrContentTypeMismatch = errors.New("cover content type does not match its contents")
	ErrCorruptImage        = errors.New("cover image is corrupt")
	ErrTooManyPixels       = errors.New("cover exceeds pixel limit")
)

type Validator struct {
	MaxBytes  int64
	MaxPixels uint64
}

type ValidatedImage struct {
	Data        []byte
	Format      string
	Extension   string
	ContentType string
	Width       int
	Height      int
}

func NewValidator(maxBytes int64, maxPixels uint64) Validator {
	if maxBytes < 1 {
		maxBytes = DefaultMaxBytes
	}
	if maxPixels < 1 {
		maxPixels = DefaultMaxPixels
	}
	return Validator{MaxBytes: maxBytes, MaxPixels: maxPixels}
}

func (v Validator) Validate(reader io.Reader, declaredContentType string) (ValidatedImage, error) {
	if reader == nil {
		return ValidatedImage{}, ErrEmpty
	}

	data, err := io.ReadAll(io.LimitReader(reader, v.MaxBytes+1))
	if err != nil {
		return ValidatedImage{}, fmt.Errorf("read cover: %w", err)
	}
	if len(data) == 0 {
		return ValidatedImage{}, ErrEmpty
	}
	if int64(len(data)) > v.MaxBytes {
		return ValidatedImage{}, ErrTooLarge
	}

	format, extension, contentType, ok := detectFormat(data)
	if !ok {
		return ValidatedImage{}, ErrUnsupportedFormat
	}
	if declaredContentType != "" && declaredContentType != contentType {
		return ValidatedImage{}, ErrContentTypeMismatch
	}

	config, decodedFormat, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || decodedFormat != format || config.Width < 1 || config.Height < 1 {
		return ValidatedImage{}, ErrCorruptImage
	}
	pixels := uint64(config.Width) * uint64(config.Height)
	if pixels > v.MaxPixels {
		return ValidatedImage{}, ErrTooManyPixels
	}
	if _, decodedFormat, err = image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return ValidatedImage{}, ErrCorruptImage
	}

	return ValidatedImage{
		Data:        data,
		Format:      format,
		Extension:   extension,
		ContentType: contentType,
		Width:       config.Width,
		Height:      config.Height,
	}, nil
}

func detectFormat(data []byte) (format, extension, contentType string, ok bool) {
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "jpeg", "jpg", "image/jpeg", true
	}
	pngSignature := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if len(data) >= len(pngSignature) && bytes.Equal(data[:len(pngSignature)], pngSignature) {
		return "png", "png", "image/png", true
	}
	return "", "", "", false
}
