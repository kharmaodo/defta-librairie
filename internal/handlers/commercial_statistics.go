package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
)

type CommercialStatisticsHandler struct {
	service *services.CommercialStatisticsService
}

func NewCommercialStatisticsHandler(service *services.CommercialStatisticsService) *CommercialStatisticsHandler {
	return &CommercialStatisticsHandler{service: service}
}

func (h *CommercialStatisticsHandler) Summary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	claims, _ := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "Authentication required"})
		return
	}
	q := r.URL.Query()
	for _, key := range []string{"libraryId", "from", "to"} {
		if len(q[key]) > 1 {
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_statistics_filter", "message": "Duplicate filter"})
			return
		}
	}
	value, err := h.service.Summary(r.Context(), claims, q.Get("libraryId"), q.Get("from"), q.Get("to"))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrBookForbidden):
			writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden", "message": "Insufficient permissions"})
		case errors.Is(err, services.ErrInvalidStatistics):
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_statistics_filter", "message": err.Error()})
		default:
			writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Statistics unavailable"})
		}
		return
	}
	writeAuthJSON(w, http.StatusOK, value)
}
