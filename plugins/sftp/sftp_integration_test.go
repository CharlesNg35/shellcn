package sftp_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/charlesng35/shellcn/plugins/sftp"
	"github.com/charlesng35/shellcn/sdk/plugin"
	"github.com/charlesng35/shellcn/sdk/plugintest"
)

const (
	itUser = "shellcn"
	itPass = "shellcnpass"
	itDir  = "upload" // atmoz/sftp pre-creates a writable subdir
)

// TestSFTPPluginIntegration self-provisions an atmoz/sftp container and exercises
// the bulk file operations end to end through the plugin's route handlers.
func TestSFTPPluginIntegration(t *testing.T) {
	if os.Getenv("SHELLCN_SFTP_INTEGRATION") != "1" {
		t.Skip("set SHELLCN_SFTP_INTEGRATION=1 to run against an SFTP server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	host, port := sftpEndpoint(ctx, t)

	cfg := map[string]any{
		"host":     host,
		"port":     port,
		"user":     itUser,
		"auth":     "password",
		"password": itPass,
	}
	p := sftp.New()
	sess, err := p.Connect(ctx, plugin.ConnectConfig{
		Config: cfg,
		Net:    plugintest.DirectTransport(),
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	routes := routeMapIT(p.Routes())

	base := "/" + itDir
	upload := func(name, content string) {
		t.Helper()
		uploadFile(ctx, t, routes["sftp.sftp.upload"], sess, base, name, content)
	}
	upload("a.txt", "alpha")
	upload("b.txt", "beta")
	upload("c.txt", "gamma")
	upload("d.txt", "delta")

	if got := listNamesIT(ctx, t, routes["sftp.sftp.list"], sess, base); !containsAll(got, "a.txt", "b.txt", "c.txt", "d.txt") {
		t.Fatalf("after upload, listing = %v", got)
	}

	// Multi-delete a subset (a.txt and b.txt) via the per-entry delete route.
	for _, name := range []string{"a.txt", "b.txt"} {
		callIT(ctx, t, routes["sftp.sftp.delete"], sess, map[string]string{"path": base + "/" + name}, nil)
	}
	if got := listNamesIT(ctx, t, routes["sftp.sftp.list"], sess, base); containsAny(got, "a.txt", "b.txt") {
		t.Fatalf("multi-delete left entries behind: %v", got)
	}

	// Move c.txt into a fresh subdirectory.
	callIT(ctx, t, routes["sftp.sftp.mkdir"], sess, map[string]string{"path": base}, mustJSONIT(t, map[string]any{"name": "moved"}))
	transferIT(ctx, t, routes["sftp.sftp.transfer"], sess, plugin.FileTransferMove, []string{base + "/c.txt"}, base+"/moved")
	if got := listNamesIT(ctx, t, routes["sftp.sftp.list"], sess, base+"/moved"); !containsAll(got, "c.txt") {
		t.Fatalf("move did not place c.txt under moved/: %v", got)
	}

	// Copy d.txt; the original must remain.
	transferIT(ctx, t, routes["sftp.sftp.transfer"], sess, plugin.FileTransferCopy, []string{base + "/d.txt"}, base+"/moved")
	if got := listNamesIT(ctx, t, routes["sftp.sftp.list"], sess, base); !containsAll(got, "d.txt") {
		t.Fatalf("copy removed the source d.txt: %v", got)
	}
	if got := listNamesIT(ctx, t, routes["sftp.sftp.list"], sess, base+"/moved"); !containsAll(got, "d.txt") {
		t.Fatalf("copy did not duplicate d.txt: %v", got)
	}

	// Chmod the copied file to 0600.
	callIT(ctx, t, routes["sftp.sftp.chmod"], sess, nil, mustJSONIT(t, map[string]any{
		"paths": []string{base + "/moved/d.txt"}, "mode": "0600",
	}))
	mode := statMode(ctx, t, routes["sftp.sftp.stat"], sess, base+"/moved/d.txt")
	if !strings.HasSuffix(mode, "rw-------") {
		t.Fatalf("chmod 0600 produced mode %q", mode)
	}

	// Archive moved/ (c.txt + d.txt) and assert the zip bytes carry the entries.
	zipBytes := archiveZip(ctx, t, routes["sftp.sftp.archive"], sess, []string{base + "/moved"})
	names := zipEntries(t, zipBytes)
	if !containsAll(names, "moved/c.txt", "moved/d.txt") {
		t.Fatalf("archive zip entries = %v, want moved/c.txt + moved/d.txt", names)
	}
}

func sftpEndpoint(ctx context.Context, t *testing.T) (string, int) {
	t.Helper()
	if host := os.Getenv("SHELLCN_SFTP_HOST"); host != "" {
		port := 22
		if p := os.Getenv("SHELLCN_SFTP_PORT"); p != "" {
			port, _ = strconv.Atoi(p)
		}
		return host, port
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker CLI unavailable and SHELLCN_SFTP_HOST is not set")
	}
	name := "shellcn-sftp-it-" + time.Now().UTC().Format("20060102150405")
	runIT(ctx, t, "docker", "run", "-d", "--rm", "--name", name,
		"-p", "127.0.0.1::22",
		"atmoz/sftp:latest", itUser+":"+itPass+":::"+itDir)
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = exec.CommandContext(c, "docker", "rm", "-f", name).Run()
	})
	out := runIT(ctx, t, "docker", "port", name, "22/tcp")
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(strings.SplitN(out, "\n", 2)[0]))
	if err != nil {
		t.Fatalf("unexpected docker port output: %q", out)
	}
	port, _ := strconv.Atoi(portStr)
	waitSSHReady(ctx, t, host, port)
	return host, port
}

func waitSSHReady(ctx context.Context, t *testing.T, host string, port int) {
	t.Helper()
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	deadline := time.Now().Add(120 * time.Second)
	cfg := &ssh.ClientConfig{
		User:            itUser,
		Auth:            []ssh.AuthMethod{ssh.Password(itPass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         3 * time.Second,
	}
	for {
		client, err := ssh.Dial("tcp", addr, cfg)
		if err == nil {
			_ = client.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("SFTP server did not become ready at %s: %v", addr, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("context cancelled waiting for SFTP: %v", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func routeMapIT(routes []plugin.Route) map[string]plugin.Route {
	out := map[string]plugin.Route{}
	for _, r := range routes {
		out[r.ID] = r
	}
	return out
}

func callIT(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, params map[string]string, body []byte) any {
	t.Helper()
	out, err := route.Handle(plugin.NewRequestContext(ctx, plugin.User{}, sess, params, nil, body))
	if err != nil {
		t.Fatalf("%s: %v", route.ID, err)
	}
	return out
}

type streamClientIT struct {
	ctx    context.Context
	cancel context.CancelFunc
	in     *bytes.Reader
	mu     sync.Mutex
	out    bytes.Buffer
}

func newStreamClientIT(ctx context.Context, payload []byte) *streamClientIT {
	streamCtx, cancel := context.WithCancel(ctx)
	return &streamClientIT{ctx: streamCtx, cancel: cancel, in: bytes.NewReader(append(payload, '\n'))}
}

func (s *streamClientIT) Read(p []byte) (int, error) {
	n, err := s.in.Read(p)
	if err == io.EOF {
		<-s.ctx.Done()
		return n, io.EOF
	}
	return n, err
}

func (s *streamClientIT) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.Write(p)
}

func (s *streamClientIT) Close() error {
	s.cancel()
	return nil
}

func (s *streamClientIT) Context() context.Context { return s.ctx }

func (s *streamClientIT) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.out.String()
}

func transferIT(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, op plugin.FileTransferOperation, paths []string, dest string) {
	t.Helper()
	payload := mustJSONIT(t, plugin.FileTransferRequest{
		Type:        plugin.FileTransferRequestStart,
		TransferID:  "transfer-it",
		Operation:   string(op),
		Paths:       paths,
		Destination: dest,
	})
	client := newStreamClientIT(ctx, payload)
	errCh := make(chan error, 1)
	go func() {
		errCh <- route.Stream(plugin.NewRequestContext(ctx, plugin.User{}, sess, nil, nil, nil), client)
	}()
	deadline := time.After(30 * time.Second)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-deadline:
			_ = client.Close()
			t.Fatalf("%s timed out waiting for %s completion; frames: %s", route.ID, op, client.String())
		case <-tick.C:
			out := client.String()
			if strings.Contains(out, `"type":"error"`) {
				_ = client.Close()
				t.Fatalf("%s failed %s: %s", route.ID, op, out)
			}
			if strings.Contains(out, `"type":"complete"`) {
				_ = client.Close()
				<-errCh
				return
			}
		}
	}
}

func uploadFile(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, dir, name, content string) {
	t.Helper()
	rc := plugin.NewMultipartRequestContext(ctx, plugin.User{}, sess,
		map[string]string{"path": dir}, nil, nil,
		map[string][]plugin.UploadedFile{"files": {makeUploadIT(t, name, []byte(content))}})
	if _, err := route.Handle(rc); err != nil {
		t.Fatalf("upload %s: %v", name, err)
	}
}

func listNamesIT(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, dir string) []string {
	t.Helper()
	out := callIT(ctx, t, route, sess, map[string]string{"path": dir}, nil)
	data, _ := json.Marshal(out)
	var page struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	_ = json.Unmarshal(data, &page)
	names := make([]string, 0, len(page.Items))
	for _, it := range page.Items {
		names = append(names, it.Name)
	}
	return names
}

func statMode(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, p string) string {
	t.Helper()
	out := callIT(ctx, t, route, sess, map[string]string{"path": p}, nil)
	data, _ := json.Marshal(out)
	var entry struct {
		Mode string `json:"mode"`
	}
	_ = json.Unmarshal(data, &entry)
	return entry.Mode
}

func archiveZip(ctx context.Context, t *testing.T, route plugin.Route, sess plugin.Session, paths []string) []byte {
	t.Helper()
	out := callIT(ctx, t, route, sess, nil, mustJSONIT(t, map[string]any{"paths": paths}))
	dl, ok := out.(*plugin.Download)
	if !ok {
		t.Fatalf("archive returned %T, want *plugin.Download", out)
	}
	b, err := io.ReadAll(dl.Body)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return b
}

func zipEntries(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func mustJSONIT(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func makeUploadIT(t *testing.T, name string, content []byte) plugin.UploadedFile {
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

func containsAll(haystack []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range haystack {
		set[h] = true
	}
	for _, w := range want {
		if !set[w] {
			return false
		}
	}
	return true
}

func containsAny(haystack []string, want ...string) bool {
	set := map[string]bool{}
	for _, h := range haystack {
		set[h] = true
	}
	for _, w := range want {
		if set[w] {
			return true
		}
	}
	return false
}

func runIT(ctx context.Context, t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return string(out)
}
