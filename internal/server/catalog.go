package server

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

func (s *Server) handleListPlugins(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Plugins.Summaries())
}

func (s *Server) handleListCredentialKinds(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Plugins.CredentialKinds())
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
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Protocol     string            `json:"protocol"`
	Icon         *plugin.Icon      `json:"icon,omitempty"`
	Transport    string            `json:"transport"`
	Online       bool              `json:"online"`
	Status       string            `json:"status,omitempty"`
	CanManage    bool              `json:"canManage"`
	Access       string            `json:"access"`
	Owned        bool              `json:"owned"`
	SharedWithMe bool              `json:"sharedWithMe"`
	SharedByMe   bool              `json:"sharedByMe"`
	Recording    map[string]string `json:"recording,omitempty"`
	FolderID     string            `json:"folderId,omitempty"`
	SortOrder    int               `json:"sortOrder"`
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
	placements, err := s.deps.Store.ConnectionPlacements.ListByUser(ctx, user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	placementByConnection := map[string]models.ConnectionPlacement{}
	for _, p := range placements {
		placementByConnection[p.ConnectionID] = p
	}
	for _, c := range conns {
		dto := s.toConnectionDTO(c)
		s.decorateConnectionAccess(ctx, user, c, &dto)
		if p, ok := placementByConnection[c.ID]; ok {
			dto.FolderID = p.FolderID
			dto.SortOrder = p.SortOrder
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) decorateConnectionAccess(ctx context.Context, user models.User, c models.Connection, dto *connectionDTO) {
	dto.Owned = c.OwnerID == user.ID
	dto.Access = string(models.AccessUse)
	if dto.Owned {
		dto.Access = "owner"
	} else if user.HasRole(models.RoleAdmin) {
		dto.Access = "admin"
	} else if g, err := s.deps.Store.Grants.Get(ctx, c.ID, user.ID); err == nil {
		dto.Access = string(g.Access)
		dto.SharedWithMe = true
	}
	dto.CanManage = s.canManageConnection(ctx, user, c)
	if dto.Owned {
		grants, err := s.deps.Store.Grants.ListByConnection(ctx, c.ID)
		dto.SharedByMe = err == nil && len(grants) > 0
	}
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
