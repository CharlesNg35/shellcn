package sshsftp

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/sftp"

	"github.com/charlesng35/shellcn/internal/plugin"
)

const (
	archiveMaxEntries = 50000
	archiveMaxBytes   = int64(2) << 30
)

type pathsRequest struct {
	Paths []string `json:"paths"`
}

type destRequest struct {
	Paths []string `json:"paths"`
	Dest  string   `json:"dest"`
}

type chmodRequest struct {
	Paths []string `json:"paths"`
	Mode  string   `json:"mode"`
}

func resolveBulkPaths(raw []string) ([]string, error) {
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

func parseMode(raw string) (os.FileMode, error) {
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

func move(rc *plugin.RequestContext) (any, error) {
	fsc, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	srcs, dest, err := bindDest(rc)
	if err != nil {
		return nil, err
	}
	for _, src := range srcs {
		if err := fsc.Rename(src, joinRemote(dest, path.Base(src))); err != nil {
			return nil, mapFileError(err)
		}
	}
	return map[string]bool{"ok": true}, nil
}

func copyFiles(rc *plugin.RequestContext) (any, error) {
	fsc, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	srcs, dest, err := bindDest(rc)
	if err != nil {
		return nil, err
	}
	for _, src := range srcs {
		if err := copyTree(rc.Ctx, fsc, src, joinRemote(dest, path.Base(src))); err != nil {
			return nil, err
		}
	}
	return map[string]bool{"ok": true}, nil
}

func copyTree(ctx context.Context, fsc *sftp.Client, src, dst string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := fsc.Stat(src)
	if err != nil {
		return mapFileError(err)
	}
	if info.IsDir() {
		if err := fsc.MkdirAll(dst); err != nil {
			return mapFileError(err)
		}
		children, err := fsc.ReadDir(src)
		if err != nil {
			return mapFileError(err)
		}
		for _, child := range children {
			if err := copyTree(ctx, fsc, joinRemote(src, child.Name()), joinRemote(dst, child.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := fsc.Open(src)
	if err != nil {
		return mapFileError(err)
	}
	defer func() { _ = in.Close() }()
	out, err := fsc.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return mapFileError(err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return mapFileError(copyErr)
	}
	if closeErr != nil {
		return mapFileError(closeErr)
	}
	return nil
}

func chmod(rc *plugin.RequestContext) (any, error) {
	fsc, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	var req chmodRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	mode, err := parseMode(req.Mode)
	if err != nil {
		return nil, err
	}
	paths, err := resolveBulkPaths(req.Paths)
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if err := fsc.Chmod(p, mode); err != nil {
			return nil, mapFileError(err)
		}
	}
	return map[string]bool{"ok": true}, nil
}

func bindDest(rc *plugin.RequestContext) (paths []string, dest string, err error) {
	var req destRequest
	if err = rc.Bind(&req); err != nil {
		return nil, "", err
	}
	dest, err = cleanRemotePath(req.Dest)
	if err != nil {
		return nil, "", err
	}
	paths, err = resolveBulkPaths(req.Paths)
	if err != nil {
		return nil, "", err
	}
	return paths, dest, nil
}

func archive(rc *plugin.RequestContext) (any, error) {
	fsc, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	var req pathsRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	paths, err := resolveBulkPaths(req.Paths)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	name := archiveName(paths)
	go func() {
		zw := zip.NewWriter(pw)
		w := &archiveWalker{ctx: rc.Ctx, fs: fsc, zw: zw}
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
	return &plugin.Download{Name: name, MIME: "application/zip", Size: -1, Body: pr}, nil
}

func archiveName(paths []string) string {
	if len(paths) == 1 {
		return path.Base(paths[0]) + ".zip"
	}
	return "archive.zip"
}

type archiveWalker struct {
	ctx     context.Context
	fs      *sftp.Client
	zw      *zip.Writer
	entries int
	bytes   int64
}

func (w *archiveWalker) add(p, base string) error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	info, err := w.fs.Stat(p)
	if err != nil {
		return mapFileError(err)
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
		children, err := w.fs.ReadDir(p)
		if err != nil {
			return mapFileError(err)
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
	f, err := w.fs.Open(p)
	if err != nil {
		return mapFileError(err)
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
