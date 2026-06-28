package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/ai"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const aiChatRouteID = "ai.chat"

var aiAccessRoute = plugin.Route{ID: aiChatRouteID, Permission: "connection.ai", Risk: plugin.RiskSafe, AuditEvent: aiChatRouteID}

func connectionAIMode(conn models.Connection) models.AIMode {
	if conn.AIMode == "" {
		return models.AIModeReadOnly
	}
	return conn.AIMode
}

type aiTurnRegistry struct {
	mu    sync.Mutex
	turns map[string]*aiTurn
}

type aiTurn struct {
	id        string
	userID    string
	connID    string
	cancel    context.CancelFunc
	confirmer *aiTurnConfirmer
}

func newAITurnRegistry() *aiTurnRegistry {
	return &aiTurnRegistry{turns: map[string]*aiTurn{}}
}

func (r *aiTurnRegistry) add(turn *aiTurn) {
	r.mu.Lock()
	r.turns[turn.id] = turn
	r.mu.Unlock()
}

func (r *aiTurnRegistry) remove(turnID string) {
	r.mu.Lock()
	delete(r.turns, turnID)
	r.mu.Unlock()
}

func (r *aiTurnRegistry) control(userID, connID, turnID string, req aiTurnControlRequest) error {
	r.mu.Lock()
	turn := r.turns[turnID]
	r.mu.Unlock()
	if turn == nil || turn.userID != userID || turn.connID != connID {
		return plugin.ErrNotFound
	}
	switch req.Type {
	case "stop":
		turn.cancel()
		return nil
	case "confirm":
		turn.confirmer.deliver(req.ToolID, true)
		return nil
	case "reject":
		turn.confirmer.deliver(req.ToolID, false)
		return nil
	default:
		return plugin.ErrInvalidInput
	}
}

type aiTurnConfirmer struct {
	turnID    string
	emit      func(any) bool
	mu        sync.Mutex
	decisions map[string]chan bool
}

func newAITurnConfirmer(turnID string, emit func(any) bool) *aiTurnConfirmer {
	return &aiTurnConfirmer{turnID: turnID, emit: emit, decisions: map[string]chan bool{}}
}

func (cf *aiTurnConfirmer) Confirm(ctx context.Context, req tools.ConfirmRequest) (bool, error) {
	ch := make(chan bool, 1)
	cf.mu.Lock()
	cf.decisions[req.ToolCallID] = ch
	cf.mu.Unlock()
	defer func() {
		cf.mu.Lock()
		delete(cf.decisions, req.ToolCallID)
		cf.mu.Unlock()
	}()

	if !cf.emit(aiConfirmFrame{
		Type: "needs_confirmation", TurnID: cf.turnID,
		ToolID: req.ToolCallID, ToolName: req.ToolName,
		RouteID: req.RouteID, Risk: string(req.Risk), Destructive: req.Destructive,
		Params: req.Params, Body: req.Body,
	}) {
		return false, context.Canceled
	}
	select {
	case ok := <-ch:
		return ok, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (cf *aiTurnConfirmer) deliver(toolID string, ok bool) {
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

type aiTurnRequest struct {
	Content        string `json:"content"`
	ProviderID     string `json:"providerId"`
	ConversationID string `json:"conversationId"`
}

type aiTurnControlRequest struct {
	Type   string `json:"type"`
	ToolID string `json:"toolId"`
}

type aiTurnFrame struct {
	Type   string `json:"type"`
	TurnID string `json:"turnId"`
}

type aiMetaFrame struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	Title          string `json:"title,omitempty"`
}

type aiConfirmFrame struct {
	Type        string            `json:"type"`
	TurnID      string            `json:"turnId"`
	ToolID      string            `json:"toolId"`
	ToolName    string            `json:"toolName"`
	RouteID     string            `json:"routeId"`
	Risk        string            `json:"risk"`
	Destructive bool              `json:"destructive"`
	Params      map[string]string `json:"params,omitempty"`
	Body        map[string]any    `json:"body,omitempty"`
}

func (s *Server) recentAuditLines(ctx context.Context, userID, connID string) []string {
	rows, err := s.deps.Store.Audit.List(ctx, store.AuditFilter{UserID: userID, ConnectionID: connID, Limit: 8})
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
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

func (s *Server) handleAITurn(w http.ResponseWriter, r *http.Request) {
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
	if connectionAIMode(conn) == models.AIModeDisabled || !s.chat.Configured(ctx, user.ID) {
		writeError(w, s.deps.Logger, plugin.ErrNotFound)
		return
	}

	var req aiTurnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, s.deps.Logger, errors.New("streaming response unsupported"))
		return
	}

	turnID := uuid.NewString()
	turnCtx, cancel := context.WithCancel(ctx)
	enc := json.NewEncoder(w)
	writeFrame := func(frame any) bool {
		if err := enc.Encode(frame); err != nil {
			cancel()
			return false
		}
		flusher.Flush()
		return true
	}
	confirmer := newAITurnConfirmer(turnID, writeFrame)
	s.aiTurns.add(&aiTurn{id: turnID, userID: user.ID, connID: conn.ID, cancel: cancel, confirmer: confirmer})
	defer func() {
		cancel()
		s.aiTurns.remove(turnID)
	}()

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-ShellCN-AI-Turn-ID", turnID)
	if !writeFrame(aiTurnFrame{Type: "turn", TurnID: turnID}) {
		return
	}

	convID, providerID, ok := s.prepareAITurn(turnCtx, user, conn, req, writeFrame)
	if !ok {
		return
	}
	in := ai.RunInput{
		User: user, ConnID: conn.ID, Protocol: conn.Protocol,
		ConnectionTitle: conn.Name, AIMode: connectionAIMode(conn),
		AllowDestructive: conn.AIAllowDestructive,
		Scope:            ai.Scope{ProviderID: providerID},
		ConversationID:   convID,
		UserMessage:      req.Content,
		RecentOps:        s.recentAuditLines(turnCtx, user.ID, conn.ID),
		Confirm:          confirmer,
	}
	if err := s.chat.Run(audit.WithSource(turnCtx, audit.SourceAI, turnID), in, func(ev engine.StreamEvent) {
		writeFrame(ev)
	}); err != nil {
		writeFrame(engine.StreamEvent{Type: engine.EventError, Err: err.Error()})
		writeFrame(engine.StreamEvent{Type: engine.EventDone})
		return
	}
	if turnCtx.Err() == nil {
		if cv, err := s.chat.Conversations().Get(turnCtx, user.ID, convID); err == nil {
			writeFrame(aiMetaFrame{Type: "conversation", ConversationID: cv.ID, Title: cv.Title})
		}
	}
}

func (s *Server) prepareAITurn(ctx context.Context, user models.User, conn models.Connection, req aiTurnRequest, emit func(any) bool) (string, string, bool) {
	convID := req.ConversationID
	providerID := req.ProviderID
	if convID == "" {
		model, err := s.aiConversationModel(ctx, user.ID, providerID)
		if err != nil {
			emit(engine.StreamEvent{Type: engine.EventError, Err: err.Error()})
			emit(engine.StreamEvent{Type: engine.EventDone})
			return "", "", false
		}
		cv, err := s.chat.Conversations().Create(ctx, user.ID, conn.ID, providerID, model)
		if err != nil {
			emit(engine.StreamEvent{Type: engine.EventError, Err: err.Error()})
			emit(engine.StreamEvent{Type: engine.EventDone})
			return "", "", false
		}
		emit(aiMetaFrame{Type: "conversation", ConversationID: cv.ID})
		return cv.ID, providerID, true
	}
	cv, err := s.chat.Conversations().Get(ctx, user.ID, convID)
	if err != nil || cv.ConnectionID != conn.ID {
		emit(engine.StreamEvent{Type: engine.EventError, Err: plugin.ErrNotFound.Error()})
		emit(engine.StreamEvent{Type: engine.EventDone})
		return "", "", false
	}
	return convID, cv.ProviderID, true
}

func (s *Server) handleAITurnControl(w http.ResponseWriter, r *http.Request) {
	user, _ := userFrom(r.Context())
	var req aiTurnControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, s.deps.Logger, plugin.ErrInvalidInput)
		return
	}
	err := s.aiTurns.control(user.ID, chi.URLParam(r, "id"), chi.URLParam(r, "turnID"), req)
	if err != nil {
		writeError(w, s.deps.Logger, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) aiConversationModel(ctx context.Context, userID, providerID string) (string, error) {
	if providerID != "" {
		if s.deps.AI == nil {
			return "", plugin.ErrNotFound
		}
		return s.deps.AI.ProviderModel(ctx, userID, providerID)
	}
	if s.deps.AI != nil {
		if g := s.deps.AI.Global(); g.Configured {
			return g.Model, nil
		}
	}
	if s.deps.AIGlobal.Configured() {
		return s.deps.AIGlobal.Model, nil
	}
	return "", nil
}
