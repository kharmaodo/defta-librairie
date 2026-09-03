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

type InventoryHandler struct{ service *services.InventoryService }

func NewInventoryHandler(service *services.InventoryService) *InventoryHandler {
	return &InventoryHandler{service: service}
}

type inventoryQuantityRequest struct {
	Quantity int    `json:"quantity"`
	Reason   string `json:"reason"`
	Version  int    `json:"version"`
}

type inventoryThresholdRequest struct {
	LowStockThreshold int `json:"lowStockThreshold"`
	Version           int `json:"version"`
}

func (h *InventoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	bookID, err := bookID(r)
	if err != nil {
		writeInventoryError(w, services.ErrInvalidInventory)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	inventory, err := h.service.Find(r.Context(), claims, bookID)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	items, total, err := h.service.List(r.Context(), claims,
		strings.TrimSpace(r.URL.Query().Get("libraryId")), r.URL.Query().Get("status"), offset, limit)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": items, "total": total, "offset": offset, "limit": limit,
	})
}

func (h *InventoryHandler) Entry(w http.ResponseWriter, r *http.Request) {
	h.move(w, r, models.InventoryMovementEntry)
}

func (h *InventoryHandler) Exit(w http.ResponseWriter, r *http.Request) {
	h.move(w, r, models.InventoryMovementExit)
}

func (h *InventoryHandler) move(w http.ResponseWriter, r *http.Request, movementType models.InventoryMovementType) {
	bookID, err := bookID(r)
	var request inventoryQuantityRequest
	if err != nil || decodeOwnerJSON(w, r, &request) != nil {
		writeInventoryError(w, services.ErrInvalidInventory)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	inventory, err := h.service.Move(r.Context(), claims, bookID, movementType,
		request.Quantity, request.Version, request.Reason)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) Adjust(w http.ResponseWriter, r *http.Request) {
	bookID, err := bookID(r)
	var request inventoryQuantityRequest
	if err != nil || decodeOwnerJSON(w, r, &request) != nil {
		writeInventoryError(w, services.ErrInvalidInventory)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	inventory, err := h.service.Adjust(r.Context(), claims, bookID, request.Quantity, request.Version, request.Reason)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) UpdateThreshold(w http.ResponseWriter, r *http.Request) {
	bookID, err := bookID(r)
	var request inventoryThresholdRequest
	if err != nil || decodeOwnerJSON(w, r, &request) != nil {
		writeInventoryError(w, services.ErrInvalidInventory)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	inventory, err := h.service.UpdateThreshold(r.Context(), claims, bookID, request.LowStockThreshold, request.Version)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, inventory)
}

func (h *InventoryHandler) ListMovements(w http.ResponseWriter, r *http.Request) {
	bookID, err := bookID(r)
	if err != nil {
		writeInventoryError(w, services.ErrInvalidInventory)
		return
	}
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	movements, total, err := h.service.ListMovements(r.Context(), claims, bookID, offset, limit)
	if err != nil {
		writeInventoryError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": movements, "total": total, "offset": offset, "limit": limit,
	})
}

func writeInventoryError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrInventoryNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "inventory_not_found", "message": "Inventory not found"})
	case errors.Is(err, repositories.ErrInventoryConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "inventory_version_conflict", "message": err.Error()})
	case errors.Is(err, repositories.ErrInsufficientStock):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "insufficient_stock", "message": err.Error()})
	case errors.Is(err, repositories.ErrInventoryUnchanged), errors.Is(err, services.ErrInvalidInventory):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_inventory", "message": err.Error()})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Inventory operation failed"})
	}
}
