package plugintest

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/pluginux"
)

// ValidatePlugin validates the plugin manifest, route contract, UX contract, and
// projected panel config shapes.
func ValidatePlugin(t testing.TB, p plugin.Plugin) {
	t.Helper()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("validate plugin manifest: %v", err)
	}
	ValidatePluginUX(t, p)
	ValidateProjectionPanelConfigs(t, plugin.BuildProjection(p.Manifest(), RouteMap(p.Routes())))
}

// ValidatePluginUX checks release-blocking generic renderer UX rules.
func ValidatePluginUX(t testing.TB, p plugin.Plugin) {
	t.Helper()
	if findings := pluginux.Errors(pluginux.Lint(p.Manifest(), p.Routes())); len(findings) > 0 {
		for _, finding := range findings {
			t.Errorf("%s: %s", finding.Path, finding.Message)
		}
		t.Fatalf("plugin manifest has UX errors")
	}
}

// Projection validates the plugin and returns its browser projection.
func Projection(t testing.TB, p plugin.Plugin) plugin.Projection {
	t.Helper()
	ValidatePlugin(t, p)
	return plugin.BuildProjection(p.Manifest(), RouteMap(p.Routes()))
}

// RouteMap indexes routes by ID for projection tests.
func RouteMap(routes []plugin.Route) map[string]plugin.Route {
	out := make(map[string]plugin.Route, len(routes))
	for _, route := range routes {
		out[route.ID] = route
	}
	return out
}

// CredentialKindSupported reports whether a schema has any credential_ref field
// that accepts kind.
func CredentialKindSupported(schema plugin.Schema, kind plugin.CredentialKind) bool {
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if fieldCredentialKindSupported(field, kind) {
				return true
			}
		}
	}
	return false
}

func fieldCredentialKindSupported(field plugin.Field, kind plugin.CredentialKind) bool {
	if field.Credential != nil && field.Credential.Kind == kind {
		return true
	}
	for _, child := range field.Fields {
		if fieldCredentialKindSupported(child, kind) {
			return true
		}
	}
	return field.Item != nil && fieldCredentialKindSupported(*field.Item, kind)
}
