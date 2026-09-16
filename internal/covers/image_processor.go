package covers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"

	"github.com/HugoSmits86/nativewebp"
	xdraw "golang.org/x/image/draw"
)

const (
	MasterCoverWidth  = 1000
	MasterCoverHeight = 1500
	LargeCoverWidth   = 800
	LargeCoverHeight  = 1200
	ThumbCoverWidth   = 160
	ThumbCoverHeight  = 240
)

var (
	ErrInvalidProcessingImage  = errors.New("invalid cover processing image")
	ErrProcessingImageTooLarge = errors.New("cover processing image too large")
)

type GeneratedVariants struct {
	MasterJPEG []byte
	LargeJPEG  []byte
	LargeWebP  []byte
	ThumbJPEG  []byte
	ThumbWebP  []byte
}

type ImageProcessor struct {
	jpegQuality int
}

func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{jpegQuality: 82}
}

func (p *ImageProcessor) Process(
	ctx context.Context,
	source io.Reader,
) (GeneratedVariants, error) {
	if ctx == nil || source == nil {
		return GeneratedVariants{}, ErrInvalidProcessingImage
	}
	if err := ctx.Err(); err != nil {
		return GeneratedVariants{}, err
	}

	data, err := io.ReadAll(io.LimitReader(source, MaxProcessingSourceSize+1))
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("read cover source: %w", err)
	}
	if int64(len(data)) > MaxProcessingSourceSize {
		return GeneratedVariants{}, ErrProcessingImageTooLarge
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("%w: %v", ErrInvalidProcessingImage, err)
	}
	if decoded.Bounds().Dx() < 2 || decoded.Bounds().Dy() < 3 {
		return GeneratedVariants{}, ErrInvalidProcessingImage
	}

	cropped := centeredTwoByThreeCrop(decoded)

	master := resizeCover(cropped, MasterCoverWidth, MasterCoverHeight)
	if err = ctx.Err(); err != nil {
		return GeneratedVariants{}, err
	}

	large := resizeCover(cropped, LargeCoverWidth, LargeCoverHeight)
	if err = ctx.Err(); err != nil {
		return GeneratedVariants{}, err
	}

	thumb := resizeCover(cropped, ThumbCoverWidth, ThumbCoverHeight)
	if err = ctx.Err(); err != nil {
		return GeneratedVariants{}, err
	}

	masterJPEG, err := encodeJPEG(master, p.jpegQuality)
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("encode master jpeg: %w", err)
	}
	largeJPEG, err := encodeJPEG(large, p.jpegQuality)
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("encode large jpeg: %w", err)
	}
	thumbJPEG, err := encodeJPEG(thumb, p.jpegQuality)
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("encode thumb jpeg: %w", err)
	}

	largeWebP, err := encodeWebP(large)
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("encode large webp: %w", err)
	}
	thumbWebP, err := encodeWebP(thumb)
	if err != nil {
		return GeneratedVariants{}, fmt.Errorf("encode thumb webp: %w", err)
	}

	if err = ctx.Err(); err != nil {
		return GeneratedVariants{}, err
	}

	return GeneratedVariants{
		MasterJPEG: masterJPEG,
		LargeJPEG:  largeJPEG,
		LargeWebP:  largeWebP,
		ThumbJPEG:  thumbJPEG,
		ThumbWebP:  thumbWebP,
	}, nil
}

func centeredTwoByThreeCrop(source image.Image) image.Image {
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	cropWidth := width
	cropHeight := height

	if width*3 > height*2 {
		cropWidth = height * 2 / 3
	} else if height*2 > width*3 {
		cropHeight = width * 3 / 2
	}

	startX := bounds.Min.X + (width-cropWidth)/2
	startY := bounds.Min.Y + (height-cropHeight)/2

	cropped := image.NewNRGBA(image.Rect(0, 0, cropWidth, cropHeight))
	xdraw.Draw(
		cropped,
		cropped.Bounds(),
		source,
		image.Pt(startX, startY),
		xdraw.Src,
	)
	return cropped
}

func resizeCover(source image.Image, width, height int) *image.NRGBA {
	resized := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(
		resized,
		resized.Bounds(),
		source,
		source.Bounds(),
		xdraw.Over,
		nil,
	)
	return resized
}

func encodeJPEG(source image.Image, quality int) ([]byte, error) {
	var output bytes.Buffer
	err := jpeg.Encode(&output, source, &jpeg.Options{Quality: quality})
	return output.Bytes(), err
}

func encodeWebP(source image.Image) ([]byte, error) {
	var output bytes.Buffer
	err := nativewebp.Encode(
		&output,
		source,
		&nativewebp.Options{
			CompressionLevel: nativewebp.DefaultCompression,
		},
	)
	return output.Bytes(), err
}
