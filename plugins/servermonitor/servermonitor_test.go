package servermonitor

import (
	"context"
	"encoding/json"
	"errors"
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
	m := New().Manifest()
	if m.SupportsTransport(plugin.TransportDirect) {
		t.Fatal("server monitor must not declare direct transport")
	}
	if !m.SupportsTransport(plugin.TransportAgent) || m.Agent == nil {
		t.Fatal("server monitor must declare agent transport and profile")
	}
}

func TestManifestKeepsCompactHostSummaryInOverview(t *testing.T) {
	m := New().Manifest()
	if len(m.Tabs) == 0 {
		t.Fatal("missing overview tab")
	}
	dash, ok := m.Tabs[0].Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("overview config = %T, want DashboardConfig", m.Tabs[0].Config)
	}
	var hostSummary *plugin.Panel
	for i := range dash.Cells {
		if dash.Cells[i].Key == "host" {
			hostSummary = &dash.Cells[i]
			break
		}
	}
	if hostSummary == nil || hostSummary.Type != plugin.PanelObjectDetail {
		t.Fatalf("overview host summary = %+v, want object_detail", hostSummary)
	}
	cfg, ok := hostSummary.Config.(plugin.ObjectDetailConfig)
	if !ok {
		t.Fatalf("host summary config = %T, want ObjectDetailConfig", hostSummary.Config)
	}
	if cfg.RawToggle || len(cfg.Sections) != 2 || !hasUsageField(cfg, "cpuPct") || !hasUsageField(cfg, "memPct") {
		t.Fatalf("host summary should be compact and include CPU/RAM usage rows: %#v", cfg)
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
	if cfg, ok := systemTab.Config.(plugin.ObjectDetailConfig); !ok || !cfg.RawToggle {
		t.Fatalf("system tab config = %#v, want full raw-toggle object detail", systemTab.Config)
	}
}

func TestOverviewDashboardKeepsOriginalReadableShape(t *testing.T) {
	m := New().Manifest()
	dash, ok := m.Tabs[0].Config.(plugin.DashboardConfig)
	if !ok {
		t.Fatalf("overview config = %T, want DashboardConfig", m.Tabs[0].Config)
	}
	cells := map[string]plugin.Panel{}
	for _, cell := range dash.Cells {
		cells[cell.Key] = cell
	}
	for _, key := range []string{"host", "cpumem", "throughput"} {
		if _, ok := cells[key]; !ok {
			t.Fatalf("overview dashboard missing %q cell", key)
		}
	}
	for _, key := range []string{"metrics", "health", "load", "system", "disks"} {
		if _, ok := cells[key]; ok {
			t.Fatalf("overview dashboard should not include duplicated %q cell", key)
		}
	}
	cpumem, ok := cells["cpumem"].Config.(plugin.MetricsConfig)
	if !ok || len(cpumem.Gauges) != 0 || len(cpumem.Usage) != 0 || len(cpumem.Series) != 2 {
		t.Fatalf("cpu/memory overview cell should be trends only: %#v", cells["cpumem"].Config)
	}
}

func hasUsageField(cfg plugin.ObjectDetailConfig, key string) bool {
	for _, section := range cfg.Sections {
		for _, field := range section.Fields {
			if field.Key == key && field.Usage != nil {
				return true
			}
		}
	}
	return false
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
		if cfg.EmptyText == "No rows collected." {
			t.Fatalf("%s table uses generic empty text", tab.Key)
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

func TestDirectCollectionRejected(t *testing.T) {
	_, err := Connect(context.Background(), plugin.ConnectConfig{
		Transport: plugin.TransportDirect,
		Config:    map[string]any{"process_limit": 50, "metrics_interval_seconds": 1},
	})
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("direct connect should be rejected with ErrInvalidInput, got %v", err)
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
