package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
	"github.com/charlesng/shellcn/internal/recording"
	"github.com/charlesng/shellcn/internal/session"
)

const (
	maxJSONBody        = 8 << 20
	maxMultipartBody   = 128 << 20
	maxMultipartMemory = 32 << 20
)

// pParams extracts route/path params (the reserved `p.` prefix) from the query.
func pParams(r *http.Request) map[string]string {
	out := map[string]string{}
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 && len(k) > 2 && k[:2] == "p." {
			out[k[2:]] = vs[0]
		}
	}
	return out
}

// resolved bundles everything the wrapper looks up for a route request.
type resolved struct {
	user   models.User
	conn   models.Connection
	plg    plugin.Plugin
	route  plugin.Route
	params map[string]string
}

// resolve loads the connection + route and authorizes the request. It returns an
// error already mapped to a sentinel; the caller normalizes + audits.
func (s *Server) resolve(r *http.Request) (resolved, error) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	connID := chi.URLParam(r, "id")
	routeID := chi.URLParam(r, "routeID")

	conn, err := s.deps.Store.Connections.Get(ctx, connID)
	if err != nil {
		return resolved{}, err // store.ErrNotFound → 404
	}
	plg, ok := s.deps.Plugins.Get(conn.Protocol)
	if !ok {
		return resolved{}, plugin.ErrNotFound
	}
	route, ok := s.deps.Plugins.Route(conn.Protocol, routeID)
	if !ok {
		return resolved{}, plugin.ErrNotFound
	}

	params, err := resolveRouteParams(route.Path, pParams(r))
	if err != nil {
		return resolved{user: user, conn: conn, plg: plg, route: route, params: pParams(r)}, err
	}
	res := resolved{user: user, conn: conn, plg: plg, route: route, params: params}
	if err := s.authorize(ctx, user, conn, route); err != nil {
		return res, err
	}
	return res, nil
}

func resolveRouteParams(path string, got map[string]string) (map[string]string, error) {
	names := templateParamNames(path)
	if len(names) == 0 {
		return got, nil
	}
	out := make(map[string]string, len(names))
	for name := range names {
		v := got[name]
		if v == "" {
			return nil, fmt.Errorf("%w: missing route param %q", plugin.ErrInvalidInput, name)
		}
		out[name] = v
	}
	for name := range got {
		if !names[name] {
			return nil, fmt.Errorf("%w: unknown route param %q", plugin.ErrInvalidInput, name)
		}
	}
	return out, nil
}

func templateParamNames(path string) map[string]bool {
	out := map[string]bool{}
	for {
		start := strings.IndexByte(path, '{')
		if start < 0 {
			return out
		}
		path = path[start+1:]
		end := strings.IndexByte(path, '}')
		if end < 0 {
			return out
		}
		name := strings.TrimSpace(path[:end])
		if name != "" {
			out[name] = true
		}
		path = path[end+1:]
	}
}

// authorize resolves the user's grant on the connection and applies policy.
func (s *Server) authorize(ctx context.Context, user models.User, conn models.Connection, route plugin.Route) error {
	in := policy.AccessInput{
		User: user, Permission: route.Permission, Risk: route.Risk,
		ConnectionID: conn.ID, OwnerID: conn.OwnerID,
	}
	if conn.OwnerID != user.ID {
		if g, err := s.deps.Store.Grants.Get(ctx, conn.ID, user.ID); err == nil {
			in.HasGrant = true
			in.GrantAccess = g.Access
		}
	}
	return s.deps.Policy.Authorize(in)
}

func (s *Server) acquireSession(ctx context.Context, res resolved) (*session.Handle, error) {
	key := session.Key{ConnectionID: res.conn.ID, OwnerScope: res.user.ID}
	return s.deps.Sessions.Acquire(ctx, key, res.user.ID, func(ctx context.Context) (plugin.Session, error) {
		cfg, plg, err := s.deps.Connector.Build(ctx, res.user, res.conn)
		if err != nil {
			return nil, err
		}
		return plg.Connect(ctx, cfg)
	})
}

func (s *Server) auditEvent(ctx context.Context, res resolved, result models.AuditResult, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: res.user, Event: res.route.AuditEvent, ConnectionID: res.conn.ID,
		RouteID: res.route.ID, Risk: string(res.route.Risk), Result: result,
		Params: res.params, Err: err,
	})
}

// handleRoute is the full middleware chain for a plugin route:
// authn (middleware) → authz → session resolve → bind/validate → audit → handler
// → normalized response.
func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	res, err := s.resolve(r)
	if err != nil {
		// A denied authorization is audited; lookups that 404 are not (no route).
		if res.route.ID != "" {
			s.auditEvent(r.Context(), res, models.AuditDenied, err)
			s.incAuthzFailure(err)
		}
		writeError(w, s.deps.Logger, err)
		return
	}

	if res.route.IsStream() {
		s.serveStream(w, r, res)
		return
	}
	if r.Method != string(res.route.Method) {
		writeJSON(w, http.StatusMethodNotAllowed, errorEnvelope{Error: "method not allowed"})
		return
	}
	s.serveHTTP(w, r, res)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request, res resolved) {
	ctx, cancel := routeContext(r.Context(), res.route)
	defer cancel()
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}

	rc, cleanup, err := s.bindRequest(w, r.WithContext(ctx), res, handle)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	defer cleanup()

	if err := rc.ValidateSchema(res.route.Input); err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}

	start := time.Now()
	result, herr := res.route.Handle(rc)
	if s.deps.Metrics != nil {
		label := "allowed"
		if herr != nil {
			label = "error"
		}
		s.deps.Metrics.ObserveAction(string(res.route.Risk), label, time.Since(start))
	}

	if herr != nil {
		s.auditEvent(ctx, res, auditResult(herr), herr)
		writeError(w, s.deps.Logger, herr)
		return
	}
	s.auditEvent(ctx, res, models.AuditAllowed, nil)
	if dl, ok := result.(*plugin.Download); ok {
		s.writeDownload(w, dl)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) writeDownload(w http.ResponseWriter, dl *plugin.Download) {
	if dl == nil || dl.Body == nil {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	defer func() { _ = dl.Body.Close() }()
	name := path.Base(dl.Name)
	if name == "." || name == "/" || name == "" {
		name = "download"
	}
	mimeType := dl.MIME
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	if dl.Size >= 0 {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", dl.Size))
	}
	if !dl.ModTime.IsZero() {
		w.Header().Set("Last-Modified", dl.ModTime.UTC().Format(http.TimeFormat))
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, dl.Body); err != nil && s.deps.Logger != nil {
		s.deps.Logger.Warn("download stream failed", "err", err)
	}
}

func (s *Server) bindRequest(w http.ResponseWriter, r *http.Request, res resolved, sess plugin.Session) (*plugin.RequestContext, func(), error) {
	if isMultipart(r) {
		if res.route.Input == nil || !res.route.Input.HasFileField() {
			return nil, func() {}, plugin.ErrInvalidInput
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBody)
		if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
			return nil, func() {}, plugin.ErrInvalidInput
		}
		cleanup := func() {
			if r.MultipartForm != nil {
				_ = r.MultipartForm.RemoveAll()
			}
		}
		files := map[string][]plugin.UploadedFile{}
		for field, headers := range r.MultipartForm.File {
			for _, header := range headers {
				files[field] = append(files[field], plugin.NewUploadedFile(field, header))
			}
		}
		return plugin.NewMultipartRequestContext(r.Context(), res.user, sess, res.params, r.URL.Query(), r.MultipartForm.Value, files).WithSnippets(s.snippetStore()), cleanup, nil
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err != nil {
		return nil, func() {}, plugin.ErrInvalidInput
	}
	return plugin.NewRequestContext(r.Context(), res.user, sess, res.params, r.URL.Query(), body).WithSnippets(s.snippetStore()), func() {}, nil
}

func (s *Server) snippetStore() plugin.SnippetStore {
	if s.deps.Store == nil {
		return nil
	}
	return s.deps.Store.Snippets
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data")
}

// wsClientStream is the browser side of a WS pipe handed to a StreamHandler.
type wsClientStream struct {
	net.Conn
	ctx context.Context
}

func (s *wsClientStream) Context() context.Context { return s.ctx }

func (s *Server) serveStream(w http.ResponseWriter, r *http.Request, res resolved) {
	ctx, cancel := routeContext(r.Context(), res.route)
	defer cancel()

	// Browsers can't set Authorization on a WS upgrade, so a param-scoped,
	// single-use ticket is mandatory; the origin is checked too.
	scope := auth.TicketScope{ConnectionID: res.conn.ID, RouteID: res.route.ID, Params: res.params, UserID: res.user.ID}
	if err := s.deps.Tickets.Redeem(r.URL.Query().Get("ticket"), scope); err != nil {
		s.auditEvent(ctx, res, models.AuditDenied, err)
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}
	if !auth.CheckWSOrigin(r, s.deps.AllowedOrigins) {
		s.auditEvent(ctx, res, models.AuditDenied, plugin.ErrForbidden)
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	// Enforce the route's declared input schema on the WS query params up front —
	// the same contract HTTP routes get — so a malformed request is rejected
	// before any upstream session or recording is opened.
	vc := plugin.NewRequestContext(ctx, res.user, nil, res.params, r.URL.Query(), nil)
	if err := vc.ValidateSchema(res.route.Input); err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}

	// Decide recording before opening the upstream session, so a forced policy
	// that cannot start denies the stream up front.
	pending, err := s.prepareRecording(ctx, r, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	defer pending.Finish()

	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return // Accept already wrote the response
	}
	if s.deps.Metrics != nil {
		s.deps.Metrics.WSOpened()
		defer s.deps.Metrics.WSClosed()
	}
	s.auditEvent(ctx, res, models.AuditAllowed, nil)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn := websocket.NetConn(streamCtx, c, websocket.MessageText)
	client := pending.Attach(&wsClientStream{Conn: conn, ctx: streamCtx})

	rc := plugin.NewRequestContext(streamCtx, res.user, handle, res.params, r.URL.Query(), nil)
	if err := res.route.Stream(rc, client); err != nil {
		_ = c.Close(websocket.StatusInternalError, "stream error")
		return
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
}

func routeContext(parent context.Context, route plugin.Route) (context.Context, context.CancelFunc) {
	if route.Timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, route.Timeout)
}

// prepareRecording resolves the stream's recording decision from plugin
// capability + connection policy. A nil engine yields a no-op Pending.
func (s *Server) prepareRecording(ctx context.Context, r *http.Request, res resolved) (*recording.Pending, error) {
	manifest, ok := s.deps.Plugins.Manifest(res.conn.Protocol)
	if !ok {
		return s.deps.Recording.Prepare(ctx, recording.StreamInfo{})
	}
	stream, _ := manifest.StreamByRoute(res.route.ID)
	return s.deps.Recording.Prepare(ctx, recording.StreamInfo{
		User: res.user, Connection: res.conn, Manifest: manifest, Route: res.route,
		StreamID: stream.ID, Params: res.params, RemoteAddr: r.RemoteAddr,
	})
}

func auditResult(err error) models.AuditResult {
	if err == nil {
		return models.AuditAllowed
	}
	return models.AuditError
}

// incAuthzFailure increments the authz-failure counter for a forbidden error.
func (s *Server) incAuthzFailure(err error) {
	if s.deps.Metrics != nil && statusFor(err) == http.StatusForbidden {
		s.deps.Metrics.IncAuthzFailure()
	}
}

type ticketRequest struct {
	RouteID string            `json:"routeId"`
	Params  map[string]string `json:"params"`
}

type ticketResponse struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func (s *Server) handleMintTicket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	connID := chi.URLParam(r, "id")

	var req ticketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}

	conn, err := s.deps.Store.Connections.Get(ctx, connID)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	route, ok := s.deps.Plugins.Route(conn.Protocol, req.RouteID)
	if !ok {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	// A ticket is only minted if the caller could perform the action over HTTP.
	if err := s.authorize(ctx, user, conn, route); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}

	token, expires := s.deps.Tickets.Mint(auth.TicketScope{
		ConnectionID: connID, RouteID: req.RouteID, Params: req.Params, UserID: user.ID,
	})
	writeJSON(w, http.StatusCreated, ticketResponse{Ticket: token, ExpiresAt: expires})
}
