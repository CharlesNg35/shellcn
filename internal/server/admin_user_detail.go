package server

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

func auditFilterFromRequest(r *http.Request, userID string, limit, offset int) (store.AuditFilter, error) {
	q := r.URL.Query()
	filter := store.AuditFilter{
		UserID: userID, Limit: limit, Offset: offset,
		Event: strings.TrimSpace(q.Get("event")), RemoteAddr: strings.TrimSpace(q.Get("remoteAddr")),
		Risk: strings.TrimSpace(q.Get("risk")), Result: strings.TrimSpace(q.Get("result")),
	}
	var err error
	if filter.Since, err = auditTimeParam(q.Get("since"), "since"); err != nil {
		return store.AuditFilter{}, err
	}
	if filter.Until, err = auditTimeParam(q.Get("until"), "until"); err != nil {
		return store.AuditFilter{}, err
	}
	return filter, nil
}

func auditTimeParam(value, name string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: invalid %s", plugin.ErrInvalidInput, name)
	}
	return t, nil
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

	filter, err := auditFilterFromRequest(r, userID, limit, offset)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	entries, err := s.deps.Store.Audit.List(r.Context(), filter)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	total, err := s.deps.Store.Audit.Count(r.Context(), filter)
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
