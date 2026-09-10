package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
	"strings"
)

type TagHandler struct{ service *services.TagService }

func NewTagHandler(service *services.TagService) *TagHandler { return &TagHandler{service: service} }

type tagRequest struct {
	Name      string `json:"name"`
	LibraryID string `json:"libraryId"`
}

func (h *TagHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	tags, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"))
	if err != nil {
		writeTagError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": tags, "total": len(tags)})
}

func (h *TagHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request tagRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil {
		writeTagError(w, services.ErrInvalidTag)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tag, err := h.service.Create(r.Context(), claims, request.Name, request.LibraryID)
	if err != nil {
		writeTagError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/tags/"+tag.ID)
	writeAuthJSON(w, http.StatusCreated, tag)
}

func (h *TagHandler) Update(w http.ResponseWriter, r *http.Request) {
	var request tagRequest
	if err := decodeOwnerJSON(w, r, &request); err != nil || strings.TrimSpace(request.LibraryID) != "" {
		writeTagError(w, services.ErrInvalidTag)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	tag, err := h.service.Update(r.Context(), claims, r.PathValue("id"), request.Name)
	if err != nil {
		writeTagError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, tag)
}

func (h *TagHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Delete(r.Context(), claims, r.PathValue("id")); err != nil {
		writeTagError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeTagError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrTagNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "tag_not_found", "message": "Tag not found"})
	case errors.Is(err, repositories.ErrTagConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "tag_conflict", "message": "Tag already exists in this library"})
	case errors.Is(err, repositories.ErrLibraryUnavailable):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "library_unavailable", "message": "Library is unavailable"})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidTag), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid tag data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Tag operation failed"})
	}
}
