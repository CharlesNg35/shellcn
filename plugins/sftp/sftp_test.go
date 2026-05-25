package sftp_test

import (
	"testing"

	"github.com/charlesng/shellcn/internal/plugin"
	"github.com/charlesng/shellcn/plugins/sftp"
)

func TestManifestValidates(t *testing.T) {
	p := sftp.New()
	if err := plugin.Validate(p.Manifest(), p.Routes()); err != nil {
		t.Fatalf("sftp manifest invalid: %v", err)
	}
}

func TestManifestIsFileOnly(t *testing.T) {
	m := sftp.New().Manifest()
	if len(m.Tabs) != 1 || m.Tabs[0].Panel != plugin.PanelFileBrowser {
		t.Fatalf("sftp should expose only file_browser tab: %+v", m.Tabs)
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
