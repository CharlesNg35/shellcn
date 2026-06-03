package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// aiConn authorizes connection access for the AI conversation endpoints and
// returns the connection.
func (s *Server) aiConn(w http.ResponseWriter, r *http.Request) (models.Connection, bool) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return models.Connection{}, false
	}
	if err := s.authorize(ctx, user, conn, aiAccessRoute); err != nil {
		writeError(w, s.deps.Logger, err)
		return models.Connection{}, false
	}
	return conn, true
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	list, err := s.chat.Conversations().List(r.Context(), user.ID, conn.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	var req struct {
		ProviderID string `json:"providerId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	model, err := s.aiConversationModel(r.Context(), user.ID, req.ProviderID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	c, err := s.chat.Conversations().Create(r.Context(), user.ID, conn.ID, req.ProviderID, model)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	id := chi.URLParam(r, "cid")
	conv, err := s.chat.Conversations().Get(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if conv.ConnectionID != conn.ID {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	page, err := s.chat.Conversations().MessagesPage(r.Context(), user.ID, id, atoiDefault(r.URL.Query().Get("limit"), 0), 0)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conv, "page": page})
}

func (s *Server) handleConversationMessages(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	if !s.aiConversationBelongsToConnection(r.Context(), user.ID, chi.URLParam(r, "cid"), conn.ID, w) {
		return
	}
	q := r.URL.Query()
	page, err := s.chat.Conversations().MessagesPage(r.Context(), user.ID, chi.URLParam(r, "cid"),
		atoiDefault(q.Get("limit"), 0), atoiDefault(q.Get("loadedCount"), 0))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil && n >= 0 {
		return n
	}
	return def
}

func (s *Server) handleRenameConversation(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	if !s.aiConversationBelongsToConnection(r.Context(), user.ID, chi.URLParam(r, "cid"), conn.ID, w) {
		return
	}
	var req struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	c, err := s.chat.Conversations().Rename(r.Context(), user.ID, chi.URLParam(r, "cid"), req.Title)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	conn, ok := s.aiConn(w, r)
	if !ok {
		return
	}
	user, _ := userFrom(r.Context())
	if !s.aiConversationBelongsToConnection(r.Context(), user.ID, chi.URLParam(r, "cid"), conn.ID, w) {
		return
	}
	if err := s.chat.Conversations().Delete(r.Context(), user.ID, chi.URLParam(r, "cid")); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) aiConversationBelongsToConnection(ctx context.Context, ownerID, convID, connID string, w http.ResponseWriter) bool {
	conv, err := s.chat.Conversations().Get(ctx, ownerID, convID)
	if err != nil || conv.ConnectionID != connID {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return false
	}
	return true
}
