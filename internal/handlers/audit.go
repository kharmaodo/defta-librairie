package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"net/http"
	"strings"
)

type AuditHandler struct{ service *services.AuditService }

func NewAuditHandler(service *services.AuditService) *AuditHandler {
	return &AuditHandler{service: service}
}

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	success, valid := parseAuditSuccess(r.URL.Query().Get("success"))
	if !valid {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter", "message": "Invalid audit success filter"})
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	logs, total, err := h.service.List(r.Context(), claims, r.URL.Query().Get("action"), success, offset, limit)
	if err != nil {
		if err == services.ErrInvalidAuditFilter {
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter", "message": err.Error()})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Audit query failed"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": logs, "total": total, "offset": offset, "limit": limit,
	})
}

func parseAuditSuccess(value string) (*bool, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return nil, true
	case "true", "1":
		result := true
		return &result, true
	case "false", "0":
		result := false
		return &result, true
	default:
		return nil, false
	}
}
