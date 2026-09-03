package middleware

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRoles(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	protected := RequireRoles(next, models.RoleSuperAdminRoot)

	tests := []struct {
		name   string
		claims *auth.Claims
		want   int
	}{
		{name: "missing identity", want: http.StatusUnauthorized},
		{name: "owner forbidden", claims: &auth.Claims{Role: models.RoleOwnerLibrary}, want: http.StatusForbidden},
		{name: "root allowed", claims: &auth.Claims{Role: models.RoleSuperAdminRoot}, want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tt.claims != nil {
				request = request.WithContext(auth.ContextWithClaims(request.Context(), tt.claims))
			}
			response := httptest.NewRecorder()
			protected.ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, response.Code)
			}
		})
	}
}

func TestRequirePasswordChanged(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	protected := RequirePasswordChanged(next)
	for _, test := range []struct {
		name string
		claims *auth.Claims
		want int
	}{
		{name: "missing identity", want: http.StatusUnauthorized},
		{name: "temporary password forbidden", claims: &auth.Claims{PasswordChangeRequired: true}, want: http.StatusForbidden},
		{name: "changed password allowed", claims: &auth.Claims{}, want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/manage", nil)
			if test.claims != nil {
				request = request.WithContext(auth.ContextWithClaims(request.Context(), test.claims))
			}
			response := httptest.NewRecorder()
			protected.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("expected %d, got %d", test.want, response.Code)
			}
		})
	}
}

func TestRequireLibraryAccess(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	protected := RequireLibraryAccess(next, func(r *http.Request) string {
		return r.Header.Get("X-Test-Library-ID")
	})

	tests := []struct {
		name      string
		claims    *auth.Claims
		libraryID string
		want      int
	}{
		{name: "missing identity", want: http.StatusUnauthorized},
		{name: "root accesses any library", claims: &auth.Claims{Role: models.RoleSuperAdminRoot}, libraryID: "library-2", want: http.StatusNoContent},
		{name: "owner accesses own library", claims: &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}, libraryID: "library-1", want: http.StatusNoContent},
		{name: "owner cannot access another library", claims: &auth.Claims{Role: models.RoleOwnerLibrary, LibraryID: "library-1"}, libraryID: "library-2", want: http.StatusForbidden},
		{name: "owner without library forbidden", claims: &auth.Claims{Role: models.RoleOwnerLibrary}, libraryID: "library-1", want: http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/libraries/"+tt.libraryID, nil)
			request.Header.Set("X-Test-Library-ID", tt.libraryID)
			if tt.claims != nil {
				request = request.WithContext(auth.ContextWithClaims(request.Context(), tt.claims))
			}
			response := httptest.NewRecorder()
			protected.ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, response.Code)
			}
		})
	}
}
