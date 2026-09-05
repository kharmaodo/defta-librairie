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

type CustomerHandler struct{ service *services.CustomerService }

func NewCustomerHandler(service *services.CustomerService) *CustomerHandler {
	return &CustomerHandler{service: service}
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	results, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"),
		models.CustomerFilter{Query: r.URL.Query().Get("q"), Status: models.CustomerStatus(r.URL.Query().Get("status"))}, offset, limit)
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": results, "total": total, "offset": offset, "limit": limit})
}

func (h *CustomerHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	customer, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, customer)
}

func (h *CustomerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CustomerInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writeCustomerError(w, services.ErrInvalidCustomer)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	customer, err := h.service.Create(r.Context(), claims, input)
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/customers/"+customer.ID)
	writeAuthJSON(w, http.StatusCreated, customer)
}

func (h *CustomerHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.CustomerInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writeCustomerError(w, services.ErrInvalidCustomer)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	customer, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, customer)
}

func (h *CustomerHandler) Disable(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, false)
}

func (h *CustomerHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	h.changeStatus(w, r, true)
}

func (h *CustomerHandler) changeStatus(w http.ResponseWriter, r *http.Request, reactivate bool) {
	version, err := strconv.Atoi(r.URL.Query().Get("version"))
	if err != nil {
		writeCustomerError(w, services.ErrInvalidCustomer)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if reactivate {
		err = h.service.Reactivate(r.Context(), claims, r.PathValue("id"), version)
	} else {
		err = h.service.Disable(r.Context(), claims, r.PathValue("id"), version)
	}
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeCustomerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrCustomerNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "customer_not_found", "message": "Customer not found"})
	case errors.Is(err, repositories.ErrCustomerConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "customer_conflict", "message": "Customer reference already exists"})
	case errors.Is(err, repositories.ErrCustomerVersion):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "customer_version_conflict", "message": "Customer was modified concurrently"})
	case errors.Is(err, repositories.ErrCustomerState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "customer_state_conflict", "message": "Customer state does not allow this operation"})
	case errors.Is(err, repositories.ErrLibraryUnavailable):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "library_unavailable", "message": "Library is unavailable"})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidCustomer), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_customer", "message": "Invalid customer data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Customer operation failed"})
	}
}
