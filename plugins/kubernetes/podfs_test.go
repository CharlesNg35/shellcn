package kubernetes

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

func sizeProbe(size int, body []byte) []byte {
	out := append([]byte(strconv.Itoa(size)), '\n')
	return append(out, body...)
}

func TestPodFileContent(t *testing.T) {
	t.Run("text reports real size from probe", func(t *testing.T) {
		c := podFileContent("/app/notes.txt", sizeProbe(11, []byte("hello world")))
		if c.Encoding != "utf8" || c.Content != "hello world" || c.Truncated {
			t.Fatalf("content = %+v", c)
		}
		if c.Size != 11 || c.MIME == "" {
			t.Fatalf("size/mime = %d %q", c.Size, c.MIME)
		}
	})

	t.Run("large text is truncated but keeps the real size", func(t *testing.T) {
		body := bytes.Repeat([]byte("a"), podFileReadLimit+1)
		c := podFileContent("/var/log/app.log", sizeProbe(5<<20, body))
		if !c.Truncated || len(c.Content) != podFileReadLimit {
			t.Fatalf("truncated=%v len=%d", c.Truncated, len(c.Content))
		}
		if c.Size != 5<<20 {
			t.Fatalf("size = %d, want real file size not preview length", c.Size)
		}
	})

	t.Run("multibyte rune split at the cap stays valid utf8", func(t *testing.T) {
		body := append(bytes.Repeat([]byte("a"), podFileReadLimit-1), []byte("€")...) // € is 3 bytes
		c := podFileContent("/app/notes.txt", sizeProbe(podFileReadLimit+4, body))
		if c.Encoding != "utf8" {
			t.Fatalf("a text file split mid-rune must stay text: %+v", c)
		}
		if !c.Truncated || len(c.Content) != podFileReadLimit-1 {
			t.Fatalf("partial trailing rune not trimmed: len=%d", len(c.Content))
		}
	})

	t.Run("binary is classified without content", func(t *testing.T) {
		c := podFileContent("/bin/tool", sizeProbe(4, []byte{0x00, 0x01, 0x02, 0xff}))
		if c.Encoding != "binary" || c.Content != "" {
			t.Fatalf("binary content = %+v", c)
		}
	})

	t.Run("missing size probe falls back to read length", func(t *testing.T) {
		c := podFileContent("/app/x", []byte("no-newline-no-size"))
		if c.Size != int64(len("no-newline-no-size")) {
			t.Fatalf("fallback size = %d", c.Size)
		}
	})
}

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
