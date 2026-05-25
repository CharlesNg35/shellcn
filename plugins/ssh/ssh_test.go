package ssh_test

import (
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/ssh"
)

func TestManifestValidates(t *testing.T) {
	p := ssh.New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("ssh manifest invalid: %v", err)
	}
}

func TestManifestExposesTerminalAndFiles(t *testing.T) {
	m := ssh.New().Manifest()
	if len(m.Tabs) != 4 {
		t.Fatalf("tabs: got %d want 4", len(m.Tabs))
	}
	if m.Tabs[0].Panel != plugin.PanelTerminal || m.Tabs[0].Source.RouteID != "ssh.shell" {
		t.Fatalf("terminal tab not wired to ssh.shell: %+v", m.Tabs[0])
	}
	files := m.Tabs[1]
	if files.Panel != plugin.PanelFileBrowser {
		t.Fatalf("files tab panel: got %q", files.Panel)
	}
	for _, key := range []string{"readRouteId", "downloadRouteId", "uploadRouteId", "mkdirRouteId", "renameRouteId", "deleteRouteId"} {
		if files.Config[key] == "" {
			t.Fatalf("files config missing %s", key)
		}
	}
	if len(m.Recording) != 1 || m.Recording[0].Class != plugin.RecordingTerminal {
		t.Fatalf("ssh should declare terminal recording: %+v", m.Recording)
	}
}
