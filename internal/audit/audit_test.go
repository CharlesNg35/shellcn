package audit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/charlesng35/shellcn/internal/audit"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/store"
)

func TestWriterAppends(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	w := audit.NewWriter(st.Audit)

	w.Record(ctx, audit.Event{
		User:         models.User{ID: "u1", Username: "alice"},
		Event:        "vm.start",
		ConnectionID: "c1",
		RouteID:      "proxmox.vm.start",
		Risk:         "write",
		Result:       models.AuditAllowed,
		Params:       map[string]string{"vmid": "101", "password": "***"},
	})

	rows, err := st.Audit.List(ctx, store.AuditFilter{ConnectionID: "c1"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: got %d err=%v", len(rows), err)
	}
	e := rows[0]
	if e.ID == "" || e.Time.IsZero() {
		t.Error("writer must assign an ID and timestamp")
	}
	if e.Username != "alice" || e.Event != "vm.start" || e.Result != models.AuditAllowed {
		t.Errorf("entry fields wrong: %+v", e)
	}
	if e.Params["password"] != "***" {
		t.Error("params should be stored as provided (already redacted)")
	}
}

func TestWriterRecordsError(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemory()
	w := audit.NewWriter(st.Audit)
	w.Record(ctx, audit.Event{
		User: models.User{ID: "u1"}, Event: "x", Result: models.AuditError,
		Err: errors.New("boom"),
	})
	rows, _ := st.Audit.List(ctx, store.AuditFilter{})
	if len(rows) != 1 || rows[0].Error != "boom" {
		t.Errorf("error not recorded: %+v", rows)
	}
}

func TestNoopSink(_ *testing.T) {
	// Must not panic and must not require a store.
	audit.Noop{}.Record(context.Background(), audit.Event{Event: "x"})
}
