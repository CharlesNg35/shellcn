package sshsftp

import (
	"encoding/base64"
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

	"github.com/charlesng35/shellcn/sdk/plugin"
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

type FilePage struct {
	Items      []FileEntry `json:"items"`
	NextCursor string      `json:"nextCursor"`
	Total      *int        `json:"total,omitempty"`
	Path       string      `json:"path"`
}

type snippet struct {
	ID        string              `json:"id"`
	Ref       *plugin.ResourceRef `json:"ref"`
	Name      string              `json:"name"`
	Body      string              `json:"body"`
	CreatedAt time.Time           `json:"createdAt"`
	UpdatedAt time.Time           `json:"updatedAt"`
}

type snippetRequest struct {
	Name string `json:"name" validate:"required"`
	Body string `json:"body" validate:"required"`
}

type snippetRunResult struct {
	OK        bool   `json:"ok"`
	Output    string `json:"output"`
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
		{ID: prefix + ".sftp.move", Method: plugin.MethodPost, Path: "/sftp/move", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.move", Handle: move},
		{ID: prefix + ".sftp.copy", Method: plugin.MethodPost, Path: "/sftp/copy", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.copy", Handle: copyFiles},
		{ID: prefix + ".sftp.chmod", Method: plugin.MethodPost, Path: "/sftp/chmod", Permission: protocol + ".files.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".sftp.chmod", Handle: chmod},
		{ID: prefix + ".sftp.archive", Method: plugin.MethodPost, Path: "/sftp/archive", Permission: protocol + ".files.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".sftp.archive", Handle: archive},
	}
	if includeShell {
		routes = append([]plugin.Route{{
			ID: prefix + ".shell", Method: plugin.MethodWS, Path: "/shell",
			Permission: protocol + ".shell", Risk: plugin.RiskPrivileged,
			AuditEvent: protocol + ".shell", Input: terminalSchema(), Stream: shell,
		}}, routes...)
		routes = append(routes,
			plugin.Route{ID: prefix + ".snippet.list", Method: plugin.MethodGet, Path: "/snippets", Permission: protocol + ".snippets.read", Risk: plugin.RiskSafe, AuditEvent: protocol + ".snippet.list", Handle: snippetList()},
			plugin.Route{ID: prefix + ".snippet.create", Method: plugin.MethodPost, Path: "/snippets", Permission: protocol + ".snippets.write", Risk: plugin.RiskWrite, AuditEvent: protocol + ".snippet.create", Input: snippetSchema(), Handle: snippetCreate()},
			plugin.Route{ID: prefix + ".snippet.run", Method: plugin.MethodPost, Path: "/snippets/{id}/run", Permission: protocol + ".snippets.run", Risk: plugin.RiskPrivileged, AuditEvent: protocol + ".snippet.run", Timeout: 30 * time.Second, Handle: snippetRun()},
			plugin.Route{ID: prefix + ".snippet.delete", Method: plugin.MethodDelete, Path: "/snippets/{id}", Permission: protocol + ".snippets.delete", Risk: plugin.RiskDestructive, AuditEvent: protocol + ".snippet.delete", Handle: snippetDelete()},
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
	return &plugin.Schema{Groups: []plugin.Group{{Name: label, Fields: []plugin.Field{{
		Key: "name", Label: label, Type: plugin.FieldText, Required: true,
		Placeholder: labelNamePlaceholder(label),
		Validators:  []plugin.Validator{{Type: plugin.ValidatorRegex, Value: `^[^/\\]+$`, Message: "Use a single name without slashes."}},
	}}}}}
}

func labelNamePlaceholder(label string) string {
	if label == "Folder" {
		return "new-folder"
	}
	return "new-name.txt"
}

func snippetSchema() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Snippet", Fields: []plugin.Field{
		{Key: "name", Label: "Name", Type: plugin.FieldText, Required: true, Placeholder: "Restart web service"},
		{Key: "body", Label: "Command", Type: plugin.FieldTextarea, Required: true, Placeholder: "sudo systemctl restart nginx"},
	}}}}
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
		errc <- plugin.CopyTerminalInput(ch, client)
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
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
	return pageEntries(p, entries, req), nil
}

func stat(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
	content := FileContent{Path: p, MIME: mimeType, Size: info.Size()}
	if isText(mimeType, buf) {
		content.Encoding = "utf8"
		content.Content = string(buf)
		content.Truncated = info.Size() > int64(n)
		return content, nil
	}
	content.Encoding = "binary"
	return content, nil
}

func download(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
		Inline:  rc.Param("inline") == "1",
		Seeker:  f,
	}, nil
}

func upload(rc *plugin.RequestContext) (any, error) {
	fs, err := fsSession(rc)
	if err != nil {
		return nil, err
	}
	dir, err := resolveRemotePath(fs, rc.Param("path"))
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
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
	dir, err := resolveRemotePath(fs, rc.Param("path"))
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
	p, err := resolveRemotePath(fs, rc.Param("path"))
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
	p, err := resolveRemotePath(fs, rc.Param("path"))
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

func snippetList() plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		snippets := newSnippetStore(rc.Storage)
		if snippets == nil {
			return nil, plugin.ErrNotSupported
		}
		rows, err := snippets.List(rc.Ctx)
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

func snippetCreate() plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		snippets := newSnippetStore(rc.Storage)
		if snippets == nil {
			return nil, plugin.ErrNotSupported
		}
		var req snippetRequest
		if err := rc.Bind(&req); err != nil {
			return nil, err
		}
		now := time.Now()
		sn := storedSnippet{
			ID:        uuid.NewString(),
			Name:      strings.TrimSpace(req.Name),
			Body:      strings.TrimSpace(req.Body),
			CreatedAt: now, UpdatedAt: now,
		}
		if sn.Name == "" || sn.Body == "" {
			return nil, plugin.ErrInvalidInput
		}
		if err := snippets.Create(rc.Ctx, &sn); err != nil {
			return nil, err
		}
		return snippetFromModel(sn), nil
	}
}

func snippetRun() plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		sn, err := ownedSnippet(rc)
		if err != nil {
			return nil, err
		}
		s, err := Unwrap(rc.Session)
		if err != nil {
			return nil, err
		}
		output, truncated, err := s.RunCommand(rc.Ctx, sn.Body)
		if err != nil {
			return nil, err
		}
		return snippetRunResult{OK: true, Output: output, Truncated: truncated}, nil
	}
}

func snippetDelete() plugin.Handler {
	return func(rc *plugin.RequestContext) (any, error) {
		sn, err := ownedSnippet(rc)
		if err != nil {
			return nil, err
		}
		snippets := newSnippetStore(rc.Storage)
		if snippets == nil {
			return nil, plugin.ErrNotSupported
		}
		if err := snippets.Delete(rc.Ctx, sn.ID); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
}

func ownedSnippet(rc *plugin.RequestContext) (storedSnippet, error) {
	snippets := newSnippetStore(rc.Storage)
	if snippets == nil {
		return storedSnippet{}, plugin.ErrNotSupported
	}
	return snippets.Get(rc.Ctx, rc.Param("id"))
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

func pageEntries(currentPath string, entries []FileEntry, req plugin.PageRequest) FilePage {
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
	return FilePage{Items: entries[offset:end], NextCursor: next, Total: &total, Path: currentPath}
}

func pageSnippets(rows []storedSnippet, req plugin.PageRequest) plugin.Page[snippet] {
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

func snippetFromModel(sn storedSnippet) snippet {
	return snippet{
		ID: sn.ID,
		Ref: &plugin.ResourceRef{
			Kind: "snippet",
			Name: sn.Name,
			UID:  sn.ID,
		},
		Name: sn.Name, Body: sn.Body, CreatedAt: sn.CreatedAt, UpdatedAt: sn.UpdatedAt,
	}
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

func resolveRemotePath(fs *sftp.Client, raw string) (string, error) {
	if strings.ContainsRune(raw, 0) {
		return "", fmt.Errorf("%w: invalid path", plugin.ErrInvalidInput)
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "." || raw == "~" {
		return homePath(fs)
	}
	if strings.HasPrefix(raw, "~/") {
		home, err := homePath(fs)
		if err != nil {
			return "", err
		}
		raw = joinRemote(home, strings.TrimPrefix(raw, "~/"))
	}
	clean, err := cleanRemotePath(raw)
	if err != nil {
		return "", err
	}
	return clean, nil
}

func homePath(fs *sftp.Client) (string, error) {
	if home, err := fs.RealPath("."); err == nil && home != "" {
		return cleanRemotePath(home)
	}
	if home, err := fs.Getwd(); err == nil && home != "" {
		return cleanRemotePath(home)
	}
	return "", fmt.Errorf("%w: resolve home directory", plugin.ErrUnavailable)
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
