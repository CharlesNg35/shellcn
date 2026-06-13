package servermonitor

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charlesng35/shellcn/plugins/shared/hostmonitor"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	if err := plugin.Validate(New().Manifest(), New().Routes()); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func TestManifestUsesObjectDetailForSystemOverview(t *testing.T) {
	m := New().Manifest()
	if len(m.Tabs) == 0 {
		t.Fatal("missing overview tab")
	}
	dash, ok := m.Tabs[0].Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("overview config = %T, want DashboardConfig", m.Tabs[0].Config)
	}
	var dashboardSystem *plugin.Panel
	for i := range dash.Cells {
		if dash.Cells[i].Key == "system" {
			dashboardSystem = &dash.Cells[i]
			break
		}
	}
	if dashboardSystem == nil || dashboardSystem.Type != plugin.PanelObjectDetail {
		t.Fatalf("dashboard system panel = %+v, want object_detail", dashboardSystem)
	}
	if cfg, ok := dashboardSystem.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
		t.Fatalf("dashboard system config = %#v, want raw-toggle object detail", dashboardSystem.Config)
	} else if len(cfg.Sections) == 0 {
		t.Fatalf("dashboard system detail should declare focused sections")
	}
	var systemTab *plugin.Panel
	for i := range m.Tabs {
		if m.Tabs[i].Key == "system" {
			systemTab = &m.Tabs[i]
			break
		}
	}
	if systemTab == nil || systemTab.Type != plugin.PanelObjectDetail {
		t.Fatalf("system tab = %+v, want object_detail", systemTab)
	}
}

func TestTablesDeclareUsefulEmptyStates(t *testing.T) {
	for _, tab := range New().Manifest().Tabs {
		if tab.Type != plugin.PanelTable {
			continue
		}
		cfg, ok := tab.Config.(plugin.TableConfig)
		if !ok {
			t.Fatalf("%s config = %T, want TableConfig", tab.Key, tab.Config)
		}
		if cfg.EmptyText == "" {
			t.Fatalf("%s table is missing empty text", tab.Key)
		}
	}
}

func TestCollectionLimitsUseBoundedSteppers(t *testing.T) {
	fields := map[string]plugin.Field{}
	for _, group := range New().Manifest().Config.Groups {
		for _, field := range group.Fields {
			fields[field.Key] = field
		}
	}
	for _, key := range []string{"metrics_interval_seconds", "process_limit", "connection_limit"} {
		field, ok := fields[key]
		if !ok {
			t.Fatalf("missing field %s", key)
		}
		if field.Type != plugin.FieldStepper {
			t.Fatalf("%s type = %q, want stepper", key, field.Type)
		}
		if field.Step == nil {
			t.Fatalf("%s should declare a step", key)
		}
		if len(field.Validators) < 2 {
			t.Fatalf("%s should declare min/max validators", key)
		}
	}
}

func TestDirectCollection(t *testing.T) {
	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportDirect,
		Config:    map[string]any{"process_limit": 50, "metrics_interval_seconds": 1},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, nil)
	out, err := Overview(rc)
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	overview, ok := out.(map[string]any)
	if !ok || overview["hostname"] == "" {
		t.Fatalf("overview = %#v", out)
	}
}

func TestRemoteAgentCollection(t *testing.T) {
	local := hostmonitor.NewLocal(hostmonitor.Options{ProcessLimit: 10})
	srv := httptest.NewServer(hostmonitor.Handler(local))
	defer srv.Close()

	sess, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportAgent,
		Config:    map[string]any{"metrics_interval_seconds": 1},
		Net:       fakeHostMonitorNet{baseURL: srv.URL},
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()

	rc := plugin.NewRequestContext(context.Background(), plugin.User{ID: "u1"}, sess, nil, nil, nil)
	out, err := Processes(rc)
	if err != nil {
		t.Fatalf("processes: %v", err)
	}
	page, ok := out.(plugin.Page[hostmonitor.Row])
	if !ok {
		t.Fatalf("processes returned %T", out)
	}
	if page.Total == nil {
		b, _ := json.Marshal(out)
		t.Fatalf("missing total in %s", b)
	}
}

type fakeHostMonitorNet struct {
	baseURL string
}

func (f fakeHostMonitorNet) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, nil
}

func (f fakeHostMonitorNet) HTTP() (string, http.RoundTripper, bool) {
	return f.baseURL, http.DefaultTransport, true
}
