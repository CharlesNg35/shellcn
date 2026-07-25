package dockerengine

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
)

func TestContainerFilePathCanonicalizes(t *testing.T) {
	cases := map[string]string{
		"":            "/",
		".":           "/",
		"/":           "/",
		"etc":         "/etc",
		"/etc/":       "/etc",
		"/a/../b":     "/b",
		"/a/./b/../c": "/a/c",
		"../../etc":   "/etc",
	}
	for in, want := range cases {
		if got := containerFilePath(in); got != want {
			t.Fatalf("containerFilePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCleanContainerFileNameRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", "a\\b", "  "} {
		if _, err := cleanContainerFileName(bad); err == nil {
			t.Fatalf("cleanContainerFileName(%q) accepted an unsafe name", bad)
		}
	}
	if got, err := cleanContainerFileName("  app.log  "); err != nil || got != "app.log" {
		t.Fatalf("cleanContainerFileName trimmed name = %q, %v", got, err)
	}
}

func TestParseContainerLsHandlesDirsSymlinksAndSizes(t *testing.T) {
	out := strings.Join([]string{
		"total 12",
		"drwxr-xr-x 2 root root 4096 Jan  1 00:00 .",
		"drwxr-xr-x 3 root root 4096 Jan  1 00:00 ..",
		"-rw-r--r-- 1 root root 1234 Jan  1 00:00 app.log",
		"drwxr-xr-x 2 root root 4096 Jan  1 00:00 data",
		"lrwxrwxrwx 1 root root    7 Jan  1 00:00 link -> app.log",
	}, "\n")
	items := parseContainerLs("/srv", out)
	if len(items) != 3 {
		t.Fatalf("parsed %d entries, want 3: %+v", len(items), items)
	}
	byName := map[string]filesystem.FileEntry{}
	for _, e := range items {
		byName[e.Name] = e
	}
	if f := byName["app.log"]; f.IsDir || f.Size != 1234 || f.Path != "/srv/app.log" {
		t.Fatalf("app.log entry = %+v", f)
	}
	if d := byName["data"]; !d.IsDir {
		t.Fatalf("data should be a directory: %+v", d)
	}
	if l := byName["link"]; l.Symlink != "app.log" {
		t.Fatalf("link symlink = %q, want app.log", l.Symlink)
	}
}

func TestContainerFileContentClassifiesTextAndProbesSize(t *testing.T) {
	// Size probe line "42\n" precedes the preview body.
	content := containerFileContent("/etc/motd", []byte("42\nhello world"))
	if content.Encoding != "utf8" || content.Content != "hello world" || content.Size != 42 {
		t.Fatalf("text content = %+v", content)
	}
	// PNG magic: sniffs to image/png and is invalid UTF-8, so it stays binary.
	pngMagic := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	bin := containerFileContent("/bin/x", append([]byte("8\n"), pngMagic...))
	if bin.Encoding != "binary" || bin.Content != "" {
		t.Fatalf("binary content = %+v", bin)
	}
}

// TestContainerWriteFileDeliversFullStdin proves the exec-backed writer streams
// the whole upload body to the container's stdin, mirroring the pod file browser
// guarantee. A short read here would silently truncate large uploads.
func TestContainerWriteFileDeliversFullStdin(t *testing.T) {
	rec := &execRecorder{exitCode: 0}
	sess := newFakeExecSession(t, rec)

	payload := strings.Repeat("shellcn-upload-", 150000) // ~2.1 MiB, many frames past one buffer
	out, err := sess.execCapture(context.Background(), "abc123", []string{"sh", "-c", `cat > "$1"`, "sh", "/data/app.bin"}, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("execCapture write: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("write returned unexpected stdout: %d bytes", len(out))
	}
	if got := rec.command(); fmt.Sprint(got) != fmt.Sprint([]any{"sh", "-c", `cat > "$1"`, "sh", "/data/app.bin"}) {
		t.Fatalf("recorded exec command = %#v", got)
	}
	if rec.stdinString() != payload {
		t.Fatalf("stdin delivered %d bytes, want %d", len(rec.stdinString()), len(payload))
	}
}

// TestContainerFileDownloadStreamsStdout proves the download path demultiplexes
// the exec stdout stream and drops stderr noise.
func TestContainerFileDownloadStreamsStdout(t *testing.T) {
	rec := &execRecorder{exitCode: 0, stdout: []byte("file-body-contents"), stderr: []byte("warning: ignore me")}
	sess := newFakeExecSession(t, rec)

	body, err := sess.execStream(context.Background(), "abc123", []string{"cat", "--", "/etc/hostname"})
	if err != nil {
		t.Fatalf("execStream: %v", err)
	}
	defer func() { _ = body.Close() }()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read download body: %v", err)
	}
	if string(got) != "file-body-contents" {
		t.Fatalf("download body = %q, want file-body-contents", got)
	}
}

// TestContainerExecReportsMissingShell maps a distroless container (exit 127) to
// the friendly no-shell error instead of a raw failure.
func TestContainerExecReportsMissingShell(t *testing.T) {
	rec := &execRecorder{exitCode: 127}
	sess := newFakeExecSession(t, rec)
	_, err := sess.execCapture(context.Background(), "abc123", []string{"ls", "-la", "--", "/"}, nil)
	if err == nil || !strings.Contains(err.Error(), "no shell or file utilities") {
		t.Fatalf("missing-shell error = %v, want no-shell message", err)
	}
}

// TestContainerWriteWaitsForExecCompletion proves the write path waits for the
// exec to finish before trusting the exit code. Over the agent loopback bridge a
// streamed write half-closes stdin and the bridge tears down the reverse channel
// on that half-close, so StdCopy returns (empty) while the command is still
// running. The fake reproduces that: the hijack sends no inline output and the
// exec reports Running on the first inspects, then fails. A single point-in-time
// inspect would report this failed write as success; execCapture must poll to
// completion and surface the non-zero exit.
func TestContainerWriteWaitsForExecCompletion(t *testing.T) {
	rec := &execRecorder{exitCode: 1, runningPolls: 3}
	sess := newFakeExecSession(t, rec)

	payload := strings.Repeat("shellcn-upload-", 150000)
	_, err := sess.execCapture(context.Background(), "abc123", []string{"sh", "-c", `cat > "$1"`, "sh", "/data/app.bin"}, strings.NewReader(payload))
	if err == nil {
		t.Fatal("a failed write whose reverse channel was torn down at stdin half-close must surface the non-zero exit, not a false success")
	}
	if !strings.Contains(err.Error(), "status 1") {
		t.Fatalf("write error = %v, want the exec's non-zero exit status", err)
	}
	if rec.stdinString() != payload {
		t.Fatalf("stdin delivered %d bytes, want %d", len(rec.stdinString()), len(payload))
	}
	if n := rec.inspectCount(); n < 4 {
		t.Fatalf("execCapture inspected %d times, want it to poll past the running exec until completion", n)
	}
}

// execRecorder captures what a fake daemon's exec endpoints saw and what to
// return.
type execRecorder struct {
	mu           sync.Mutex
	cmd          []any
	stdin        []byte
	stdout       []byte
	stderr       []byte
	exitCode     int
	hasStdin     bool
	runningPolls int // inspects that report Running=true before the exec is reported done
	inspects     int
}

// inspectState mimics an exec that is still running for the first runningPolls
// inspects (as over the agent bridge, where the reverse channel is torn down at
// stdin half-close while the command is still finishing) and then reports its
// final exit code.
func (r *execRecorder) inspectState() (running bool, exit int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inspects++
	if r.inspects <= r.runningPolls {
		return true, 0
	}
	return false, r.exitCode
}

func (r *execRecorder) inspectCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inspects
}

func (r *execRecorder) setAttachStdin(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hasStdin = v
}

func (r *execRecorder) attachStdin() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hasStdin
}

func (r *execRecorder) setCommand(c []any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cmd = c
}

func (r *execRecorder) command() []any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd
}

func (r *execRecorder) appendStdin(b []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stdin = append(r.stdin, b...)
}

func (r *execRecorder) stdinString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.stdin)
}

// newFakeExecSession builds a Session whose moby client dials the fake daemon
// directly (no agent loopback bridge), so the test exercises execCapture's own
// hijack, stdin streaming, and stdcopy demux end to end.
func newFakeExecSession(t *testing.T, rec *execRecorder) *Session {
	t.Helper()
	srv := fakeExecDaemon(t, rec)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	addr := u.Host
	cli, err := dockerclient.New(
		dockerclient.WithHost("http://docker"),
		dockerclient.WithDialContext(func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		}),
	)
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}
	s := &Session{cli: cli}
	if err := s.HealthCheck(context.Background()); err != nil {
		t.Fatalf("health check: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func fakeExecDaemon(t *testing.T, rec *execRecorder) *httptest.Server {
	t.Helper()
	const execID = "exec-xyz"
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dockerAPIPath(r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case p == "/_ping":
			w.Header().Set("Api-Version", "1.54")
			_, _ = w.Write([]byte("OK"))
		case p == "/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"Version": "28.5.2"})
		case r.Method == http.MethodPost && p == "/containers/abc123/exec":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if cmd, ok := body["Cmd"].([]any); ok {
				rec.setCommand(cmd)
			}
			attach, _ := body["AttachStdin"].(bool)
			rec.setAttachStdin(attach)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"Id": execID})
		case r.Method == http.MethodPost && p == "/exec/"+execID+"/start":
			serveFakeExecStart(t, w, r, rec)
		case p == "/exec/"+execID+"/json":
			running, exit := rec.inspectState()
			_ = json.NewEncoder(w).Encode(map[string]any{"ID": execID, "Running": running, "ExitCode": exit})
		default:
			t.Logf("unexpected exec request %s %s", r.Method, p)
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(h)
}

// serveFakeExecStart emulates the daemon's exec attach hijack: it consumes the
// ExecStartRequest body, upgrades the connection, drains stdin, then writes the
// framed stdout/stderr.
func serveFakeExecStart(t *testing.T, w http.ResponseWriter, r *http.Request, rec *execRecorder) {
	t.Helper()
	// Consume the JSON ExecStartRequest so the hijacked reader is positioned at
	// the client's stdin bytes (if any).
	_, _ = io.Copy(io.Discard, r.Body)
	hj, ok := w.(http.Hijacker)
	if !ok {
		t.Fatal("fake daemon response writer is not a Hijacker")
	}
	conn, brw, err := hj.Hijack()
	if err != nil {
		t.Fatalf("hijack: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := io.WriteString(conn, "HTTP/1.1 101 UPGRADED\r\nContent-Type: application/vnd.docker.multiplexed-stream\r\nConnection: Upgrade\r\nUpgrade: tcp\r\n\r\n"); err != nil {
		t.Errorf("write upgrade: %v", err)
		return
	}
	// Drain stdin until the client half-closes (CloseWrite -> EOF). Only the
	// client attaches stdin for writes; for read-only execs it never sends any,
	// so draining would block forever.
	if rec.attachStdin() {
		drainExecStdin(brw, rec)
	}
	if len(rec.stdout) > 0 {
		_, _ = conn.Write(frameStd(1, rec.stdout))
	}
	if len(rec.stderr) > 0 {
		_, _ = conn.Write(frameStd(2, rec.stderr))
	}
}

func drainExecStdin(brw *bufio.ReadWriter, rec *execRecorder) {
	buf := make([]byte, 32*1024)
	for {
		n, err := brw.Read(buf)
		if n > 0 {
			rec.appendStdin(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// frameStd wraps a payload in a stdcopy frame (8-byte header: stream type +
// big-endian size).
func frameStd(stream byte, payload []byte) []byte {
	header := make([]byte, 8)
	header[0] = stream
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}
