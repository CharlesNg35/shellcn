package server

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestAITurnRegistryScopesControlsAndDeliversConfirmation(t *testing.T) {
	reg := newAITurnRegistry()
	emitted := make(chan aiConfirmFrame, 1)
	confirmer := newAITurnConfirmer("turn-1", func(frame any) bool {
		confirm, ok := frame.(aiConfirmFrame)
		if !ok {
			t.Fatalf("frame type = %T, want aiConfirmFrame", frame)
		}
		emitted <- confirm
		return true
	})

	cancelled := make(chan struct{})
	var once sync.Once
	reg.add(&aiTurn{
		id: "turn-1", userID: "user-1", connID: "conn-1",
		cancel:    func() { once.Do(func() { close(cancelled) }) },
		confirmer: confirmer,
	})
	defer reg.remove("turn-1")

	result := make(chan bool, 1)
	go func() {
		ok, err := confirmer.Confirm(context.Background(), tools.ConfirmRequest{
			ToolCallID: "tool-1",
			ToolName:   "delete_file",
		})
		if err != nil {
			t.Errorf("confirm returned error: %v", err)
		}
		result <- ok
	}()

	select {
	case frame := <-emitted:
		if frame.TurnID != "turn-1" || frame.ToolID != "tool-1" {
			t.Fatalf("confirm frame = %+v", frame)
		}
	case <-time.After(time.Second):
		t.Fatal("confirmation frame was not emitted")
	}

	if err := reg.control("other-user", "conn-1", "turn-1", aiTurnControlRequest{Type: "confirm", ToolID: "tool-1"}); !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("wrong user control error = %v, want not found", err)
	}
	if err := reg.control("user-1", "conn-1", "turn-1", aiTurnControlRequest{Type: "confirm", ToolID: "tool-1"}); err != nil {
		t.Fatalf("confirm control failed: %v", err)
	}
	select {
	case ok := <-result:
		if !ok {
			t.Fatal("confirmation was rejected, want approved")
		}
	case <-time.After(time.Second):
		t.Fatal("confirmation decision was not delivered")
	}

	if err := reg.control("user-1", "conn-1", "turn-1", aiTurnControlRequest{Type: "stop"}); err != nil {
		t.Fatalf("stop control failed: %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("stop did not cancel the turn")
	}
}
