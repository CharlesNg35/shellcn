package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/charlesng35/shellcn/plugins/shared/filesystem"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

const podFileReadLimit = 1 << 20 // 1 MiB preview cap

// podFilesTab is a generic file browser over one pod container, backed by exec.
// The browser merges the panel source params (namespace/name) into every operation
// route, so the per-pod identity flows to each handler.
func podFilesTab() plugin.Panel {
	return plugin.Panel{
		Key: "files", Label: "Files", Icon: lucide("folder"), Type: plugin.PanelFileBrowser,
		Source: &plugin.DataSource{RouteID: "kubernetes.pod.files.list", Params: podRefParams(map[string]string{"path": "/"})},
		Config: plugin.FileBrowserConfig{
			PathParam: "path",
			Routes: plugin.FileBrowserRoutes{
				Read:     "kubernetes.pod.files.read",
				Download: "kubernetes.pod.files.download",
				Write:    "kubernetes.pod.files.write",
				Mkdir:    "kubernetes.pod.files.mkdir",
				Delete:   "kubernetes.pod.files.delete",
			},
			Upload:   plugin.FileUploadConfig{RouteID: "kubernetes.pod.files.upload", FieldName: "files", Multiple: true},
			Writable: true,
		},
		VisibleWhen: runningPod(),
	}
}

func podUploadSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Upload", Fields: []plugin.Field{
		{Key: "files", Label: "Files", Type: plugin.FieldFile, Required: true},
	}}}}
}

func podFileTarget(rc *plugin.RequestContext) (*Session, string, string, string, error) {
	s, err := sess(rc)
	if err != nil {
		return nil, "", "", "", err
	}
	ns, pod := rc.Param("namespace"), rc.Param("name")
	if err := validateNamespace(ns); err != nil {
		return nil, "", "", "", err
	}
	if err := validateName(pod); err != nil {
		return nil, "", "", "", err
	}
	return s, ns, pod, param(rc, "container"), nil
}

func podPath(p string) string {
	if p == "" || p == "." {
		return "/"
	}
	return p
}

// cleanFileName rejects path separators and traversal so an upload/mkdir name can
// only land directly under the chosen directory.
func cleanFileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return "", fmt.Errorf("%w: invalid file name", plugin.ErrInvalidInput)
	}
	return name, nil
}

func PodFilesList(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := podPath(rc.Param("path"))
	out, err := s.execCapture(rc.Ctx, ns, pod, container, []string{"ls", "-la", "--", dir}, nil)
	if err != nil {
		return nil, err
	}
	return filesystem.FilePage{Items: parseLsOutput(dir, string(out)), Path: dir}, nil
}

func PodFileRead(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := podPath(rc.Param("path"))
	out, err := s.execCapture(rc.Ctx, ns, pod, container, []string{"head", "-c", strconv.Itoa(podFileReadLimit), p}, nil)
	if err != nil {
		return nil, err
	}
	content := filesystem.FileContent{Path: p, Size: int64(len(out))}
	if utf8.Valid(out) {
		content.Encoding = "utf8"
		content.Content = string(out)
		content.Truncated = len(out) >= podFileReadLimit
	} else {
		content.Encoding = "binary"
	}
	return content, nil
}

func PodFileDownload(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := podPath(rc.Param("path"))
	body, err := s.execStream(rc.Ctx, ns, pod, container, []string{"cat", "--", p})
	if err != nil {
		return nil, err
	}
	return &plugin.Download{Name: path.Base(p), MIME: "application/octet-stream", Inline: rc.Param("inline") == "1", Body: body}, nil
}

func PodFileWrite(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := podPath(rc.Param("path"))
	var req struct {
		Content string `json:"content"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	return podWriteFile(rc.Ctx, s, ns, pod, container, p, strings.NewReader(req.Content))
}

func PodFileUpload(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := podPath(rc.Param("path"))
	files := rc.Uploads("files")
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no files uploaded", plugin.ErrInvalidInput)
	}
	for _, file := range files {
		name, err := cleanFileName(file.Filename)
		if err != nil {
			return nil, err
		}
		src, err := file.Open()
		if err != nil {
			return nil, apiErr(err)
		}
		_, err = podWriteFile(rc.Ctx, s, ns, pod, container, path.Join(dir, name), src)
		_ = src.Close()
		if err != nil {
			return nil, err
		}
	}
	return map[string]bool{"ok": true}, nil
}

func PodFileMkdir(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	dir := podPath(rc.Param("path"))
	var req struct {
		Name string `json:"name" validate:"required"`
	}
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := cleanFileName(req.Name)
	if err != nil {
		return nil, err
	}
	if _, err := s.execCapture(rc.Ctx, ns, pod, container, []string{"mkdir", "-p", "--", path.Join(dir, name)}, nil); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func PodFileDelete(rc *plugin.RequestContext) (any, error) {
	s, ns, pod, container, err := podFileTarget(rc)
	if err != nil {
		return nil, err
	}
	p := podPath(rc.Param("path"))
	if p == "/" {
		return nil, fmt.Errorf("%w: refusing to delete the root directory", plugin.ErrInvalidInput)
	}
	if _, err := s.execCapture(rc.Ctx, ns, pod, container, []string{"rm", "-rf", "--", p}, nil); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func podWriteFile(ctx context.Context, s *Session, ns, pod, container, p string, src io.Reader) (any, error) {
	// Path is passed as a positional arg ($1), never interpolated into the script,
	// so an arbitrary file name can't inject shell.
	if _, err := s.execCapture(ctx, ns, pod, container, []string{"sh", "-c", `cat > "$1"`, "sh", p}, src); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

// parseLsOutput parses `ls -la` defensively (size, mode, dir flag, name). Names
// with embedded whitespace collapse and times are skipped — acceptable for a
// portable browser across busybox and coreutils userlands.
func parseLsOutput(dir, out string) []filesystem.FileEntry {
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
		if i := strings.Index(name, " -> "); i >= 0 {
			name = name[:i] // strip symlink target
		}
		if name == "." || name == ".." {
			continue
		}
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		items = append(items, filesystem.FileEntry{
			Name:  name,
			Path:  path.Join(dir, name),
			IsDir: strings.HasPrefix(fields[0], "d"),
			Size:  size,
			Mode:  fields[0],
		})
	}
	return items
}

// execCapture runs a non-interactive command in a pod container and returns its
// stdout, surfacing stderr on a non-zero exit.
func (s *Session) execCapture(ctx context.Context, ns, pod, container string, command []string, stdin io.Reader) ([]byte, error) {
	opts := &corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdin:     stdin != nil,
		Stdout:    true,
		Stderr:    true,
	}
	executor, err := s.podExecutor(ns, pod, opts)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: stdin, Stdout: &stdout, Stderr: &stderr}); err != nil {
		if execMissingTool(err) {
			return nil, errNoShell
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", plugin.ErrUnavailable, msg)
		}
		return stdout.Bytes(), apiErr(err)
	}
	return stdout.Bytes(), nil
}

// errNoShell is returned when the container lacks the coreutils the exec-backed
// file browser needs (e.g. a distroless image); kubectl cp fails the same way.
var errNoShell = fmt.Errorf("%w: this container has no shell or file utilities (e.g. a distroless image), so file browsing is unavailable", plugin.ErrNotSupported)

func execMissingTool(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "executable file not found") ||
		(strings.Contains(s, "exec:") && strings.Contains(s, "no such file or directory"))
}

// execStream runs a command and streams its stdout, for downloads.
func (s *Session) execStream(ctx context.Context, ns, pod, container string, command []string) (io.ReadCloser, error) {
	opts := &corev1.PodExecOptions{Container: container, Command: command, Stdout: true}
	executor, err := s.podExecutor(ns, pod, opts)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		_ = pw.CloseWithError(executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: pw}))
	}()
	return pr, nil
}
