package server

import (
	"encoding/json"
	"net/http"

	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type userDTO struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	DisplayName string        `json:"displayName,omitempty"`
	Email       string        `json:"email,omitempty"`
	Roles       []models.Role `json:"roles"`
	Protected   bool          `json:"protected"`
}

type sessionDTO struct {
	User      userDTO `json:"user"`
	CSRFToken string  `json:"csrfToken"`
}

func toUserDTO(u models.User) userDTO {
	roles := u.Roles
	if roles == nil {
		roles = []models.Role{}
	}
	return userDTO{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, Roles: roles, Protected: u.Protected}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	user, err := s.deps.Auth.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	sess := s.deps.SessionMgr.Create(user.ID)
	auth.SetSessionCookie(w, sess, isTLS(r))
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(user), CSRFToken: sess.CSRFToken})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if sess, ok := sessionFrom(r.Context()); ok {
		s.deps.SessionMgr.Destroy(sess.ID)
	}
	auth.ClearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	sess, _ := sessionFrom(r.Context())
	writeJSON(w, http.StatusOK, sessionDTO{User: toUserDTO(user), CSRFToken: sess.CSRFToken})
}
