package plugin_test

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

type stubPlugin struct {
	manifest plugin.Manifest
	routes   []plugin.Route
}

func (s *stubPlugin) Manifest() plugin.Manifest { return s.manifest }
func (s *stubPlugin) Routes() []plugin.Route    { return s.routes }
func (s *stubPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

func TestRegistryRegisterGetAll(t *testing.T) {
	m, routes := sampleManifest()
	reg := plugin.NewRegistry()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); !errors.Is(err, plugin.ErrAlreadyExists) {
		t.Fatalf("duplicate register: want ErrAlreadyExists, got %v", err)
	}
	if _, ok := reg.Get("sample"); !ok {
		t.Error("Get(sample) not found")
	}
	if all := reg.All(); len(all) != 1 {
		t.Errorf("All: want 1, got %d", len(all))
	}
	if rt, ok := reg.Route("sample", "sample.start"); !ok || rt.Risk != plugin.RiskWrite {
		t.Errorf("Route lookup failed: ok=%v risk=%v", ok, rt.Risk)
	}
	if s := reg.Summaries(); len(s) != 1 || s[0].Name != "sample" {
		t.Errorf("Summaries unexpected: %+v", s)
	}
}

func TestValidateRejectsBadManifests(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	base := func() (plugin.Manifest, []plugin.Route) {
		return plugin.Manifest{
				APIVersion: plugin.CurrentAPIVersion, Name: "x", Title: "X",
				Layout: plugin.LayoutTabs, SupportedTransports: []plugin.Transport{plugin.TransportDirect},
			}, []plugin.Route{
				{ID: "x.list", Method: plugin.MethodGet, Permission: "x.read", Risk: plugin.RiskSafe, Handle: noop},
			}
	}

	tests := []struct {
		name string
		want string
		mut  func(*plugin.Manifest, *[]plugin.Route)
	}{
		{"unsupported api version", "APIVersion", func(m *plugin.Manifest, _ *[]plugin.Route) { m.APIVersion = 99 }},
		{"missing name", "Name is required", func(m *plugin.Manifest, _ *[]plugin.Route) { m.Name = "" }},
		{"missing direct transport", "must include", func(m *plugin.Manifest, _ *[]plugin.Route) { m.SupportedTransports = nil }},
		{"agent without profile", "AgentProfile is required", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.SupportedTransports = []plugin.Transport{plugin.TransportDirect, plugin.TransportAgent}
		}},
		{"duplicate route id", "duplicate route ID", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.list", Method: plugin.MethodGet, Permission: "p", Risk: plugin.RiskSafe, Handle: noop})
		}},
		{"route missing permission", "missing a Permission", func(_ *plugin.Manifest, r *[]plugin.Route) {
			(*r)[0].Permission = ""
		}},
		{"ws route missing stream", "missing a Stream", func(_ *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.ws", Method: plugin.MethodWS, Permission: "p", Risk: plugin.RiskSafe})
		}},
		{"tab references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "t", Label: "T", Panel: plugin.PanelTable, Source: &plugin.DataSource{RouteID: "ghost"}}}
		}},
		{"file browser config references unknown route", "uploadRouteId references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "files", Label: "Files", Panel: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"uploadRouteId": "ghost"}}}
		}},
		{"file browser upload route requires file input", "without a file input schema", func(m *plugin.Manifest, r *[]plugin.Route) {
			*r = append(*r, plugin.Route{ID: "x.upload", Method: plugin.MethodPost, Permission: "x.write", Risk: plugin.RiskWrite, Handle: noop})
			m.Tabs = []plugin.Tab{{Key: "files", Label: "Files", Panel: plugin.PanelFileBrowser, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"uploadRouteId": "x.upload"}}}
		}},
		{"form submit route must be write method", "invalid write method", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Tabs = []plugin.Tab{{Key: "form", Label: "Form", Panel: plugin.PanelForm, Source: &plugin.DataSource{RouteID: "x.list"}, Config: map[string]any{"submitRouteId": "x.list"}}}
		}},
		{"action references unknown route", "references unknown route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Actions = []plugin.Action{{ID: "a", Label: "A", RouteID: "ghost"}}
		}},
		{"stream references non-ws route", "non-WS route", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Streams = []plugin.Stream{{ID: "s", Kind: plugin.StreamLogs, RouteID: "x.list"}}
		}},
		{"resource references unknown action", "references unknown action", func(m *plugin.Manifest, _ *[]plugin.Route) {
			m.Resources = []plugin.ResourceType{{Kind: "k", Title: "K", List: plugin.DataSource{RouteID: "x.list"}, ActionIDs: []string{"ghost"}}}
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, routes := base()
			tc.mut(&m, &routes)
			err := plugin.Validate(m, routes)
			if err == nil {
				t.Fatalf("expected validation error containing %q", tc.want)
			}
			if !contains(err.Error(), tc.want) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestValidateAcceptsGoodManifest(t *testing.T) {
	m, routes := sampleManifest()
	if err := plugin.Validate(m, routes); err != nil {
		t.Errorf("valid manifest rejected: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func newRC(params map[string]string, query url.Values, body string) *plugin.RequestContext {
	return plugin.NewRequestContext(context.Background(), models.User{ID: "u1"}, nil, params, query, []byte(body))
}

func TestRequestContextBindTypedNoPanic(t *testing.T) {
	type scaleReq struct {
		Replicas int `json:"replicas" validate:"min=0,max=1000"`
	}

	// JSON number decodes to int — the case that panicked with map[string]any.
	rc := newRC(nil, nil, `{"replicas": 3}`)
	var req scaleReq
	if err := rc.Bind(&req); err != nil {
		t.Fatalf("bind valid: %v", err)
	}
	if req.Replicas != 3 {
		t.Errorf("replicas: want 3, got %d", req.Replicas)
	}

	// Out-of-range fails validation, does not panic.
	rc = newRC(nil, nil, `{"replicas": 99999}`)
	if err := rc.Bind(&scaleReq{}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("bind out-of-range: want ErrInvalidInput, got %v", err)
	}

	// Wrong type (string where int expected) fails cleanly, no panic.
	rc = newRC(nil, nil, `{"replicas": "lots"}`)
	if err := rc.Bind(&scaleReq{}); !errors.Is(err, plugin.ErrInvalidInput) {
		t.Errorf("bind wrong-type: want ErrInvalidInput, got %v", err)
	}

	// Empty body: validation still runs (missing required handled by tags).
	rc = newRC(nil, nil, ``)
	if err := rc.Bind(&scaleReq{}); err != nil {
		t.Errorf("bind empty body: %v", err)
	}
}

func TestRequestContextParamAndPage(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "25")
	q.Set("cursor", "abc")
	q.Set("filter", "web")
	q.Set("filter.state", "running")
	q.Set("sort", "-name")
	rc := newRC(map[string]string{"vmid": "101"}, q, "")

	if rc.Param("vmid") != "101" {
		t.Errorf("param vmid: got %q", rc.Param("vmid"))
	}
	page, err := rc.Page()
	if err != nil {
		t.Fatalf("page: %v", err)
	}
	if page.Limit != 25 || page.Cursor != "abc" {
		t.Errorf("page limit/cursor: %+v", page)
	}
	if page.Filter["q"] != "web" || page.Filter["state"] != "running" {
		t.Errorf("page filter: %+v", page.Filter)
	}
	if len(page.Sort) != 1 || page.Sort[0].Field != "name" || !page.Sort[0].Desc {
		t.Errorf("page sort: %+v", page.Sort)
	}
}

func TestPageLimitClamp(t *testing.T) {
	q := url.Values{}
	q.Set("limit", "100000")
	page, _ := newRC(nil, q, "").Page()
	if page.Limit != plugin.MaxPageLimit {
		t.Errorf("limit clamp: want %d, got %d", plugin.MaxPageLimit, page.Limit)
	}
}
