package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
	"strconv"
)

type SupplierHandler struct{ service *services.SupplierService }

func NewSupplierHandler(service *services.SupplierService) *SupplierHandler { return &SupplierHandler{service: service} }

func (h *SupplierHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	results, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"),
		r.URL.Query().Get("q"), models.SupplierStatus(r.URL.Query().Get("status")), offset, limit)
	if err != nil { writeSupplierError(w, err); return }
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results":results,"total":total,"offset":offset,"limit":limit})
}

func (h *SupplierHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	supplier, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil { writeSupplierError(w, err); return }
	writeAuthJSON(w, http.StatusOK, supplier)
}

func (h *SupplierHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.SupplierInput
	if decodeOwnerJSON(w, r, &input) != nil { writeSupplierError(w, services.ErrInvalidSupplier); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	supplier, err := h.service.Create(r.Context(), claims, input)
	if err != nil { writeSupplierError(w, err); return }
	w.Header().Set("Location", "/api/manage/suppliers/"+supplier.ID)
	writeAuthJSON(w, http.StatusCreated, supplier)
}

func (h *SupplierHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.SupplierInput
	if decodeOwnerJSON(w, r, &input) != nil { writeSupplierError(w, services.ErrInvalidSupplier); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	supplier, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil { writeSupplierError(w, err); return }
	writeAuthJSON(w, http.StatusOK, supplier)
}

func (h *SupplierHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, false)
}

func (h *SupplierHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, true)
}

func (h *SupplierHandler) changeStatus(w http.ResponseWriter, r *http.Request, reactivate bool) {
	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil { writeSupplierError(w, services.ErrInvalidSupplier); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	if reactivate { err = h.service.Reactivate(r.Context(), claims, r.PathValue("id"), version) } else {
		err = h.service.Disable(r.Context(), claims, r.PathValue("id"), version)
	}
	if err != nil { writeSupplierError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func writeSupplierError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrSupplierNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error":"supplier_not_found","message":"Supplier not found"})
	case errors.Is(err, repositories.ErrSupplierConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"supplier_conflict","message":"Supplier already exists in this library"})
	case errors.Is(err, repositories.ErrSupplierVersion):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"supplier_version_conflict","message":"Supplier was modified concurrently"})
	case errors.Is(err, repositories.ErrSupplierState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"supplier_state_conflict","message":"Supplier state does not allow this operation"})
	case errors.Is(err, repositories.ErrLibraryUnavailable):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error":"library_unavailable","message":"Library is unavailable"})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error":"forbidden","message":"Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidSupplier), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid_supplier","message":"Invalid supplier data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error":"internal_error","message":"Supplier operation failed"})
	}
}
