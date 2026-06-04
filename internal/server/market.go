package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginmarket"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const marketInstallEvent = "market.install"

type marketVersionDTO struct {
	Version         string          `json:"version"`
	APIVersion      int             `json:"apiVersion"`
	ProtocolVersion int             `json:"protocolVersion"`
	Platforms       []string        `json:"platforms"`
	Icon            plugin.Icon     `json:"icon"`
	Projection      json.RawMessage `json:"projection,omitempty"`
}

type marketEntryDTO struct {
	Name             string            `json:"name"`
	DisplayName      string            `json:"displayName"`
	Description      string            `json:"description"`
	Repo             string            `json:"repo"`
	Homepage         string            `json:"homepage,omitempty"`
	License          string            `json:"license"`
	Maintainers      []string          `json:"maintainers"`
	Latest           *marketVersionDTO `json:"latest,omitempty"`
	Compatible       bool              `json:"compatible"`
	InstalledVersion string            `json:"installedVersion,omitempty"`
	Managed          bool              `json:"managed"`
	UpdateAvailable  bool              `json:"updateAvailable"`
}

type marketListDTO struct {
	Enabled bool             `json:"enabled"`
	Plugins []marketEntryDTO `json:"plugins"`
}

func (s *Server) marketEnabled() bool {
	return s.deps.Market != nil && s.deps.ExtPlugins != nil && s.deps.PluginsDir != ""
}

func (s *Server) handleAdminMarketList(w http.ResponseWriter, r *http.Request) {
	if !s.marketEnabled() {
		writeJSON(w, http.StatusOK, marketListDTO{Enabled: false, Plugins: []marketEntryDTO{}})
		return
	}
	entries, err := s.deps.Market.Entries(r.Context())
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}

	out := make([]marketEntryDTO, 0, len(entries))
	for _, e := range entries {
		dto := marketEntryDTO{
			Name: e.Name, DisplayName: e.DisplayName, Description: e.Description,
			Repo: e.Repo, Homepage: e.Homepage, License: e.License, Maintainers: e.Maintainers,
			Managed: s.deps.ExtPlugins.IsManaged(e.Name),
		}
		if v, ok := pluginmarket.Installable(e); ok {
			dto.Compatible = true
			dto.Latest = &marketVersionDTO{
				Version:    v.Version,
				APIVersion: v.APIVersion, ProtocolVersion: v.ProtocolVersion,
				Platforms: platformKeys(v), Icon: v.Icon, Projection: v.Projection,
			}
		}
		if proj, ok := s.deps.Plugins.Projection(e.Name); ok && dto.Managed {
			dto.InstalledVersion = proj.Version
			dto.UpdateAvailable = dto.Latest != nil && dto.Latest.Version != proj.Version
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, marketListDTO{Enabled: true, Plugins: out})
}

func platformKeys(v pluginmarket.Version) []string {
	keys := make([]string, 0, len(v.Assets))
	for k := range v.Assets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Server) handleAdminMarketInstall(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor, _ := userFrom(ctx)
	if !s.marketEnabled() {
		writeError(w, s.deps.Logger, plugin.ErrNotSupported)
		return
	}
	name := chi.URLParam(r, "name")

	var req struct {
		Version string `json:"version"`
	}
	if body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)); err == nil && len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
			return
		}
	}

	auditParams := map[string]string{"plugin": name, "version": req.Version}
	fail := func(err error) {
		s.auditAdminEvent(ctx, actor, marketInstallEvent, models.AuditError, auditParams, err)
		writeError(w, s.deps.Logger, err)
	}

	entry, err := s.deps.Market.Entry(ctx, name)
	if err != nil {
		fail(err)
		return
	}
	var version pluginmarket.Version
	if req.Version != "" {
		version, err = pluginmarket.FindVersion(entry, req.Version)
	} else {
		var ok bool
		version, ok = pluginmarket.Installable(entry)
		if !ok {
			err = errors.New("no installable version for this gateway")
		}
	}
	if err != nil {
		fail(err)
		return
	}
	auditParams["version"] = version.Version

	managed := s.deps.ExtPlugins.IsManaged(name)
	if _, registered := s.deps.Plugins.Get(name); registered && !managed {
		fail(errors.Join(plugin.ErrConflict, errors.New("name is taken by a built-in protocol")))
		return
	}

	path, err := s.deps.Market.Install(ctx, entry, version, s.deps.PluginsDir)
	if err != nil {
		fail(err)
		return
	}
	if managed {
		err = s.deps.ExtPlugins.Update(ctx, s.deps.Plugins, name, path)
	} else {
		err = s.deps.ExtPlugins.LoadOne(ctx, s.deps.Plugins, path)
	}
	if err != nil {
		fail(err)
		return
	}

	s.auditAdminEvent(ctx, actor, marketInstallEvent, models.AuditAllowed, auditParams, nil)
	writeJSON(w, http.StatusOK, map[string]any{
		"name": name, "version": version.Version, "updated": managed,
	})
}
