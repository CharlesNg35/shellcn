package filesystem

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// capFS extends memFS with the optional Move/Copy/Chmod capabilities.
type capFS struct {
	*memFS
	modes map[string]fs.FileMode
}

func newCapFS() *capFS {
	return &capFS{memFS: newMemFS(), modes: map[string]fs.FileMode{}}
}

func (c *capFS) Filesystem() (Client, error) { return c, nil }

func (c *capFS) Move(_ context.Context, src, dst string) error {
	return c.Rename(context.Background(), src, dst)
}

func (c *capFS) Copy(_ context.Context, src, dst string) error {
	data, ok := c.files[src]
	if !ok {
		return os.ErrNotExist
	}
	c.files[dst] = append([]byte(nil), data...)
	return nil
}

func (c *capFS) Chmod(_ context.Context, p string, mode fs.FileMode) error {
	if _, ok := c.files[p]; !ok && !c.dirs[p] {
		return os.ErrNotExist
	}
	c.modes[p] = mode
	return nil
}

func bulkRoutes(t *testing.T) map[string]plugin.Route {
	t.Helper()
	routes := map[string]plugin.Route{}
	for _, r := range Routes("test", "test") {
		routes[r.ID] = r
	}
	return routes
}

func handle(t *testing.T, route plugin.Route, sess plugin.Session, body []byte) (any, error) {
	t.Helper()
	return route.Handle(plugin.NewRequestContext(context.Background(), plugin.User{}, sess, nil, nil, body))
}

func TestBulkMoveCopyChmodOnCapableBackend(t *testing.T) {
	c := newCapFS()
	c.files["/a.txt"] = []byte("alpha")
	c.files["/b.txt"] = []byte("beta")
	c.dirs["/dest"] = true
	routes := bulkRoutes(t)

	if err := runFileJob(context.Background(), c, plugin.FileJobRequest{
		Type:        plugin.FileJobRequestStart,
		JobID:       "move-1",
		Operation:   string(plugin.FileJobMove),
		Paths:       []string{"/a.txt"},
		Destination: "/dest",
	}, func(plugin.FileJobFrame) error { return nil }); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, ok := c.files["/dest/a.txt"]; !ok {
		t.Fatal("move did not relocate /a.txt to /dest/a.txt")
	}

	var copyFrames []plugin.FileJobFrame
	if err := runFileJob(context.Background(), c, plugin.FileJobRequest{
		Type:        plugin.FileJobRequestStart,
		JobID:       "copy-1",
		Operation:   string(plugin.FileJobCopy),
		Paths:       []string{"/b.txt"},
		Destination: "/dest",
	}, func(frame plugin.FileJobFrame) error {
		copyFrames = append(copyFrames, frame)
		return nil
	}); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if _, ok := c.files["/b.txt"]; !ok {
		t.Fatal("copy removed the source")
	}
	if !bytes.Equal(c.files["/dest/b.txt"], []byte("beta")) {
		t.Fatal("copy did not duplicate the content")
	}
	if len(copyFrames) < 2 || copyFrames[len(copyFrames)-1].Type != plugin.FileJobFrameComplete {
		t.Fatalf("copy frames missing completion: %+v", copyFrames)
	}

	if _, err := handle(t, routes["test.files.chmod"], c, mustJSON(t, map[string]any{
		"paths": []string{"/dest/b.txt"}, "mode": "0600",
	})); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	if c.modes["/dest/b.txt"] != 0o600 {
		t.Fatalf("chmod set %o, want 0600", c.modes["/dest/b.txt"])
	}
}

func TestBulkRoutesDeclareActionableInputSchemas(t *testing.T) {
	routes := bulkRoutes(t)
	for _, id := range []string{"test.files.chmod", "test.files.archive"} {
		if routes[id].Input == nil {
			t.Fatalf("%s missing input schema", id)
		}
	}
	if routes["test.files.jobs"].Method != plugin.MethodWS || routes["test.files.jobs"].Stream == nil {
		t.Fatalf("file job route should be websocket-backed: %+v", routes["test.files.jobs"])
	}

	chmodMode := requireBulkField(t, routes["test.files.chmod"].Input, "mode")
	if chmodMode.Type != plugin.FieldAutocomplete || len(chmodMode.Options) < 2 || len(chmodMode.Validators) == 0 {
		t.Fatalf("chmod mode should suggest common octal modes and validate input: %+v", chmodMode)
	}
	archivePaths := requireBulkField(t, routes["test.files.archive"].Input, "paths")
	if archivePaths.Type != plugin.FieldArray || archivePaths.MinItems != 1 || archivePaths.Item == nil {
		t.Fatalf("archive paths should be a non-empty path array: %+v", archivePaths)
	}
}

func TestBulkUnsupportedReturnsCleanError(t *testing.T) {
	m := newMemFS() // no Move/Copy/Chmod capabilities
	routes := bulkRoutes(t)

	for _, op := range []plugin.FileJobOperation{plugin.FileJobMove, plugin.FileJobCopy} {
		err := runFileJob(context.Background(), m, plugin.FileJobRequest{
			Type:        plugin.FileJobRequestStart,
			JobID:       "file-job-1",
			Operation:   string(op),
			Paths:       []string{"/x"},
			Destination: "/d",
		}, func(plugin.FileJobFrame) error { return nil })
		if !errors.Is(err, plugin.ErrInvalidInput) {
			t.Fatalf("%s: expected ErrInvalidInput for unsupported backend, got %v", op, err)
		}
	}
	_, err := handle(t, routes["test.files.chmod"], m, mustJSON(t, map[string]any{"paths": []string{"/x"}, "mode": "0644"}))
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("chmod: expected ErrInvalidInput, got %v", err)
	}
}

func TestBulkRejectsEmptyAndRootPaths(t *testing.T) {
	c := newCapFS()

	err := runFileJob(context.Background(), c, plugin.FileJobRequest{
		Type:        plugin.FileJobRequestStart,
		JobID:       "file-job-1",
		Operation:   string(plugin.FileJobMove),
		Paths:       []string{},
		Destination: "/d",
	}, func(plugin.FileJobFrame) error { return nil })
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("empty paths: expected ErrInvalidInput, got %v", err)
	}
	err = runFileJob(context.Background(), c, plugin.FileJobRequest{
		Type:        plugin.FileJobRequestStart,
		JobID:       "file-job-1",
		Operation:   string(plugin.FileJobMove),
		Paths:       []string{"/"},
		Destination: "/d",
	}, func(plugin.FileJobFrame) error { return nil })
	if !errors.Is(err, plugin.ErrInvalidInput) {
		t.Fatalf("root path: expected ErrInvalidInput, got %v", err)
	}
}

func TestParseMode(t *testing.T) {
	cases := map[string]struct {
		want fs.FileMode
		ok   bool
	}{
		"0644":  {0o644, true},
		"755":   {0o755, true},
		"":      {0, false},
		"abc":   {0, false},
		"99999": {0, false},
	}
	for in, want := range cases {
		got, err := parseMode(in)
		if want.ok && (err != nil || got != want.want) {
			t.Fatalf("parseMode(%q) = %o, %v; want %o", in, got, err, want.want)
		}
		if !want.ok && err == nil {
			t.Fatalf("parseMode(%q) expected error", in)
		}
	}
}

func TestArchiveBuildsZipGenerically(t *testing.T) {
	m := newMemFS()
	m.files["/a.txt"] = []byte("alpha")
	m.dirs["/sub"] = true
	m.files["/sub/b.txt"] = []byte("beta")
	routes := bulkRoutes(t)

	out, err := handle(t, routes["test.files.archive"], m, mustJSON(t, map[string]any{
		"paths": []string{"/a.txt", "/sub"},
	}))
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	dl, ok := out.(*plugin.Download)
	if !ok {
		t.Fatalf("archive returned %T, want *plugin.Download", out)
	}
	if dl.MIME != "application/zip" {
		t.Fatalf("archive MIME = %q", dl.MIME)
	}
	data, err := io.ReadAll(dl.Body)
	if err != nil {
		t.Fatalf("read archive body: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	contents := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", f.Name, err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		contents[f.Name] = string(b)
	}
	if contents["a.txt"] != "alpha" {
		t.Fatalf("zip missing a.txt, got %v", keys(contents))
	}
	if contents["sub/b.txt"] != "beta" {
		t.Fatalf("zip missing sub/b.txt, got %v", keys(contents))
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func requireBulkField(t *testing.T, schema *plugin.Schema, key string) plugin.Field {
	t.Helper()
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Key == key {
				return field
			}
		}
	}
	t.Fatalf("missing field %q in schema %+v", key, schema)
	return plugin.Field{}
}

func TestZipName(t *testing.T) {
	if got := zipName("/sub/b.txt", path.Dir("/sub")); got != "sub/b.txt" {
		t.Fatalf("zipName = %q, want sub/b.txt", got)
	}
	if got := zipName("/a.txt", path.Dir("/a.txt")); got != "a.txt" {
		t.Fatalf("zipName = %q, want a.txt", got)
	}
}
