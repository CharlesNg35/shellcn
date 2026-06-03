package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	defaultAuditPageSize = 25
	maxAuditPageSize     = 200
)

type auditEntryDTO struct {
	ID           string            `json:"id"`
	Time         time.Time         `json:"time"`
	Event        string            `json:"event"`
	Risk         string            `json:"risk,omitempty"`
	Result       string            `json:"result"`
	ConnectionID string            `json:"connectionId,omitempty"`
	Params       map[string]string `json:"params,omitempty"`
	Error        string            `json:"error,omitempty"`
	RemoteAddr   string            `json:"remoteAddr,omitempty"`
}

type auditPage struct {
	Items []auditEntryDTO `json:"items"`
	Total int64           `json:"total"`
}

func toAuditEntryDTO(e models.AuditEntry) auditEntryDTO {
	return auditEntryDTO{
		ID: e.ID, Time: e.Time, Event: e.Event, Risk: e.Risk,
		Result: string(e.Result), ConnectionID: e.ConnectionID,
		Params: e.Params, Error: e.Error, RemoteAddr: e.RemoteAddr,
	}
}

// writeAuditPage serves a paginated audit slice for one user.
func (s *Server) writeAuditPage(w http.ResponseWriter, r *http.Request, userID string) {
	limit := defaultAuditPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= maxAuditPageSize {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v > 0 {
		offset = v
	}

	entries, err := s.deps.Store.Audit.List(r.Context(), store.AuditFilter{UserID: userID, Limit: limit, Offset: offset})
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	total, err := s.deps.Store.Audit.Count(r.Context(), store.AuditFilter{UserID: userID})
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	items := make([]auditEntryDTO, 0, len(entries))
	for _, e := range entries {
		items = append(items, toAuditEntryDTO(e))
	}
	writeJSON(w, http.StatusOK, auditPage{Items: items, Total: total})
}

func (s *Server) handleAdminUserAudit(w http.ResponseWriter, r *http.Request) {
	user, err := s.deps.Users.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	s.writeAuditPage(w, r, user.ID)
}

type userConnectionDTO struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Protocol  string       `json:"protocol"`
	Icon      *plugin.Icon `json:"icon,omitempty"`
	CreatedAt time.Time    `json:"createdAt"`
}

// handleAdminUserConnections lists a user's connections as metadata only (name,
// protocol, icon, created date) — never config, secrets, or access to them.
func (s *Server) handleAdminUserConnections(w http.ResponseWriter, r *http.Request) {
	user, err := s.deps.Users.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	conns, err := s.deps.Store.Connections.ListByOwner(r.Context(), user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]userConnectionDTO, 0, len(conns))
	for _, c := range conns {
		dto := userConnectionDTO{ID: c.ID, Name: c.Name, Protocol: c.Protocol, CreatedAt: c.CreatedAt}
		if m, ok := s.deps.Plugins.Manifest(c.Protocol); ok {
			icon := m.Icon
			dto.Icon = &icon
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}
