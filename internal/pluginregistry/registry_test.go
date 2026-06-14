package pluginregistry

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

const testCredentialPrivateKey plugin.CredentialKind = "sample_private_key"

type stubPlugin struct {
	manifest plugin.Manifest
	routes   []plugin.Route
}

func (s *stubPlugin) Manifest() plugin.Manifest { return s.manifest }
func (s *stubPlugin) Routes() []plugin.Route    { return s.routes }
func (s *stubPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return nil, nil
}

func TestRegisterGetAll(t *testing.T) {
	m, routes := sampleManifest()
	reg := New()
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
	} else if s[0].Category.Key != plugin.CategoryShell {
		t.Errorf("Summary category = %+v, want %q", s[0].Category, plugin.CategoryShell)
	}
}

func TestRoutesAreScopedByPluginName(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	reg := New()
	for _, name := range []string{"alpha", "beta"} {
		m := plugin.Manifest{
			APIVersion:          plugin.CurrentAPIVersion,
			Name:                name,
			Title:               strings.ToUpper(name),
			Category:            plugin.CategoryOther,
			Layout:              plugin.LayoutTabs,
			SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		}
		routes := []plugin.Route{{
			ID:         name + ".list",
			Method:     plugin.MethodGet,
			Permission: name + ".read",
			Risk:       plugin.RiskSafe,
			AuditEvent: name + ".list",
			Handle:     noop,
		}}
		if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	if _, ok := reg.Route("alpha", "alpha.list"); !ok {
		t.Fatal("alpha should resolve its own route")
	}
	if _, ok := reg.Route("alpha", "beta.list"); ok {
		t.Fatal("alpha must not resolve beta's route")
	}
	if _, ok := reg.Route("beta", "beta.list"); !ok {
		t.Fatal("beta should resolve its own route")
	}
}

func TestRegisterRejectsRouteOutsidePluginNamespace(t *testing.T) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "alpha",
		Title:               "Alpha",
		Category:            plugin.CategoryOther,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
	err := New().Register(&stubPlugin{
		manifest: m,
		routes: []plugin.Route{{
			ID:         "beta.list",
			Method:     plugin.MethodGet,
			Permission: "alpha.read",
			Risk:       plugin.RiskSafe,
			AuditEvent: "alpha.list",
			Handle:     noop,
		}},
	})
	if err == nil || !contains(err.Error(), "must be namespaced under plugin") {
		t.Fatalf("namespace error = %v", err)
	}
}

func TestSummariesSortByCategory(t *testing.T) {
	m, routes := sampleManifest()
	db := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "aaa-db",
		Title:               "AAA Database",
		Category:            plugin.CategoryDatabases,
		Layout:              plugin.LayoutTabs,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
	reg := New()
	if err := reg.Register(&stubPlugin{manifest: db}); err != nil {
		t.Fatalf("register db: %v", err)
	}
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register shell: %v", err)
	}
	s := reg.Summaries()
	if got := []string{s[0].Name, s[1].Name}; got[0] != "sample" || got[1] != "aaa-db" {
		t.Fatalf("summary order = %v, want [sample aaa-db]", got)
	}
}

func TestDerivesCredentialKindProtocolsFromSelectors(t *testing.T) {
	m, routes := sampleManifest()
	reg := New()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}
	info, ok := reg.CredentialKindLookup(testCredentialPrivateKey)
	if !ok {
		t.Fatal("ssh private key kind not registered")
	}
	if len(info.CompatibleProtocols) != 1 || info.CompatibleProtocols[0] != "ssh" {
		t.Fatalf("derived protocols = %+v, want [ssh]", info.CompatibleProtocols)
	}
	if !reg.CredentialKindSupportsProtocol(testCredentialPrivateKey, "ssh") {
		t.Fatal("ssh private key should support ssh")
	}
	if reg.CredentialKindSupportsProtocol(testCredentialPrivateKey, "postgres") {
		t.Fatal("ssh private key should not support postgres")
	}
}

func TestBuiltInCredentialKindsCanBeUsedByMultiplePlugins(t *testing.T) {
	reg := New()
	for _, protocol := range []string{"ssh", "sftp"} {
		m, routes := protocolCredentialManifest(protocol, []plugin.CredentialKind{
			plugin.CredentialSSHPrivateKey,
			plugin.CredentialSSHPassword,
		})
		if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
			t.Fatalf("register %s: %v", protocol, err)
		}
	}
	for _, kind := range []plugin.CredentialKind{plugin.CredentialSSHPrivateKey, plugin.CredentialSSHPassword} {
		info, ok := reg.CredentialKindLookup(kind)
		if !ok {
			t.Fatalf("credential kind %q was not registered", kind)
		}
		if got := strings.Join(info.CompatibleProtocols, ","); got != "sftp,ssh" {
			t.Fatalf("%s protocols = %q, want sftp,ssh", kind, got)
		}
	}
}

func TestUnregisterRecomputesCredentialCatalog(t *testing.T) {
	reg := New()
	for _, protocol := range []string{"ssh", "sftp"} {
		m, routes := protocolCredentialManifest(protocol, []plugin.CredentialKind{plugin.CredentialSSHPassword})
		if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
			t.Fatalf("register %s: %v", protocol, err)
		}
	}
	m, routes := sampleManifest()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register sample: %v", err)
	}

	if err := reg.Unregister("sftp"); err != nil {
		t.Fatalf("unregister sftp: %v", err)
	}
	if reg.CredentialKindSupportsProtocol(plugin.CredentialSSHPassword, "sftp") {
		t.Fatal("ssh password should no longer support sftp after unregister")
	}
	if !reg.CredentialKindSupportsProtocol(plugin.CredentialSSHPassword, "ssh") {
		t.Fatal("ssh password should still support ssh")
	}

	if err := reg.Unregister("sample"); err != nil {
		t.Fatalf("unregister sample: %v", err)
	}
	if _, ok := reg.CredentialKindLookup(testCredentialPrivateKey); ok {
		t.Fatal("custom credential kind should be removed with its plugin")
	}
}

func TestRejectsDuplicatePluginCredentialKind(t *testing.T) {
	m, routes := sampleManifest()
	reg := New()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register first plugin: %v", err)
	}
	dup := m
	dup.Name = "duplicate"
	for i := range routes {
		routes[i].ID = "duplicate." + routes[i].ID
	}
	if err := reg.Register(&stubPlugin{manifest: dup, routes: routes}); err == nil || !contains(err.Error(), "duplicate credential kind") {
		t.Fatalf("duplicate credential kind error = %v", err)
	}
}

func TestRegisterRejectsUXContractErrors(t *testing.T) {
	m, routes := sampleManifest()
	m.Actions = []plugin.Action{{
		ID:      "sample.open",
		Label:   "Open",
		Icon:    plugin.Icon{Type: plugin.IconLucide, Value: "external-link"},
		RouteID: "sample.open",
		Open:    plugin.OpenURL,
	}}
	routes = append(routes, plugin.Route{
		ID:         "sample.open",
		Method:     plugin.MethodGet,
		Permission: "sample.open",
		Risk:       plugin.RiskSafe,
		AuditEvent: "sample.open",
		Input:      &plugin.Schema{Groups: []plugin.Group{{Name: "Target", Fields: []plugin.Field{{Key: "port", Label: "Port", Type: plugin.FieldSelect, Required: true}}}}},
		Handle:     func(_ *plugin.RequestContext) (any, error) { return nil, nil },
	})

	err := New().Register(&stubPlugin{manifest: m, routes: routes})
	if err == nil || !contains(err.Error(), "UX contract") || !contains(err.Error(), "OpenURL action input fields are submitted as route params") {
		t.Fatalf("register UX error = %v", err)
	}
}

func TestReplace(t *testing.T) {
	m, routes := sampleManifest()
	reg := New()

	if err := reg.Replace(&stubPlugin{manifest: m, routes: routes}); !errors.Is(err, plugin.ErrNotFound) {
		t.Fatalf("replace before register: want ErrNotFound, got %v", err)
	}
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}

	updated := m
	updated.Version = "9.9.9"
	if err := reg.Replace(&stubPlugin{manifest: updated, routes: routes}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, ok := reg.Manifest(m.Name)
	if !ok || got.Version != "9.9.9" {
		t.Fatalf("manifest after replace: %+v %v", got, ok)
	}
}

func TestReplaceKeepsOwnCredentialKinds(t *testing.T) {
	m, routes := sampleManifest()
	m.CredentialKinds = []plugin.CredentialKindInfo{{
		Kind: "sample_token", Label: "Sample token",
		Fields: []plugin.Field{plugin.CredentialSecretField(plugin.Field{Key: "token", Label: "Token", Type: plugin.FieldPassword, Required: true})},
	}}
	m.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Auth", Fields: []plugin.Field{{
		Key: "credential", Label: "Credential", Type: plugin.FieldCredentialRef,
		Credential: &plugin.CredentialSelector{Kind: "sample_token"},
	}}}}}
	reg := New()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := reg.Replace(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("replace with own kind: %v", err)
	}
	if _, ok := reg.CredentialKindLookup("sample_token"); !ok {
		t.Fatal("own credential kind must survive the replace")
	}
}

func TestReplaceRecomputesCredentialProtocols(t *testing.T) {
	m, routes := sampleManifest()
	reg := New()
	if err := reg.Register(&stubPlugin{manifest: m, routes: routes}); err != nil {
		t.Fatalf("register: %v", err)
	}

	updated := m
	updated.Config = plugin.Schema{Groups: []plugin.Group{{Name: "Basic", Fields: []plugin.Field{{
		Key: "credential_id", Label: "Credential", Type: plugin.FieldCredentialRef,
		Credential: &plugin.CredentialSelector{Kind: testCredentialPrivateKey, Protocols: []string{"sftp"}},
	}}}}}
	if err := reg.Replace(&stubPlugin{manifest: updated, routes: routes}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if reg.CredentialKindSupportsProtocol(testCredentialPrivateKey, "ssh") {
		t.Fatal("replaced credential selector should no longer support ssh")
	}
	if !reg.CredentialKindSupportsProtocol(testCredentialPrivateKey, "sftp") {
		t.Fatal("replaced credential selector should support sftp")
	}
}

func sampleManifest() (plugin.Manifest, []plugin.Route) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                "sample",
		Version:             "0.1.0",
		Title:               "Sample",
		Description:         "A representative plugin.",
		Icon:                plugin.Icon{Type: plugin.IconLucide, Value: "terminal"},
		Category:            plugin.CategoryShell,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		CredentialKinds: []plugin.CredentialKindInfo{{
			Kind: testCredentialPrivateKey, Label: "Sample private key",
			Fields: []plugin.Field{
				plugin.CredentialPublicField(plugin.Field{Key: "username", Label: "Username", Type: plugin.FieldText, Required: true}),
				plugin.CredentialSecretField(plugin.Field{Key: "private_key", Label: "Private key", Type: plugin.FieldTextarea, Required: true}),
			},
		}},
		Layout: plugin.LayoutTabs,
		Config: plugin.Schema{Groups: []plugin.Group{{Name: "Basic", Fields: []plugin.Field{{
			Key: "credential_id", Label: "Credential", Type: plugin.FieldCredentialRef,
			Credential: &plugin.CredentialSelector{Kind: testCredentialPrivateKey, Protocols: []string{"ssh"}},
		}}}}},
	}
	routes := []plugin.Route{
		{ID: "sample.list", Method: plugin.MethodGet, Permission: "sample.read", Risk: plugin.RiskSafe, AuditEvent: "sample.list", Handle: noop},
		{ID: "sample.start", Method: plugin.MethodPost, Permission: "sample.start", Risk: plugin.RiskWrite, AuditEvent: "sample.start", Handle: noop},
	}
	return m, routes
}

func protocolCredentialManifest(name string, kinds []plugin.CredentialKind) (plugin.Manifest, []plugin.Route) {
	noop := func(_ *plugin.RequestContext) (any, error) { return nil, nil }
	fields := make([]plugin.Field, 0, len(kinds))
	for _, kind := range kinds {
		fields = append(fields, plugin.Field{
			Key: string(kind) + "_credential_id", Label: "Credential", Type: plugin.FieldCredentialRef,
			Credential: &plugin.CredentialSelector{Kind: kind, Protocols: []string{name}},
		})
	}
	m := plugin.Manifest{
		APIVersion:          plugin.CurrentAPIVersion,
		Name:                name,
		Title:               strings.ToUpper(name),
		Category:            plugin.CategoryShell,
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
		Layout:              plugin.LayoutTabs,
		Config:              plugin.Schema{Groups: []plugin.Group{{Name: "Auth", Fields: fields}}},
	}
	routes := []plugin.Route{{
		ID: name + ".list", Method: plugin.MethodGet, Permission: name + ".read",
		Risk: plugin.RiskSafe, AuditEvent: name + ".list", Handle: noop,
	}}
	return m, routes
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
