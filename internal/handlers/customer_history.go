package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/services"
	"net/http"
	"strconv"
)

func (h *CustomerHandler) SaleHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	q := r.URL.Query()
	for key, values := range q {
		if len(values) != 1 || (key != "status" && key != "from" && key != "to" && key != "offset" && key != "limit") {
			writeCustomerError(w, services.ErrInvalidCustomerHistory)
			return
		}
	}
	offset, limit := 0, 30
	for _, pair := range []struct {
		key    string
		target *int
	}{{"offset", &offset}, {"limit", &limit}} {
		if values, ok := q[pair.key]; ok {
			n, err := strconv.Atoi(values[0])
			if err != nil {
				writeCustomerError(w, services.ErrInvalidCustomerHistory)
				return
			}
			*pair.target = n
		}
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	values, total, err := h.service.SaleHistory(r.Context(), claims, r.PathValue("id"), models.CustomerHistoryFilter{Status: models.SaleStatus(q.Get("status")), From: q.Get("from"), To: q.Get("to")}, offset, limit)
	if err != nil {
		writeCustomerError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{"results": values, "total": total, "offset": offset, "limit": limit})
}
