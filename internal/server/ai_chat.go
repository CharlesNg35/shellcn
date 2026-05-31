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

// aiChatRouteID is the synthetic route a chat ticket is scoped to. The chat is a
// core endpoint, not a plugin route; the agent's actual tool calls are each
// authorized separately through InvokeRoute.
const aiChatRouteID = "ai.chat"

// aiAccessRoute is the synthetic route used to authorize opening the chat: any
// user with access to the connection (owner/grant) may use the assistant; the
// real per-action gating happens per tool call.
var aiAccessRoute = plugin.Route{ID: aiChatRouteID, Permission: "connection.ai", Risk: plugin.RiskSafe, AuditEvent: aiChatRouteID}

// connectionAIMode reports the connection's AI mode. Until the per-connection
// columns land, AI is available read-only whenever a provider is configured.
func connectionAIMode(_ models.Connection) string { return "read_only" }

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
	Type           string `json:"type"` // user_message | stop
	Content        string `json:"content"`
	ProviderID     string `json:"providerId"`
	ConversationID string `json:"conversationId"`
}

// aiMetaFrame tells the client which conversation a turn belongs to (so a newly
// created thread can be selected/persisted client-side).
type aiMetaFrame struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
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
	var mu sync.Mutex // serializes one active turn (queueing arrives later)
	var cancel context.CancelFunc

	send := func(ev engine.StreamEvent) {
		_ = wsjson.Write(ctx, c, ev)
	}
	send2 := func(convID, title string) {
		_ = wsjson.Write(ctx, c, aiMetaFrame{Type: "conversation", ConversationID: convID, Title: title})
	}

	for {
		var frame aiClientFrame
		if err := wsjson.Read(ctx, c, &frame); err != nil {
			mu.Lock()
			if cancel != nil {
				cancel()
			}
			mu.Unlock()
			return
		}

		switch frame.Type {
		case "stop":
			mu.Lock()
			if cancel != nil {
				cancel()
			}
			mu.Unlock()
		case "user_message":
			mu.Lock()
			if cancel != nil { // a turn is already running; ignore (no queue yet)
				mu.Unlock()
				continue
			}
			turnCtx, c2 := context.WithCancel(ctx)
			cancel = c2
			mu.Unlock()

			// Run the turn off the read loop so a stop frame can still be read and
			// cancel it mid-stream. Only this goroutine writes to the socket.
			go func(frame aiClientFrame) {
				turnID := uuid.NewString()
				tctx := audit.WithSource(turnCtx, audit.SourceAI, turnID)

				// Ensure a conversation exists; create one and tell the client its id.
				convID := frame.ConversationID
				if convID == "" {
					if c, err := s.chat.Conversations().Create(tctx, user.ID, conn.ID, frame.ProviderID, ""); err == nil {
						convID = c.ID
						send2(c.ID, "")
					}
				}

				in := ai.RunInput{
					User: user, ConnID: conn.ID, Protocol: conn.Protocol,
					ConnectionTitle: conn.Name, AIMode: connectionAIMode(conn),
					Scope:          ai.Scope{ProviderID: frame.ProviderID},
					ConversationID: convID,
					UserMessage:    frame.Content,
					RecentOps:      s.recentAuditLines(tctx, user.ID, conn.ID),
				}
				// On success the provider already emits a terminal done; on a setup
				// error (no stream) we synthesize an error + done so the client ends.
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
