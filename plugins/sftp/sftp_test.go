package sftp_test

import (
	"testing"

	"github.com/charlesng35/shellcn/plugins/sftp"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

func TestManifestValidates(t *testing.T) {
	plugintest.ValidatePlugin(t, sftp.New())
}

func TestManifestIsFileOnly(t *testing.T) {
	m := sftp.New().Manifest()
	if m.Icon.Value == "folder" {
		t.Fatal("sftp connection icon must not reuse the folder glyph")
	}
	if len(m.Tabs) != 1 || m.Tabs[0].Type != plugin.PanelFileBrowser {
		t.Fatalf("sftp should expose only file_browser tab: %+v", m.Tabs)
	}
	cfg, ok := m.Tabs[0].Config.(plugin.FileBrowserConfig)
	if !ok || cfg.MoveRouteID == "" || cfg.CopyRouteID == "" || cfg.ChmodRouteID == "" || cfg.ArchiveRouteID == "" {
		t.Fatalf("sftp file browser missing bulk affordances: %#v", m.Tabs[0].Config)
	}
	if len(m.Streams) != 0 || len(m.Recording) != 0 {
		t.Fatalf("sftp must not expose terminal streams/recording: streams=%+v recording=%+v", m.Streams, m.Recording)
	}
	for _, route := range sftp.New().Routes() {
		if route.ID == "sftp.shell" {
			t.Fatal("file-only sftp plugin exposed shell route")
		}
	}
}

func TestManifestSurfacesHostKeyPinning(t *testing.T) {
	m := sftp.New().Manifest()
	for _, group := range m.Config.Groups {
		for _, field := range group.Fields {
			if field.Key == "host_key" {
				if field.Type != plugin.FieldTextarea || field.Secret || field.Help == "" {
					t.Fatalf("host_key field should be a visible textarea with help: %+v", field)
				}
				return
			}
		}
	}
	t.Fatal("missing host_key field")
}
