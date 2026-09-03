package handlers

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
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
	login        *services.LoginService
	sessions     *services.SessionService
	passwords    *services.PasswordService
	cookieSecure bool
}

const (
	refreshCookieName   = "defta_refresh"
	cookieSessionHeader = "X-Defta-Session"
)

func NewAuthHandler(login *services.LoginService, sessions *services.SessionService, passwords *services.PasswordService, cookieSecure bool) *AuthHandler {
	return &AuthHandler{login: login, sessions: sessions, passwords: passwords, cookieSecure: cookieSecure}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
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
	payload := map[string]interface{}{
		"accessToken":      session.AccessToken,
		"tokenType":        "Bearer",
		"expiresIn":        int64(time.Until(session.AccessExpiresAt).Seconds()),
		"refreshExpiresIn": int64(time.Until(session.RefreshExpiresAt).Seconds()),
		"user": map[string]interface{}{
			"id": result.User.ID, "username": result.User.Username,
			"email": result.User.Email, "role": result.User.Role,
			"libraryId":              nullableValue(result.User.LibraryID),
			"passwordChangeRequired": result.User.MustChangePassword,
		},
	}
	if wantsCookieSession(r) {
		h.setRefreshCookie(w, session.RefreshToken, session.RefreshExpiresAt)
	} else {
		payload["refreshToken"] = session.RefreshToken
	}
	writeAuthJSON(w, http.StatusOK, payload)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	rawToken, cookieSession, err := refreshTokenFromRequest(w, r)
	if err != nil || rawToken == "" {
		if cookieSession {
			h.clearRefreshCookie(w)
		}
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid refresh request"})
		return
	}
	result, err := h.sessions.Refresh(r.Context(), rawToken, remoteIP(r), userAgent(r))
	if err != nil {
		if errors.Is(err, services.ErrInvalidRefreshToken) || errors.Is(err, services.ErrRefreshTokenReuse) {
			if cookieSession {
				h.clearRefreshCookie(w)
			}
			writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_refresh_token", "message": "Refresh token is invalid or expired"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Token refresh failed"})
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	payload := map[string]interface{}{
		"accessToken": result.AccessToken,
		"tokenType":   "Bearer", "expiresIn": int64(time.Until(result.AccessExpiresAt).Seconds()),
		"refreshExpiresIn": int64(time.Until(result.RefreshExpiresAt).Seconds()),
	}
	if cookieSession {
		h.setRefreshCookie(w, result.RefreshToken, result.RefreshExpiresAt)
	} else {
		payload["refreshToken"] = result.RefreshToken
	}
	writeAuthJSON(w, http.StatusOK, payload)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	rawToken, cookieSession, err := refreshTokenFromRequest(w, r)
	if cookieSession {
		h.clearRefreshCookie(w)
	}
	if err != nil || rawToken == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid logout request"})
		return
	}
	if err := h.sessions.Logout(r.Context(), rawToken); err != nil && !errors.Is(err, services.ErrInvalidRefreshToken) {
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Logout failed"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var request changePasswordRequest
	if err := decodeAuthJSON(w, r, &request); err != nil || request.CurrentPassword == "" || request.NewPassword == "" {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid password change request"})
		return
	}
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		writeAuthJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized", "message": "Authentication required"})
		return
	}
	err := h.passwords.Change(r.Context(), claims.Subject, request.CurrentPassword, request.NewPassword, remoteIP(r))
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidCurrentPassword):
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_current_password", "message": "Current password is invalid"})
		case errors.Is(err, services.ErrPasswordUnchanged), errors.Is(err, services.ErrPasswordReused), errors.Is(err, auth.ErrPasswordTooShort), errors.Is(err, auth.ErrPasswordTooWeak):
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_new_password", "message": err.Error()})
		default:
			writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Password change failed"})
		}
		return
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) ActiveSessions(w http.ResponseWriter, r *http.Request) {
	offset, limit := normalizeAPIPagination(r.URL.Query().Get("offset"), r.URL.Query().Get("limit"), 30)
	claims, _ := auth.ClaimsFromContext(r.Context())
	filter := models.SessionFilter{
		Username: r.URL.Query().Get("username"), Role: r.URL.Query().Get("role"),
		IPAddress: r.URL.Query().Get("ipAddress"), UserAgent: r.URL.Query().Get("userAgent"),
	}
	sessions, total, err := h.sessions.ListActive(r.Context(), claims, filter, offset, limit)
	if err != nil {
		if errors.Is(err, services.ErrInvalidSessionFilter) {
			writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_filter", "message": err.Error()})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Session query failed"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"results": sessions, "total": total, "offset": offset, "limit": limit,
		"currentSessionId": claims.SessionID,
	})
}

func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	sessionID := strings.TrimSpace(r.PathValue("id"))
	if sessionID == "" || len(sessionID) > 128 {
		writeAuthJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request", "message": "Invalid session identifier"})
		return
	}
	if err := h.sessions.RevokeActive(r.Context(), claims, sessionID); err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "session_not_found", "message": "Active session not found"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Session revocation failed"})
		return
	}
	if sessionID == claims.SessionID {
		h.clearRefreshCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) RevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	revoked, err := h.sessions.RevokeOthers(r.Context(), claims)
	if err != nil {
		if errors.Is(err, services.ErrSessionNotFound) {
			writeAuthJSON(w, http.StatusNotFound, map[string]string{"error": "session_not_found", "message": "Current active session not found"})
			return
		}
		writeAuthJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error", "message": "Other sessions revocation failed"})
		return
	}
	writeAuthJSON(w, http.StatusOK, map[string]interface{}{
		"revoked": revoked, "currentSessionId": claims.SessionID,
	})
}

func wantsCookieSession(r *http.Request) bool {
	return strings.EqualFold(strings.TrimSpace(r.Header.Get(cookieSessionHeader)), "cookie")
}

func refreshTokenFromRequest(w http.ResponseWriter, r *http.Request) (string, bool, error) {
	if wantsCookieSession(r) {
		cookie, err := r.Cookie(refreshCookieName)
		if err != nil {
			return "", true, err
		}
		return cookie.Value, true, nil
	}
	var request refreshRequest
	if err := decodeAuthJSON(w, r, &request); err != nil {
		return "", false, err
	}
	return request.RefreshToken, false, nil
}

func (h *AuthHandler) setRefreshCookie(w http.ResponseWriter, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: token, Path: "/api/auth",
		Expires: expires, MaxAge: maxAge, HttpOnly: true, Secure: h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/api/auth",
		Expires: time.Unix(1, 0), MaxAge: -1, HttpOnly: true,
		Secure: h.cookieSecure, SameSite: http.SameSiteStrictMode,
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
		"sessionId": claims.SessionID, "passwordChangeRequired": claims.PasswordChangeRequired,
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
