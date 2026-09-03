package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRefreshTokenFromCookieSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	request.Header.Set(cookieSessionHeader, "cookie")
	request.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "opaque-token"})

	token, cookieSession, err := refreshTokenFromRequest(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatalf("refresh token from cookie: %v", err)
	}
	if !cookieSession || token != "opaque-token" {
		t.Fatalf("token = %q, cookie session = %t", token, cookieSession)
	}
}

func TestRefreshTokenJSONCompatibility(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", strings.NewReader(`{"refreshToken":"cli-token"}`))
	token, cookieSession, err := refreshTokenFromRequest(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatalf("refresh token from JSON: %v", err)
	}
	if cookieSession || token != "cli-token" {
		t.Fatalf("token = %q, cookie session = %t", token, cookieSession)
	}
}

func TestRefreshCookieSecurityAttributes(t *testing.T) {
	handler := &AuthHandler{cookieSecure: true}
	response := httptest.NewRecorder()
	handler.setRefreshCookie(response, "opaque-token", time.Now().Add(time.Hour))

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/auth" {
		t.Fatalf("unexpected cookie attributes: %#v", cookie)
	}
}
