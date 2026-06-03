package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/extplugin"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const protocolAvailabilityEvent = "protocol.availability"

type protocolAdminDTO struct {
	Name         string                      `json:"name"`
	Title        string                      `json:"title"`
	Icon         plugin.Icon                 `json:"icon"`
	Category     plugin.CategoryInfo         `json:"category"`
	Version      string                      `json:"version"`
	Transports   []plugin.Transport          `json:"transports"`
	External     bool                        `json:"external"`
	Healthy      bool                        `json:"healthy"`
	Availability models.ProtocolAvailability `json:"availability"`
}

func (s *Server) handleAdminListProtocols(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	states, err := s.deps.Protocols.States(ctx)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}

	external := map[string]extplugin.Loaded{}
	if s.deps.ExtPlugins != nil {
		for _, l := range s.deps.ExtPlugins.Loaded() {
			external[l.Name] = l
		}
	}

	summaries := s.deps.Plugins.Summaries()
	out := make([]protocolAdminDTO, 0, len(summaries))
	for _, su := range summaries {
		avail := states[su.Name]
		if avail == "" {
			avail = models.ProtocolEnabled
		}
		dto := protocolAdminDTO{
			Name: su.Name, Title: su.Title, Icon: su.Icon, Category: su.Category,
			Availability: avail, Healthy: true,
		}
		if proj, ok := s.deps.Plugins.Projection(su.Name); ok {
			dto.Version = proj.Version
			dto.Transports = proj.SupportedTransports
		}
		if l, ok := external[su.Name]; ok {
			dto.External = true
			dto.Healthy = l.Healthy
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAdminSetProtocolAvailability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)
	name := chi.URLParam(r, "name")
	if _, ok := s.deps.Plugins.Get(name); !ok {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}

	var req struct {
		Availability models.ProtocolAvailability `json:"availability"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}

	if err := s.deps.Protocols.Set(ctx, name, req.Availability); err != nil {
		s.auditAdminEvent(ctx, actor, protocolAvailabilityEvent, models.AuditError,
			map[string]string{"protocol": name, "availability": string(req.Availability)}, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditAdminEvent(ctx, actor, protocolAvailabilityEvent, models.AuditAllowed,
		map[string]string{"protocol": name, "availability": string(req.Availability)}, nil)
	w.WriteHeader(http.StatusNoContent)
}
