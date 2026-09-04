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

type PurchaseHandler struct{ service *services.PurchaseService }

func NewPurchaseHandler(service *services.PurchaseService) *PurchaseHandler {
	return &PurchaseHandler{service: service}
}

func (h *PurchaseHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	purchases, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"),
		models.PurchaseFilter{Status:models.PurchaseStatus(r.URL.Query().Get("status")),
			SupplierID:strings.TrimSpace(r.URL.Query().Get("supplierId")), From:strings.TrimSpace(r.URL.Query().Get("from")),
			To:strings.TrimSpace(r.URL.Query().Get("to"))}, offset, limit)
	if err != nil { writePurchaseError(w, err); return }
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results":purchases,"total":total,"offset":offset,"limit":limit})
}

func (h *PurchaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	purchase, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil { writePurchaseError(w, err); return }
	writeAuthJSON(w, http.StatusOK, purchase)
}

func (h *PurchaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.PurchaseInput
	if decodeOwnerJSON(w, r, &input) != nil { writePurchaseError(w, services.ErrInvalidPurchase); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	purchase, err := h.service.Create(r.Context(), claims, input)
	if err != nil { writePurchaseError(w, err); return }
	w.Header().Set("Location", "/api/manage/purchases/"+purchase.ID)
	writeAuthJSON(w, http.StatusCreated, purchase)
}

func (h *PurchaseHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.PurchaseInput
	if decodeOwnerJSON(w, r, &input) != nil { writePurchaseError(w, services.ErrInvalidPurchase); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	purchase, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil { writePurchaseError(w, err); return }
	writeAuthJSON(w, http.StatusOK, purchase)
}

func (h *PurchaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil { writePurchaseError(w, services.ErrInvalidPurchase); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err = h.service.Delete(r.Context(), claims, r.PathValue("id"), version); err != nil {
		writePurchaseError(w, err); return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePurchaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrPurchaseNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error":"purchase_not_found","message":"Purchase not found"})
	case errors.Is(err, repositories.ErrPurchaseConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"purchase_version_conflict","message":err.Error()})
	case errors.Is(err, repositories.ErrPurchaseState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"purchase_not_editable","message":err.Error()})
	case errors.Is(err, repositories.ErrPurchaseBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid_purchase_book","message":err.Error()})
	case errors.Is(err, repositories.ErrPurchaseSupplier):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error":"supplier_unavailable","message":err.Error()})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error":"forbidden","message":"Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidPurchase), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid_purchase","message":"Invalid purchase data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error":"internal_error","message":"Purchase operation failed"})
	}
}
