// Package memory persists AI conversations and assembles context windows.
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/charlesng35/shellcn/internal/ai/budget"
	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

// Compaction keeps at least minKeptTurns recent turns intact.
const (
	maxLoadedMessages        = 40
	minKeptTurns             = 4
	summaryCharBudget        = 12_000
	compactedUserCharLimit   = 400
	compactedAssistCharLimit = 500
	toolResultCharLimit      = 600
	toolResultCountLimit     = 6
	defaultMessagePageSize   = 30
)

// Store is the conversation persistence and context-assembly surface.
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
		Title:        DefaultTitle,
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

// Rename sets a user-provided title.
func (s *Store) Rename(ctx context.Context, ownerID, id, title string) (models.AIConversation, error) {
	c, err := s.Get(ctx, ownerID, id)
	if err != nil {
		return models.AIConversation{}, err
	}
	c.Title = strings.TrimSpace(title)
	c.TitleResolved = true
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

// DefaultTitle is shown until the first successful automatic or manual title.
const DefaultTitle = "New conversation"

// AppendUser stores a user message.
func (s *Store) AppendUser(ctx context.Context, convID, content string) error {
	return s.msg.Append(ctx, &models.AIMessage{
		ID: uuid.NewString(), ConversationID: convID, Seq: -1,
		Role: string(engine.RoleUser), Content: content, CreatedAt: s.now(),
	})
}

// MessageCount returns how many messages a conversation has.
func (s *Store) MessageCount(ctx context.Context, convID string) int {
	msgs, err := s.msg.List(ctx, convID)
	if err != nil {
		return 0
	}
	return len(msgs)
}

// SetAutoTitle sets a system-generated title, leaving a user-set title untouched.
func (s *Store) SetAutoTitle(ctx context.Context, convID, title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	c, err := s.conv.Get(ctx, convID)
	if err != nil || !CanAutoTitle(c) {
		return
	}
	c.Title = title
	c.TitleResolved = true
	c.UpdatedAt = s.now()
	_ = s.conv.Update(ctx, &c)
}

// CanAutoTitle reports whether the conversation still owns the initial title slot.
func CanAutoTitle(c models.AIConversation) bool {
	return !c.TitleResolved
}

// AppendAssistant stores a finalized assistant message.
func (s *Store) AppendAssistant(ctx context.Context, convID, content, reasoning string, calls []models.AIToolCallRecord, truncated bool) error {
	if err := s.msg.Append(ctx, &models.AIMessage{
		ID: uuid.NewString(), ConversationID: convID, Seq: -1,
		Role: string(engine.RoleAssistant), Content: content, Reasoning: reasoning,
		ToolCalls: calls, Truncated: truncated, CreatedAt: s.now(),
	}); err != nil {
		return err
	}
	s.touch(ctx, convID)
	return nil
}

// History returns compacted older turns plus recent messages within tokenBudget.
func (s *Store) History(ctx context.Context, convID string, tokenBudget int) (summary string, msgs []engine.Message, err error) {
	all, err := s.msg.Recent(ctx, convID, maxLoadedMessages)
	if err != nil {
		return "", nil, err
	}
	if tokenBudget <= 0 {
		tokenBudget = budget.DefaultHistoryBudget
	}
	return splitByTokenBudget(all, tokenBudget)
}

// MessagePage is one UI window of conversation messages.
type MessagePage struct {
	Messages    []models.AIMessage `json:"messages"`
	LoadedCount int                `json:"loadedCount"`
	TotalCount  int                `json:"totalCount"`
	HasMore     bool               `json:"hasMore"`
}

// MessagesPage returns one page of an owned conversation's history.
func (s *Store) MessagesPage(ctx context.Context, ownerID, convID string, limit, loadedCount int) (MessagePage, error) {
	if _, err := s.Get(ctx, ownerID, convID); err != nil {
		return MessagePage{}, err
	}
	if limit <= 0 {
		limit = defaultMessagePageSize
	}
	if loadedCount < 0 {
		loadedCount = 0
	}
	total, err := s.msg.Count(ctx, convID)
	if err != nil {
		return MessagePage{}, err
	}
	remaining := max(0, total-loadedCount)
	pageSize := min(limit, remaining)
	startOffset := max(0, total-loadedCount-pageSize)

	var msgs []models.AIMessage
	if pageSize > 0 {
		if msgs, err = s.msg.Range(ctx, convID, startOffset, pageSize); err != nil {
			return MessagePage{}, err
		}
	}
	return MessagePage{
		Messages:    msgs,
		LoadedCount: loadedCount + len(msgs),
		TotalCount:  total,
		HasMore:     startOffset > 0,
	}, nil
}

// formatted is a role + flattened content (main text + tool-result lines).
type formatted struct {
	role    engine.Role
	content string
}

// splitByTokenBudget compacts old messages until recent history fits the budget.
func splitByTokenBudget(messages []models.AIMessage, tokenBudget int) (string, []engine.Message, error) {
	full := make([]formatted, len(messages))
	for i, m := range messages {
		full[i] = formatMessage(m, -1, false) // recent: verbatim, raw tool results
	}

	total := 0
	for _, f := range full {
		total += budget.Estimate(f.content)
	}

	splitIndex := 0
	minRecentIndex := max(0, len(messages)-minKeptTurns*2)
	for total > tokenBudget && splitIndex < minRecentIndex {
		total -= budget.Estimate(full[splitIndex].content)
		splitIndex++
	}

	older := make([]formatted, 0, splitIndex)
	for _, m := range messages[:splitIndex] {
		limit := compactedAssistCharLimit
		if m.Role == string(engine.RoleUser) {
			limit = compactedUserCharLimit
		}
		older = append(older, formatMessage(m, limit, true))
	}

	recent := make([]engine.Message, 0, len(full)-splitIndex)
	for _, f := range full[splitIndex:] {
		if f.content == "" {
			continue
		}
		recent = append(recent, engine.Message{Role: f.role, Content: f.content})
	}

	return buildSummary(older, summaryCharBudget), recent, nil
}

// formatMessage flattens a message into role plus content.
func formatMessage(m models.AIMessage, limit int, compact bool) formatted {
	main := normalizeWhitespace(m.Content)
	var lines []string
	if compact {
		for i, tc := range m.ToolCalls {
			if i >= toolResultCountLimit {
				break
			}
			lines = append(lines, "[Tool] "+summarizeToolCall(tc, toolResultCharLimit))
		}
	} else {
		for _, tc := range m.ToolCalls {
			lines = append(lines, formatToolCallRaw(tc))
		}
	}
	parts := make([]string, 0, len(lines)+1)
	if main != "" {
		parts = append(parts, main)
	}
	parts = append(parts, lines...)
	content := strings.Join(parts, "\n")
	if compact && limit >= 0 {
		content = truncate(content, limit)
	}
	return formatted{role: engine.Role(m.Role), content: content}
}

// summarizeToolCall renders a tool call/result compactly: "name: <preview>".
func summarizeToolCall(tc models.AIToolCallRecord, limit int) string {
	if tc.Err != "" {
		return tc.Name + " failed: " + truncate(normalizeWhitespace(tc.Err), limit)
	}
	body := normalizeWhitespace(stringify(tc.Output))
	if body == "" {
		return tc.Name
	}
	return tc.Name + ": " + truncate(body, limit)
}

// formatToolCallRaw renders a tool result verbatim (already capped at execution).
func formatToolCallRaw(tc models.AIToolCallRecord) string {
	if tc.Err != "" {
		return "[Tool:" + tc.Name + "] error: " + tc.Err
	}
	body := stringify(tc.Output)
	if body == "" {
		return "[Tool:" + tc.Name + "]"
	}
	return "[Tool:" + tc.Name + "] " + body
}

// buildSummary keeps the newest compacted lines that fit charBudget.
func buildSummary(messages []formatted, charBudget int) string {
	if len(messages) == 0 {
		return ""
	}
	lines := make([]string, len(messages))
	for i, m := range messages {
		role := "Assistant"
		if m.role == engine.RoleUser {
			role = "User"
		}
		lines[i] = role + ": " + m.content
	}
	full := strings.Join(lines, "\n")
	if len(full) <= charBudget {
		return full
	}

	var kept []string
	used, dropped := 0, 0
	for i := len(lines) - 1; i >= 0; i-- {
		next := used + len(lines[i])
		if len(kept) > 0 {
			next++
		}
		if next > charBudget {
			dropped = i + 1
			break
		}
		kept = append([]string{lines[i]}, kept...)
		used = next
	}
	if dropped > 0 {
		kept = append([]string{"[Earlier conversation compacted: " + itoa(dropped) + " message(s)]"}, kept...)
	}
	return strings.Join(kept, "\n")
}

func normalizeWhitespace(s string) string { return strings.Join(strings.Fields(s), " ") }

func stringify(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func (s *Store) touch(ctx context.Context, convID string) {
	c, err := s.conv.Get(ctx, convID)
	if err != nil {
		return
	}
	c.UpdatedAt = s.now()
	_ = s.conv.Update(ctx, &c)
}

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
