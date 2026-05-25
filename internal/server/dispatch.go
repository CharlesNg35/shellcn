package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/charlesng/shellcn/internal/audit"
	"github.com/charlesng/shellcn/internal/auth"
	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/internal/policy"
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

	if err := s.authorize(ctx, user, conn, route); err != nil {
		return resolved{}, err
	}
	return resolved{user: user, conn: conn, plg: plg, route: route, params: pParams(r)}, nil
}

// authorize resolves the user's grant on the connection and applies policy.
func (s *Server) authorize(ctx context.Context, user models.User, conn models.Connection, route plugin.Route) error {
	in := policy.AccessInput{
		User: user, Risk: route.Risk,
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
	ctx := r.Context()
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}

	rc, cleanup, err := s.bindRequest(w, r, res, handle.Session())
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	defer cleanup()

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
	writeJSON(w, http.StatusOK, result)
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
		return plugin.NewMultipartRequestContext(r.Context(), res.user, sess, res.params, r.URL.Query(), r.MultipartForm.Value, files), cleanup, nil
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err != nil {
		return nil, func() {}, plugin.ErrInvalidInput
	}
	return plugin.NewRequestContext(r.Context(), res.user, sess, res.params, r.URL.Query(), body), func() {}, nil
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
	ctx := r.Context()

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

	streamCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn := websocket.NetConn(streamCtx, c, websocket.MessageText)
	client := &wsClientStream{Conn: conn, ctx: streamCtx}

	rc := plugin.NewRequestContext(streamCtx, res.user, handle.Session(), res.params, r.URL.Query(), nil)
	if err := res.route.Stream(rc, client); err != nil {
		_ = c.Close(websocket.StatusInternalError, "stream error")
		return
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
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
