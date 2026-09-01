package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/services"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type AuthHandler struct{ login *services.LoginService }

func NewAuthHandler(login *services.LoginService) *AuthHandler { return &AuthHandler{login: login} }

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request loginRequest
	if err := decoder.Decode(&request); err != nil || strings.TrimSpace(request.Username) == "" || request.Password == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid login request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid login request"})
		return
	}

	result, err := h.login.Login(r.Context(), request.Username, request.Password, remoteIP(r))
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) || errors.Is(err, services.ErrAccountDisabled) || errors.Is(err, services.ErrAccountLocked) {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials", "message": "Invalid username or password"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Authentication failed"})
		return
	}

	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": result.AccessToken,
		"tokenType": "Bearer",
		"expiresIn": int64(time.Until(result.ExpiresAt).Seconds()),
		"user": map[string]interface{}{
			"id": result.User.ID, "username": result.User.Username,
			"email": result.User.Email, "role": result.User.Role,
			"libraryId": nullableValue(result.User.LibraryID),
		},
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "Authentication required"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"id": claims.Subject, "role": claims.Role, "libraryId": nullableValue(claims.LibraryID),
	})
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil { return host }
	return r.RemoteAddr
}

func nullableValue(value string) interface{} {
	if value == "" { return nil }
	return value
}

func writeAuthJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
