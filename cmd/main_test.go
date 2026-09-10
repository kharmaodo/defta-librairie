//go:build fts5

package main

import (
	"defta-librairie/internal/config"
	"defta-librairie/internal/handlers"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnknownRoutesDoNotFallBackToCatalogue(t *testing.T) {
	mux := http.NewServeMux()
	registerPublicRoutes(mux, handlers.NewAdminUIHandler(&config.Config{}))

	for _, target := range []string{"/unknown", "/api/unknown", "/api/auth/sessions/"} {
		request := httptest.NewRequest(http.MethodDelete, target, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("DELETE %s returned %d, want %d", target, response.Code, http.StatusNotFound)
		}
	}
}
