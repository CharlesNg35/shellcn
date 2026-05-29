package server

import (
	"net/http"
	"strings"

	"github.com/charlesng35/shellcn/internal/models"
)

type userSummary struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
}

const userSearchLimit = 20

// handleSearchUsers returns non-secret user summaries matching ?query=, backing
// the admin-only share-picker autocomplete. It is admin-gated by its route group;
// operators share by exact email instead of enumerating accounts.
func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	users, err := s.deps.Store.Users.List(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]userSummary, 0, userSearchLimit)
	for _, u := range users {
		if len(out) >= userSearchLimit {
			break
		}
		if q != "" && !matchesUser(u, q) {
			continue
		}
		out = append(out, userSummary{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName})
	}
	writeJSON(w, http.StatusOK, out)
}

func matchesUser(u models.User, q string) bool {
	return strings.Contains(strings.ToLower(u.Username), q) ||
		strings.Contains(strings.ToLower(u.DisplayName), q)
}
