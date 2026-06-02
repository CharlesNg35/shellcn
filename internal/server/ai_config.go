package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	aiconfig "github.com/charlesng35/shellcn/internal/ai/config"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

type aiProviderRequest struct {
	Kind    models.AIProviderKind `json:"kind"`
	Name    string                `json:"name"`
	BaseURL string                `json:"baseUrl"`
	APIKey  string                `json:"apiKey"`
	Models  []string              `json:"models"`
	Model   string                `json:"model"`
}

func (r aiProviderRequest) input() aiconfig.Input {
	return aiconfig.Input{
		Kind:    r.Kind,
		Name:    r.Name,
		BaseURL: r.BaseURL,
		APIKey:  r.APIKey,
		Models:  r.Models,
		Model:   r.Model,
	}
}

// handleAIGlobal returns the read-only shared-AI status (presence + provider/
// model, never the key). There is no global write path: the shared config is
// env/config only.
func (s *Server) handleAIGlobal(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.AI.Global())
}

func (s *Server) handleListAIProviders(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	list, err := s.deps.AI.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateAIProvider(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	var req aiProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	sum, err := s.deps.AI.Create(r.Context(), user.ID, req.input())
	if err != nil {
		writeError(w, s.deps.Logger, aiConfigError(err))
		return
	}
	writeJSON(w, http.StatusCreated, sum)
}

func (s *Server) handleUpdateAIProvider(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	var req aiProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	sum, err := s.deps.AI.Update(r.Context(), user.ID, chi.URLParam(r, "id"), req.input())
	if err != nil {
		writeError(w, s.deps.Logger, aiConfigError(err))
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (s *Server) handleDeleteAIProvider(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	if err := s.deps.AI.Delete(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestAIProvider(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	if err := s.deps.AI.Test(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAIProviderModels(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	models, err := s.deps.AI.Models(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) handlePreviewAIProviderModels(w http.ResponseWriter, r *http.Request) {
	var req aiProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	models, err := s.deps.AI.ModelsForInput(r.Context(), req.input())
	if err != nil {
		writeError(w, s.deps.Logger, aiConfigError(err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

// aiConfigError maps a validation failure to ErrInvalidInput (400); other errors
// pass through to their normal status.
func aiConfigError(err error) error {
	if errors.Is(err, aiconfig.ErrInvalid()) {
		return plugin.ErrInvalidInput
	}
	return err
}
