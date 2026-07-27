package server

import (
	"net/http"

	"github.com/charlesng35/shellcn/internal/version"
)

// handleVersion reports the running build and, for release builds, whether a
// newer release is available. refresh=1 forces a re-check (rate-floored).
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if s.deps.Version == nil {
		writeJSON(w, http.StatusOK, version.Info{Current: "dev", Dev: true})
		return
	}
	info := s.deps.Version.Check(r.Context(), r.URL.Query().Get("refresh") == "1")
	writeJSON(w, http.StatusOK, info)
}
