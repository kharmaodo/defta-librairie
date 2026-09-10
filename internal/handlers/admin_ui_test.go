package handlers

import (
	"defta-librairie/internal/config"
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminUIPages(t *testing.T) {
	previous := Tmpl
	Tmpl = template.Must(template.New("").Parse(`
		{{define "login.html"}}login {{.Version}} {{.BuildDate}}{{end}}
		{{define "admin.html"}}admin {{.Version}} {{.BuildDate}}{{end}}
	`))
	t.Cleanup(func() { Tmpl = previous })

	handler := NewAdminUIHandler(&config.Config{Version: "1.2.3", BuildDate: "2026-09-02"})
	tests := []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
		want string
	}{
		{name: "login", call: handler.Login, want: "login 1.2.3 2026-09-02"},
		{name: "dashboard", call: handler.Dashboard, want: "admin 1.2.3 2026-09-02"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.call(response, httptest.NewRequest(http.MethodGet, "/", nil))
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if !strings.Contains(response.Body.String(), test.want) {
				t.Fatalf("body = %q, want content %q", response.Body.String(), test.want)
			}
			if cache := response.Header().Get("Cache-Control"); cache != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", cache)
			}
		})
	}
}
