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

type CustomerReturnHandler struct{ service *services.CustomerReturnService }
type customerReturnTransitionRequest struct { Version int `json:"version"` }

func NewCustomerReturnHandler(service *services.CustomerReturnService) *CustomerReturnHandler {
	return &CustomerReturnHandler{service: service}
}

func (h *CustomerReturnHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	values, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("libraryId"),
		models.CustomerReturnFilter{
			Status: models.CustomerReturnStatus(r.URL.Query().Get("status")),
			SaleID: strings.TrimSpace(r.URL.Query().Get("saleId")),
			CustomerID: strings.TrimSpace(r.URL.Query().Get("customerId")),
			From: strings.TrimSpace(r.URL.Query().Get("from")), To: strings.TrimSpace(r.URL.Query().Get("to")),
		}, offset, limit)
	if err != nil { writeCustomerReturnError(w, err); return }
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results":values,"total":total,"offset":offset,"limit":limit})
}

func (h *CustomerReturnHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	value, err := h.service.Find(r.Context(), claims, r.PathValue("id"))
	if err != nil { writeCustomerReturnError(w, err); return }
	writeAuthJSON(w, http.StatusOK, value)
}

func (h *CustomerReturnHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.CustomerReturnInput
	if decodeOwnerJSON(w, r, &input) != nil { writeCustomerReturnError(w, services.ErrInvalidCustomerReturn); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	value, err := h.service.Create(r.Context(), claims, input)
	if err != nil { writeCustomerReturnError(w, err); return }
	writeAuthJSON(w, http.StatusCreated, value)
}

func (h *CustomerReturnHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input models.CustomerReturnInput
	if decodeOwnerJSON(w, r, &input) != nil { writeCustomerReturnError(w, services.ErrInvalidCustomerReturn); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	value, err := h.service.Update(r.Context(), claims, r.PathValue("id"), input)
	if err != nil { writeCustomerReturnError(w, err); return }
	writeAuthJSON(w, http.StatusOK, value)
}

func (h *CustomerReturnHandler) Complete(w http.ResponseWriter, r *http.Request) { h.transition(w, r, true) }
func (h *CustomerReturnHandler) Cancel(w http.ResponseWriter, r *http.Request) { h.transition(w, r, false) }
func (h *CustomerReturnHandler) transition(w http.ResponseWriter, r *http.Request, complete bool) {
	var request customerReturnTransitionRequest
	if decodeOwnerJSON(w, r, &request) != nil { writeCustomerReturnError(w, services.ErrInvalidCustomerReturn); return }
	claims, _ := auth.ClaimsFromContext(r.Context())
	var value models.CustomerReturn; var err error
	if complete { value, err = h.service.Complete(r.Context(), claims, r.PathValue("id"), request.Version) } else {
		value, err = h.service.Cancel(r.Context(), claims, r.PathValue("id"), request.Version)
	}
	if err != nil { writeCustomerReturnError(w, err); return }
	writeAuthJSON(w, http.StatusOK, value)
}

func writeCustomerReturnError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrCustomerReturnNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error":"customer_return_not_found","message":"Customer return not found"})
	case errors.Is(err, repositories.ErrCustomerReturnConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"customer_return_version_conflict","message":err.Error()})
	case errors.Is(err, repositories.ErrCustomerReturnState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"customer_return_not_editable","message":err.Error()})
	case errors.Is(err, repositories.ErrCustomerReturnQuantity):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error":"return_quantity_exceeded","message":err.Error()})
	case errors.Is(err, repositories.ErrCustomerReturnSale):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error":"sale_unavailable","message":err.Error()})
	case errors.Is(err, repositories.ErrCustomerReturnLine):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid_return_line","message":err.Error()})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error":"forbidden","message":"Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidCustomerReturn), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid_customer_return","message":"Invalid customer return data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error":"internal_error","message":"Customer return operation failed"})
	}
}
