package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
	"strings"
)

type SaleHandler struct{ service *services.SaleService }

type saleTransitionRequest struct {
	Version int `json:"version"`
}

func NewSaleHandler(service *services.SaleService) *SaleHandler { return &SaleHandler{service: service} }

func (h *SaleHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	sales, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"), models.SaleFilter{
		Status: models.SaleStatus(r.URL.Query().Get("status")),
		From: strings.TrimSpace(r.URL.Query().Get("from")), To: strings.TrimSpace(r.URL.Query().Get("to")),
	}, offset, limit)
	if err != nil {
		writeSaleError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": sales, "total": total, "offset": offset, "limit": limit,
	})
}

func (h *SaleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.SaleInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writeSaleError(w, services.ErrInvalidSale)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	sale, err := h.service.Create(r.Context(), claims, input)
	if err != nil {
		writeSaleError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/sales/"+sale.ID)
	writeAuthJSON(w, http.StatusCreated, sale)
}

func (h *SaleHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	sale, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil {
		writeSaleError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, sale)
}

func (h *SaleHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.SaleInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writeSaleError(w, services.ErrInvalidSale)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	sale, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil {
		writeSaleError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, sale)
}

func (h *SaleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Delete(r.Context(), claims, r.PathValue("id")); err != nil {
		writeSaleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SaleHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, true)
}

func (h *SaleHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, false)
}

func (h *SaleHandler) transition(w http.ResponseWriter, r *http.Request, confirm bool) {
	var request saleTransitionRequest
	if decodeOwnerJSON(w, r, &request) != nil {
		writeSaleError(w, services.ErrInvalidSale)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	var sale models.Sale
	var err error
	if confirm {
		sale, err = h.service.Confirm(r.Context(), claims, r.PathValue("id"), request.Version)
	} else {
		sale, err = h.service.Cancel(r.Context(), claims, r.PathValue("id"), request.Version)
	}
	if err != nil {
		writeSaleError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, sale)
}

func writeSaleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrSaleNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "sale_not_found", "message": "Sale not found"})
	case errors.Is(err, repositories.ErrSaleConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "sale_version_conflict", "message": err.Error()})
	case errors.Is(err, repositories.ErrSaleState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "sale_not_editable", "message": err.Error()})
	case errors.Is(err, repositories.ErrSaleBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sale_book", "message": err.Error()})
	case errors.Is(err, repositories.ErrInsufficientStock):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "insufficient_stock", "message": err.Error()})
	case errors.Is(err, repositories.ErrInventoryConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "inventory_version_conflict", "message": err.Error()})
	case errors.Is(err, services.ErrInvalidSale):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_sale", "message": err.Error()})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Sale operation failed"})
	}
}
