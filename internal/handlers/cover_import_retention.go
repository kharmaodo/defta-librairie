package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"defta-librairie/internal/auth"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
)

type coverImportHoldService interface {
	Hold(context.Context, *auth.Claims, string, string, time.Time) error
	Release(context.Context, *auth.Claims, string) error
}

type CoverImportRetentionHandler struct{ service coverImportHoldService }

func NewCoverImportRetentionHandler(service coverImportHoldService) *CoverImportRetentionHandler {
	return &CoverImportRetentionHandler{service: service}
}

func (h *CoverImportRetentionHandler) Hold(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeHoldError(w, services.ErrCoversDisabled)
		return
	}
	var request struct {
		Reason    string `json:"reason"`
		ExpiresAt string `json:"expiresAt"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2048)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeHoldError(w, repositories.ErrCoverImportHoldInvalid)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeHoldError(w, repositories.ErrCoverImportHoldInvalid)
		return
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, request.ExpiresAt)
	if err != nil {
		writeHoldError(w, repositories.ErrCoverImportHoldInvalid)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err = h.service.Hold(r.Context(), claims, r.PathValue("id"), request.Reason, expiresAt); err != nil {
		writeHoldError(w, err)
		return
	}
	writeAuthJSON(w, http.StatusCreated, map[string]string{"jobId": r.PathValue("id"), "expiresAt": expiresAt.UTC().Format(time.RFC3339Nano)})
}

func (h *CoverImportRetentionHandler) Release(w http.ResponseWriter, r *http.Request) {
	if h.service == nil {
		writeHoldError(w, services.ErrCoversDisabled)
		return
	}
	claims, _ := auth.ClaimsFromContext(r.Context())
	if err := h.service.Release(r.Context(), claims, r.PathValue("id")); err != nil {
		writeHoldError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeHoldError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrBookForbidden):
		writeAuthJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, repositories.ErrCoverImportHoldInvalid):
		writeAuthJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "invalid_cover_import_hold"})
	case errors.Is(err, repositories.ErrCoverImportHoldNotFound):
		writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "cover_import_not_found"})
	case errors.Is(err, repositories.ErrCoverImportHoldConflict):
		writeAuthJSON(w, http.StatusConflict, map[string]string{"error": "cover_import_cleanup_started"})
	case errors.Is(err, services.ErrCoversDisabled):
		writeAuthJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "covers_disabled"})
	default:
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
	}
}
