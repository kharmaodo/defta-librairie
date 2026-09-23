package handlers

import (
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/covers"
	"defta-librairie/internal/models"
	"defta-librairie/internal/services"
	"errors"
	"io"
	"net/http"
	"strconv"
)

type bookSubmissionCreator interface {
	Submit(context.Context, *auth.Claims, models.BookInput, string, io.Reader) (services.PendingBookSubmission, error)
	List(context.Context, *auth.Claims, string, int) ([]models.BookSubmission, error)
}

type BookSubmissionHandler struct {
	service  bookSubmissionCreator
	enabled  bool
	maxBytes int64
}

func NewBookSubmissionHandler(service bookSubmissionCreator, enabled bool, maxBytes int64) *BookSubmissionHandler {
	if maxBytes < 1 {
		maxBytes = covers.DefaultMaxBytes
	}
	return &BookSubmissionHandler{service: service, enabled: enabled, maxBytes: maxBytes}
}

func (h *BookSubmissionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil {
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled", "message": "Book cover uploads are disabled"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes+multipartEnvelopeAllowance)
	if err := r.ParseMultipartForm(h.maxBytes + multipartEnvelopeAllowance); err != nil {
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
	input, err := submissionBookInput(r)
	if err != nil {
		writeBookCoverError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	submission, err := h.service.Submit(r.Context(), claims, input, header.Header.Get("Content-Type"), file)
	if err != nil {
		writeBookCoverError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/book-submissions/"+submission.ID)
	writeAuthJSON(w, http.StatusAccepted, submission)
}

func submissionBookInput(r *http.Request) (models.BookInput, error) {
	price, err := strconv.ParseFloat(r.FormValue("price"), 64)
	if err != nil {
		return models.BookInput{}, err
	}
	volume, err := strconv.Atoi(r.FormValue("volume"))
	if err != nil {
		return models.BookInput{}, err
	}
	return models.BookInput{
		Title: r.FormValue("title"), Auteur: r.FormValue("auteur"),
		Editeur: r.FormValue("editeur"), Price: price, Volume: volume,
		Status: r.FormValue("status"), Tags: r.FormValue("tags"),
		Categorie: r.FormValue("categorie"), CoverURL: r.FormValue("coverUrl"),
		LibraryID: r.FormValue("libraryId"),
	}, nil
}

var _ = errors.Is


func (h *BookSubmissionHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.enabled || h.service == nil { writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error":"covers_disabled"}); return }
	limit := 30
	if value := r.URL.Query().Get("limit"); value != "" { if parsed, err := strconv.Atoi(value); err == nil { limit = parsed } }
	claims, _ := auth.ClaimsFromContext(r.Context())
	items, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"), limit)
	if err != nil { writeBookCoverError(w, err); return }
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results":items, "total":len(items)})
}
