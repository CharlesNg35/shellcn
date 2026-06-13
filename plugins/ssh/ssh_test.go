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
	if m.Tabs[0].Type != plugin.PanelTerminalGrid || m.Tabs[0].Source.RouteID != "ssh.shell" {
		t.Fatalf("terminal tab not wired to ssh.shell: %+v", m.Tabs[0])
	}
	if cfg, ok := m.Tabs[0].Config.(plugin.TerminalGridConfig); !ok || cfg.MaxPanes != 6 || !cfg.Zoom || !cfg.Search {
		t.Fatalf("terminal grid config missing split/search/zoom support: %#v", m.Tabs[0].Config)
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

func TestManifestSurfacesHostKeyPinning(t *testing.T) {
	m := ssh.New().Manifest()
	var hostKey plugin.Field
	for _, group := range m.Config.Groups {
		for _, field := range group.Fields {
			if field.Key == "host_key" {
				hostKey = field
			}
		}
	}
	if hostKey.Key == "" {
		t.Fatal("missing host_key field")
	}
	if hostKey.Type != plugin.FieldTextarea || hostKey.Secret || hostKey.Help == "" {
		t.Fatalf("host_key field should be a visible textarea with help: %+v", hostKey)
	}
}
