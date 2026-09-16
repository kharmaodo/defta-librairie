package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
)

type CoverImageTransformer interface {
	Process(
		ctx context.Context,
		source io.Reader,
	) (covers.GeneratedVariants, error)
}

type StoredCoverVariantProcessor struct {
	sources     covers.SourceReader
	variants    covers.VariantStore
	transformer CoverImageTransformer
}

func NewStoredCoverVariantProcessor(
	sources covers.SourceReader,
	variants covers.VariantStore,
	transformer CoverImageTransformer,
) (*StoredCoverVariantProcessor, error) {
	if sources == nil || variants == nil || transformer == nil {
		return nil, fmt.Errorf("invalid stored cover variant processor configuration")
	}
	return &StoredCoverVariantProcessor{
		sources:     sources,
		variants:    variants,
		transformer: transformer,
	}, nil
}

func (p *StoredCoverVariantProcessor) Process(
	ctx context.Context,
	event covers.ProcessingEvent,
) (repositories.ProcessedCover, error) {
	if ctx == nil {
		return repositories.ProcessedCover{}, fmt.Errorf("nil cover processing context")
	}

	keys, err := covers.NewProcessingObjectKeys(
		event.LibraryID,
		event.BookID,
		event.CoverID,
	)
	if err != nil {
		return repositories.ProcessedCover{}, err
	}

	source, err := p.sources.OpenSource(ctx, event.SourceObjectKey)
	if err != nil {
		return repositories.ProcessedCover{}, fmt.Errorf("open cover source: %w", err)
	}
	defer source.Close()

	generated, err := p.transformer.Process(ctx, source)
	if err != nil {
		return repositories.ProcessedCover{}, fmt.Errorf("transform cover source: %w", err)
	}

	objects := []generatedCoverObject{
		{key: keys.MasterJPEG, contentType: covers.CoverJPEGContentType, data: generated.MasterJPEG},
		{key: keys.LargeJPEG, contentType: covers.CoverJPEGContentType, data: generated.LargeJPEG},
		{key: keys.LargeWebP, contentType: covers.CoverWebPContentType, data: generated.LargeWebP},
		{key: keys.ThumbJPEG, contentType: covers.CoverJPEGContentType, data: generated.ThumbJPEG},
		{key: keys.ThumbWebP, contentType: covers.CoverWebPContentType, data: generated.ThumbWebP},
	}

	written := make([]string, 0, len(objects))
	for _, object := range objects {
		if len(object.data) == 0 {
			return repositories.ProcessedCover{}, errors.Join(
				fmt.Errorf("empty generated cover object %s", object.key),
				p.cleanup(ctx, written),
			)
		}

		err = p.variants.PutVariant(
			ctx,
			object.key,
			bytes.NewReader(object.data),
			int64(len(object.data)),
			object.contentType,
		)
		if err != nil {
			return repositories.ProcessedCover{}, errors.Join(
				fmt.Errorf("store generated cover %s: %w", object.key, err),
				p.cleanup(ctx, written),
			)
		}
		written = append(written, object.key)
	}

	return repositories.ProcessedCover{
		MasterObjectKey:    keys.MasterJPEG,
		LargeJPEGObjectKey: keys.LargeJPEG,
		LargeWebPObjectKey: keys.LargeWebP,
		ThumbJPEGObjectKey: keys.ThumbJPEG,
		ThumbWebPObjectKey: keys.ThumbWebP,
	}, nil
}

type generatedCoverObject struct {
	key         string
	contentType string
	data        []byte
}

func (p *StoredCoverVariantProcessor) cleanup(
	ctx context.Context,
	written []string,
) error {
	var cleanupErr error
	for index := len(written) - 1; index >= 0; index-- {
		if err := p.variants.DeleteVariant(ctx, written[index]); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("delete generated cover %s: %w", written[index], err),
			)
		}
	}
	return cleanupErr
}

var _ CoverVariantProcessor = (*StoredCoverVariantProcessor)(nil)
