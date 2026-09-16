package covers

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/HugoSmits86/nativewebp"
)

func TestImageProcessorGeneratesExpectedVariants(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1200, 1200))
	for y := 0; y < 1200; y++ {
		for x := 0; x < 1200; x++ {
			source.Set(x, y, color.NRGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: 120,
				A: 255,
			})
		}
	}

	var input bytes.Buffer
	if err := png.Encode(&input, source); err != nil {
		t.Fatalf("encode input: %v", err)
	}

	variants, err := NewImageProcessor().Process(
		context.Background(),
		bytes.NewReader(input.Bytes()),
	)
	if err != nil {
		t.Fatalf("process: %v", err)
	}

	assertJPEGDimensions(
		t,
		variants.MasterJPEG,
		MasterCoverWidth,
		MasterCoverHeight,
	)
	assertJPEGDimensions(
		t,
		variants.LargeJPEG,
		LargeCoverWidth,
		LargeCoverHeight,
	)
	assertJPEGDimensions(
		t,
		variants.ThumbJPEG,
		ThumbCoverWidth,
		ThumbCoverHeight,
	)
	assertWebPDimensions(
		t,
		variants.LargeWebP,
		LargeCoverWidth,
		LargeCoverHeight,
	)
	assertWebPDimensions(
		t,
		variants.ThumbWebP,
		ThumbCoverWidth,
		ThumbCoverHeight,
	)
}

func TestImageProcessorRejectsInvalidAndOversizedSources(t *testing.T) {
	processor := NewImageProcessor()

	_, err := processor.Process(
		context.Background(),
		bytes.NewReader([]byte("not an image")),
	)
	if !errors.Is(err, ErrInvalidProcessingImage) {
		t.Fatalf("invalid image err=%v", err)
	}

	oversized := bytes.Repeat(
		[]byte{0},
		int(MaxProcessingSourceSize)+1,
	)
	_, err = processor.Process(
		context.Background(),
		bytes.NewReader(oversized),
	)
	if !errors.Is(err, ErrProcessingImageTooLarge) {
		t.Fatalf("oversized err=%v", err)
	}
}

func TestImageProcessorHonorsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewImageProcessor().Process(
		ctx,
		bytes.NewReader([]byte("ignored")),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v expected=%v", err, context.Canceled)
	}
}

func assertJPEGDimensions(
	t *testing.T,
	data []byte,
	width int,
	height int,
) {
	t.Helper()

	config, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode jpeg config: %v", err)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf(
			"jpeg dimensions=%dx%d expected=%dx%d",
			config.Width,
			config.Height,
			width,
			height,
		)
	}
}

func assertWebPDimensions(
	t *testing.T,
	data []byte,
	width int,
	height int,
) {
	t.Helper()

	if len(data) < 12 ||
		string(data[0:4]) != "RIFF" ||
		string(data[8:12]) != "WEBP" {
		t.Fatalf("invalid webp signature")
	}

	config, err := nativewebp.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode webp config: %v", err)
	}
	if config.Width != width || config.Height != height {
		t.Fatalf(
			"webp dimensions=%dx%d expected=%dx%d",
			config.Width,
			config.Height,
			width,
			height,
		)
	}
}
