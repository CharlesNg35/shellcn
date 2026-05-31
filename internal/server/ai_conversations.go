package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
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
	c, err := s.chat.Conversations().Create(r.Context(), user.ID, conn.ID, req.ProviderID, "")
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.aiConn(w, r); !ok {
		return
	}
	user, _ := userFrom(r.Context())
	id := chi.URLParam(r, "cid")
	conv, err := s.chat.Conversations().Get(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	msgs, err := s.chat.Conversations().Messages(r.Context(), user.ID, id)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conv, "messages": msgs})
}

func (s *Server) handleRenameConversation(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.aiConn(w, r); !ok {
		return
	}
	user, _ := userFrom(r.Context())
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
	if _, ok := s.aiConn(w, r); !ok {
		return
	}
	user, _ := userFrom(r.Context())
	if err := s.chat.Conversations().Delete(r.Context(), user.ID, chi.URLParam(r, "cid")); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
