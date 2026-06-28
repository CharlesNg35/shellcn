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

	if _, err := m.Get(ctx, "u2", c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("cross-owner get: want ErrNotFound, got %v", err)
	}

	renamed, err := m.Rename(ctx, "u1", c.ID, "My thread")
	if err != nil || renamed.Title != "My thread" || !renamed.TitleResolved {
		t.Fatalf("rename failed: %+v err=%v", renamed, err)
	}

	if err := m.Delete(ctx, "u1", c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if list, _ := m.List(ctx, "u1", "c1"); len(list) != 0 {
		t.Fatalf("conversation not deleted: %d", len(list))
	}
}

func TestSetAutoTitleOnlyReplacesDefault(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")
	if c.Title != memory.DefaultTitle {
		t.Fatalf("new conversation should start with the default title, got %q", c.Title)
	}
	if !memory.CanAutoTitle(c) {
		t.Fatalf("new conversation should be eligible for auto-title: %+v", c)
	}

	m.SetAutoTitle(ctx, c.ID, "Running containers")
	got, _ := m.Get(ctx, "u1", c.ID)
	if !got.TitleResolved || got.Title != "Running containers" {
		t.Fatalf("auto-title should set a system title: %+v", got)
	}

	if _, err := m.Rename(ctx, "u1", c.ID, "My thread"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	m.SetAutoTitle(ctx, c.ID, "Something else")
	got, _ = m.Get(ctx, "u1", c.ID)
	if got.Title != "My thread" || !got.TitleResolved {
		t.Fatalf("user title must survive auto-title: %+v", got)
	}
}

func TestSetAutoTitleDoesNotReplaceManualDefaultTitle(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")

	if _, err := m.Rename(ctx, "u1", c.ID, memory.DefaultTitle); err != nil {
		t.Fatalf("rename: %v", err)
	}
	m.SetAutoTitle(ctx, c.ID, "Generated title")

	got, _ := m.Get(ctx, "u1", c.ID)
	if got.Title != memory.DefaultTitle || !got.TitleResolved || memory.CanAutoTitle(got) {
		t.Fatalf("manual default title must survive auto-title: %+v", got)
	}
}

func TestTitleFromHeuristic(t *testing.T) {
	title := memory.TitleFrom("show me all the running containers please right now immediately")
	if title == "" || len(strings.Fields(title)) > 8 {
		t.Fatalf("heuristic title should be <= 8 words: %q", title)
	}
}

func TestHistoryKeepsRecentAndCompactsOlder(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")

	for i := 0; i < 12; i++ {
		_ = m.AppendUser(ctx, c.ID, "user message number "+itoa(i)+" "+strings.Repeat("x", 200))
		_ = m.AppendAssistant(ctx, c.ID, "assistant reply "+itoa(i)+" "+strings.Repeat("y", 200), "", nil, false)
	}

	summary, msgs, err := m.History(ctx, c.ID, 500)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if summary == "" {
		t.Fatal("expected a compaction summary for older turns")
	}
	if len(msgs) == 0 {
		t.Fatal("expected recent messages kept verbatim")
	}
	last := msgs[len(msgs)-1]
	if !strings.Contains(last.Content, "11") {
		t.Fatalf("most recent turn missing from history: %q", last.Content)
	}
	if len(msgs) >= 24 {
		t.Fatalf("history not bounded: kept %d of 24", len(msgs))
	}
}

func TestAppendAssignsSequentialMessageSeq(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")

	if err := m.AppendUser(ctx, c.ID, "first"); err != nil {
		t.Fatalf("append user: %v", err)
	}
	if err := m.AppendAssistant(ctx, c.ID, "second", "", nil, false); err != nil {
		t.Fatalf("append assistant: %v", err)
	}
	msgs, err := m.Messages(ctx, "u1", c.ID)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(msgs) != 2 || msgs[0].Seq != 0 || msgs[1].Seq != 1 {
		t.Fatalf("message seq not assigned by store: %+v", msgs)
	}
}

func TestMessagesPagePaginatesNewestFirst(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")
	for i := 0; i < 25; i++ {
		_ = m.AppendUser(ctx, c.ID, "msg "+itoa(i))
	}

	p1, err := m.MessagesPage(ctx, "u1", c.ID, 10, 0)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if p1.TotalCount != 25 || len(p1.Messages) != 10 || !p1.HasMore || p1.LoadedCount != 10 {
		t.Fatalf("page1 wrong: %+v", p1)
	}
	if p1.Messages[len(p1.Messages)-1].Content != "msg 24" {
		t.Fatalf("page1 should end at the newest message: %q", p1.Messages[len(p1.Messages)-1].Content)
	}

	p2, _ := m.MessagesPage(ctx, "u1", c.ID, 10, p1.LoadedCount)
	if len(p2.Messages) != 10 || !p2.HasMore || p2.Messages[len(p2.Messages)-1].Content != "msg 14" {
		t.Fatalf("page2 wrong: %+v", p2)
	}

	p3, _ := m.MessagesPage(ctx, "u1", c.ID, 10, p2.LoadedCount)
	if len(p3.Messages) != 5 || p3.HasMore {
		t.Fatalf("page3 wrong: %+v", p3)
	}

	if _, err := m.MessagesPage(ctx, "u2", c.ID, 10, 0); err == nil {
		t.Fatal("cross-owner page should fail")
	}
}

func TestHistoryTrimsToRecentWindow(t *testing.T) {
	m := newStore()
	ctx := context.Background()
	c, _ := m.Create(ctx, "u1", "c1", "", "gpt-4o")
	for i := 0; i < 60; i++ {
		_ = m.AppendUser(ctx, c.ID, "u"+itoa(i))
		_ = m.AppendAssistant(ctx, c.ID, "a"+itoa(i), "", nil, false)
	}
	_, msgs, err := m.History(ctx, c.ID, 1_000_000)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(msgs) > 40 {
		t.Fatalf("history not trimmed to recent window: %d", len(msgs))
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
