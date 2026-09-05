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

type CashRegisterHandler struct{ service *services.CashRegisterService }

func NewCashRegisterHandler(service *services.CashRegisterService) *CashRegisterHandler {
	return &CashRegisterHandler{service: service}
}

func (h *CashRegisterHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	results, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"),
		models.CashRegisterFilter{Query: r.URL.Query().Get("q"),
			Status: models.CashRegisterStatus(r.URL.Query().Get("status"))}, offset, limit)
	if err != nil { writeCashRegisterError(w, err); return }
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": results, "total": total, "offset": offset, "limit": limit})
}

func (h *CashRegisterHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	register, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil { writeCashRegisterError(w, err); return }
	writeAuthJSON(w, http.StatusOK, register)
}

func (h *CashRegisterHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CashRegisterInput
	if decodeOwnerJSON(w, r, &input) != nil { writeCashRegisterError(w, services.ErrInvalidCashRegister); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	register, err := h.service.Create(r.Context(), claims, input)
	if err != nil { writeCashRegisterError(w, err); return }
	w.Header().Set("Location", "/api/manage/cash-registers/"+register.ID)
	writeAuthJSON(w, http.StatusCreated, register)
}

func (h *CashRegisterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.CashRegisterInput
	if decodeOwnerJSON(w, r, &input) != nil { writeCashRegisterError(w, services.ErrInvalidCashRegister); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	register, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil { writeCashRegisterError(w, err); return }
	writeAuthJSON(w, http.StatusOK, register)
}

func (h *CashRegisterHandler) Disable(w http.ResponseWriter, r *http.Request) { h.changeStatus(w, r, false) }
func (h *CashRegisterHandler) Reactivate(w http.ResponseWriter, r *http.Request) { h.changeStatus(w, r, true) }

func (h *CashRegisterHandler) changeStatus(w http.ResponseWriter, r *http.Request, reactivate bool) {
	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil { writeCashRegisterError(w, services.ErrInvalidCashRegister); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	if reactivate { err = h.service.Reactivate(r.Context(), claims, r.PathValue("id"), version) } else {
		err = h.service.Disable(r.Context(), claims, r.PathValue("id"), version)
	}
	if err != nil { writeCashRegisterError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func writeCashRegisterError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrCashRegisterNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "cash_register_not_found", "message": "Cash register not found"})
	case errors.Is(err, repositories.ErrCashRegisterConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "cash_register_conflict", "message": "Cash register name already exists"})
	case errors.Is(err, repositories.ErrCashRegisterVersion):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "cash_register_version_conflict", "message": "Cash register was modified concurrently"})
	case errors.Is(err, repositories.ErrCashRegisterState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "cash_register_state_conflict", "message": "Cash register state does not allow this operation"})
	case errors.Is(err, repositories.ErrLibraryUnavailable):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "library_unavailable", "message": "Library is unavailable"})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidCashRegister), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_cash_register", "message": "Invalid cash register data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Cash register operation failed"})
	}
}
