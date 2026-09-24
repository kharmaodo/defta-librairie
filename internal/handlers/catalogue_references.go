package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
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


func (h *CatalogueReferenceHandler) DisableCategory(w http.ResponseWriter,r *http.Request){h.setActive(w,r,"category",false)}
func (h *CatalogueReferenceHandler) DisablePublisher(w http.ResponseWriter,r *http.Request){h.setActive(w,r,"publisher",false)}
func (h *CatalogueReferenceHandler) setActive(w http.ResponseWriter,r *http.Request,kind string,active bool){
 claims,_:=auth.ClaimsFromContext(r.Context())
 if err:=h.service.SetActive(r.Context(),claims,kind,r.PathValue("id"),active);err!=nil{
  status:=http.StatusInternalServerError; code:="internal_error"
  if errors.Is(err,services.ErrBookForbidden){status=http.StatusForbidden;code="forbidden"} else if errors.Is(err,services.ErrInvalidCatalogueReference){status=http.StatusBadRequest;code="invalid_catalogue_reference"} else if errors.Is(err,repositories.ErrCatalogueReferenceNotFound){status=http.StatusNotFound;code="catalogue_reference_not_found"}
  writeAuthJSON(w,status,map[string]string{"error":code,"message":"Catalogue reference operation failed"});return
 };w.WriteHeader(http.StatusNoContent)
}


func (h *CatalogueReferenceHandler) UpdateCategory(w http.ResponseWriter,r *http.Request){h.update(w,r,"category")}
func (h *CatalogueReferenceHandler) UpdatePublisher(w http.ResponseWriter,r *http.Request){h.update(w,r,"publisher")}
func (h *CatalogueReferenceHandler) update(w http.ResponseWriter,r *http.Request,kind string){
 var value models.CatalogueReference
 if decodeOwnerJSON(w,r,&value)!=nil{writeAuthJSON(w,http.StatusBadRequest,map[string]string{"error":"invalid_catalogue_reference","message":"Invalid catalogue reference"});return}
 claims,_:=auth.ClaimsFromContext(r.Context())
 if err:=h.service.Update(r.Context(),claims,kind,r.PathValue("id"),value);err!=nil{h.setActiveError(w,err);return}
 w.WriteHeader(http.StatusNoContent)
}
func (h *CatalogueReferenceHandler) setActiveError(w http.ResponseWriter,err error){
 status:=http.StatusInternalServerError;code:="internal_error"
 if errors.Is(err,services.ErrBookForbidden){status=http.StatusForbidden;code="forbidden"} else if errors.Is(err,services.ErrInvalidCatalogueReference){status=http.StatusBadRequest;code="invalid_catalogue_reference"} else if errors.Is(err,repositories.ErrCatalogueReferenceNotFound){status=http.StatusNotFound;code="catalogue_reference_not_found"} else if errors.Is(err,repositories.ErrCatalogueReferenceConflict){status=http.StatusConflict;code="catalogue_reference_conflict"}
 writeAuthJSON(w,status,map[string]string{"error":code,"message":"Catalogue reference operation failed"})
}
