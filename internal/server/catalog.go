package server

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

func (s *Server) handleListPlugins(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Plugins.Summaries())
}

func (s *Server) handleListCredentialKinds(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, plugin.CredentialKinds())
}

func (s *Server) handleGetPlugin(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	proj, ok := s.deps.Plugins.Projection(name)
	if !ok {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, proj)
}

type connectionDTO struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Protocol  string            `json:"protocol"`
	Icon      *plugin.Icon      `json:"icon,omitempty"`
	Transport string            `json:"transport"`
	Online    bool              `json:"online"`
	Status    string            `json:"status,omitempty"`
	CanManage bool              `json:"canManage"`
	Recording map[string]string `json:"recording,omitempty"`
}

func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)

	conns, err := s.accessibleConnections(ctx, user)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}

	out := make([]connectionDTO, 0, len(conns))
	for _, c := range conns {
		dto := s.toConnectionDTO(c)
		dto.CanManage = s.canManageConnection(ctx, user, c)
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

// accessibleConnections returns the connections a user owns or has a grant on.
func (s *Server) accessibleConnections(ctx context.Context, user models.User) ([]models.Connection, error) {
	owned, err := s.deps.Store.Connections.ListByOwner(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]models.Connection, 0, len(owned))
	for _, c := range owned {
		seen[c.ID] = true
		out = append(out, c)
	}

	grants, err := s.deps.Store.Grants.ListBySubject(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	for _, g := range grants {
		if seen[g.ConnectionID] {
			continue
		}
		c, err := s.deps.Store.Connections.Get(ctx, g.ConnectionID)
		if err != nil {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Server) handleListCredentials(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	var kinds []string
	if raw := r.URL.Query().Get("kind"); raw != "" {
		kinds = strings.Split(raw, ",")
	}
	protocol := r.URL.Query().Get("protocol")

	summaries, err := s.deps.Credentials.ListUsable(r.Context(), user.ID, kinds, protocol)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if summaries == nil {
		summaries = []models.CredentialSummary{}
	}
	writeJSON(w, http.StatusOK, summaries)
}
