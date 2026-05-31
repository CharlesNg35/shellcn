package memory_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/internal/ai/memory"
	"github.com/charlesng35/shellcn/internal/store"
)

func newStore() *memory.Store {
	st := store.NewMemory()
	return memory.New(st.AIConversations, st.AIMessages)
}

func TestCreateListGetRenameDelete(t *testing.T) {
	m := newStore()
	ctx := context.Background()

	c, err := m.Create(ctx, "u1", "c1", "", "gpt-4o")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	list, _ := m.List(ctx, "u1", "c1")
	if len(list) != 1 {
		t.Fatalf("want 1 conversation, got %d", len(list))
	}

	// Owner scoping: another user cannot see/get it.
	if _, err := m.Get(ctx, "u2", c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner get: want ErrNotFound, got %v", err)
	}

	renamed, err := m.Rename(ctx, "u1", c.ID, "My thread")
	if err != nil || renamed.Title != "My thread" || renamed.AutoTitled {
		t.Fatalf("rename failed: %+v err=%v", renamed, err)
	}

	if err := m.Delete(ctx, "u1", c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := m.List(ctx, "u1", "c1"); len(list) != 0 {
		t.Fatalf("conversation not deleted: %d", len(list))
	}
}

func TestAutoTitleOnFirstMessage(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")

	if err := m.AppendUser(ctx, c.ID, "show me all the running containers please right now"); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, _ := m.Get(ctx, "u1", c.ID)
	if !got.AutoTitled || got.Title == "New conversation" {
		t.Fatalf("first message should auto-title: %+v", got)
	}
	if len(strings.Fields(got.Title)) > 8 {
		t.Fatalf("title should be trimmed to ~8 words: %q", got.Title)
	}
}

func TestHistoryKeepsRecentAndCompactsOlder(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")

	// Append many turns so older ones must compact under a tiny window.
	for i := 0; i < 12; i++ {
		_ = m.AppendUser(ctx, c.ID, "user message number "+itoa(i)+" "+strings.Repeat("x", 200))
		_ = m.AppendAssistant(ctx, c.ID, "assistant reply "+itoa(i)+" "+strings.Repeat("y", 200), "", nil, false)
	}

	// Small window forces compaction; recent messages stay verbatim.
	summary, msgs, err := m.History(ctx, c.ID, 2000, "")
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if summary == "" {
		t.Fatal("expected a compaction summary for older turns")
	}
	if len(msgs) == 0 {
		t.Fatal("expected recent messages kept verbatim")
	}
	// The most recent message must be present in full (not compacted).
	last := msgs[len(msgs)-1]
	if !strings.Contains(last.Content, "11") {
		t.Fatalf("most recent turn missing from history: %q", last.Content)
	}
	// Budget bound: kept messages should be far fewer than the full 24.
	if len(msgs) >= 24 {
		t.Fatalf("history not bounded: kept %d of 24", len(msgs))
	}
}

func TestContextWindowFallback(t *testing.T) {
	if memory.ContextWindow("gpt-4o") != 128000 {
		t.Fatal("known model window wrong")
	}
	if memory.ContextWindow("totally-unknown") != 128000 {
		t.Fatal("unknown model should fall back to default")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
