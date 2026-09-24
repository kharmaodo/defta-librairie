package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
)

type CatalogueReferenceHandler struct {
	service *services.CatalogueReferenceService
}

func NewCatalogueReferenceHandler(service *services.CatalogueReferenceService) *CatalogueReferenceHandler {
	return &CatalogueReferenceHandler{service: service}
}

func (h *CatalogueReferenceHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, "category")
}

func (h *CatalogueReferenceHandler) ListPublishers(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, "publisher")
}

func (h *CatalogueReferenceHandler) list(w http.ResponseWriter, r *http.Request, kind string) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	items, err := h.service.List(r.Context(), claims, kind)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCatalogueReference) {
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_catalogue_reference", "message": "Invalid catalogue reference request"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Catalogue reference operation failed"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": items, "total": len(items)})
}
