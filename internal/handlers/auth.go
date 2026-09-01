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

type AuthHandler struct {
	login    *services.LoginService
	sessions *services.SessionService
}

func NewAuthHandler(login *services.LoginService, sessions *services.SessionService) *AuthHandler {
	return &AuthHandler{login: login, sessions: sessions}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
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

	session, err := h.sessions.Start(r.Context(), result.User, remoteIP(r), userAgent(r))
	if err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Session creation failed"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": session.AccessToken,
		"refreshToken": session.RefreshToken,
		"tokenType":   "Bearer",
		"expiresIn":   int64(time.Until(session.AccessExpiresAt).Seconds()),
		"refreshExpiresIn": int64(time.Until(session.RefreshExpiresAt).Seconds()),
		"user": map[string]interface{}{
			"id": result.User.ID, "username": result.User.Username,
			"email": result.User.Email, "role": result.User.Role,
			"libraryId": nullableValue(result.User.LibraryID),
		},
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := decodeAuthJSON(w, r, &request); err != nil || request.RefreshToken == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid refresh request"})
		return
	}
	result, err := h.sessions.Refresh(r.Context(), request.RefreshToken, remoteIP(r), userAgent(r))
	if err != nil {
		if errors.Is(err, services.ErrInvalidRefreshToken) || errors.Is(err, services.ErrRefreshTokenReuse) {
			writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_refresh_token", "message": "Refresh token is invalid or expired"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Token refresh failed"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": result.AccessToken, "refreshToken": result.RefreshToken,
		"tokenType": "Bearer", "expiresIn": int64(time.Until(result.AccessExpiresAt).Seconds()),
		"refreshExpiresIn": int64(time.Until(result.RefreshExpiresAt).Seconds()),
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var request refreshRequest
	if err := decodeAuthJSON(w, r, &request); err != nil || request.RefreshToken == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid logout request"})
		return
	}
	if err := h.sessions.Logout(r.Context(), request.RefreshToken); err != nil && !errors.Is(err, services.ErrInvalidRefreshToken) {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Logout failed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func nullableValue(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func decodeAuthJSON(w http.ResponseWriter, r *http.Request, target interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func userAgent(r *http.Request) string {
	value := r.UserAgent()
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func writeAuthJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
