package kubernetes

import (
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func TestParseLsOutput(t *testing.T) {
	out := "total 20\n" +
		"drwxr-xr-x    2 root     root          4096 Jun 19 12:00 bin\n" +
		"-rw-r--r--    1 root     root           123 Jun 19 12:00 hello.txt\n" +
		"lrwxrwxrwx    1 root     root             7 Jun 19 12:00 link -> bin/sh\n" +
		"drwxr-xr-x    1 root     root          4096 Jun 19 12:00 .\n" +
		"drwxr-xr-x    1 root     root          4096 Jun 19 12:00 ..\n"
	items := parseLsOutput("/data", out)
	if len(items) != 3 {
		t.Fatalf("got %d entries, want 3: %+v", len(items), items)
	}
	byName := map[string]int{}
	for i, e := range items {
		byName[e.Name] = i
	}
	bin := items[byName["bin"]]
	if !bin.IsDir || bin.Size != 4096 || bin.Path != "/data/bin" {
		t.Fatalf("bin entry = %+v", bin)
	}
	file := items[byName["hello.txt"]]
	if file.IsDir || file.Size != 123 || file.Path != "/data/hello.txt" {
		t.Fatalf("hello.txt entry = %+v", file)
	}
	if link, ok := byName["link"]; !ok || items[link].Path != "/data/link" {
		t.Fatalf("symlink target should be stripped from name: %+v", items)
	}
}

func TestCleanFileName(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", "a\\b", "  "} {
		if _, err := cleanFileName(bad); err == nil {
			t.Errorf("cleanFileName(%q) should fail", bad)
		}
	}
	if got, err := cleanFileName("ok.txt"); err != nil || got != "ok.txt" {
		t.Fatalf("cleanFileName(ok.txt) = %q, %v", got, err)
	}
}

func TestPodFilesWired(t *testing.T) {
	want := map[string]plugin.Method{
		"kubernetes.pod.files.list":     plugin.MethodGet,
		"kubernetes.pod.files.read":     plugin.MethodGet,
		"kubernetes.pod.files.download": plugin.MethodGet,
		"kubernetes.pod.files.write":    plugin.MethodPut,
		"kubernetes.pod.files.upload":   plugin.MethodPost,
		"kubernetes.pod.files.mkdir":    plugin.MethodPost,
		"kubernetes.pod.files.delete":   plugin.MethodDelete,
	}
	for _, r := range Routes() {
		if m, ok := want[r.ID]; ok {
			if r.Method != m || r.Handle == nil {
				t.Errorf("route %s = %s (handler nil=%v)", r.ID, r.Method, r.Handle == nil)
			}
			delete(want, r.ID)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing pod file routes: %v", want)
	}

	tab := podFilesTab()
	cfg, ok := tab.Config.(plugin.FileBrowserConfig)
	if !ok || tab.Type != plugin.PanelFileBrowser {
		t.Fatalf("files tab = %+v", tab)
	}
	if cfg.Routes.Read != "kubernetes.pod.files.read" || cfg.Upload.RouteID != "kubernetes.pod.files.upload" {
		t.Fatalf("files config = %+v", cfg)
	}
	if tab.Source == nil || tab.Source.Params["name"] == "" {
		t.Fatalf("files tab must carry pod identity params: %+v", tab.Source)
	}
}
