package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

const multipartEnvelopeAllowance int64 = 64 * 1024

type bookCoverStatusService interface {
	Status(ctx context.Context, claims *auth.Claims, bookID int) (services.BookCoverStatus, error)
	Retry(ctx context.Context, claims *auth.Claims, bookID int) (services.BookCoverStatus, error)
}

type bookCoverUploader interface {
	Upload(
		ctx context.Context,
		claims *auth.Claims,
		bookID int,
		declaredContentType string,
		body io.Reader,
	) (services.PendingBookCover, error)
}

type BookCoverHandler struct {
	service       bookCoverUploader
	statusService bookCoverStatusService
	enabled       bool
	maxBytes      int64
}

func NewBookCoverHandler(
	service bookCoverUploader,
	enabled bool,
	maxBytes int64,
) *BookCoverHandler {
	if maxBytes < 1 {
		maxBytes = covers.DefaultMaxBytes
	}
	handler := &BookCoverHandler{service: service, enabled: enabled, maxBytes: maxBytes}
	if statusService, ok := service.(bookCoverStatusService); ok {
		handler.statusService = statusService
	}
	return handler
}

func (h *BookCoverHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "covers_disabled", "message": "Book cover uploads are disabled",
		})
		return
	}
	id, err := bookID(r)
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+multipartEnvelopeAllowance)
	if err = r.ParseMultipartForm(h.maxBytes + multipartEnvelopeAllowance); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeBookCoverError(w, covers.ErrTooLarge)
			return
		}
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if !singleCoverFile(r.MultipartForm) {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}

	file, header, err := r.FormFile("cover")
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	defer file.Close()
	if header.Size < 1 {
		writeBookCoverError(w, covers.ErrEmpty)
		return
	}
	if header.Size > h.maxBytes {
		writeBookCoverError(w, covers.ErrTooLarge)
		return
	}

	claims, _ := auth.ClaimsFromContext(r.Context())
	pending, err := h.service.Upload(
		r.Context(), claims, id, header.Header.Get("Content-Type"), file,
	)
	if err != nil {
		writeBookCoverError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/books/"+strconv.Itoa(id)+"/cover/"+pending.ID)
	writeAuthJSON(w, http.StatusAccepted, pending)
}

// ModerationRequired rejects the legacy direct-replacement endpoint. Existing
// book covers must not bypass the quarantine and NSFW decision workflow.
func (h *BookCoverHandler) ModerationRequired(w http.ResponseWriter, r *http.Request) {
	writeAuthJSON(w, http.StatusConflict, map[string]string{
		"error": "cover_moderation_required",
		"message": "Existing book cover replacements require moderation",
	})
}

func (h *BookCoverHandler) Status(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.statusService == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "covers_disabled", "message": "Book cover uploads are disabled",
		})
		return
	}
	id, err := bookID(r)
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	status, err := h.statusService.Status(r.Context(), claims, id)
	if err != nil {
		writeBookCoverError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, status)
}

func (h *BookCoverHandler) Retry(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.statusService == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "covers_disabled", "message": "Book cover uploads are disabled",
		})
		return
	}
	id, err := bookID(r)
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	status, err := h.statusService.Retry(r.Context(), claims, id)
	if err != nil {
		writeBookCoverError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/books/"+strconv.Itoa(id)+"/cover/status")
	writeAuthJSON(w, http.StatusAccepted, status)
}

func singleCoverFile(form *multipart.Form) bool {
	if form == nil || len(form.File) != 1 {
		return false
	}
	files, ok := form.File["cover"]
	return ok && len(files) == 1
}

func writeBookCoverError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, covers.ErrTooLarge):
		writeAuthJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
			"error": "cover_too_large", "message": err.Error(),
		})
	case errors.Is(err, covers.ErrEmpty),
		errors.Is(err, covers.ErrUnsupportedFormat),
		errors.Is(err, covers.ErrContentTypeMismatch),
		errors.Is(err, covers.ErrCorruptImage),
		errors.Is(err, covers.ErrTooManyPixels):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "invalid_cover", "message": err.Error(),
		})
	case errors.Is(err, repositories.ErrBookNotFound),
		errors.Is(err, repositories.ErrCoverBookNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{
			"error": "book_not_found", "message": "Book not found",
		})
	case errors.Is(err, repositories.ErrActiveCoverNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{
			"error": "cover_not_found", "message": "Active book cover not found",
		})
	case errors.Is(err, repositories.ErrBookCoverNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{
			"error": "cover_not_found", "message": "Book cover not found",
		})
	case errors.Is(err, repositories.ErrBookCoverNotRetryable):
		writeAuthJSON(w, http.StatusConflict, map[string]string{
			"error": "cover_not_retryable", "message": "Book cover cannot be retried",
		})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{
			"error": "forbidden", "message": "Insufficient permissions",
		})
	case errors.Is(err, services.ErrCoversDisabled):
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "covers_disabled", "message": err.Error(),
		})
	case errors.Is(err, covers.ErrStoreUnavailable),
		errors.Is(err, services.ErrCoverPersistence):
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "cover_storage_unavailable", "message": "Book cover storage is unavailable",
		})
	case errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid_request", "message": err.Error(),
		})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "internal_error", "message": "Book cover upload failed",
		})
	}
}
