package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"errors"
	"net/http"
	"strconv"
)

type BusinessAlertHandler struct {
	service *services.BusinessAlertService
}

func NewBusinessAlertHandler(s *services.BusinessAlertService) *BusinessAlertHandler {
	return &BusinessAlertHandler{service: s}
}
func (h *BusinessAlertHandler) List(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	claims, _ := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeAuthJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	q, err := r.URL.Query(), error(nil)
	offset, limit := 0, 30
	for k, v := range q {
		if len(v) != 1 || (k != "libraryId" && k != "kind" && k != "offset" && k != "limit") {
			err = services.ErrInvalidBusinessAlert
		}
	}
	for _, p := range []struct {
		k string
		n *int
	}{{"offset", &offset}, {"limit", &limit}} {
		if v, ok := q[p.k]; ok {
			n, e := strconv.Atoi(v[0])
			if e != nil {
				err = services.ErrInvalidBusinessAlert
			}
			*p.n = n
		}
	}
	if err != nil {
		writeAuthJSON(w, 400, map[string]string{"error": "invalid_alert_filter"})
		return
	}
	result, err := h.service.List(r.Context(), claims, q.Get("libraryId"), q.Get("kind"), offset, limit)
	if err != nil {
		status, code := 500, "internal_error"
		if errors.Is(err, services.ErrInvalidBusinessAlert) {
			status, code = 400, "invalid_alert_filter"
		}
		if errors.Is(err, services.ErrBookForbidden) {
			status, code = 403, "forbidden"
		}
		writeAuthJSON(w, status, map[string]string{"error": code})
		return
	}
	writeAuthJSON(w, 200, result)
}
