package covers

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodedImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, width, height))
	picture.Set(0, 0, color.White)
	var output bytes.Buffer
	var err error
	switch format {
	case "jpeg":
		err = jpeg.Encode(&output, picture, &jpeg.Options{Quality: 80})
	case "png":
		err = png.Encode(&output, picture)
	default:
		t.Fatalf("unsupported test format %q", format)
	}
	if err != nil {
		t.Fatalf("encode test image: %v", err)
	}
	return output.Bytes()
}

func TestValidatorAcceptsRealJPEGAndPNG(t *testing.T) {
	validator := NewValidator(1024*1024, 100)

	tests := []struct {
		name        string
		format      string
		contentType string
		extension   string
	}{
		{name: "jpeg", format: "jpeg", contentType: "image/jpeg", extension: "jpg"},
		{name: "png", format: "png", contentType: "image/png", extension: "png"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := validator.Validate(
				bytes.NewReader(encodedImage(t, test.format, 4, 5)),
				test.contentType,
			)
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if result.Format != test.format || result.Extension != test.extension ||
				result.ContentType != test.contentType || result.Width != 4 || result.Height != 5 {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestValidatorRejectsUnsafeInputs(t *testing.T) {
	jpegData := encodedImage(t, "jpeg", 4, 5)
	pngData := encodedImage(t, "png", 4, 5)

	tests := []struct {
		name        string
		validator   Validator
		data        []byte
		contentType string
		want        error
	}{
		{name: "empty", validator: NewValidator(100, 100), want: ErrEmpty},
		{name: "too large", validator: NewValidator(2, 100), data: jpegData, contentType: "image/jpeg", want: ErrTooLarge},
		{name: "unsupported signature", validator: NewValidator(100, 100), data: []byte("GIF89a"), contentType: "image/gif", want: ErrUnsupportedFormat},
		{name: "mime mismatch", validator: NewValidator(1024*1024, 100), data: pngData, contentType: "image/jpeg", want: ErrContentTypeMismatch},
		{name: "corrupt jpeg", validator: NewValidator(100, 100), data: []byte{0xff, 0xd8, 0xff, 0x00}, contentType: "image/jpeg", want: ErrCorruptImage},
		{name: "too many pixels", validator: NewValidator(1024*1024, 19), data: jpegData, contentType: "image/jpeg", want: ErrTooManyPixels},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.validator.Validate(bytes.NewReader(test.data), test.contentType)
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
		})
	}
}

func TestValidatorUsesSafeDefaults(t *testing.T) {
	validator := NewValidator(0, 0)
	if validator.MaxBytes != DefaultMaxBytes || validator.MaxPixels != DefaultMaxPixels {
		t.Fatalf("unexpected defaults: %+v", validator)
	}
}
