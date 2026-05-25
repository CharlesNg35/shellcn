package server

import (
	"net/http"
	"strings"

	"github.com/charlesng/shellcn/internal/models"
)

type userSummary struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
}

const userLookupLimit = 20

// handleListUsers returns non-secret user summaries matching ?query=.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	users, err := s.deps.Store.Users.List(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]userSummary, 0, userLookupLimit)
	for _, u := range users {
		if len(out) >= userLookupLimit {
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
