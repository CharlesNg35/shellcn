package filesystem

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// memFS is an in-memory filesystem.Client used to exercise the shared file
// browser handlers end to end without a live remote server.
type memFS struct {
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS() *memFS {
	return &memFS{files: map[string][]byte{}, dirs: map[string]bool{"/": true}}
}

func (m *memFS) Filesystem() (Client, error) { return m, nil }

func (m *memFS) HealthCheck(context.Context) error { return nil }
func (m *memFS) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}
func (m *memFS) Close() error { return nil }

func (m *memFS) Home(context.Context) (string, error) { return "/", nil }

func (m *memFS) ReadDir(_ context.Context, p string) ([]os.FileInfo, error) {
	if !m.dirs[p] {
		return nil, os.ErrNotExist
	}
	seen := map[string]os.FileInfo{}
	for f := range m.files {
		if parent := path.Dir(f); parent == p {
			seen[path.Base(f)] = memInfo{name: path.Base(f), size: int64(len(m.files[f]))}
		}
	}
	for d := range m.dirs {
		if d == "/" {
			continue
		}
		if parent := path.Dir(d); parent == p {
			seen[path.Base(d)] = memInfo{name: path.Base(d), dir: true}
		}
	}
	out := make([]os.FileInfo, 0, len(seen))
	for _, info := range seen {
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out, nil
}

func (m *memFS) Stat(_ context.Context, p string) (os.FileInfo, error) {
	if m.dirs[p] {
		return memInfo{name: path.Base(p), dir: true}, nil
	}
	if data, ok := m.files[p]; ok {
		return memInfo{name: path.Base(p), size: int64(len(data))}, nil
	}
	return nil, os.ErrNotExist
}

func (m *memFS) Open(_ context.Context, p string) (io.ReadCloser, error) {
	data, ok := m.files[p]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (m *memFS) Write(_ context.Context, p string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	m.files[p] = data
	return nil
}

func (m *memFS) Mkdir(_ context.Context, p string) error {
	m.dirs[p] = true
	return nil
}

func (m *memFS) Rename(_ context.Context, from, to string) error {
	if data, ok := m.files[from]; ok {
		m.files[to] = data
		delete(m.files, from)
		return nil
	}
	if m.dirs[from] {
		m.dirs[to] = true
		delete(m.dirs, from)
		return nil
	}
	return os.ErrNotExist
}

func (m *memFS) Remove(_ context.Context, p string, isDir bool) error {
	if isDir {
		delete(m.dirs, p)
		for f := range m.files {
			if strings.HasPrefix(f, p+"/") {
				delete(m.files, f)
			}
		}
		return nil
	}
	if _, ok := m.files[p]; !ok {
		return os.ErrNotExist
	}
	delete(m.files, p)
	return nil
}

type memInfo struct {
	name string
	size int64
	dir  bool
}

func (i memInfo) Name() string { return i.name }
func (i memInfo) Size() int64  { return i.size }
func (i memInfo) Mode() os.FileMode {
	if i.dir {
		return os.ModeDir | 0o755
	}
	return 0o644
}
func (i memInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (i memInfo) IsDir() bool        { return i.dir }
func (i memInfo) Sys() any           { return nil }

func TestFilesystemHandlersRoundTrip(t *testing.T) {
	fs := newMemFS()
	routes := map[string]plugin.Route{}
	for _, r := range Routes("test", "test") {
		routes[r.ID] = r
	}

	run := func(id string, params map[string]string, body []byte) any {
		t.Helper()
		out, err := routes[id].Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, params, nil, body))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		return out
	}

	run("test.files.mkdir", map[string]string{"path": "/"}, mustJSON(t, map[string]any{"name": "docs"}))
	if !fs.dirs["/docs"] {
		t.Fatal("mkdir did not create /docs")
	}

	run("test.files.write", map[string]string{"path": "/docs/readme.txt"}, mustJSON(t, map[string]any{"content": "hello world"}))
	content := run("test.files.read", map[string]string{"path": "/docs/readme.txt"}, nil).(FileContent)
	if content.Content != "hello world" {
		t.Fatalf("read returned %q", content.Content)
	}

	uploadRC := plugin.NewMultipartRequestContext(context.Background(), plugin.User{}, fs,
		map[string]string{"path": "/docs"}, nil, nil, map[string][]plugin.UploadedFile{"files": {makeUpload(t, "data.bin", []byte("binary"))}})
	if _, err := routes["test.files.upload"].Handle(uploadRC); err != nil {
		t.Fatalf("upload: %v", err)
	}

	names := listNames(t, run("test.files.list", map[string]string{"path": "/docs"}, nil))
	if !contains(names, "readme.txt") || !contains(names, "data.bin") {
		t.Fatalf("expected uploaded + written files, got %v", names)
	}

	run("test.files.rename", map[string]string{"path": "/docs/readme.txt"}, mustJSON(t, map[string]any{"name": "renamed.txt"}))
	if _, ok := fs.files["/docs/renamed.txt"]; !ok {
		t.Fatal("rename did not move the file")
	}

	run("test.files.delete", map[string]string{"path": "/docs/renamed.txt"}, nil)
	if _, ok := fs.files["/docs/renamed.txt"]; ok {
		t.Fatal("delete did not remove the file")
	}

	run("test.files.delete", map[string]string{"path": "/docs"}, nil)
	if fs.dirs["/docs"] {
		t.Fatal("delete did not remove the directory")
	}
}

var pngBytes = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00}

func TestDetectMIME(t *testing.T) {
	cases := []struct {
		name string
		path string
		buf  []byte
		want string
	}{
		{"known extension wins over content", "/x/a.txt", pngBytes, "text/plain; charset=utf-8"},
		{"extensionless image sniffed", "/x/logo", pngBytes, "image/png"},
		{"extensionless script sniffed as text", "/x/run", []byte("#!/bin/sh\necho hi\n"), "text/plain; charset=utf-8"},
		{"extensionless binary stays octet-stream", "/x/blob", []byte{0x00, 0x01, 0x02, 0xff}, "application/octet-stream"},
		{"empty extensionless treated as text", "/x/empty", nil, "text/plain; charset=utf-8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetectMIME(tc.path, tc.buf); got != tc.want {
				t.Fatalf("DetectMIME(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestReadPreviewsExtensionlessFiles(t *testing.T) {
	fs := newMemFS()
	fs.files["/logo"] = pngBytes
	fs.files["/run"] = []byte("#!/bin/sh\necho hi\n")
	route := map[string]plugin.Route{}
	for _, r := range Routes("test", "test") {
		route[r.ID] = r
	}
	read := func(p string) FileContent {
		out, err := route["test.files.read"].Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, map[string]string{"path": p}, nil, nil))
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		return out.(FileContent)
	}

	img := read("/logo")
	if img.MIME != "image/png" || img.Encoding != "binary" || img.Content != "" {
		t.Fatalf("extensionless image must sniff to an image mime served as binary: %+v", img)
	}
	script := read("/run")
	if !strings.HasPrefix(script.MIME, "text/plain") || script.Encoding != "utf8" || script.Content == "" {
		t.Fatalf("extensionless text must sniff to text and inline its content: %+v", script)
	}
}

func TestDownloadSniffsExtensionlessMIME(t *testing.T) {
	fs := newMemFS()
	fs.files["/logo"] = pngBytes
	fs.files["/photo.png"] = pngBytes
	route := map[string]plugin.Route{}
	for _, r := range Routes("test", "test") {
		route[r.ID] = r
	}
	get := func(p string) *plugin.Download {
		out, err := route["test.files.download"].Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, fs, map[string]string{"path": p}, nil, nil))
		if err != nil {
			t.Fatalf("download %s: %v", p, err)
		}
		return out.(*plugin.Download)
	}

	// Extensionless: the download must sniff so its Content-Type matches the read
	// preview's MIME, or the inline viewer the frontend picks renders nothing.
	dl := get("/logo")
	if dl.MIME != "image/png" {
		t.Fatalf("extensionless download MIME = %q, want image/png", dl.MIME)
	}
	body, err := io.ReadAll(dl.Body)
	_ = dl.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, pngBytes) {
		t.Fatalf("sniffed download replayed %d bytes, want the whole %d-byte file", len(body), len(pngBytes))
	}

	// Known extension: unchanged, no sniff round-trip.
	if got := get("/photo.png").MIME; got != "image/png" {
		t.Fatalf("extensioned download MIME = %q, want image/png", got)
	}
}

func TestNameSchemasGuideSingleNameInputs(t *testing.T) {
	routes := map[string]plugin.Route{}
	for _, r := range Routes("test", "test") {
		routes[r.ID] = r
	}
	for _, id := range []string{"test.files.mkdir", "test.files.rename"} {
		route := routes[id]
		if route.Input == nil {
			t.Fatalf("%s missing input schema", id)
		}
		field := route.Input.Groups[0].Fields[0]
		if field.Key != "name" || field.Placeholder == "" {
			t.Fatalf("%s name field missing key/placeholder: %+v", id, field)
		}
		if len(field.Validators) != 1 || field.Validators[0].Type != plugin.ValidatorRegex {
			t.Fatalf("%s name field missing regex validator: %+v", id, field.Validators)
		}
	}
}

func makeUpload(t *testing.T, name string, content []byte) plugin.UploadedFile {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("files", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	form, err := multipart.NewReader(&buf, w.Boundary()).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	headers := form.File["files"]
	if len(headers) != 1 {
		t.Fatalf("expected one parsed file, got %d", len(headers))
	}
	return plugin.NewUploadedFile("files", headers[0])
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func listNames(t *testing.T, page any) []string {
	t.Helper()
	fp, ok := page.(FilePage)
	if !ok {
		t.Fatalf("list did not return FilePage, got %T", page)
	}
	names := make([]string, 0, len(fp.Items))
	for _, item := range fp.Items {
		names = append(names, item.Name)
	}
	return names
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
