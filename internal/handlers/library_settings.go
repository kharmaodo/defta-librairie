package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

type LibrarySettingsHandler struct {
	service *services.LibrarySettingsService
}

func NewLibrarySettingsHandler(s *services.LibrarySettingsService) *LibrarySettingsHandler {
	return &LibrarySettingsHandler{service: s}
}
func (h *LibrarySettingsHandler) Settings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	fail := func(err error) {
		status, code := 500, "internal_error"
		switch {
		case errors.Is(err, services.ErrBookForbidden):
			status, code = 403, "forbidden"
		case errors.Is(err, services.ErrInvalidBook), errors.Is(err, services.ErrInvalidSettings):
			status, code = 400, "invalid_settings"
		case errors.Is(err, repositories.ErrSettingsNotFound):
			status, code = 404, "library_not_found"
		case errors.Is(err, repositories.ErrSettingsConflict):
			status, code = 409, "version_conflict"
		}
		writeAuthJSON(w, status, map[string]string{"error": code})
	}
	c, _ := auth.ClaimsFromContext(r.Context())
	if c == nil {
		writeAuthJSON(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	for k, v := range q {
		if k != "libraryId" || len(v) != 1 {
			err = services.ErrInvalidSettings
		}
	}
	if err != nil {
		fail(services.ErrInvalidSettings)
		return
	}
	if r.Method == http.MethodPut {
		r.Body = http.MaxBytesReader(w, r.Body, 200000)
		d := json.NewDecoder(r.Body)
		d.DisallowUnknownFields()
		var v models.LibrarySettings
		if err = d.Decode(&v); err != nil {
			fail(services.ErrInvalidSettings)
			return
		}
		var extra interface{}
		if d.Decode(&extra) != io.EOF {
			fail(services.ErrInvalidSettings)
			return
		}
		if err = h.service.Update(r.Context(), c, q.Get("libraryId"), v); err != nil {
			fail(err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	v, err := h.service.Find(r.Context(), c, q.Get("libraryId"))
	if err != nil {
		fail(err)
		return
	}
	writeAuthJSON(w, 200, v)
}
