package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	defaultChunkLimit = 8 << 20

	recReadEvent   = "recording.read"
	recDeleteEvent = "recording.delete"
)

type recordingDTO struct {
	ID             string     `json:"id"`
	UserID         string     `json:"userId"`
	Username       string     `json:"username,omitempty"`
	ConnectionID   string     `json:"connectionId"`
	ConnectionName string     `json:"connectionName,omitempty"`
	Protocol       string     `json:"protocol"`
	Class          string     `json:"class"`
	Format         string     `json:"format"`
	Authoritative  bool       `json:"authoritative"`
	Status         string     `json:"status"`
	Title          string     `json:"title,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	DurationMS     int64      `json:"durationMs"`
	Size           int64      `json:"size"`
}

func toRecordingDTO(r models.Recording) recordingDTO {
	return recordingDTO{
		ID: r.ID, UserID: r.UserID, Username: r.Username,
		ConnectionID: r.ConnectionID, ConnectionName: r.ConnectionName, Protocol: r.Protocol,
		Class: r.Class, Format: r.Format, Authoritative: r.Authoritative, Status: string(r.Status),
		Title: r.Title, StartedAt: r.StartedAt, EndedAt: r.EndedAt, DurationMS: r.DurationMS, Size: r.Size,
	}
}

// recordingFilter builds a store filter from query params (admin-only fields are
// applied here; the service re-scopes non-admins to what they may see).
func recordingFilter(r *http.Request) store.RecordingFilter {
	q := r.URL.Query()
	f := store.RecordingFilter{
		UserID: q.Get("user"), ConnectionID: q.Get("connection"), Protocol: q.Get("protocol"),
		Class: q.Get("class"), Format: q.Get("format"), Status: q.Get("status"),
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		f.Limit = v
	}
	if t, err := time.Parse(time.RFC3339, q.Get("since")); err == nil {
		f.Since = t
	}
	if t, err := time.Parse(time.RFC3339, q.Get("until")); err == nil {
		f.Until = t
	}
	return f
}

func (s *Server) auditRecordingEvent(ctx context.Context, user models.User, rec models.Recording, event string, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: user, Event: event, ConnectionID: rec.ConnectionID, RouteID: event,
		Risk: string(plugin.RiskSafe), Result: result, Err: err,
	})
}

func (s *Server) handleListRecordings(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	recs, err := s.deps.Recordings.List(r.Context(), user, recordingFilter(r))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, recordingDTOs(recs))
}

func (s *Server) handleListConnectionRecordings(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	f := recordingFilter(r)
	f.ConnectionID = chi.URLParam(r, "id")
	recs, err := s.deps.Recordings.List(r.Context(), user, f)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, recordingDTOs(recs))
}

func recordingDTOs(recs []models.Recording) []recordingDTO {
	out := make([]recordingDTO, 0, len(recs))
	for _, r := range recs {
		out = append(out, toRecordingDTO(r))
	}
	return out
}

func (s *Server) handleGetRecording(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	rec, err := s.deps.Recordings.Get(r.Context(), user, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecordingDTO(rec))
}

func (s *Server) handleRecordingContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	id := chi.URLParam(r, "id")
	rc, rec, err := s.deps.Recordings.Content(ctx, user, id)
	if err != nil {
		recForAudit := models.Recording{ID: id}
		if s.deps.Store != nil && s.deps.Store.Recordings != nil {
			if stored, getErr := s.deps.Store.Recordings.Get(ctx, id); getErr == nil {
				recForAudit = stored
			}
		}
		result := models.AuditError
		if statusFor(err) == http.StatusForbidden {
			result = models.AuditDenied
		}
		s.auditRecordingEvent(ctx, user, recForAudit, recReadEvent, result, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	defer func() { _ = rc.Close() }()

	s.auditRecordingEvent(ctx, user, rec, recReadEvent, models.AuditAllowed, nil)
	w.Header().Set("Content-Type", recording.ContentType(plugin.RecordingFormat(rec.Format)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	disposition := "inline"
	if r.URL.Query().Get("download") == "1" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", disposition+"; filename=\""+rec.ID+contentExt(rec.Format)+"\"")

	name := rec.ID + contentExt(rec.Format)
	modTime := rec.StartedAt
	if rec.EndedAt != nil {
		modTime = *rec.EndedAt
	}
	// A seekable blob enables Range/seek (video scrubbing) and HEAD via ServeContent.
	if seeker, ok := rc.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, modTime, seeker)
		return
	}
	if rec.Size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(rec.Size, 10))
	}
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, rc)
	}
}

func contentExt(format string) string {
	switch plugin.RecordingFormat(format) {
	case plugin.FormatAsciicastV2:
		return ".cast"
	case plugin.FormatWebMCanvas:
		return ".webm"
	default:
		return ".bin"
	}
}

func (s *Server) handleDeleteRecording(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	rec, err := s.deps.Recordings.Delete(ctx, user, chi.URLParam(r, "id"))
	if err != nil {
		s.auditRecordingEvent(ctx, user, models.Recording{ID: chi.URLParam(r, "id")}, recDeleteEvent, models.AuditDenied, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	s.auditRecordingEvent(ctx, user, rec, recDeleteEvent, models.AuditAllowed, nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// --- live recording control (manual terminal + desktop chunk uploads) -------

type recordingControlRequest struct {
	RouteID string            `json:"routeId"`
	Params  map[string]string `json:"params"`
	Action  string            `json:"action"` // start | stop
	Format  string            `json:"format"` // desktop only
}

// streamInfoFor builds the recording StreamInfo for a connection's WS route.
func (s *Server) streamInfoFor(r *http.Request, user models.User, conn models.Connection, routeID string, params map[string]string) (recording.StreamInfo, plugin.Route, bool) {
	manifest, ok := s.deps.Plugins.Manifest(conn.Protocol)
	if !ok {
		return recording.StreamInfo{}, plugin.Route{}, false
	}
	route, ok := s.deps.Plugins.Route(conn.Protocol, routeID)
	if !ok {
		return recording.StreamInfo{}, plugin.Route{}, false
	}
	stream, _ := manifest.StreamByRoute(routeID)
	return recording.StreamInfo{
		User: user, Connection: conn, Manifest: manifest, Route: route,
		StreamID: stream.ID, Params: params, RemoteAddr: r.RemoteAddr,
	}, route, true
}

// handleManualRecordingControl starts/stops a manual recording on a live stream
// the caller already owns (the StreamKey is scoped to the user).
func (s *Server) handleManualRecordingControl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canAccessConnection(ctx, user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	var req recordingControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	_, route, ok := s.streamInfoFor(r, user, conn, req.RouteID, req.Params)
	if !ok || !route.IsStream() {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	if err := s.authorize(ctx, user, conn, route); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	key := recording.StreamKey(user.ID, conn.ID, req.RouteID, req.Params)
	switch req.Action {
	case "start":
		rec, err := s.deps.Recording.Start(ctx, key)
		if err != nil {
			writeError(w, s.deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, toRecordingDTO(rec))
	case "stop":
		if err := s.deps.Recording.Stop(ctx, key); err != nil {
			writeError(w, s.deps.Logger, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
	}
}

func (s *Server) handleStartDesktopRecording(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if !s.canAccessConnection(ctx, user, conn) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	var req recordingControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	info, route, ok := s.streamInfoFor(r, user, conn, req.RouteID, req.Params)
	if !ok {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	if !route.IsStream() {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	if err := s.authorize(ctx, user, conn, route); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	format := plugin.RecordingFormat(req.Format)
	if format == "" {
		format = plugin.FormatWebMCanvas
	}
	rec, err := s.deps.Recording.BeginChunked(ctx, info, format)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusCreated, toRecordingDTO(rec))
}

func (s *Server) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil || index < 0 {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	limit := s.deps.RecordingMaxChunk
	if limit <= 0 {
		limit = defaultChunkLimit
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	replace := r.URL.Query().Get("replace") == "1"
	if replace {
		err = s.deps.Recording.ReplaceChunk(ctx, chi.URLParam(r, "id"), user.ID, index, data)
	} else {
		err = s.deps.Recording.AppendChunk(ctx, chi.URLParam(r, "id"), user.ID, index, data)
	}
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "index": index})
}

func (s *Server) handleFinalizeRecording(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	rec, err := s.deps.Recording.FinalizeChunked(ctx, chi.URLParam(r, "id"), user.ID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, toRecordingDTO(rec))
}

func (s *Server) handleAbortRecording(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	if err := s.deps.Recording.AbortChunked(ctx, chi.URLParam(r, "id"), user.ID); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// canAccessConnection reports whether the user may reach a connection at all
// (owner or any grant) — the gate for recording its own session.
func (s *Server) canAccessConnection(ctx context.Context, user models.User, conn models.Connection) bool {
	if user.Disabled || len(user.Roles) == 0 {
		return false
	}
	if conn.OwnerID == user.ID {
		return true
	}
	_, err := s.deps.Store.Grants.Get(ctx, conn.ID, user.ID)
	return err == nil
}
