package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

// recordTerminalSession force-records a terminal stream end to end and returns
// the finalized recording id for the connection.
func recordTerminalSession(t *testing.T, h *harness, user string) (connID, recID string) {
	t.Helper()
	ctx := context.Background()
	resp := h.do(t, http.MethodPost, "/api/connections", user,
		strings.NewReader(`{"name":"rec","protocol":"tester","config":{"host":"h"},"recording":{"terminal":"auto"}}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("create: %d (%s)", resp.Status, resp.Body)
	}
	connID = createConnID(t, resp)

	tok := h.mintTicket(t, user, connID, "t.ws", nil)
	c, err := h.dialWS(t, user, "/api/connections/"+connID+"/x/t.ws?ticket="+tok)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	wctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = c.Write(wctx, websocket.MessageText, []byte("ping"))
	_, _, _ = c.Read(wctx)
	_ = c.CloseNow()

	for range 50 {
		recs, _ := h.store.Recordings.List(ctx, store.RecordingFilter{ConnectionID: connID})
		if len(recs) == 1 && recs[0].Status == models.RecordingFinalized {
			return connID, recs[0].ID
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("recording did not finalize")
	return "", ""
}

func recordingIDs(t *testing.T, body []byte) []string {
	t.Helper()
	var list []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode recordings: %v (%s)", err, body)
	}
	out := make([]string, len(list))
	for i, r := range list {
		out[i] = r.ID
	}
	return out
}

func TestRecordingListScopeAndContent(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	_, recID := recordTerminalSession(t, h, "op")

	// Owner sees it.
	if ids := recordingIDs(t, h.do(t, http.MethodGet, "/api/recordings", "op", nil).Body); len(ids) != 1 || ids[0] != recID {
		t.Fatalf("owner list: want [%s], got %v", recID, ids)
	}
	// Stranger sees none.
	if ids := recordingIDs(t, h.do(t, http.MethodGet, "/api/recordings", "viewer", nil).Body); len(ids) != 0 {
		t.Fatalf("stranger list: want none, got %v", ids)
	}
	// Admin has no special access: recordings are private to their creator, so
	// admin sees only their own (none) and a ?user filter is ignored.
	if ids := recordingIDs(t, h.do(t, http.MethodGet, "/api/recordings", "admin", nil).Body); len(ids) != 0 {
		t.Fatalf("admin list: want none (own only), got %v", ids)
	}
	if ids := recordingIDs(t, h.do(t, http.MethodGet, "/api/recordings?user=op", "admin", nil).Body); len(ids) != 0 {
		t.Fatalf("admin must not drill into another user's recordings, got %v", ids)
	}

	// Content is asciicast for the owner, forbidden for a stranger.
	resp := h.do(t, http.MethodGet, "/api/recordings/"+recID+"/content", "op", nil)
	if resp.Status != http.StatusOK || !strings.HasPrefix(string(resp.Body), `{"`) {
		t.Fatalf("content: status=%d body=%.40s", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/recordings/"+recID+"/content", "viewer", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("stranger content: want 403, got %d", resp.Status)
	}
	// Admin is no exception — it cannot read another user's recording content.
	if resp := h.do(t, http.MethodGet, "/api/recordings/"+recID+"/content", "admin", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("admin content of another's recording: want 403, got %d", resp.Status)
	}

	// Stranger cannot delete; owner can, and the delete is audited.
	if resp := h.do(t, http.MethodDelete, "/api/recordings/"+recID, "viewer", nil); resp.Status != http.StatusForbidden {
		t.Fatalf("stranger delete: want 403, got %d", resp.Status)
	}
	if resp := h.do(t, http.MethodDelete, "/api/recordings/"+recID, "op", nil); resp.Status != http.StatusOK {
		t.Fatalf("owner delete: want 200, got %d (%s)", resp.Status, resp.Body)
	}
	if resp := h.do(t, http.MethodGet, "/api/recordings/"+recID, "op", nil); resp.Status != http.StatusNotFound {
		t.Fatalf("deleted recording: want 404, got %d", resp.Status)
	}
	rows, _ := h.store.Audit.List(ctx, store.AuditFilter{})
	var audited bool
	for _, r := range rows {
		if r.Event == "recording.delete" && r.Result == models.AuditAllowed {
			audited = true
		}
	}
	if !audited {
		t.Error("expected an allowed recording.delete audit row")
	}
}

func TestDesktopChunkFlow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	resp := h.do(t, http.MethodPost, "/api/connections", "op",
		strings.NewReader(`{"name":"vnc","protocol":"tester","config":{"host":"h"},"recording":{"desktop":"manual"}}`))
	id := createConnID(t, resp)

	resp = h.do(t, http.MethodPost, "/api/connections/"+id+"/recordings/desktop", "op",
		strings.NewReader(`{"routeId":"t.desk","format":"webm_canvas"}`))
	if resp.Status != http.StatusCreated {
		t.Fatalf("start desktop: %d (%s)", resp.Status, resp.Body)
	}
	recID := createConnID(t, resp)

	for i, chunk := range []string{"chunk-a", "chunk-b"} {
		r := h.do(t, http.MethodPost, "/api/recordings/"+recID+"/chunks?index="+itoa(i), "op", strings.NewReader(chunk))
		if r.Status != http.StatusOK {
			t.Fatalf("chunk %d: %d (%s)", i, r.Status, r.Body)
		}
	}
	// Out-of-order chunk rejected.
	if r := h.do(t, http.MethodPost, "/api/recordings/"+recID+"/chunks?index=5", "op", strings.NewReader("x")); r.Status != http.StatusBadRequest {
		t.Fatalf("out-of-order chunk: want 400, got %d", r.Status)
	}

	if r := h.do(t, http.MethodPost, "/api/recordings/"+recID+"/finalize", "op", nil); r.Status != http.StatusOK {
		t.Fatalf("finalize: %d (%s)", r.Status, r.Body)
	}
	rec, _ := h.store.Recordings.Get(ctx, recID)
	if rec.Status != models.RecordingFinalized || rec.Class != "desktop" || rec.Authoritative {
		t.Fatalf("desktop recording metadata: %+v", rec)
	}
	resp = h.do(t, http.MethodGet, "/api/recordings/"+recID+"/content", "op", nil)
	if resp.Status != http.StatusOK || string(resp.Body) != "chunk-achunk-b" {
		t.Fatalf("content: status=%d body=%q", resp.Status, resp.Body)
	}
}

func itoa(i int) string { return string(rune('0' + i)) }
