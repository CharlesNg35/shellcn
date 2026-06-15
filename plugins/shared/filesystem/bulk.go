package filesystem

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Optional Client capabilities. The base Client interface stays minimal so every
// backend keeps working; handlers type-assert these and report a clean
// unsupported error when a backend doesn't implement one.

// Mover relocates a path within the backend.
type Mover interface {
	Move(ctx context.Context, src, dst string) error
}

// Copier duplicates a path within the backend.
type Copier interface {
	Copy(ctx context.Context, src, dst string) error
}

// Chmodder changes a path's permission bits.
type Chmodder interface {
	Chmod(ctx context.Context, p string, mode fs.FileMode) error
}

// Bounds for the generic archive walk so a bad selection can't exhaust memory or
// run unbounded.
const (
	archiveMaxEntries = 50000
	archiveMaxBytes   = int64(2) << 30 // 2 GiB
)

func errUnsupported(op string) error {
	return fmt.Errorf("%w: %s is not supported by this backend", plugin.ErrInvalidInput, op)
}

type pathsRequest struct {
	Paths []string `json:"paths"`
}

type chmodRequest struct {
	Paths []string `json:"paths"`
	Mode  string   `json:"mode"`
}

func pathsSchema(groupName string) *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: groupName, Fields: []plugin.Field{
		{
			Key: "paths", Label: "Selection", Type: plugin.FieldArray, Required: true,
			ItemLabel: "Path", AddLabel: "Add path", MinItems: 1,
			Item: &plugin.Field{Type: plugin.FieldText, Required: true, Placeholder: "/path/to/item"},
		},
	}}}}
}

func chmodSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Permissions", Fields: []plugin.Field{
		{
			Key: "paths", Label: "Selection", Type: plugin.FieldArray, Required: true,
			ItemLabel: "Path", AddLabel: "Add path", MinItems: 1,
			Item: &plugin.Field{Type: plugin.FieldText, Required: true, Placeholder: "/path/to/item"},
		},
		{
			Key: "mode", Label: "Octal mode", Type: plugin.FieldAutocomplete, Required: true,
			Placeholder: "0644", Help: "Use a 3 or 4 digit octal mode, such as 0644 for files or 0755 for folders.",
			Options: []plugin.Option{
				{Label: "0644 - owner write, everyone read", Value: "0644"},
				{Label: "0600 - owner read/write only", Value: "0600"},
				{Label: "0755 - executable folder/script", Value: "0755"},
				{Label: "0700 - owner-only folder/script", Value: "0700"},
			},
			Validators: []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^0?[0-7]{3,4}$`, Message: "Enter a 3 or 4 digit octal mode, e.g. 0644."}},
		},
	}}}}
}

// resolvePaths validates and cleans each requested path, rejecting an empty
// selection or any malformed (e.g. traversal) entry.
func resolvePaths(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("%w: no paths provided", plugin.ErrInvalidInput)
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		clean, err := cleanRemotePath(r)
		if err != nil {
			return nil, err
		}
		if clean == "/" {
			return nil, fmt.Errorf("%w: refusing to operate on root", plugin.ErrInvalidInput)
		}
		out = append(out, clean)
	}
	return out, nil
}

// parseMode parses an octal permission string (e.g. "0644" or "755").
func parseMode(raw string) (fs.FileMode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("%w: mode is required", plugin.ErrInvalidInput)
	}
	v, err := strconv.ParseUint(raw, 8, 32)
	if err != nil || v > 0o7777 {
		return 0, fmt.Errorf("%w: invalid octal mode %q", plugin.ErrInvalidInput, raw)
	}
	return fs.FileMode(v), nil
}

func fileJob(rc *plugin.RequestContext, client plugin.ClientStream) error {
	c, err := fsSession(rc)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(client)
	enc := json.NewEncoder(client)
	var mu sync.Mutex
	var cancel context.CancelFunc
	active := false

	writeFrame := func(frame plugin.FileJobFrame) error {
		mu.Lock()
		defer mu.Unlock()
		return enc.Encode(frame)
	}

	for {
		var req plugin.FileJobRequest
		if err := dec.Decode(&req); err != nil {
			if cancel != nil {
				cancel()
			}
			return err
		}
		switch req.Type {
		case plugin.FileJobRequestCancel:
			if cancel != nil {
				cancel()
			}
		case plugin.FileJobRequestStart:
			mu.Lock()
			if active {
				mu.Unlock()
				_ = writeFrame(plugin.FileJobFrame{
					Type:  plugin.FileJobFrameError,
					JobID: req.JobID,
					Error: "Another file job is already running.",
				})
				continue
			}
			var ctx context.Context
			ctx, cancel = context.WithCancel(client.Context())
			active = true
			mu.Unlock()
			go func(req plugin.FileJobRequest) {
				defer func() {
					mu.Lock()
					active = false
					cancel = nil
					mu.Unlock()
				}()
				if err := runFileJob(ctx, c, req, writeFrame); err != nil {
					_ = writeFrame(plugin.FileJobFrame{
						Type:      plugin.FileJobFrameError,
						JobID:     req.JobID,
						Operation: req.Operation,
						Error:     err.Error(),
					})
				}
			}(req)
		}
	}
}

func runFileJob(ctx context.Context, c Client, req plugin.FileJobRequest, writeFrame func(plugin.FileJobFrame) error) error {
	paths, err := resolvePaths(req.Paths)
	if err != nil {
		return err
	}
	dest, err := cleanRemotePath(req.Destination)
	if err != nil {
		return err
	}
	switch plugin.FileJobOperation(req.Operation) {
	case plugin.FileJobMove:
		mover, ok := c.(Mover)
		if !ok {
			return errUnsupported("move")
		}
		return jobEach(ctx, c, mover.Move, req, paths, dest, writeFrame)
	case plugin.FileJobCopy:
		copier, ok := c.(Copier)
		if !ok {
			return errUnsupported("copy")
		}
		return jobEach(ctx, c, copier.Copy, req, paths, dest, writeFrame)
	default:
		return errUnsupported(req.Operation)
	}
}

func jobEach(
	ctx context.Context,
	c Client,
	run func(context.Context, string, string) error,
	req plugin.FileJobRequest,
	paths []string,
	dest string,
	writeFrame func(plugin.FileJobFrame) error,
) error {
	total := len(paths)
	if err := writeFrame(plugin.FileJobFrame{
		Type:       plugin.FileJobFrameStatus,
		JobID:      req.JobID,
		Operation:  req.Operation,
		Status:     "Starting",
		FilesTotal: total,
	}); err != nil {
		return err
	}
	for i, src := range paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		dst := joinRemote(dest, path.Base(src))
		pct := float64(i) / float64(total) * 100
		if err := writeFrame(plugin.FileJobFrame{
			Type:       plugin.FileJobFrameProgress,
			JobID:      req.JobID,
			Operation:  req.Operation,
			Status:     jobOperationLabel(req.Operation),
			Path:       src,
			Percent:    &pct,
			FilesDone:  i,
			FilesTotal: total,
			Message:    fmt.Sprintf("%s %s", jobOperationLabel(req.Operation), src),
		}); err != nil {
			return err
		}
		if err := run(ctx, src, dst); err != nil {
			return mapClientError(c, err)
		}
	}
	done := 100.0
	return writeFrame(plugin.FileJobFrame{
		Type:       plugin.FileJobFrameComplete,
		JobID:      req.JobID,
		Operation:  req.Operation,
		Status:     "Complete",
		Percent:    &done,
		FilesDone:  total,
		FilesTotal: total,
		Message:    "Job complete.",
	})
}

func jobOperationLabel(operation string) string {
	switch plugin.FileJobOperation(operation) {
	case plugin.FileJobMove:
		return "Move"
	case plugin.FileJobCopy:
		return "Copy"
	default:
		if operation == "" {
			return "File operation"
		}
		return operation
	}
}

func chmod(rc *plugin.RequestContext) (any, error) {
	c, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	chmodder, ok := c.(Chmodder)
	if !ok {
		return nil, errUnsupported("chmod")
	}
	var req chmodRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	mode, err := parseMode(req.Mode)
	if err != nil {
		return nil, err
	}
	paths, err := resolvePaths(req.Paths)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if err := chmodder.Chmod(rc.Ctx, p, mode); err != nil {
			return nil, mapClientError(c, err)
		}
	}
	return map[string]bool{"ok": true}, nil
}

// archive streams a zip built generically over the base Client (Stat/ReadDir/
// Open), so it works for any backend without a capability.
func archive(rc *plugin.RequestContext) (any, error) {
	c, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	var req pathsRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	paths, err := resolvePaths(req.Paths)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	name := archiveName(paths)
	go func() {
		zw := zip.NewWriter(pw)
		w := &archiveWalker{ctx: rc.Ctx, client: c, zw: zw}
		var werr error
		for _, p := range paths {
			if werr = w.add(p, path.Dir(p)); werr != nil {
				break
			}
		}
		if werr == nil {
			werr = zw.Close()
		} else {
			_ = zw.Close()
		}
		_ = pw.CloseWithError(werr)
	}()
	return &plugin.Download{
		Name: name,
		MIME: "application/zip",
		Size: -1,
		Body: pr,
	}, nil
}

func archiveName(paths []string) string {
	if len(paths) == 1 {
		return path.Base(paths[0]) + ".zip"
	}
	return "archive.zip"
}

type archiveWalker struct {
	ctx     context.Context
	client  Client
	zw      *zip.Writer
	entries int
	bytes   int64
}

// add zips p (a file or directory tree). base is stripped so the zip holds names
// relative to the selection root.
func (w *archiveWalker) add(p, base string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	info, err := w.client.Stat(w.ctx, p)
	if err != nil {
		return mapClientError(w.client, err)
	}
	rel := zipName(p, base)
	if rel == "" {
		return nil
	}
	w.entries++
	if w.entries > archiveMaxEntries {
		return fmt.Errorf("%w: archive exceeds %d entries", plugin.ErrInvalidInput, archiveMaxEntries)
	}
	if info.IsDir() {
		if _, err := w.zw.Create(rel + "/"); err != nil {
			return err
		}
		children, err := w.client.ReadDir(w.ctx, p)
		if err != nil {
			return mapClientError(w.client, err)
		}
		for _, child := range children {
			if err := w.add(joinRemote(p, child.Name()), base); err != nil {
				return err
			}
		}
		return nil
	}
	w.bytes += info.Size()
	if w.bytes > archiveMaxBytes {
		return fmt.Errorf("%w: archive exceeds size limit", plugin.ErrInvalidInput)
	}
	f, err := w.client.Open(w.ctx, p)
	if err != nil {
		return mapClientError(w.client, err)
	}
	defer func() { _ = f.Close() }()
	hw, err := w.zw.CreateHeader(&zip.FileHeader{Name: rel, Method: zip.Deflate, Modified: info.ModTime()})
	if err != nil {
		return err
	}
	_, err = io.Copy(hw, f)
	return err
}

func zipName(p, base string) string {
	rel := strings.TrimPrefix(p, strings.TrimSuffix(base, "/")+"/")
	rel = strings.TrimPrefix(rel, "/")
	return strings.TrimPrefix(rel, "./")
}
