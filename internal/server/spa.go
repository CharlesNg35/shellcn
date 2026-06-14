package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves the embedded SPA: a real asset is returned as-is; any other
// path falls back to index.html so client-side routing works on deep links.
func (s *Server) spaHandler() http.HandlerFunc {
	fileServer := http.FileServerFS(s.deps.StaticFS)
	return func(w http.ResponseWriter, r *http.Request) {
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			clean = "index.html"
		}
		info, err := fs.Stat(s.deps.StaticFS, clean)
		if err != nil || info.IsDir() {
			// Not a real file → serve the SPA shell.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	}
}
