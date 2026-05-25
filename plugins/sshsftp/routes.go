package sshsftp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/pkg/sftp"

	"github.com/charlesng/shellcn/internal/models"
	"github.com/charlesng/shellcn/internal/plugin"
)

const (
	previewLimit = 1 << 20
)

// FileEntry is the generic file_browser listing row.
type FileEntry struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"isDir"`
	Size    int64     `json:"size,omitempty"`
	MIME    string    `json:"mime,omitempty"`
	ModTime time.Time `json:"modTime,omitzero"`
	Mode    string    `json:"mode,omitempty"`
	Symlink string    `json:"symlink,omitempty"`
}

// FileContent is the generic file_browser preview payload.
type FileContent struct {
	Path      string `json:"path"`
	MIME      string `json:"mime,omitempty"`
	Encoding  string `json:"encoding,omitempty"`
	Content   string `json:"content,omitempty"`
	URL       string `json:"url,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Routes returns the shared SFTP route handlers using the provided route prefix.
func Routes(prefix, protocol string, includeShell bool) []plugin.Route {
	routes := []plugin.Route{
		{ID: prefix + ".sftp.list", Method: plugin.MethodGet, Path: "/sftp/list/{path}", Permission: protocol + ".files.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".sftp.list", Handle: list},
		{ID: prefix + ".sftp.read", Method: plugin.MethodGet, Path: "/sftp/read/{path}", Permission: protocol + ".files.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".sftp.read", Handle: read},
		{ID: prefix + ".sftp.download", Method: plugin.MethodGet, Path: "/sftp/download/{path}", Permission: protocol + ".files.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".sftp.download", Handle: download},
		{ID: prefix + ".sftp.stat", Method: plugin.MethodGet, Path: "/sftp/stat/{path}", Permission: protocol + ".files.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".sftp.stat", Handle: stat},
		{ID: prefix + ".sftp.write", Method: plugin.MethodPut, Path: "/sftp/write/{path}", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.write", Input: writeSchema(), Handle: writeFile},
		{ID: prefix + ".sftp.upload", Method: plugin.MethodPost, Path: "/sftp/upload/{path}", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.upload", Input: uploadSchema(), Handle: upload},
		{ID: prefix + ".sftp.mkdir", Method: plugin.MethodPost, Path: "/sftp/mkdir/{path}", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.mkdir", Input: nameSchema("Folder"), Handle: mkdir},
		{ID: prefix + ".sftp.rename", Method: plugin.MethodPatch, Path: "/sftp/rename/{path}", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.rename", Input: nameSchema("Name"), Handle: renameEntry},
		{ID: prefix + ".sftp.delete", Method: plugin.MethodDelete, Path: "/sftp/delete/{path}", Permission: protocol + ".files.write", Risk: plugin.RiskDestructive, AuditEvent: protocol + ".sftp.delete", Handle: deleteEntry},
		{ID: prefix + ".snippet.list", Method: plugin.MethodGet, Path: "/snippets", Permission: protocol + ".snippets.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".snippet.list", Handle: snippetList(protocol)},
		{ID: prefix + ".snippet.create", Method: plugin.MethodPost, Path: "/snippets", Permission: protocol + ".snippets.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".snippet.create", Input: snippetSchema(), Handle: snippetCreate(protocol)},
	}
	if includeShell {
		routes = append([]plugin.Route{{
			ID: prefix + ".shell", Method: plugin.MethodWS, Path: "/shell",
			Permission: protocol + ".shell", Risk: plugin.RiskPrivileged,
			AuditEvent: protocol + ".shell", Input: terminalSchema(), Stream: shell,
		}}, routes...)
		routes = append(routes,
			plugin.Route{ID: prefix + ".tunnel.list", Method: plugin.MethodGet, Path: "/tunnels", Permission: protocol + ".tunnels.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".tunnel.list", Handle: tunnelList},
			plugin.Route{ID: prefix + ".tunnel.open", Method: plugin.MethodPost, Path: "/tunnels", Permission: protocol + ".tunnels.open", Risk: plugin.RiskPrivileged, AuditEvent: protocol + ".tunnel.open", Input: tunnelSchema(), Handle: tunnelOpen},
			plugin.Route{ID: prefix + ".tunnel.close", Method: plugin.MethodDelete, Path: "/tunnels/{id}", Permission: protocol + ".tunnels.close", Risk: plugin.RiskPrivileged, AuditEvent: protocol + ".tunnel.close", Handle: tunnelClose},
		)
	}
	return routes
}

func terminalSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Terminal", Fields: []plugin.Field{
		{Key: "cols", Label: "Columns", Type: plugin.FieldNumber},
		{Key: "rows", Label: "Rows", Type: plugin.FieldNumber},
	}}}}
}

func uploadSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Upload", Fields: []plugin.Field{{Key: "files", Label: "Files", Type: plugin.FieldFile, Required: true}}}}}
}

func writeSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Content", Fields: []plugin.Field{{Key: "content", Label: "Content", Type: plugin.FieldTextarea, Required: true}}}}}
}

func nameSchema(label string) *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: label, Fields: []plugin.Field{{Key: "name", Label: label, Type: plugin.FieldText, Required: true}}}}}
}

func snippetSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Snippet", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "body", Label: "Command", Type: plugin.FieldTextarea, Required: true},
	}}}}
}

func tunnelSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Tunnel", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true},
		{Key: "listen", Label: "Listen address", Type: plugin.FieldText, Required: true},
		{Key: "target", Label: "Target address", Type: plugin.FieldText, Required: true},
	}}}}
}

type tunnelRequest struct {
	Name   string `json:"name" validate:"required"`
	Listen string `json:"listen" validate:"required"`
	Target string `json:"target" validate:"required"`
}

func shell(rc *plugin.RequestContext, client plugin.ClientStream) error {
	ch, err := rc.Session.OpenChannel(rc.Ctx, plugin.ChannelRequest{Kind: plugin.StreamTerminal, Params: terminalParams(rc.Query())})
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(client, ch)
		errc <- err
	}()
	go func() {
		errc <- copyTerminalInput(ch, client)
	}()
	select {
	case <-client.Context().Done():
		return nil
	case err := <-errc:
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func terminalParams(q url.Values) map[string]string {
	params := map[string]string{}
	for _, key := range []string{"cols", "rows"} {
		if v := q.Get(key); v != "" {
			params[key] = v
		}
	}
	return params
}

type resizer interface {
	Resize(cols, rows int) error
}

func copyTerminalInput(ch plugin.Channel, client plugin.ClientStream) error {
	buf := make([]byte, 32<<10)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			frame := buf[:n]
			if len(frame) > 1 && frame[0] == 0 {
				_ = handleTerminalControl(ch, frame[1:])
			} else if _, werr := ch.Write(frame); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func handleTerminalControl(ch plugin.Channel, frame []byte) error {
	var msg struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(frame, &msg); err != nil || msg.Type != "resize" {
		return err
	}
	if r, ok := ch.(resizer); ok {
		return r.Resize(msg.Cols, msg.Rows)
	}
	return nil
}

func fsSession(rc *plugin.RequestContext) (*sftp.Client, error) {
	s, err := Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	return s.Filesystem()
}

func list(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	infos, err := fs.ReadDirContext(rc.Ctx, p)
	if err != nil {
		return nil, mapFileError(err)
	}
	entries := make([]FileEntry, 0, len(infos))
	for _, info := range infos {
		entryPath := joinRemote(p, info.Name())
		entry := fileEntry(entryPath, info)
		if info.Mode()&os.ModeSymlink != 0 {
			if target, err := fs.ReadLink(entryPath); err == nil {
				entry.Symlink = target
			}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	return pageEntries(entries, req), nil
}

func stat(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	info, err := fs.Lstat(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	return fileEntry(p, info), nil
}

func read(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	info, err := fs.Stat(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	if info.IsDir() {
		return nil, plugin.ErrInvalidInput
	}
	f, err := fs.Open(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	defer func() { _ = f.Close() }()
	limit := previewLimit
	if info.Size() < int64(limit) {
		limit = int(info.Size())
	}
	buf := make([]byte, limit)
	n, rerr := io.ReadFull(f, buf)
	if rerr != nil && rerr != io.ErrUnexpectedEOF && rerr != io.EOF {
		return nil, mapFileError(rerr)
	}
	buf = buf[:n]
	mimeType := mimeFor(p)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	content := FileContent{Path: p, MIME: mimeType, Size: info.Size(), Truncated: info.Size() > int64(n)}
	if isText(mimeType, buf) {
		content.Encoding = "utf8"
		content.Content = string(buf)
		return content, nil
	}
	content.Encoding = "base64"
	content.Content = base64.StdEncoding.EncodeToString(buf)
	return content, nil
}

func download(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	info, err := fs.Stat(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	if info.IsDir() {
		return nil, plugin.ErrInvalidInput
	}
	f, err := fs.Open(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	return &plugin.Download{
		Name:    path.Base(p),
		MIME:    mimeFor(p),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Body:    f,
	}, nil
}

func upload(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	dir, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	files := rc.Uploads("files")
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no files uploaded", plugin.ErrInvalidInput)
	}
	for _, file := range files {
		name, err := cleanName(file.Filename)
		if err != nil {
			return nil, err
		}
		src, err := file.Open()
		if err != nil {
			return nil, mapFileError(err)
		}
		dst, err := fs.OpenFile(joinRemote(dir, name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			_ = src.Close()
			return nil, mapFileError(err)
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		_ = src.Close()
		if copyErr != nil {
			return nil, mapFileError(copyErr)
		}
		if closeErr != nil {
			return nil, mapFileError(closeErr)
		}
	}
	return map[string]bool{"ok": true}, nil
}

type writeRequest struct {
	Content string `json:"content"`
}

func writeFile(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	var req writeRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	dst, err := fs.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return nil, mapFileError(err)
	}
	if _, err := io.WriteString(dst, req.Content); err != nil {
		_ = dst.Close()
		return nil, mapFileError(err)
	}
	if err := dst.Close(); err != nil {
		return nil, mapFileError(err)
	}
	return map[string]bool{"ok": true}, nil
}

type nameRequest struct {
	Name string `json:"name" validate:"required"`
}

func mkdir(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	dir, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	var req nameRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := cleanName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := fs.Mkdir(joinRemote(dir, name)); err != nil {
		return nil, mapFileError(err)
	}
	return map[string]bool{"ok": true}, nil
}

func renameEntry(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	var req nameRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	name, err := cleanName(req.Name)
	if err != nil {
		return nil, err
	}
	if err := fs.Rename(p, joinRemote(path.Dir(p), name)); err != nil {
		return nil, mapFileError(err)
	}
	return map[string]bool{"ok": true}, nil
}

func deleteEntry(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := cleanRemotePath(rc.Param("path"))
	if err != nil {
		return nil, err
	}
	info, err := fs.Lstat(p)
	if err != nil {
		return nil, mapFileError(err)
	}
	if info.IsDir() {
		err = fs.RemoveDirectory(p)
	} else {
		err = fs.Remove(p)
	}
	if err != nil {
		return nil, mapFileError(err)
	}
	return map[string]bool{"ok": true}, nil
}

func snippetList(protocol string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		if rc.Snippets == nil {
			return nil, plugin.ErrNotSupported
		}
		rows, err := rc.Snippets.ListByOwner(rc.Ctx, rc.User.ID, protocol)
		if err != nil {
			return nil, err
		}
		req, err := rc.Page()
		if err != nil {
			return nil, err
		}
		return pageSnippets(rows, req), nil
	}
}

type snippet struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type snippetRequest struct {
	Name string `json:"name" validate:"required"`
	Body string `json:"body" validate:"required"`
}

func snippetCreate(protocol string) plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		if rc.Snippets == nil {
			return nil, plugin.ErrNotSupported
		}
		var req snippetRequest
		if err := rc.Bind(&req); err != nil {
			return nil, err
		}
		now := time.Now()
		sn := models.Snippet{
			ID: uuid.NewString(), OwnerID: rc.User.ID, Protocol: protocol,
			Name: strings.TrimSpace(req.Name), Body: req.Body,
			CreatedAt: now, UpdatedAt: now,
		}
		if sn.Name == "" || strings.TrimSpace(sn.Body) == "" {
			return nil, plugin.ErrInvalidInput
		}
		if err := rc.Snippets.Create(rc.Ctx, &sn); err != nil {
			return nil, err
		}
		return snippetFromModel(sn), nil
	}
}

func tunnelList(rc *plugin.RequestContext) (any, error) {
	s, err := Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	rows := s.ListTunnels()
	req, err := rc.Page()
	if err != nil {
		return nil, err
	}
	offset := cursorOffset(req.Cursor)
	if offset < 0 || offset > len(rows) {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = plugin.DefaultPageLimit
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	next := ""
	if end < len(rows) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	total := len(rows)
	return plugin.Page[Tunnel]{Items: rows[offset:end], NextCursor: next, Total: &total}, nil
}

func tunnelOpen(rc *plugin.RequestContext) (any, error) {
	s, err := Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	var req tunnelRequest
	if err := rc.Bind(&req); err != nil {
		return nil, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Listen = strings.TrimSpace(req.Listen)
	req.Target = strings.TrimSpace(req.Target)
	if req.Name == "" || req.Listen == "" || req.Target == "" {
		return nil, plugin.ErrInvalidInput
	}
	t, err := s.OpenTunnel(uuid.NewString(), req.Name, req.Listen, req.Target)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func tunnelClose(rc *plugin.RequestContext) (any, error) {
	s, err := Unwrap(rc.Session)
	if err != nil {
		return nil, err
	}
	if err := s.CloseTunnel(rc.Param("id")); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func fileEntry(p string, info os.FileInfo) FileEntry {
	return FileEntry{
		Name:    info.Name(),
		Path:    p,
		IsDir:   info.IsDir(),
		Size:    info.Size(),
		MIME:    mimeFor(p),
		ModTime: info.ModTime(),
		Mode:    info.Mode().String(),
	}
}

func pageEntries(entries []FileEntry, req plugin.PageRequest) plugin.Page[FileEntry] {
	offset := 0
	if req.Cursor != "" {
		if raw, err := base64.RawURLEncoding.DecodeString(req.Cursor); err == nil {
			offset, _ = strconv.Atoi(string(raw))
		}
	}
	if offset < 0 || offset > len(entries) {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = plugin.DefaultPageLimit
	}
	end := offset + limit
	if end > len(entries) {
		end = len(entries)
	}
	next := ""
	if end < len(entries) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	total := len(entries)
	return plugin.Page[FileEntry]{Items: entries[offset:end], NextCursor: next, Total: &total}
}

func pageSnippets(rows []models.Snippet, req plugin.PageRequest) plugin.Page[snippet] {
	offset := cursorOffset(req.Cursor)
	if offset < 0 || offset > len(rows) {
		offset = 0
	}
	limit := req.Limit
	if limit <= 0 {
		limit = plugin.DefaultPageLimit
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	next := ""
	if end < len(rows) {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(end)))
	}
	items := make([]snippet, 0, end-offset)
	for _, row := range rows[offset:end] {
		items = append(items, snippetFromModel(row))
	}
	total := len(rows)
	return plugin.Page[snippet]{Items: items, NextCursor: next, Total: &total}
}

func snippetFromModel(sn models.Snippet) snippet {
	return snippet{ID: sn.ID, Name: sn.Name, Body: sn.Body, CreatedAt: sn.CreatedAt, UpdatedAt: sn.UpdatedAt}
}

func cursorOffset(cursor string) int {
	if cursor == "" {
		return 0
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(string(raw))
	if err != nil {
		return 0
	}
	return n
}

func cleanRemotePath(raw string) (string, error) {
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("%w: invalid path", plugin.ErrInvalidInput)
	}
	if strings.TrimSpace(raw) == "" {
		raw = "/"
	}
	clean := path.Clean("/" + strings.TrimPrefix(raw, "/"))
	if clean == "." {
		clean = "/"
	}
	return clean, nil
}

func cleanName(raw string) (string, error) {
	name := path.Base(strings.TrimSpace(raw))
	if name == "." || name == "/" || name == "" || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: invalid name", plugin.ErrInvalidInput)
	}
	return name, nil
}

func joinRemote(dir, name string) string {
	if dir == "/" {
		return "/" + name
	}
	return path.Join(dir, name)
}

func mimeFor(p string) string {
	return mime.TypeByExtension(strings.ToLower(path.Ext(p)))
}

func isText(mimeType string, buf []byte) bool {
	return strings.HasPrefix(mimeType, "text/") ||
		strings.Contains(mimeType, "json") ||
		strings.Contains(mimeType, "xml") ||
		strings.Contains(mimeType, "yaml") ||
		utf8.Valid(buf)
}

func mapFileError(err error) error {
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return plugin.ErrNotFound
	}
	if os.IsPermission(err) {
		return plugin.ErrForbidden
	}
	return fmt.Errorf("%w: %v", plugin.ErrUnavailable, err)
}
