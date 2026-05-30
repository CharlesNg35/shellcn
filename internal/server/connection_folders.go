package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/service"
)

type connectionFolderRequest struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	ParentID string `json:"parentId"`
}

type connectionLayoutRequest struct {
	Items   []service.ConnectionPlacementInput   `json:"items"`
	Folders []service.ConnectionFolderOrderInput `json:"folders"`
}

func (s *Server) handleListConnectionFolders(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	folders, err := s.deps.Store.ConnectionFolders.ListByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	out := make([]service.ConnectionFolderDTO, 0, len(folders))
	for _, f := range folders {
		out = append(out, service.FolderDTO(f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateConnectionFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	if !canCreate(user) {
		s.auditConnEvent(ctx, user, "", connFolderCreateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	var req connectionFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	folder, err := s.deps.Connections.CreateFolder(ctx, s.deps.Store.ConnectionFolders, user.ID, service.ConnectionFolderInput{
		Name: req.Name, Color: req.Color, ParentID: req.ParentID,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, "", connFolderCreateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, "", connFolderCreateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusCreated, service.FolderDTO(folder))
}

func (s *Server) handleUpdateConnectionFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	folder, err := s.deps.Store.ConnectionFolders.Get(ctx, chi.URLParam(r, "folderId"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if folder.UserID != user.ID {
		s.auditConnEvent(ctx, user, "", connFolderUpdateEvent, plugin.RiskWrite, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	var req connectionFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	updated, err := s.deps.Connections.UpdateFolder(ctx, s.deps.Store.ConnectionFolders, folder, service.ConnectionFolderInput{
		Name: req.Name, Color: req.Color,
	})
	if err != nil {
		s.auditConnEvent(ctx, user, "", connFolderUpdateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, "", connFolderUpdateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, service.FolderDTO(updated))
}

func (s *Server) handleDeleteConnectionFolder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	folder, err := s.deps.Store.ConnectionFolders.Get(ctx, chi.URLParam(r, "folderId"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if folder.UserID != user.ID {
		s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	folders, err := s.deps.Store.ConnectionFolders.ListByUser(ctx, folder.UserID)
	if err != nil {
		s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	now := time.Now()
	for _, child := range folders {
		if child.ParentID != folder.ID {
			continue
		}
		child.ParentID = folder.ParentID
		child.UpdatedAt = now
		if err := s.deps.Store.ConnectionFolders.Update(ctx, &child); err != nil {
			s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
			writeError(w, s.deps.Logger, err)
			return
		}
	}
	if err := s.deps.Store.ConnectionPlacements.MoveFolder(ctx, folder.UserID, folder.ID, folder.ParentID); err != nil {
		s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	if err := s.deps.Store.ConnectionFolders.Delete(ctx, folder.ID); err != nil {
		s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, "", connFolderDeleteEvent, plugin.RiskDestructive, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSaveConnectionLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	var req connectionLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	conns, err := s.accessibleConnections(ctx, user)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	accessible := map[string]bool{}
	for _, c := range conns {
		accessible[c.ID] = true
	}
	folders, err := s.deps.Store.ConnectionFolders.ListByUser(ctx, user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	folderIDs := map[string]bool{}
	for _, f := range folders {
		folderIDs[f.ID] = true
	}
	if err := service.SaveConnectionLayout(ctx, s.deps.Store.ConnectionPlacements, user.ID, accessible, folderIDs, req.Items); err != nil {
		result := models.AuditError
		if errors.Is(err, plugin.ErrForbidden) {
			result = models.AuditDenied
		}
		s.auditConnEvent(ctx, user, "", connLayoutUpdateEvent, plugin.RiskWrite, result, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	if err := service.SaveConnectionFolderOrder(ctx, s.deps.Store.ConnectionFolders, user.ID, req.Folders); err != nil {
		s.auditConnEvent(ctx, user, "", connLayoutUpdateEvent, plugin.RiskWrite, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditConnEvent(ctx, user, "", connLayoutUpdateEvent, plugin.RiskWrite, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
