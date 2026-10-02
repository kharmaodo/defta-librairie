package middleware

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// StaticFiles serves regular browser assets only; directories, dotfiles and
// source maps are never exposed, even when added accidentally to static/.
func StaticFiles(root fs.FS) http.Handler {
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		for _, component := range strings.Split(name, "/") {
			if strings.HasPrefix(component, ".") {
				http.NotFound(w, r)
				return
			}
		}
		switch strings.ToLower(path.Ext(name)) {
		case ".js", ".css", ".svg", ".png", ".jpg", ".jpeg", ".webp", ".ico", ".woff", ".woff2", ".ttf":
		default:
			if name != "openapi.json" {
				http.NotFound(w, r)
				return
			}
		}
		info, err := fs.Stat(root, name)
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}
