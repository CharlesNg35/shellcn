package dockerengine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moby/moby/api/pkg/stdcopy"
	dockerclient "github.com/moby/moby/client"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/plugins/shared/termshell"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const containerFileReadLimit = 1 << 20 // 1 MiB preview cap

// errNoContainerShell mirrors the Kubernetes pod file browser: a container
// without the coreutils the exec-backed browser needs (e.g. a distroless image)
// can't be browsed.
var errNoContainerShell = fmt.Errorf("%w: this container has no shell or file utilities (e.g. a distroless image), so file browsing is unavailable", plugin.ErrNotSupported)

// ContainerFilesTab builds a generic file browser over a running container,
// backed by exec. prefix is the route-ID namespace (e.g. "docker.container");
// each plugin wires the concrete route IDs so the shared handlers stay
// plugin-agnostic.
func ContainerFilesTab(prefix string) plugin.Panel {
	return plugin.Panel{
		Key: "files", Label: "Files", Icon: plugin.Icon{Type: plugin.IconLucide, Value: "folder"},
		Type:   plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: prefix + ".files.list", Params: map[string]string{"id": "${resource.uid}", "path": "/"}},
		Config: plugin.FileBrowserConfig{
			PathParam: "path",
			Routes: plugin.FileBrowserRoutes{
				Read:     prefix + ".files.read",
				Download: prefix + ".files.download",
				Write:    prefix + ".files.write",
				Mkdir:    prefix + ".files.mkdir",
				Rename:   prefix + ".files.rename",
				Delete:   prefix + ".files.delete",
			},
			Upload:   plugin.FileUploadConfig{RouteID: prefix + ".files.upload", FieldName: "files", Multiple: true},
			Writable: true,
		},
		VisibleWhen: WhenState("running"),
	}
}

// ContainerFileUploadSchema declares the multipart upload input.
func ContainerFileUploadSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Upload", Fields: []plugin.Field{
		{Key: "files", Label: "Files", Type: plugin.FieldFile, Required: true},
	}}}}
}

func containerFileTarget(rc *plugin.RequestContext) (*Session, string, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, "", err
	}
	id := strings.TrimSpace(rc.Param("id"))
	if id == "" {
		return nil, "", fmt.Errorf("%w: container id is required", plugin.ErrInvalidInput)
	}
	return s, id, nil
}

// containerFilePath canonicalizes a path to an absolute, traversal-free form.
func containerFilePath(p string) string {
	if p = strings.TrimSpace(p); p == "" || p == "." {
		return "/"
	}
	clean := path.Clean("/" + strings.TrimPrefix(p, "/"))
	if clean == "." {
		clean = "/"
	}
	return clean
}

// cleanContainerFileName rejects separators and traversal so an upload/mkdir
// name can only land directly under the chosen directory.
func cleanContainerFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("%w: invalid file name", plugin.ErrInvalidInput)
	}
	return name, nil
}

func ContainerFilesList(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := containerFilePath(rc.Param("path"))
	out, err := s.execCapture(rc.Ctx, id, []string{"ls", "-la", "--", dir}, nil)
	if err != nil {
		return nil, err
	}
	return filesystem.FilePage{Items: parseContainerLs(dir, string(out)), Path: dir}, nil
}

func ContainerFileRead(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := containerFilePath(rc.Param("path"))
	// One round-trip: the real byte size (stat, falling back to wc) on the first
	// line, then a preview read one byte past the cap so truncation is detectable.
	// The path is a positional arg ($1), never interpolated, so it can't inject shell.
	script := fmt.Sprintf(`s=$(stat -c %%s "$1" 2>/dev/null || wc -c < "$1" 2>/dev/null); printf '%%s\n' "$s"; head -c %d "$1"`, containerFileReadLimit+1)
	out, err := s.execCapture(rc.Ctx, id, []string{"sh", "-c", script, "sh", p}, nil)
	if err != nil {
		return nil, err
	}
	return containerFileContent(p, out), nil
}

// containerFileContent splits the size probe from the preview bytes and
// classifies the result, mirroring the shared filesystem preview contract.
func containerFileContent(p string, raw []byte) filesystem.FileContent {
	size, body := splitContainerSizeProbe(raw)
	truncated := len(body) > containerFileReadLimit
	if truncated {
		body = trimContainerPartialRune(body[:containerFileReadLimit])
	}
	if size <= 0 {
		size = int64(len(body))
	}
	mimeType := filesystem.DetectMIME(p, body)
	content := filesystem.FileContent{Path: p, MIME: mimeType, Size: size}
	if filesystem.IsText(mimeType, body) {
		content.Encoding = "utf8"
		content.Content = string(body)
		content.Truncated = truncated
	} else {
		content.Encoding = "binary"
	}
	return content
}

func splitContainerSizeProbe(raw []byte) (int64, []byte) {
	head, body, found := bytes.Cut(raw, []byte{'\n'})
	if !found {
		return 0, raw
	}
	size, _ := strconv.ParseInt(strings.TrimSpace(string(head)), 10, 64)
	return size, body
}

func trimContainerPartialRune(b []byte) []byte {
	for i := 0; i < utf8.UTFMax && len(b) > 0 && !utf8.Valid(b); i++ {
		b = b[:len(b)-1]
	}
	return b
}

func ContainerFileDownload(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := containerFilePath(rc.Param("path"))
	body, err := s.execStream(rc.Ctx, id, []string{"cat", "--", p})
	if err != nil {
		return nil, err
	}
	return containerDownload(p, rc.Param("inline") == "1", body), nil
}

// containerDownload builds the streamed download response. Size is -1 because the
// byte count is unknown up front (cat streams); a zero Size would be sent as
// Content-Length: 0 and truncate the body, breaking downloads and inline
// image/pdf/audio/video previews.
func containerDownload(p string, inline bool, body io.ReadCloser) *plugin.Download {
	mimeType := filesystem.MimeFor(p)
	if mimeType == "" {
		mimeType, body = filesystem.SniffStream(body)
	}
	return &plugin.Download{Name: path.Base(p), MIME: mimeType, Size: -1, Inline: inline, Body: body}
}

func ContainerFileWrite(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := containerFilePath(rc.Param("path"))
	var req struct {
		Content string `json:"content"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	return containerWriteFile(rc.Ctx, s, id, p, strings.NewReader(req.Content))
}

func ContainerFileUpload(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := containerFilePath(rc.Param("path"))
	files := rc.Uploads("files")
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no files uploaded", plugin.ErrInvalidInput)
	}
	for _, file := range files {
		name, err := cleanContainerFileName(file.Filename)
		if err != nil {
			return nil, err
		}
		src, err := file.Open()
		if err != nil {
			return nil, DockerErr(err)
		}
		_, err = containerWriteFile(rc.Ctx, s, id, path.Join(dir, name), src)
		_ = src.Close()
		if err != nil {
			return nil, err
		}
	}
	return ActionResult{OK: true}, nil
}

func ContainerFileMkdir(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := containerFilePath(rc.Param("path"))
	var req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := cleanContainerFileName(req.Name)
	if err != nil {
		return nil, err
	}
	if _, err := s.execCapture(rc.Ctx, id, []string{"mkdir", "-p", "--", path.Join(dir, name)}, nil); err != nil {
		return nil, err
	}
	return ActionResult{OK: true}, nil
}

func ContainerFileRename(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := containerFilePath(rc.Param("path"))
	if p == "/" {
		return nil, fmt.Errorf("%w: refusing to rename the root directory", plugin.ErrInvalidInput)
	}
	var req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := cleanContainerFileName(req.Name)
	if err != nil {
		return nil, err
	}
	// src/dst are separate argv elements, never interpolated, so a name can't
	// inject shell; the cleaned name keeps the entry in its own directory.
	dst := path.Join(path.Dir(p), name)
	if _, err := s.execCapture(rc.Ctx, id, []string{"mv", "--", p, dst}, nil); err != nil {
		return nil, err
	}
	return ActionResult{OK: true}, nil
}

func ContainerFileDelete(rc *plugin.RequestContext) (any, error) {
	s, id, err := containerFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := containerFilePath(rc.Param("path"))
	if p == "/" {
		return nil, fmt.Errorf("%w: refusing to delete the root directory", plugin.ErrInvalidInput)
	}
	if _, err := s.execCapture(rc.Ctx, id, []string{"rm", "-rf", "--", p}, nil); err != nil {
		return nil, err
	}
	return ActionResult{OK: true}, nil
}

func containerWriteFile(ctx context.Context, s *Session, id, p string, src io.Reader) (any, error) {
	// Path is a positional arg ($1), never interpolated into the script, so an
	// arbitrary file name can't inject shell.
	if _, err := s.execCapture(ctx, id, []string{"sh", "-c", `cat > "$1"`, "sh", p}, src); err != nil {
		return nil, err
	}
	return ActionResult{OK: true}, nil
}

// parseContainerLs parses `ls -la` defensively (size, mode, dir flag, name).
// Names with embedded whitespace collapse and times are skipped — acceptable for
// a portable browser across busybox and coreutils userlands.
func parseContainerLs(dir, out string) []filesystem.FileEntry {
	items := make([]filesystem.FileEntry, 0)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		name := strings.Join(fields[8:], " ")
		symlink := ""
		if strings.HasPrefix(fields[0], "l") {
			if i := strings.Index(name, " -> "); i >= 0 {
				symlink = name[i+len(" -> "):]
				name = name[:i]
			}
		}
		if name == "." || name == ".." {
			continue
		}
		isDir := strings.HasPrefix(fields[0], "d")
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		entry := filesystem.FileEntry{
			Name:    name,
			Path:    path.Join(dir, name),
			IsDir:   isDir,
			Size:    size,
			Mode:    fields[0],
			Symlink: symlink,
		}
		if !isDir {
			entry.MIME = filesystem.MimeFor(entry.Path)
		}
		items = append(items, entry)
	}
	return items
}

// execCapture runs a non-interactive command in a container over exec and
// returns its stdout, surfacing stderr on a non-zero exit. A missing shell or
// coreutil (distroless) is reported as errNoContainerShell. When stdin is
// non-nil it is streamed to the command; the copy runs concurrently with the
// output read so a bulk upload can't deadlock on a full pipe.
func (s *Session) execCapture(ctx context.Context, id string, command []string, stdin io.Reader) ([]byte, error) {
	resp, execID, err := s.execAttach(ctx, id, command, stdin != nil)
	if err != nil {
		return nil, err
	}
	defer resp.Close()
	if stdin != nil {
		go func() {
			_, _ = io.Copy(resp.Conn, stdin)
			_ = resp.CloseWrite()
		}()
	}
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return stdout.Bytes(), fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
	}
	// Wait for the exec to actually finish before trusting the exit code. Over the
	// agent loopback bridge a streamed write half-closes stdin, and the bridge
	// tears down the reverse direction on that half-close, so StdCopy can return
	// before the command has finished writing. A single point-in-time inspect
	// would then race a still-running exec and report a failed write (e.g. exit
	// 1/126/127) as success; polling to completion makes the result authoritative
	// on both the direct socket and the agent bridge.
	inspect, err := s.waitExecDone(ctx, execID)
	if err == nil && inspect.ExitCode != 0 {
		if inspect.ExitCode == 126 || inspect.ExitCode == 127 {
			return stdout.Bytes(), errNoContainerShell
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", plugin.ErrUnavailable, msg)
		}
		return stdout.Bytes(), fmt.Errorf("%w: command exited with status %d", plugin.ErrUnavailable, inspect.ExitCode)
	}
	return stdout.Bytes(), nil
}

// waitExecDone polls ExecInspect until the exec is no longer running so its exit
// code is authoritative. It returns as soon as the exec is done (immediately on a
// direct socket, where StdCopy already blocked to completion) and honors context
// cancellation. An inspect error is returned to the caller, which falls back to
// treating the command as successful — matching the prior best-effort behavior.
func (s *Session) waitExecDone(ctx context.Context, execID string) (dockerclient.ExecInspectResult, error) {
	for {
		inspect, err := s.cli.ExecInspect(ctx, execID, dockerclient.ExecInspectOptions{})
		if err != nil || !inspect.Running {
			return inspect, err
		}
		select {
		case <-ctx.Done():
			return inspect, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// execStream runs a command and streams its demultiplexed stdout, for downloads.
func (s *Session) execStream(ctx context.Context, id string, command []string) (io.ReadCloser, error) {
	resp, _, err := s.execAttach(ctx, id, command, false)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		_, err := stdcopy.StdCopy(pw, io.Discard, resp.Reader)
		resp.Close()
		_ = pw.CloseWithError(err)
	}()
	return pr, nil
}

// execAttach creates and attaches to a non-TTY exec so stdout/stderr stay
// demultiplexable via stdcopy and stdin can carry bulk binary writes.
func (s *Session) execAttach(ctx context.Context, id string, command []string, stdin bool) (dockerclient.HijackedResponse, string, error) {
	created, err := s.cli.ExecCreate(ctx, id, dockerclient.ExecCreateOptions{
		AttachStdin:  stdin,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		if termshell.MissingExecutableError(err) {
			return dockerclient.HijackedResponse{}, "", errNoContainerShell
		}
		return dockerclient.HijackedResponse{}, "", DockerErr(err)
	}
	resp, err := s.cli.ExecAttach(ctx, created.ID, dockerclient.ExecAttachOptions{})
	if err != nil {
		if termshell.MissingExecutableError(err) {
			return dockerclient.HijackedResponse{}, "", errNoContainerShell
		}
		return dockerclient.HijackedResponse{}, "", DockerErr(err)
	}
	return resp.HijackedResponse, created.ID, nil
}
