// Command memo is a reference out-of-tree ShellCN plugin: an in-memory notes
// store. Build it, drop the binary in the gateway's plugins.d/, and it appears
// in the connection catalog with no core changes.
//
// It shows the whole authoring surface for a typical plugin: a declarative
// manifest (a table panel with a create form and a row delete), unary routes
// (list/create/delete), and per-connection session state. Streaming, channels,
// egress (cfg.Net), and the HTTP proxy use the same contract — see the SDK docs.
package main

import (
	"context"
	"sort"
	"strconv"
	"sync"

	"github.com/charlesng35/shellcn/sdk"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func main() { sdk.Serve(memo{}) }

func icon(name string) plugin.Icon { return plugin.Icon{Type: plugin.IconLucide, Value: name} }

// memo is the stateless plugin singleton: it declares and connects, holding no
// per-connection state itself.
type memo struct{}

func (memo) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "memo",
		Version:             "0.1.0",
		Title:               "Memo",
		Description:         "In-memory notes — a reference out-of-tree plugin.",
		Icon:                icon("sticky-note"),
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Tabs: []plugin.Panel{{
			Key: "notes", Label: "Notes", Icon: icon("list"), Type: plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "memo.list"},
			Config: plugin.TableConfig{
				Columns: []plugin.Column{
					{Key: "title", Label: "Title", Sortable: true},
					{Key: "body", Label: "Body"},
				},
				ActionIDs:    []string{"memo.create"},
				RowActionIDs: []string{"memo.delete"},
			},
		}},
		Actions: []plugin.Action{
			{ID: "memo.create", Label: "New note", Icon: icon("plus"), RouteID: "memo.create"},
			{
				ID: "memo.delete", Label: "Delete", Icon: icon("trash-2"), RouteID: "memo.delete",
				Params: map[string]string{"id": "${resource.uid}"}, Confirm: true, ConfirmText: "Delete this note?",
			},
		},
	}
}

func (memo) Routes() []plugin.Route {
	return []plugin.Route{
		{ID: "memo.list", Method: plugin.MethodGet, Path: "/notes", Permission: "memo.read", Risk: plugin.RiskSafe, AuditEvent: "memo.list", Handle: list},
		{ID: "memo.create", Method: plugin.MethodPost, Path: "/notes", Permission: "memo.write", Risk: plugin.RiskWrite, AuditEvent: "memo.create", Input: createSchema(), Handle: create},
		{ID: "memo.delete", Method: plugin.MethodDelete, Path: "/notes/{id}", Permission: "memo.delete", Risk: plugin.RiskDestructive, AuditEvent: "memo.delete", Handle: del},
	}
}

func (memo) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return &session{notes: map[string]note{}}, nil
}

func createSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Note", Fields: []plugin.Field{
		{Key: "title", Label: "Title", Type: plugin.FieldText, Required: true},
		{Key: "body", Label: "Body", Type: plugin.FieldTextarea},
	}}}}
}

type note struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// session holds the per-connection state; the SDK gives each handler the session
// via rc.Session.
type session struct {
	mu    sync.Mutex
	seq   uint64
	notes map[string]note
}

func (s *session) HealthCheck(context.Context) error { return nil }

func (s *session) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}

func (s *session) Close() error { return nil }

func list(rc *plugin.RequestContext) (any, error) {
	s := rc.Session.(*session)
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]note, 0, len(s.notes))
	for _, n := range s.notes {
		items = append(items, n)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return plugin.Page[note]{Items: items}, nil
}

func create(rc *plugin.RequestContext) (any, error) {
	var in struct {
		Title string `json:"title" validate:"required"`
		Body  string `json:"body"`
	}
	if err := rc.Bind(&in); err != nil {
		return nil, err
	}
	s := rc.Session.(*session)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	n := note{ID: strconv.FormatUint(s.seq, 10), Title: in.Title, Body: in.Body}
	s.notes[n.ID] = n
	return n, nil
}

func del(rc *plugin.RequestContext) (any, error) {
	s := rc.Session.(*session)
	s.mu.Lock()
	delete(s.notes, rc.Param("id"))
	s.mu.Unlock()
	return map[string]any{"ok": true}, nil
}
