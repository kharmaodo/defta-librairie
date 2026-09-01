package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

type BookManagementHandler struct{ service *services.BookService }

func NewBookManagementHandler(service *services.BookService) *BookManagementHandler {
	return &BookManagementHandler{service: service}
}

func (h *BookManagementHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	books, total, err := h.service.List(r.Context(), claims, strings.TrimSpace(r.URL.Query().Get("libraryId")), offset, limit)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": books, "total": total, "offset": offset, "limit": limit,
	})
}

func (h *BookManagementHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		writeBookError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	book, err := h.service.Find(r.Context(), claims, id)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, book)
}

func (h *BookManagementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.BookInput
	if err := decodeOwnerJSON(w, r, &input); err != nil {
		writeBookError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	book, err := h.service.Create(r.Context(), claims, input)
	if err != nil {
		writeBookError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/books/"+strconv.Itoa(book.ID))
	writeAuthJSON(w, http.StatusCreated, book)
}

func (h *BookManagementHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		writeBookError(w, services.ErrInvalidBook)
		return
	}
	var input models.BookInput
	if err = decodeOwnerJSON(w, r, &input); err != nil {
		writeBookError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	book, err := h.service.Update(r.Context(), claims, id, input)
	if err != nil {
		writeBookError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, book)
}

func (h *BookManagementHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := bookID(r)
	if err != nil {
		writeBookError(w, services.ErrInvalidBook)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err = h.service.Delete(r.Context(), claims, id); err != nil {
		writeBookError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func bookID(r *http.Request) (int, error) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		return 0, services.ErrInvalidBook
	}
	return id, nil
}

func writeBookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrBookNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "book_not_found", "message": "Book not found"})
	case errors.Is(err, repositories.ErrBookConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "version_conflict", "message": err.Error()})
	case errors.Is(err, repositories.ErrLibraryUnavailable):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "library_unavailable", "message": err.Error()})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": err.Error()})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Book operation failed"})
	}
}
