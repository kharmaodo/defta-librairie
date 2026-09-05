package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
)

type PaymentHandler struct{ service *services.PaymentService }

func NewPaymentHandler(service *services.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

func (h *PaymentHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	results, total, err := h.service.List(r.Context(), claims, r.PathValue("id"), models.PaymentFilter{
		Method: models.PaymentMethod(r.URL.Query().Get("method")), Status: models.PaymentStatus(r.URL.Query().Get("status")),
	}, offset, limit)
	if err != nil {
		writePaymentError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": results, "total": total, "offset": offset, "limit": limit})
}

func (h *PaymentHandler) Balance(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	balance, err := h.service.Balance(r.Context(), claims, r.PathValue("id"))
	if err != nil {
		writePaymentError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, balance)
}

func (h *PaymentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input models.PaymentInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writePaymentError(w, services.ErrInvalidPayment)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	payment, err := h.service.Create(r.Context(), claims, r.PathValue("id"), input)
	if err != nil {
		writePaymentError(w, err)
		return
	}
	w.Header().Set("Location", "/api/manage/payments/"+payment.ID)
	writeAuthJSON(w, http.StatusCreated, payment)
}

func (h *PaymentHandler) Void(w http.ResponseWriter, r *http.Request) {
	var input models.PaymentVoidInput
	if decodeOwnerJSON(w, r, &input) != nil {
		writePaymentError(w, services.ErrInvalidPayment)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	payment, err := h.service.Void(r.Context(), claims, r.PathValue("id"), input)
	if err != nil {
		writePaymentError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, payment)
}

func writePaymentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repositories.ErrPaymentNotFound), errors.Is(err, repositories.ErrPaymentSaleNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "payment_not_found", "message": "Payment or sale not found"})
	case errors.Is(err, repositories.ErrPaymentConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "payment_reference_conflict", "message": "Payment reference already exists"})
	case errors.Is(err, repositories.ErrPaymentVersion):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "payment_version_conflict", "message": "Payment was modified concurrently"})
	case errors.Is(err, repositories.ErrPaymentState):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "payment_state_conflict", "message": "Payment state does not allow this operation"})
	case errors.Is(err, repositories.ErrPaymentOverpaid):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "payment_exceeds_balance", "message": "Payment exceeds sale remaining amount"})
	case errors.Is(err, repositories.ErrPaymentUnavailable):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "payment_context_unavailable", "message": "Sale or cash register is unavailable"})
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
	case errors.Is(err, services.ErrInvalidPayment), errors.Is(err, services.ErrInvalidBook):
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_payment", "message": "Invalid payment data"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Payment operation failed"})
	}
}
