package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/ai"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
	"github.com/charlesng35/shellcn/internal/store"
)

// recentAuditLines returns the user's recent operations on a connection, oldest
// last, as compact lines for the agent's prompt (so it can explain failures).
func (s *Server) recentAuditLines(ctx context.Context, userID, connID string) []string {
	rows, err := s.deps.Store.Audit.List(ctx, store.AuditFilter{UserID: userID, ConnectionID: connID, Limit: 8})
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- { // reverse: newest last
		r := rows[i]
		if r.Event == aiChatRouteID {
			continue
		}
		line := string(r.Result) + " " + r.Event
		if r.Result != models.AuditAllowed && r.Error != "" {
			line += ": " + r.Error
		}
		out = append(out, line)
	}
	return out
}

const aiChatRouteID = "ai.chat"

// aiAccessRoute authorizes opening the chat (connection access); per-tool calls
// are gated separately through InvokeRoute.
var aiAccessRoute = plugin.Route{ID: aiChatRouteID, Permission: "connection.ai", Risk: plugin.RiskSafe, AuditEvent: aiChatRouteID}

// connectionAIMode is the stored mode, or read-only when AI is configured but the
// owner hasn't chosen one.
func connectionAIMode(conn models.Connection) string {
	if conn.AIMode == "" {
		return models.AIModeReadOnly
	}
	return conn.AIMode
}

type aiTicketResponse struct {
	Ticket string `json:"ticket"`
}

// handleMintAITicket authorizes connection access + AI availability, then mints a
// single-use WS ticket for the chat stream.
func (s *Server) handleMintAITicket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if err := s.authorize(ctx, user, conn, aiAccessRoute); err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	if connectionAIMode(conn) == "disabled" || !s.chat.Configured(ctx, user.ID) {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}
	token, _ := s.deps.Tickets.Mint(auth.TicketScope{ConnectionID: conn.ID, RouteID: aiChatRouteID, UserID: user.ID})
	writeJSON(w, http.StatusCreated, aiTicketResponse{Ticket: token})
}

type aiClientFrame struct {
	Type           string `json:"type"` // user_message | stop | confirm | reject
	Content        string `json:"content"`
	ProviderID     string `json:"providerId"`
	Model          string `json:"model"`
	ConversationID string `json:"conversationId"`
	ToolID         string `json:"toolId"` // for confirm/reject
}

// aiMetaFrame tells the client which conversation a turn belongs to (so a newly
// created thread can be selected/persisted client-side).
type aiMetaFrame struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
}

type aiConfirmFrame struct {
	Type        string            `json:"type"` // needs_confirmation
	ToolID      string            `json:"toolId"`
	ToolName    string            `json:"toolName"`
	RouteID     string            `json:"routeId"`
	Risk        string            `json:"risk"`
	Destructive bool              `json:"destructive"`
	Params      map[string]string `json:"params,omitempty"`
	Body        map[string]any    `json:"body,omitempty"`
}

// wsConfirmer emits a needs_confirmation frame and blocks the tool call until the
// client answers (or the turn is cancelled).
type wsConfirmer struct {
	out       chan<- any
	mu        sync.Mutex
	decisions map[string]chan bool
}

func (cf *wsConfirmer) Confirm(ctx context.Context, req tools.ConfirmRequest) (bool, error) {
	ch := make(chan bool, 1)
	cf.mu.Lock()
	cf.decisions[req.ToolCallID] = ch
	cf.mu.Unlock()
	defer func() {
		cf.mu.Lock()
		delete(cf.decisions, req.ToolCallID)
		cf.mu.Unlock()
	}()

	cf.out <- aiConfirmFrame{
		Type: "needs_confirmation", ToolID: req.ToolCallID, ToolName: req.ToolName,
		RouteID: req.RouteID, Risk: string(req.Risk), Destructive: req.Destructive,
		Params: req.Params, Body: req.Body,
	}
	select {
	case ok := <-ch:
		return ok, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (cf *wsConfirmer) deliver(toolID string, ok bool) {
	cf.mu.Lock()
	ch := cf.decisions[toolID]
	cf.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ok:
		default:
		}
	}
}

// handleAIChat is the chat WebSocket: it redeems the ticket, then runs a turn per
// user_message frame, streaming engine events back as JSON. A stop frame cancels
// the active turn.
func (s *Server) handleAIChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user, _ := userFrom(ctx)
	conn, err := s.deps.Store.Connections.Get(ctx, chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	scope := auth.TicketScope{ConnectionID: conn.ID, RouteID: aiChatRouteID, UserID: user.ID}
	if err := s.deps.Tickets.Redeem(r.URL.Query().Get("ticket"), scope); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrUnauthorized)
		return
	}
	if !auth.CheckWSOrigin(r, s.deps.AllowedOrigins) {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}
	if connectionAIMode(conn) == "disabled" {
		writeError(w, s.deps.Logger, plugin.ErrForbidden)
		return
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{})
	if err != nil {
		return
	}
	if s.deps.Metrics != nil {
		s.deps.Metrics.WSOpened()
		defer s.deps.Metrics.WSClosed()
	}
	defer func() { _ = c.Close(websocket.StatusNormalClosure, "") }()

	s.runAIChat(ctx, c, user, conn)
}

func (s *Server) runAIChat(ctx context.Context, c *websocket.Conn, user models.User, conn models.Connection) {
	// All outbound frames go through one writer goroutine, so the streaming turn
	// and a paused tool-confirmation can both emit without racing on the socket.
	out := make(chan any, 64)
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		for f := range out {
			_ = wsjson.Write(ctx, c, f)
		}
	}()

	var mu sync.Mutex
	var cancel context.CancelFunc
	var wg sync.WaitGroup
	confirmer := &wsConfirmer{out: out, decisions: map[string]chan bool{}}

	defer func() {
		mu.Lock()
		if cancel != nil {
			cancel()
		}
		mu.Unlock()
		wg.Wait()
		close(out)
		<-writerDone
	}()

	send := func(ev engine.StreamEvent) { out <- ev }

	for {
		var frame aiClientFrame
		if err := wsjson.Read(ctx, c, &frame); err != nil {
			return
		}
		switch frame.Type {
		case "stop":
			mu.Lock()
			if cancel != nil {
				cancel()
			}
			mu.Unlock()
		case "confirm", "reject":
			confirmer.deliver(frame.ToolID, frame.Type == "confirm")
		case "user_message":
			mu.Lock()
			if cancel != nil { // one turn at a time; queueing arrives later
				mu.Unlock()
				continue
			}
			turnCtx, c2 := context.WithCancel(ctx)
			cancel = c2
			wg.Add(1)
			mu.Unlock()

			go func(frame aiClientFrame) {
				defer wg.Done()
				tctx := audit.WithSource(turnCtx, audit.SourceAI, uuid.NewString())

				convID := frame.ConversationID
				if convID == "" {
					if cv, err := s.chat.Conversations().Create(tctx, user.ID, conn.ID, frame.ProviderID, ""); err == nil {
						convID = cv.ID
						out <- aiMetaFrame{Type: "conversation", ConversationID: cv.ID}
					}
				}

				in := ai.RunInput{
					User: user, ConnID: conn.ID, Protocol: conn.Protocol,
					ConnectionTitle: conn.Name, AIMode: connectionAIMode(conn),
					AllowDestructive: conn.AIAllowDestructive,
					Scope:            ai.Scope{ProviderID: frame.ProviderID, Model: frame.Model},
					ConversationID:   convID,
					UserMessage:      frame.Content,
					RecentOps:        s.recentAuditLines(tctx, user.ID, conn.ID),
					Confirm:          confirmer,
				}
				if err := s.chat.Run(tctx, in, send); err != nil {
					send(engine.StreamEvent{Type: engine.EventError, Err: err.Error()})
					send(engine.StreamEvent{Type: engine.EventDone})
				}
				mu.Lock()
				c2()
				cancel = nil
				mu.Unlock()
			}(frame)
		}
	}
}
