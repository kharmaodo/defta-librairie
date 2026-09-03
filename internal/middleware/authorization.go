package middleware

import (
	"defta-librairie/internal/auth"
	"defta-librairie/internal/models"
	"encoding/json"
	"net/http"
)

// RequireRoles autorise uniquement les rôles explicitement déclarés.
// Authenticate doit être exécuté avant ce middleware.
func RequireRoles(next http.Handler, allowed ...models.UserRole) http.Handler {
	roles := make(map[models.UserRole]struct{}, len(allowed))
	for _, role := range allowed {
		roles[role] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}
		if _, ok = roles[claims.Role]; !ok {
			writeForbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequirePasswordChanged(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}
		if claims.PasswordChangeRequired {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "password_change_required", "message": "Password change required before accessing this resource",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireLibraryAccess applique l'isolation des données par librairie.
// SUPER_ADMIN_ROOT peut accéder à toutes les librairies. OWNER_LIBRARY ne
// peut accéder qu'à l'identifiant library_id porté par son JWT.
func RequireLibraryAccess(next http.Handler, libraryID func(*http.Request) string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			writeUnauthorized(w)
			return
		}

		switch claims.Role {
		case models.RoleSuperAdminRoot:
			next.ServeHTTP(w, r)
		case models.RoleOwnerLibrary:
			if claims.LibraryID == "" || libraryID == nil || libraryID(r) != claims.LibraryID {
				writeForbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		default:
			writeForbidden(w)
		}
	})
}

func writeForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": "Insufficient permissions",
	})
}
