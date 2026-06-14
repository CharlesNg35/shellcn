package ssh_test

import (
	"testing"

	"github.com/charlesng35/shellcn/plugins/ssh"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestManifestValidates(t *testing.T) {
	p := ssh.New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("ssh manifest invalid: %v", err)
	}
}

func TestManifestExposesTerminalAndFiles(t *testing.T) {
	m := ssh.New().Manifest()
	if len(m.Tabs) != 3 {
		t.Fatalf("tabs: got %d want 3", len(m.Tabs))
	}
	if m.Tabs[0].Key != "terminal" || m.Tabs[0].Type != plugin.PanelTerminal || m.Tabs[0].Source.RouteID != "ssh.shell" {
		t.Fatalf("terminal tab not wired to ssh.shell: %+v", m.Tabs[0])
	}
	if cfg, ok := m.Tabs[0].Config.(plugin.TerminalConfig); !ok || !cfg.Zoom || !cfg.Search {
		t.Fatalf("terminal config missing search/zoom support: %#v", m.Tabs[0].Config)
	}
	if len(m.Tabs[0].Variants) != 1 || m.Tabs[0].Variants[0].Type != plugin.PanelTerminalGrid || m.Tabs[0].Variants[0].VisibleWhen == nil {
		t.Fatalf("terminal grid variant not conditionally wired: %+v", m.Tabs[0].Variants)
	}
	if cfg, ok := m.Tabs[0].Variants[0].Config.(plugin.TerminalGridConfig); !ok || cfg.MaxPanes != 6 || !cfg.Zoom || !cfg.Search {
		t.Fatalf("terminal grid config missing split/search/zoom support: %#v", m.Tabs[0].Variants[0].Config)
	}
	files := m.Tabs[1]
	if files.Type != plugin.PanelFileBrowser {
		t.Fatalf("files tab panel: got %q", files.Type)
	}
	fb, ok := files.Config.(plugin.FileBrowserConfig)
	if !ok || fb.ReadRouteID == "" || fb.DownloadRouteID == "" || fb.UploadRouteID == "" ||
		fb.MkdirRouteID == "" || fb.RenameRouteID == "" || fb.DeleteRouteID == "" ||
		fb.MoveRouteID == "" || fb.CopyRouteID == "" || fb.ChmodRouteID == "" || fb.ArchiveRouteID == "" {
		t.Fatalf("files config missing route ids: %#v", files.Config)
	}
	if len(m.Recording) != 1 || m.Recording[0].Class != plugin.RecordingTerminal {
		t.Fatalf("ssh should declare terminal recording: %+v", m.Recording)
	}
	if m.Tabs[2].Type != plugin.PanelTable || m.Tabs[2].Source.RouteID != "ssh.snippet.list" {
		t.Fatalf("snippets tab not wired to table/list route: %+v", m.Tabs[2])
	}
	if cfg, ok := m.Tabs[2].Config.(plugin.TableConfig); !ok || cfg.EmptyText == "" {
		t.Fatalf("snippets table should declare an empty state: %#v", m.Tabs[2].Config)
	}
	for _, route := range ssh.New().Routes() {
		if route.ID == "ssh.tunnel.list" || route.ID == "ssh.tunnel.open" || route.ID == "ssh.tunnel.close" {
			t.Fatalf("ssh should not expose browser-local tunnel route %q", route.ID)
		}
	}
}

func TestManifestRunsSnippetsInVisibleTerminal(t *testing.T) {
	m := ssh.New().Manifest()
	var run plugin.Action
	for _, action := range m.Actions {
		if action.ID == "ssh.snippet.run" {
			run = action
			break
		}
	}
	if run.Params["id"] != "${record.id}" {
		t.Fatalf("snippet run should use row data, not a fake resource ref: %+v", run.Params)
	}
	if run.OnSuccess == nil || run.OnSuccess.SelectTab != "terminal" || len(run.OnSuccess.Effects) != 1 {
		t.Fatalf("snippet run should target the terminal on success: %+v", run.OnSuccess)
	}
	effect := run.OnSuccess.Effects[0]
	if effect.Type != plugin.ActionEffectTerminalInput || effect.TerminalInput == nil {
		t.Fatalf("snippet run effect = %+v", effect)
	}
	input := effect.TerminalInput
	if input.Tab != "terminal" || input.ResultField != "command" || !input.AppendNewline {
		t.Fatalf("snippet terminal input = %+v", input)
	}
}

func TestManifestSurfacesHostKeyVerification(t *testing.T) {
	m := ssh.New().Manifest()
	policy := requireField(t, m.Config, "host_key_verification")
	if policy.Type != plugin.FieldSelect || policy.Default != "pinned" || len(policy.Options) != 2 {
		t.Fatalf("host key verification should be an explicit select: %+v", policy)
	}
	if policy.Options[0].Value != "pinned" || policy.Options[1].Value != "insecure" {
		t.Fatalf("host key verification options should prefer pinned verification: %+v", policy.Options)
	}
	hostKey := requireField(t, m.Config, "host_key")
	if hostKey.Type != plugin.FieldTextarea || hostKey.Secret || hostKey.Help == "" || hostKey.VisibleWhen == nil {
		t.Fatalf("host_key field should be a conditional visible textarea with help: %+v", hostKey)
	}
}

func TestManifestSurfacesTerminalLayout(t *testing.T) {
	m := ssh.New().Manifest()
	layout := requireField(t, m.Config, "terminal_layout")
	if layout.Type != plugin.FieldSelect || layout.Default != "single" || len(layout.Options) != 2 {
		t.Fatalf("terminal layout should be an explicit select: %+v", layout)
	}
	if layout.Options[0].Value != "single" || layout.Options[1].Value != "grid" {
		t.Fatalf("terminal layout options should prefer single terminal: %+v", layout.Options)
	}
}

func requireField(t *testing.T, schema plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("missing field %q", key)
	return plugin.Field{}
}
