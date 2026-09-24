package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"defta-librairie/internal/models"
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


func (h *CatalogueReferenceHandler) CreateCategory(w http.ResponseWriter, r *http.Request) { h.create(w,r,"category") }
func (h *CatalogueReferenceHandler) CreatePublisher(w http.ResponseWriter, r *http.Request) { h.create(w,r,"publisher") }
func (h *CatalogueReferenceHandler) create(w http.ResponseWriter, r *http.Request, kind string) {
 var value models.CatalogueReference
 if decodeOwnerJSON(w,r,&value)!=nil { writeAuthJSON(w,http.StatusBadRequest,map[string]string{"error":"invalid_catalogue_reference","message":"Invalid catalogue reference"}); return }
 claims,_:=auth.ClaimsFromContext(r.Context())
 if err:=h.service.Create(r.Context(),claims,kind,value);err!=nil { writeAuthJSON(w,http.StatusForbidden,map[string]string{"error":"forbidden","message":"Insufficient permissions"});return }
 w.WriteHeader(http.StatusCreated)
}
