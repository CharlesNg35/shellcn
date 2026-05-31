// Package memory persists AI conversations and messages and assembles the
// compacted history a turn feeds to the model. Older turns are rolled into a
// rolling summary and tool results are truncated so the prompt stays within the
// model's context window. The token estimate is a deterministic heuristic
// (~4 chars/token); a tokenizer can replace estimate() without touching callers.
package memory

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

const (
	// minKeptMessages stay verbatim at the tail; older ones are compacted.
	minKeptMessages = 8
	// defaultContextWindow is used when a model's window is unknown.
	defaultContextWindow = 128_000
	// historyFraction of the window is budgeted for history (the rest is system
	// prompt + tools + the model's output).
	historyFraction      = 0.5
	toolResultCharLimit  = 600
	compactedMessageChar = 400
	titleWords           = 8
)

// contextWindows is a small static registry; unknown models fall back to default.
var contextWindows = map[string]int{
	"gpt-4o":            128_000,
	"gpt-4o-mini":       128_000,
	"o3-mini":           200_000,
	"claude-opus-4-1":   200_000,
	"claude-sonnet-4-5": 200_000,
	"claude-haiku-4-5":  200_000,
	"gemini-2.5-pro":    1_000_000,
	"gemini-2.5-flash":  1_000_000,
}

// ContextWindow returns a model's token window, or a safe default.
func ContextWindow(model string) int {
	if w, ok := contextWindows[model]; ok {
		return w
	}
	return defaultContextWindow
}

// Store is the conversation/message persistence + context-assembly surface.
type Store struct {
	conv store.AIConversationStore
	msg  store.AIMessageStore
	now  func() time.Time
}

// New wires the conversation and message repos.
func New(conv store.AIConversationStore, msg store.AIMessageStore) *Store {
	return &Store{conv: conv, msg: msg, now: time.Now}
}

// Create starts a new conversation owned by the user on a connection.
func (s *Store) Create(ctx context.Context, ownerID, connID, providerID, model string) (models.AIConversation, error) {
	now := s.now()
	c := models.AIConversation{
		ID:           uuid.NewString(),
		OwnerID:      ownerID,
		ConnectionID: connID,
		Title:        "New conversation",
		ProviderID:   providerID,
		Model:        model,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.conv.Create(ctx, &c); err != nil {
		return models.AIConversation{}, err
	}
	return c, nil
}

// Get returns an owned conversation (others are hidden as not-found).
func (s *Store) Get(ctx context.Context, ownerID, id string) (models.AIConversation, error) {
	c, err := s.conv.Get(ctx, id)
	if err != nil {
		return models.AIConversation{}, err
	}
	if c.OwnerID != ownerID {
		return models.AIConversation{}, store.ErrNotFound
	}
	return c, nil
}

// List returns the user's conversations for a connection (newest first).
func (s *Store) List(ctx context.Context, ownerID, connID string) ([]models.AIConversation, error) {
	return s.conv.List(ctx, ownerID, connID)
}

// Messages returns a conversation's full ordered message history.
func (s *Store) Messages(ctx context.Context, ownerID, id string) ([]models.AIMessage, error) {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return nil, err
	}
	return s.msg.List(ctx, id)
}

// Rename sets a user-provided title (clearing the auto-titled flag).
func (s *Store) Rename(ctx context.Context, ownerID, id, title string) (models.AIConversation, error) {
	c, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return models.AIConversation{}, err
	}
	c.Title = strings.TrimSpace(title)
	c.AutoTitled = false
	c.UpdatedAt = s.now()
	if err := s.conv.Update(ctx, &c); err != nil {
		return models.AIConversation{}, err
	}
	return c, nil
}

// Delete removes an owned conversation and its messages.
func (s *Store) Delete(ctx context.Context, ownerID, id string) error {
	if _, err := s.Get(ctx, ownerID, id); err != nil {
		return err
	}
	if err := s.msg.DeleteByConversation(ctx, id); err != nil {
		return err
	}
	return s.conv.Delete(ctx, id)
}

// AppendUser stores a user message and, on the first exchange, auto-titles the
// conversation from its content.
func (s *Store) AppendUser(ctx context.Context, convID, content string) error {
	existing, err := s.msg.List(ctx, convID)
	if err != nil {
		return err
	}
	if err := s.msg.Append(ctx, &models.AIMessage{
		ID: uuid.NewString(), ConversationID: convID, Seq: len(existing),
		Role: string(engine.RoleUser), Content: content, CreatedAt: s.now(),
	}); err != nil {
		return err
	}
	if len(existing) == 0 {
		s.autoTitle(ctx, convID, content)
	}
	return nil
}

// AppendAssistant stores a finalized assistant message.
func (s *Store) AppendAssistant(ctx context.Context, convID, content, reasoning string, calls []models.AIToolCallRecord, truncated bool) error {
	existing, err := s.msg.List(ctx, convID)
	if err != nil {
		return err
	}
	if err := s.msg.Append(ctx, &models.AIMessage{
		ID: uuid.NewString(), ConversationID: convID, Seq: len(existing),
		Role: string(engine.RoleAssistant), Content: content, Reasoning: reasoning,
		ToolCalls: calls, Truncated: truncated, CreatedAt: s.now(),
	}); err != nil {
		return err
	}
	s.touch(ctx, convID)
	return nil
}

// History assembles the compacted context for a turn: a summary string (prior
// conversation memory, prepended to the system prompt) plus the recent messages
// kept verbatim within the model's history budget.
func (s *Store) History(ctx context.Context, convID string, contextWindow int, priorSummary string) (summary string, msgs []engine.Message, err error) {
	all, err := s.msg.List(ctx, convID)
	if err != nil {
		return "", nil, err
	}
	budget := int(float64(contextWindow) * historyFraction)

	// Walk newest→oldest, keeping messages until the budget is spent (always keep
	// at least the most recent minKeptMessages).
	keepFrom := len(all)
	used := 0
	for i := len(all) - 1; i >= 0; i-- {
		used += estimate(all[i].Content) + 4
		keep := used <= budget || (len(all)-i) <= minKeptMessages
		if !keep {
			break
		}
		keepFrom = i
	}

	var older []models.AIMessage
	if keepFrom > 0 {
		older = all[:keepFrom]
	}
	recent := all[keepFrom:]

	summary = compact(priorSummary, older)
	msgs = toEngine(recent)
	return summary, msgs, nil
}

func toEngine(msgs []models.AIMessage) []engine.Message {
	out := make([]engine.Message, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, engine.Message{Role: engine.Role(m.Role), Content: m.Content})
	}
	return out
}

// compact folds older messages into the existing summary, truncating each.
func compact(prior string, older []models.AIMessage) string {
	if len(older) == 0 {
		return prior
	}
	var b strings.Builder
	if prior != "" {
		b.WriteString(prior)
		b.WriteString("\n")
	}
	for _, m := range older {
		line := truncate(m.Content, compactedMessageChar)
		if len(m.ToolCalls) > 0 {
			line += " [used " + itoa(len(m.ToolCalls)) + " tool(s)]"
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return truncate(b.String(), toolResultCharLimit*10)
}

func (s *Store) autoTitle(ctx context.Context, convID, firstMessage string) {
	c, err := s.conv.Get(ctx, convID)
	if err != nil {
		return
	}
	c.Title = titleFrom(firstMessage)
	c.AutoTitled = true
	c.UpdatedAt = s.now()
	_ = s.conv.Update(ctx, &c)
}

func (s *Store) touch(ctx context.Context, convID string) {
	c, err := s.conv.Get(ctx, convID)
	if err != nil {
		return
	}
	c.UpdatedAt = s.now()
	_ = s.conv.Update(ctx, &c)
}

// UpdateSummary persists the rolling compaction summary on the conversation.
func (s *Store) UpdateSummary(ctx context.Context, convID, summary string) {
	c, err := s.conv.Get(ctx, convID)
	if err != nil {
		return
	}
	c.Summary = summary
	c.UpdatedAt = s.now()
	_ = s.conv.Update(ctx, &c)
}

func titleFrom(msg string) string {
	words := strings.Fields(strings.TrimSpace(msg))
	if len(words) == 0 {
		return "New conversation"
	}
	if len(words) > titleWords {
		words = words[:titleWords]
	}
	t := strings.Join(words, " ")
	if len(t) > 60 {
		t = t[:60]
	}
	return t
}

func estimate(s string) int { return len(s)/4 + 1 }

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "…"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
