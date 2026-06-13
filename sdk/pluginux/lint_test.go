package pluginux_test

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/pluginux"
)

func noop(_ *plugin.RequestContext) (any, error) { return nil, nil }

func stream(_ *plugin.RequestContext, _ plugin.ClientStream) error { return nil }

func hasError(findings []pluginux.Finding, message string) bool {
	for _, finding := range pluginux.Errors(findings) {
		if finding.Message == message {
			return true
		}
	}
	return false
}

func TestLintRejectsPrivilegedActionWithoutConfirm(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Actions: []plugin.Action{{
			ID:      "x.shell",
			Label:   "Shell",
			RouteID: "x.shell",
		}},
	}
	routes := []plugin.Route{{
		ID: "x.shell", Method: plugin.MethodPost, Permission: "x.shell",
		Risk: plugin.RiskPrivileged, Handle: noop,
	}}
	if !hasError(pluginux.Lint(m, routes), "privileged action must require confirmation") {
		t.Fatalf("expected privileged action confirmation error")
	}
}

func TestLintRejectsRequiredInputOnOpenURLAction(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Actions: []plugin.Action{{
			ID:      "x.open",
			Label:   "Open",
			RouteID: "x.open",
			Open:    plugin.OpenURL,
		}},
	}
	routes := []plugin.Route{{
		ID: "x.open", Method: plugin.MethodGet, Permission: "x.read",
		Risk: plugin.RiskSafe, Handle: noop,
		Input: &plugin.Schema{Groups: []plugin.Group{{
			Name:   "Open",
			Fields: []plugin.Field{{Key: "port", Label: "Port", Type: plugin.FieldSelect, Required: true}},
		}}},
	}}
	if !hasError(pluginux.Lint(m, routes), "OpenURL action input fields are submitted as route params; required body fields would fail core validation") {
		t.Fatalf("expected OpenURL required input error")
	}
}

func TestLintRequiresStreamDeclarationAndMatchingKind(t *testing.T) {
	panel := plugin.Panel{
		Key:    "shell",
		Label:  "Shell",
		Type:   plugin.PanelTerminal,
		Source: &plugin.DataSource{RouteID: "x.shell", Method: plugin.MethodWS},
	}
	base := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs:       []plugin.Panel{panel},
	}
	routes := []plugin.Route{{
		ID: "x.shell", Method: plugin.MethodWS, Permission: "x.shell",
		Risk: plugin.RiskPrivileged, Stream: stream,
	}}
	if !hasError(pluginux.Lint(base, routes), `stream panel route "x.shell" is not declared in manifest streams`) {
		t.Fatalf("expected undeclared stream error")
	}
	base.Streams = []plugin.Stream{{ID: "x.shell", Kind: plugin.StreamLogs, RouteID: "x.shell"}}
	if !hasError(pluginux.Lint(base, routes), `stream route "x.shell" is "logs" but panel "terminal" requires "terminal"`) {
		t.Fatalf("expected stream kind mismatch error")
	}
}

func TestLintAcceptsTaskProgressTaskStream(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:    "task",
			Label:  "Task",
			Type:   plugin.PanelTaskProgress,
			Source: &plugin.DataSource{RouteID: "x.task", Method: plugin.MethodWS},
		}},
		Streams: []plugin.Stream{{ID: "x.task", Kind: plugin.StreamTask, RouteID: "x.task"}},
	}
	routes := []plugin.Route{{
		ID: "x.task", Method: plugin.MethodWS, Permission: "x.task",
		Risk: plugin.RiskSafe, Stream: stream,
	}}
	if findings := pluginux.Errors(pluginux.Lint(m, routes)); len(findings) != 0 {
		t.Fatalf("unexpected UX errors: %#v", findings)
	}
}

func TestLintAcceptsCanvasStream(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:    "canvas",
			Label:  "Canvas",
			Type:   plugin.PanelCanvas,
			Source: &plugin.DataSource{RouteID: "x.canvas", Method: plugin.MethodWS},
			Config: plugin.CanvasConfig{Interactive: true, Keyboard: true, Pointer: true},
		}},
		Streams: []plugin.Stream{{ID: "x.canvas", Kind: plugin.StreamCanvas, RouteID: "x.canvas"}},
	}
	routes := []plugin.Route{{
		ID: "x.canvas", Method: plugin.MethodWS, Permission: "x.canvas",
		Risk: plugin.RiskSafe, Stream: stream,
	}}
	if findings := pluginux.Errors(pluginux.Lint(m, routes)); len(findings) != 0 {
		t.Fatalf("unexpected UX errors: %#v", findings)
	}
}

func TestLintRejectsFitCanvasWithoutLogicalSize(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:    "canvas",
			Label:  "Canvas",
			Type:   plugin.PanelCanvas,
			Source: &plugin.DataSource{RouteID: "x.canvas", Method: plugin.MethodWS},
			Config: plugin.CanvasConfig{ScaleMode: plugin.CanvasScaleFit, Interactive: true},
		}},
		Streams: []plugin.Stream{{ID: "x.canvas", Kind: plugin.StreamCanvas, RouteID: "x.canvas"}},
	}
	routes := []plugin.Route{{
		ID: "x.canvas", Method: plugin.MethodWS, Permission: "x.canvas",
		Risk: plugin.RiskSafe, Stream: stream,
	}}
	if !hasError(pluginux.Lint(m, routes), `canvas "fit" scale mode requires positive width and height`) {
		t.Fatalf("expected fit canvas size error")
	}
}

func TestLintRejectsPartialWasmDimensions(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:  "wasm",
			Type: plugin.PanelWasm,
			Config: plugin.WasmConfig{
				Entry:     "app.wasm",
				Width:     1280,
				ScaleMode: plugin.WasmScaleFit,
				Assets: []plugin.WasmAsset{{
					Path:   "app.wasm",
					Source: plugin.DataSource{RouteID: "x.asset"},
				}},
			},
		}},
	}
	routes := []plugin.Route{{
		ID: "x.asset", Method: plugin.MethodGet, Permission: "x.read",
		Risk: plugin.RiskSafe, Handle: noop,
	}}
	if !hasError(pluginux.Lint(m, routes), "wasm width and height must be declared together") {
		t.Fatalf("expected wasm dimension error")
	}
}

func TestLintRejectsWasmFitWithoutDimensions(t *testing.T) {
	m := plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion,
		Name:       "x",
		Title:      "X",
		Category:   plugin.CategoryOther,
		Layout:     plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:  "wasm",
			Type: plugin.PanelWasm,
			Config: plugin.WasmConfig{
				Entry:     "app.wasm",
				ScaleMode: plugin.WasmScaleFit,
				Assets: []plugin.WasmAsset{{
					Path:   "app.wasm",
					Source: plugin.DataSource{RouteID: "x.asset"},
				}},
			},
		}},
	}
	routes := []plugin.Route{{
		ID: "x.asset", Method: plugin.MethodGet, Permission: "x.read",
		Risk: plugin.RiskSafe, Handle: noop,
	}}
	if !hasError(pluginux.Lint(m, routes), "wasm scaleMode fit requires width and height") {
		t.Fatalf("expected wasm fit dimension error")
	}
}
