package kubernetes

import (
	"bytes"
	"io"
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

	t.Run("extensionless image sniffs to an image mime as binary", func(t *testing.T) {
		png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}
		c := podFileContent("/app/logo", sizeProbe(len(png), png))
		if c.MIME != "image/png" || c.Encoding != "binary" || c.Content != "" {
			t.Fatalf("extensionless image = %+v", c)
		}
	})

	t.Run("extensionless script sniffs to text and inlines content", func(t *testing.T) {
		body := []byte("#!/bin/sh\necho hi\n")
		c := podFileContent("/usr/local/bin/run", sizeProbe(len(body), body))
		if c.Encoding != "utf8" || c.Content != string(body) {
			t.Fatalf("extensionless script = %+v", c)
		}
		if c.MIME == "" || c.MIME == "application/octet-stream" {
			t.Fatalf("extensionless text should carry a text mime, got %q", c.MIME)
		}
	})
}

func TestPodPath(t *testing.T) {
	cases := map[string]string{
		"":                 "/",
		".":                "/",
		"  ":               "/",
		"/":                "/",
		"app/data":         "/app/data",
		"/app/data":        "/app/data",
		"/app//data/":      "/app/data",
		"/app/../etc/motd": "/etc/motd",
		"/app/./sub":       "/app/sub",
	}
	for in, want := range cases {
		if got := podPath(in); got != want {
			t.Errorf("podPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPodDownloadSniffsExtensionlessMedia(t *testing.T) {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00}
	dl := podDownload("/app/logo", true, io.NopCloser(bytes.NewReader(png)))
	if dl.MIME != "image/png" {
		t.Fatalf("extensionless download MIME = %q, want image/png", dl.MIME)
	}
	if body, _ := io.ReadAll(dl.Body); !bytes.Equal(body, png) {
		t.Fatalf("sniffed download dropped bytes: got %d, want %d", len(body), len(png))
	}
}

func TestParseLsOutput(t *testing.T) {
	out := "total 20\n" +
		"drwxr-xr-x    2 root     root          4096 Jun 19 12:00 bin\n" +
		"-rw-r--r--    1 root     root           123 Jun 19 12:00 hello.txt\n" +
		"-rw-r--r--    1 root     root           456 Jun 19 12:00 photo.png\n" +
		"-rw-r--r--    1 root     root             0 Jun 19 12:00 a -> b.txt\n" +
		"lrwxrwxrwx    1 root     root             7 Jun 19 12:00 link -> bin/sh\n" +
		"drwxr-xr-x    1 root     root          4096 Jun 19 12:00 .\n" +
		"drwxr-xr-x    1 root     root          4096 Jun 19 12:00 ..\n"
	items := parseLsOutput("/data", out)
	if len(items) != 5 {
		t.Fatalf("got %d entries, want 5: %+v", len(items), items)
	}
	byName := map[string]int{}
	for i, e := range items {
		byName[e.Name] = i
	}
	bin := items[byName["bin"]]
	if !bin.IsDir || bin.Size != 4096 || bin.Path != "/data/bin" || bin.MIME != "" {
		t.Fatalf("bin entry = %+v", bin)
	}
	file := items[byName["hello.txt"]]
	if file.IsDir || file.Size != 123 || file.Path != "/data/hello.txt" {
		t.Fatalf("hello.txt entry = %+v", file)
	}
	png := items[byName["photo.png"]]
	if png.MIME != "image/png" || png.Size != 456 {
		t.Fatalf("png entry should carry an image mime and real size: %+v", png)
	}
	link := items[byName["link"]]
	if link.Path != "/data/link" || link.Symlink != "bin/sh" {
		t.Fatalf("symlink target should be captured, not folded into the name: %+v", link)
	}
	// A regular file whose name literally contains " -> " must not be truncated;
	// only symlink (mode l...) lines carry a target.
	if _, ok := byName["a -> b.txt"]; !ok {
		t.Fatalf("non-symlink name containing ' -> ' was mangled: %+v", items)
	}
}

func TestPodDownload(t *testing.T) {
	dl := podDownload("/app/assets/photo.png", true, nil)
	if dl.Size != -1 {
		t.Fatalf("streamed download must report Size=-1 (unknown), got %d; a 0 becomes Content-Length: 0 and truncates the body", dl.Size)
	}
	if dl.MIME != "image/png" {
		t.Fatalf("mime = %q, want image/png", dl.MIME)
	}
	if !dl.Inline || dl.Name != "photo.png" {
		t.Fatalf("download = %+v", dl)
	}
	got := podDownload("/tmp/unknownext.zzz", false, io.NopCloser(bytes.NewReader([]byte{0x00, 0x01, 0x02, 0xff})))
	if got.MIME != "application/octet-stream" {
		t.Fatalf("unknown extension mime = %q, want octet-stream", got.MIME)
	}
	if body, _ := io.ReadAll(got.Body); !bytes.Equal(body, []byte{0x00, 0x01, 0x02, 0xff}) {
		t.Fatalf("unknown extension download dropped bytes: %v", body)
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
