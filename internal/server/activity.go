package server

import "net/http"

// handleMyAudit serves the signed-in user's own audit trail ("My activity").
func (s *Server) handleMyAudit(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	s.writeAuditPage(w, r, user.ID)
}
