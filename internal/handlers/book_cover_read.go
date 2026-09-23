package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"io"
	"net/http"
)

type bookCoverReader interface {
	Open(
		ctx context.Context,
		claims *auth.Claims,
		bookID int,
		variant string,
		format string,
	) (services.ActiveBookCover, error)
}

type publicBookCoverReader interface {
	OpenPublic(ctx context.Context, bookID int, variant, format string) (services.ActiveBookCover, error)
}

type BookCoverReadHandler struct {
	service bookCoverReader
	enabled bool
}

func NewBookCoverReadHandler(
	service bookCoverReader,
	enabled bool,
) *BookCoverReadHandler {
	return &BookCoverReadHandler{service: service, enabled: enabled}
}

func (h *BookCoverReadHandler) Serve(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, false)
}

// ServePublic serves only active processed variants for books in the public catalogue.
func (h *BookCoverReadHandler) ServePublic(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, true)
}

func (h *BookCoverReadHandler) serve(w http.ResponseWriter, r *http.Request, public bool) {
	if !h.enabled || h.service == nil {
		writeBookCoverError(w, services.ErrCoversDisabled)
		return
	}

	id, err := bookID(r)
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	variant := r.URL.Query().Get("variant")
	if variant == "" {
		variant = "large"
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "jpeg"
	}
	if !validCoverRepresentation(variant, format) {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}

	var cover services.ActiveBookCover
	if public {
		reader, ok := h.service.(publicBookCoverReader)
		if !ok {
			writeBookCoverError(w, services.ErrCoversDisabled)
			return
		}
		cover, err = reader.OpenPublic(r.Context(), id, variant, format)
	} else {
		claims, _ := auth.ClaimsFromContext(r.Context())
		cover, err = h.service.Open(r.Context(), claims, id, variant, format)
	}
	if err != nil {
		writeBookCoverError(w, err)
		return
	}
	defer cover.Body.Close()

	w.Header().Set("Content-Type", cover.ContentType)
	cacheControl := "private, max-age=300"
	if public {
		cacheControl = "public, max-age=300"
	}
	w.Header().Set("Cache-Control", cacheControl)
	w.Header().Set("ETag", cover.ETag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Header.Get("If-None-Match") == cover.ETag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, cover.Body)
}

func validCoverRepresentation(variant, format string) bool {
	switch variant {
	case "master":
		return format == "jpeg"
	case "large", "thumb":
		return format == "jpeg" || format == "webp"
	default:
		return false
	}
}
