package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/go-chi/chi/v5"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/policy"
	"github.com/charlesng35/shellcn/internal/recording"
	"github.com/charlesng35/shellcn/internal/session"
	"github.com/charlesng35/shellcn/internal/transport"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const (
	maxJSONBody        = 8 << 20
	maxMultipartBody   = 128 << 20
	maxMultipartMemory = 32 << 20
)

// pParams extracts renderer-supplied route params and scoped context values.
func pParams(r *http.Request) map[string]string {
	out := map[string]string{}
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 && len(k) > 2 && k[:2] == "p." {
			out[k[2:]] = vs[0]
		}
	}
	return out
}

// resolved bundles route request state.
type resolved struct {
	user   models.User
	conn   models.Connection
	plg    plugin.Plugin
	route  plugin.Route
	params map[string]string
}

// resolve loads and authorizes a route request.
func (s *Server) resolve(r *http.Request) (resolved, error) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	connID := chi.URLParam(r, "id")
	routeID := chi.URLParam(r, "routeID")
	return s.resolveRoute(ctx, user, connID, routeID, pParams(r))
}

// resolveRoute is shared by HTTP dispatch and AI tool invocation.
func (s *Server) resolveRoute(ctx context.Context, user models.User, connID, routeID string, rawParams map[string]string) (resolved, error) {
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

	params, err := resolveRouteParams(route.Path, rawParams)
	if err != nil {
		return resolved{user: user, conn: conn, plg: plg, route: route, params: rawParams}, err
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
	out := make(map[string]string, len(got))
	for name, value := range got {
		out[name] = value
	}
	for name := range names {
		v := got[name]
		if v == "" {
			return nil, fmt.Errorf("%w: missing route param %q", plugin.ErrInvalidInput, name)
		}
		out[name] = v
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

// handleConnectionProxy reverse-proxies a browser request through a connection.
func (s *Server) handleConnectionProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	// Proxying can reach internal services, so it is privileged.
	route := plugin.Route{ID: "connection.proxy", Permission: "connection.proxy", Risk: plugin.RiskPrivileged, AuditEvent: "connection.proxy"}
	res := resolved{user: user, conn: conn, route: route}
	if err := s.authorize(ctx, user, conn, route); err != nil {
		s.auditEvent(ctx, res, models.AuditDenied, err)
		s.incAuthzFailure(err)
		writeError(w, s.deps.Logger, err)
		return
	}
	if s.proxyIfRemoteOwner(w, r, conn, user.ID) {
		return
	}
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		writeError(w, s.deps.Logger, err)
		return
	}
	release := handle.TrackStream()
	defer release()
	proxier, ok := handle.Session().(plugin.HTTPProxy)
	if !ok {
		writeError(w, s.deps.Logger, plugin.ErrNotSupported)
		return
	}
	// The wildcard holds the plugin-defined target path; hand it over as the path,
	// preserving the original percent-encoding (chunk names carry %5B/%28 etc.).
	rp := r.Clone(ctx)
	rp.URL.Path = "/" + chi.URLParam(r, "*")
	rp.URL.RawPath = ""
	mark := "/" + conn.ID + "/proxy/"
	if i := strings.Index(r.URL.EscapedPath(), mark); i >= 0 {
		rp.URL.RawPath = "/" + r.URL.EscapedPath()[i+len(mark):]
	}
	rp.Header.Set(plugin.ProxyPrefixHeader, connProxyPrefix(conn.ID))
	s.auditEvent(ctx, res, models.AuditAllowed, nil)
	proxier.ServeHTTPProxy(w, rp)
}

// checkProtocolAvailable blocks opening a session for a protocol an admin has
// disabled, or restricted to admins, for this user.
func (s *Server) checkProtocolAvailable(ctx context.Context, user models.User, protocol string) error {
	if s.deps.Protocols == nil {
		return nil
	}
	ok, err := s.deps.Protocols.Allowed(ctx, protocol, user.HasRole(models.RoleAdmin))
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: this protocol is not available", plugin.ErrForbidden)
	}
	return nil
}

func (s *Server) acquireSession(ctx context.Context, res resolved) (*session.Handle, error) {
	if err := s.checkProtocolAvailable(ctx, res.user, res.conn.Protocol); err != nil {
		return nil, err
	}
	key := session.Key{ConnectionID: res.conn.ID, ActorScope: res.user.ID}
	return s.deps.Sessions.Acquire(ctx, key, res.user.ID, func(ctx context.Context) (plugin.Session, error) {
		cfg, plg, err := s.deps.Connector.Build(ctx, res.user, res.conn)
		if err != nil {
			return nil, err
		}
		cfg.Storage = s.pluginStorage(res)
		return plg.Connect(ctx, cfg)
	})
}

func (s *Server) auditEvent(ctx context.Context, res resolved, result models.AuditResult, err error) {
	s.auditEventParams(ctx, res, result, res.params, err)
}

func (s *Server) auditEventParams(ctx context.Context, res resolved, result models.AuditResult, params map[string]string, err error) {
	s.deps.Audit.Record(ctx, audit.Event{
		User: res.user, Event: res.route.AuditEvent, ConnectionID: res.conn.ID,
		RouteID: res.route.ID, Risk: string(res.route.Risk), Result: result,
		Params: params, Err: err,
	})
}

// handleRoute runs a plugin route through authz, session resolve, validation,
// audit, handler execution, and response normalization.
func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	res, err := s.resolve(r)
	if err != nil {
		// A matched-but-rejected route (bad param, authz) is audited and logged so
		// a 4xx isn't silent; a pure 404 (no route) stays quiet.
		if res.route.ID != "" {
			s.deps.Logger.Warn("route rejected", "route", res.route.ID, "connection", res.conn.ID, "err", err)
			s.auditEvent(r.Context(), res, models.AuditDenied, err)
			s.incAuthzFailure(err)
		}
		writeError(w, s.deps.Logger, err)
		return
	}

	if s.proxyIfRemoteOwner(w, r, res.conn, res.user.ID) {
		return
	}

	if res.route.IsStream() {
		s.serveStream(w, r, res)
		return
	}
	// HEAD is allowed on GET routes so players/ServeContent can probe range support.
	methodOK := r.Method == string(res.route.Method) ||
		(r.Method == http.MethodHead && res.route.Method == plugin.MethodGet)
	if !methodOK {
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

	result, herr := s.invoke(ctx, res, rc)
	if herr != nil {
		writeError(w, s.deps.Logger, herr)
		return
	}
	if dl, ok := result.(*plugin.Download); ok {
		s.writeDownload(w, r, dl)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// invoke is the shared validation, handler, metrics, and audit core.
func (s *Server) invoke(ctx context.Context, res resolved, rc *plugin.RequestContext) (any, error) {
	if err := rc.ValidateSchema(res.route.Input); err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		return nil, err
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
		return nil, herr
	}
	s.auditEvent(ctx, res, models.AuditAllowed, nil)
	return result, nil
}

// InvokeRoute runs one non-streaming plugin route as the given user through the
// same authorization, validation, and audit path as HTTP dispatch.
func (s *Server) InvokeRoute(ctx context.Context, user models.User, connID, routeID string, params map[string]string, body []byte) (any, error) {
	res, err := s.resolveRoute(ctx, user, connID, routeID, params)
	if err != nil {
		if res.route.ID != "" {
			s.auditEvent(ctx, res, models.AuditDenied, err)
			s.incAuthzFailure(err)
		}
		return nil, err
	}
	if res.route.IsStream() {
		return nil, plugin.ErrNotSupported
	}

	ctx, cancel := routeContext(ctx, res.route)
	defer cancel()
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		return nil, err
	}
	rc := plugin.NewRequestContext(ctx, toPluginUser(user), handle, res.params, nil, body).
		WithStorage(s.pluginStorage(res)).
		WithProxyPrefix(connProxyPrefix(res.conn.ID))
	return s.invoke(ctx, res, rc)
}

func (s *Server) writeDownload(w http.ResponseWriter, r *http.Request, dl *plugin.Download) {
	if dl == nil || (dl.Body == nil && dl.Seeker == nil && dl.OpenRange == nil) {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	name := path.Base(dl.Name)
	if name == "." || name == "/" || name == "" {
		name = "download"
	}
	mimeType := dl.MIME
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	h := w.Header()
	h.Set("Content-Type", mimeType)
	h.Set("X-Content-Type-Options", "nosniff")
	disposition := "attachment"
	if dl.Inline {
		disposition = "inline"
		h.Set("Content-Security-Policy", "sandbox")
	}
	h.Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": name}))
	if !dl.ModTime.IsZero() {
		h.Set("Last-Modified", dl.ModTime.UTC().Format(http.TimeFormat))
	}

	if dl.Seeker != nil {
		defer func() { _ = dl.Seeker.Close() }()
		http.ServeContent(w, r, name, dl.ModTime, dl.Seeker)
		return
	}

	if dl.OpenRange != nil && dl.Size >= 0 {
		h.Set("Accept-Ranges", "bytes")
		start, length, status := resolveRange(r.Header.Get("Range"), dl.Size)
		if status == http.StatusRequestedRangeNotSatisfiable {
			h.Set("Content-Range", fmt.Sprintf("bytes */%d", dl.Size))
			writeJSON(w, status, errorEnvelope{Error: "range not satisfiable"})
			return
		}
		off, n := int64(0), dl.Size
		if status == http.StatusPartialContent {
			off, n = start, length
		}
		body, err := dl.OpenRange(off, n)
		if err != nil {
			writeError(w, s.deps.Logger, err)
			return
		}
		defer func() { _ = body.Close() }()
		if status == http.StatusPartialContent {
			h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", off, off+n-1, dl.Size))
		}
		h.Set("Content-Length", strconv.FormatInt(n, 10))
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			s.streamBody(io.LimitReader(body, n), w)
		}
		return
	}

	defer func() { _ = dl.Body.Close() }()
	if dl.Size >= 0 {
		h.Set("Content-Length", strconv.FormatInt(dl.Size, 10))
	}
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		s.streamBody(dl.Body, w)
	}
}

// resolveRange parses a single byte range.
func resolveRange(header string, size int64) (start, length int64, status int) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, http.StatusOK
	}
	spec := strings.TrimPrefix(header, "bytes=")
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, http.StatusOK
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, http.StatusOK
	}
	startStr, endStr := spec[:dash], spec[dash+1:]
	if size == 0 {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	if startStr == "" {
		n, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, http.StatusRequestedRangeNotSatisfiable
		}
		if n > size {
			n = size
		}
		return size - n, n, http.StatusPartialContent
	}
	begin, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || begin < 0 || begin >= size {
		return 0, 0, http.StatusRequestedRangeNotSatisfiable
	}
	end := size - 1
	if endStr != "" {
		if end, err = strconv.ParseInt(endStr, 10, 64); err != nil || end < begin {
			return 0, 0, http.StatusRequestedRangeNotSatisfiable
		}
		if end > size-1 {
			end = size - 1
		}
	}
	return begin, end - begin + 1, http.StatusPartialContent
}

func (s *Server) streamBody(src io.Reader, w http.ResponseWriter) {
	if _, err := io.Copy(w, src); err != nil && !isBenignStreamError(err) && s.deps.Logger != nil {
		s.deps.Logger.Warn("download stream failed", "err", err)
	}
}

func isBenignStreamError(err error) bool {
	return errors.Is(err, context.Canceled) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, net.ErrClosed)
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
		return plugin.NewMultipartRequestContext(r.Context(), toPluginUser(res.user), sess, res.params, r.URL.Query(), r.MultipartForm.Value, files).
			WithStorage(s.pluginStorage(res)).
			WithProxyPrefix(connProxyPrefix(res.conn.ID)), cleanup, nil
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxJSONBody))
	if err != nil {
		return nil, func() {}, plugin.ErrInvalidInput
	}
	return plugin.NewRequestContext(r.Context(), toPluginUser(res.user), sess, res.params, r.URL.Query(), body).
		WithStorage(s.pluginStorage(res)).
		WithProxyPrefix(connProxyPrefix(res.conn.ID)), func() {}, nil
}

// connProxyPrefix is the single source of truth for a connection's public
// proxy mount; plugins receive it via the request context and proxy header.
func connProxyPrefix(connID string) string {
	return "/api/connections/" + url.PathEscape(connID) + "/proxy"
}

func (s *Server) pluginStorage(res resolved) plugin.Storage {
	if s.deps.Store == nil || s.deps.Store.PluginStorage == nil {
		return nil
	}
	return storageBridge{
		inner:        s.deps.Store.PluginStorage,
		pluginID:     res.conn.Protocol,
		connectionID: res.conn.ID,
		ownerID:      res.user.ID,
	}
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

type activeConn struct {
	net.Conn
	lastActive atomic.Int64
	writes     atomic.Int64
}

func newActiveConn(conn net.Conn) *activeConn {
	c := &activeConn{Conn: conn}
	c.touch()
	return c
}

func (c *activeConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		c.touch()
	}
	return n, err
}

func (c *activeConn) Write(p []byte) (int, error) {
	c.writes.Add(1)
	c.touch()
	defer c.writes.Add(-1)
	n, err := c.Conn.Write(p)
	if n > 0 {
		c.touch()
	}
	return n, err
}

func (c *activeConn) LastActive() time.Time {
	if c.writes.Load() > 0 {
		return time.Now()
	}
	n := c.lastActive.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}

func (c *activeConn) touch() {
	c.lastActive.Store(time.Now().UnixNano())
}

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

	// Reject malformed WS params before any upstream session or recording opens.
	vc := plugin.NewRequestContext(ctx, toPluginUser(res.user), nil, res.params, r.URL.Query(), nil)
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

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		Subprotocols:       []string{"binary"},
	})
	if err != nil {
		return // Accept already wrote the response
	}
	if s.deps.Metrics != nil {
		s.deps.Metrics.WSOpened()
		defer s.deps.Metrics.WSClosed()
	}

	// After accept, dial/auth failures can be sent as readable close reasons.
	handle, err := s.acquireSession(ctx, res)
	if err != nil {
		s.auditEvent(ctx, res, models.AuditError, err)
		_ = c.Close(websocket.StatusInternalError, streamCloseReason(err))
		return
	}
	releaseStream := handle.TrackStream()
	defer releaseStream()
	s.auditEvent(ctx, res, models.AuditAllowed, nil)

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// noVNC streams raw RFB bytes over the negotiated "binary" subprotocol;
	// terminal/log/query streams stay on text frames.
	msgType := websocket.MessageText
	if c.Subprotocol() == "binary" {
		msgType = websocket.MessageBinary
	}
	conn := newActiveConn(websocket.NetConn(streamCtx, c, msgType))
	if keepAlive := s.streamKeepAlivePolicy(res); keepAlive.enabled {
		if keepAlive.controlReader {
			go discardWebSocketReads(streamCtx, c, cancel)
		}
		go func() {
			if err := transport.KeepAliveWebSocketWhenIdle(streamCtx, c, conn.LastActive); err != nil {
				cancel()
				_ = c.CloseNow()
			}
		}()
	}
	client := pending.Attach(&wsClientStream{Conn: conn, ctx: streamCtx})

	rc := plugin.NewRequestContext(streamCtx, toPluginUser(res.user), handle, res.params, r.URL.Query(), nil).
		WithAuditHook(func(ctx context.Context, result plugin.AuditResult, params map[string]string, err error) {
			s.auditEventParams(ctx, res, models.AuditResult(result), params, err)
		}).
		WithProxyPrefix(connProxyPrefix(res.conn.ID))
	if err := res.route.Stream(rc, client); err != nil {
		_ = c.Close(websocket.StatusInternalError, streamCloseReason(err))
		return
	}
	_ = c.Close(websocket.StatusNormalClosure, "")
}

type streamKeepAlivePolicy struct {
	enabled       bool
	controlReader bool
}

func (s *Server) streamKeepAlivePolicy(res resolved) streamKeepAlivePolicy {
	m, ok := s.deps.Plugins.Manifest(res.conn.Protocol)
	if !ok {
		return streamKeepAlivePolicy{}
	}
	stream, ok := m.StreamByRoute(res.route.ID)
	if !ok {
		return streamKeepAlivePolicy{}
	}
	return streamKindKeepAlivePolicy(stream.Kind)
}

func streamKindHasContinuousClientReader(kind plugin.StreamKind) bool {
	return kind == plugin.StreamTerminal || kind == plugin.StreamDesktop || kind == plugin.StreamCanvas
}

func streamKindKeepAlivePolicy(kind plugin.StreamKind) streamKeepAlivePolicy {
	if streamKindHasContinuousClientReader(kind) {
		return streamKeepAlivePolicy{enabled: true}
	}
	if kind == plugin.StreamLogs {
		return streamKeepAlivePolicy{enabled: true, controlReader: true}
	}
	if kind == plugin.StreamQuery || kind == plugin.StreamFileTransfer {
		return streamKeepAlivePolicy{enabled: true}
	}
	return streamKeepAlivePolicy{}
}

func discardWebSocketReads(ctx context.Context, c *websocket.Conn, cancel context.CancelFunc) {
	defer cancel()
	for {
		_, r, err := c.Reader(ctx)
		if err != nil {
			return
		}
		if _, err := io.Copy(io.Discard, r); err != nil {
			return
		}
	}
}

// streamCloseReason fits an error into a WebSocket close reason.
func streamCloseReason(err error) string {
	msg := err.Error()
	const maxCloseReasonBytes = 120
	if len(msg) <= maxCloseReasonBytes {
		return msg
	}
	b := []byte(msg)[:maxCloseReasonBytes]
	for len(b) > 0 && !utf8.RuneStart(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return string(b)
}

func routeContext(parent context.Context, route plugin.Route) (context.Context, context.CancelFunc) {
	if route.Timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, route.Timeout)
}

// prepareRecording resolves the stream recording decision.
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
