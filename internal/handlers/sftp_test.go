package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	stdpath "path"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/internal/services"
	shellsftp "github.com/charlesng35/shellcn/internal/sftp"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSFTPHandler_ListSuccess(t *testing.T) {
	client := &stubSFTPClient{
		dirs: map[string][]os.FileInfo{
			"/": {
				fileInfoStub{name: "dir", mode: os.ModeDir | 0o755, modTime: time.Unix(100, 0)},
				fileInfoStub{name: "file.txt", size: 42, mode: 0o644, modTime: time.Unix(200, 0)},
			},
		},
		stats: map[string]os.FileInfo{
			"/dir":      fileInfoStub{name: "dir", mode: os.ModeDir | 0o755, modTime: time.Unix(100, 0)},
			"/file.txt": fileInfoStub{name: "file.txt", size: 42, mode: 0o644, modTime: time.Unix(200, 0)},
		},
	}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn-1"}}
	checker := &stubResourceChecker{allowed: true}

	h := NewSFTPHandler(channels, lifecycle, checker, nil)

	req := httptest.NewRequest(http.MethodGet, "https://example.test/?path=/", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session-1"}}
	c.Set(middleware.CtxUserIDKey, "user-1")

	h.List(c)

	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Success bool             `json:"success"`
		Data    sftpListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	resp := envelope.Data
	require.Equal(t, "/", resp.Path)
	require.Len(t, resp.Entries, 2)
	require.Equal(t, "dir", resp.Entries[0].Name)
	require.True(t, resp.Entries[0].IsDir)
	require.Equal(t, "file.txt", resp.Entries[1].Name)
	require.False(t, resp.Entries[1].IsDir)
}

func TestSFTPHandler_ListInvalidPath(t *testing.T) {
	h := NewSFTPHandler(&stubSFTPChannel{}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "c"}}, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example.invalid/?path=../etc", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "s"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.List(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSFTPHandler_ListPermissionDenied(t *testing.T) {
	client := &stubSFTPClient{}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}
	checker := &stubResourceChecker{allowed: false}

	h := NewSFTPHandler(channels, lifecycle, checker, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=.", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.List(c)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestSFTPHandler_ListBorrowError(t *testing.T) {
	channels := &stubSFTPChannel{err: services.ErrSFTPSessionNotFound}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=.", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.List(c)

	require.Equal(t, http.StatusConflict, w.Code)
}

func TestSFTPHandler_MetadataSuccess(t *testing.T) {
	info := fileInfoStub{name: "file.txt", size: 128, mode: 0o644, modTime: time.Unix(300, 0)}
	client := &stubSFTPClient{
		stats: map[string]os.FileInfo{"/file.txt": info},
	}
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=/file.txt", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.Metadata(c)

	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Success bool         `json:"success"`
		Data    sftpEntryDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, "/file.txt", envelope.Data.Path)
	require.Equal(t, int64(128), envelope.Data.Size)
}

func TestSFTPHandler_ReadFileTooLarge(t *testing.T) {
	large := fileInfoStub{name: "big.dat", size: maxInlineFileBytes + 1, mode: 0o600}
	client := &stubSFTPClient{
		stats: map[string]os.FileInfo{"/big.dat": large},
	}
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=/big.dat", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.ReadFile(c)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestSFTPHandler_ReadFileSuccess(t *testing.T) {
	data := []byte("hello world")
	info := fileInfoStub{name: "file.txt", size: int64(len(data)), mode: 0o644, modTime: time.Unix(123, 0)}
	client := &stubSFTPClient{
		stats: map[string]os.FileInfo{"/file.txt": info},
		files: map[string][]byte{"/file.txt": data},
	}
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=/file.txt", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.ReadFile(c)

	require.Equal(t, http.StatusOK, w.Code)

	var envelope struct {
		Success bool                    `json:"success"`
		Data    sftpFileContentResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
	require.Equal(t, base64.StdEncoding.EncodeToString(data), envelope.Data.Content)
}

func TestSFTPHandler_DownloadSuccess(t *testing.T) {
	data := []byte("download me")
	info := fileInfoStub{name: "file.txt", size: int64(len(data)), mode: 0o644, modTime: time.Unix(400, 0)}
	client := &stubSFTPClient{
		stats: map[string]os.FileInfo{"/file.txt": info},
		files: map[string][]byte{"/file.txt": data},
	}
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/?path=/file.txt", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.Download(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, data, w.Body.Bytes())
	require.Contains(t, w.Header().Get("Content-Disposition"), "file.txt")
}

func TestSFTPHandler_UploadWritesData(t *testing.T) {
	client := &stubSFTPClient{}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodPost, "http://example/upload?path=/file.txt", bytes.NewBufferString("hello"))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.Upload(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "5", w.Header().Get("Upload-Offset"))
	require.Equal(t, []byte("hello"), client.files["/file.txt"])
}

func TestSFTPHandler_SaveFile(t *testing.T) {
	client := &stubSFTPClient{}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	body := `{"path":"/note.txt","content":"aGVsbG8=","encoding":"base64"}`
	req := httptest.NewRequest(http.MethodPut, "http://example/file", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.SaveFile(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []byte("hello"), client.files["/note.txt"])
}

func TestSFTPHandler_Rename(t *testing.T) {
	client := &stubSFTPClient{files: map[string][]byte{"/old.txt": []byte("data")}, stats: map[string]os.FileInfo{"/old.txt": fileInfoStub{name: "old.txt", size: 4}}}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodPost, "http://example/rename", bytes.NewBufferString(`{"source":"/old.txt","target":"/new.txt"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.Rename(c)

	require.Equal(t, http.StatusOK, w.Code)
	_, existsOld := client.files["/old.txt"]
	require.False(t, existsOld)
	require.Equal(t, []byte("data"), client.files["/new.txt"])
}

func TestSFTPHandler_DeleteFile(t *testing.T) {
	client := &stubSFTPClient{files: map[string][]byte{"/temp.txt": []byte("tmp")}}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodDelete, "http://example/file?path=/temp.txt", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.DeleteFile(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, client.files, "/temp.txt")
}

func TestSanitizeSFTPPath(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name      string
		input     string
		expect    string
		expectErr string
	}

	invalidUTF8 := string([]byte{0xff, 0xfe, 0xfd})

	cases := []testCase{
		{name: "empty becomes dot", input: "", expect: "."},
		{name: "whitespace trimmed", input: "   /var/log/ ", expect: "/var/log"},
		{name: "dot returns dot", input: ".", expect: "."},
		{name: "root retained", input: "/", expect: "/"},
		{name: "duplicate slashes collapsed", input: "//home///user", expect: "/home/user"},
		{name: "relative path cleaned", input: "config/app.yaml", expect: "config/app.yaml"},
		{name: "reject parent segments", input: "../etc/passwd", expectErr: "parent directory segments"},
		{name: "reject embedded parent segments", input: "home/../etc", expectErr: "parent directory segments"},
		{name: "reject traversal after clean", input: "/../../etc", expectErr: "parent directory segments"},
		{name: "reject null byte", input: "foo\x00bar", expectErr: "invalid characters"},
		{name: "reject invalid utf8", input: invalidUTF8, expectErr: "valid UTF-8"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			result, err := sanitizeSFTPPath(tc.input)
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expect, result)
		})
	}
}

func TestSFTPHandler_DownloadRange(t *testing.T) {
	client := &stubSFTPClient{
		files: map[string][]byte{"/file.txt": []byte("download")},
		stats: map[string]os.FileInfo{"/file.txt": fileInfoStub{name: "file.txt", size: 8}},
	}
	channels := &stubSFTPChannel{client: client}
	lifecycle := &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true}, nil)

	req := httptest.NewRequest(http.MethodGet, "http://example/download?path=/file.txt", nil)
	req.Header.Set("Range", "bytes=1-3")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{gin.Param{Key: "sessionID", Value: "session"}}
	c.Set(middleware.CtxUserIDKey, "user")

	h.Download(c)

	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, []byte("own"), w.Body.Bytes())
	require.Equal(t, "bytes 1-3/8", w.Header().Get("Content-Range"))
}

func TestTransferEmitter_EmitsLifecycleEvents(t *testing.T) {
	handler := NewSFTPHandler(&stubSFTPChannel{}, &stubSessionAuthorizer{
		session: &models.ConnectionSession{
			BaseModel:    models.BaseModel{ID: "sess-1"},
			ConnectionID: "conn-1",
		},
	}, &stubResourceChecker{allowed: true}, nil)

	var mu sync.Mutex
	var messages []realtime.Message
	handler.broadcast = func(stream string, message realtime.Message) {
		require.Equal(t, realtime.StreamSFTPTransfers, stream)
		mu.Lock()
		messages = append(messages, message)
		mu.Unlock()
	}

	emitter := newTransferEmitter(handler, &models.ConnectionSession{
		BaseModel:    models.BaseModel{ID: "sess-1"},
		ConnectionID: "conn-1",
	}, "user-1", "upload", "/file.txt")
	require.NotNil(t, emitter)

	emitter.progressStep = 1
	emitter.setTotal(3)
	emitter.start()
	emitter.add(1)
	emitter.add(2)
	emitter.complete()

	mu.Lock()
	defer mu.Unlock()

	require.Len(t, messages, 3)

	start := messages[0]
	require.Equal(t, "sftp.transfer.started", start.Event)
	startData, ok := start.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "sess-1", startData["session_id"])
	require.Equal(t, "upload", startData["direction"])
	require.NotEmpty(t, startData["transfer_id"])

	progress := messages[1]
	require.Equal(t, "sftp.transfer.progress", progress.Event)
	progressData, ok := progress.Data.(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 1, progressData["bytes_transferred"], 0.001)
	require.Equal(t, startData["transfer_id"], progressData["transfer_id"])

	completed := messages[2]
	require.Equal(t, "sftp.transfer.completed", completed.Event)
	finalData, ok := completed.Data.(map[string]any)
	require.True(t, ok)
	require.InDelta(t, 3, finalData["bytes_transferred"], 0.001)
	require.Equal(t, startData["transfer_id"], finalData["transfer_id"])
}

type stubSFTPChannel struct {
	client     shellsftp.Client
	releaseErr error
	err        error
	called     string
}

func (s *stubSFTPChannel) Borrow(sessionID string) (shellsftp.Client, func() error, error) {
	s.called = sessionID
	if s.err != nil {
		return nil, nil, s.err
	}
	client := s.client
	if client == nil {
		client = &stubSFTPClient{}
	}
	release := func() error { return s.releaseErr }
	return client, release, nil
}

type stubSessionAuthorizer struct {
	session *models.ConnectionSession
	err     error
}

func (s *stubSessionAuthorizer) AuthorizeSessionAccess(ctx context.Context, sessionID, userID string) (*models.ConnectionSession, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.session, nil
}

type stubResourceChecker struct {
	allowed bool
	err     error
}

func (s *stubResourceChecker) CheckResource(ctx context.Context, userID, resourceType, resourceID, permission string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	return s.allowed, nil
}

type stubSFTPClient struct {
	dirs  map[string][]os.FileInfo
	stats map[string]os.FileInfo
	files map[string][]byte
	err   error
}

func (s *stubSFTPClient) ensureMaps() {
	if s.stats == nil {
		s.stats = make(map[string]os.FileInfo)
	}
	if s.files == nil {
		s.files = make(map[string][]byte)
	}
	if s.dirs == nil {
		s.dirs = make(map[string][]os.FileInfo)
	}
}

func (s *stubSFTPClient) ReadDir(path string) ([]os.FileInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.dirs == nil {
		return nil, nil
	}
	return s.dirs[path], nil
}

func (s *stubSFTPClient) Stat(path string) (os.FileInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.stats == nil {
		return nil, os.ErrNotExist
	}
	info, ok := s.stats[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return info, nil
}

func (s *stubSFTPClient) Open(path string) (shellsftp.ReadableFile, error) {
	if s.err != nil {
		return nil, s.err
	}
	data, ok := s.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &stubReadableFile{Reader: bytes.NewReader(append([]byte(nil), data...))}, nil
}

func (s *stubSFTPClient) OpenFile(path string, flag int) (shellsftp.WritableFile, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.ensureMaps()
	if flag&os.O_TRUNC != 0 {
		s.files[path] = nil
		s.stats[path] = fileInfoStub{name: stdpath.Base(path), mode: 0o644, modTime: time.Now()}
	}
	return &stubWritableFile{client: s, path: path}, nil
}

func (s *stubSFTPClient) Create(path string) (shellsftp.WritableFile, error) {
	return s.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
}

func (s *stubSFTPClient) MkdirAll(path string) error {
	s.ensureMaps()
	return nil
}

func (s *stubSFTPClient) Remove(path string) error {
	if s.err != nil {
		return s.err
	}
	if s.files == nil {
		return os.ErrNotExist
	}
	delete(s.files, path)
	delete(s.stats, path)
	return nil
}

func (s *stubSFTPClient) RemoveDirectory(path string) error {
	if s.err != nil {
		return s.err
	}
	delete(s.dirs, path)
	return nil
}

func (s *stubSFTPClient) Rename(oldPath, newPath string) error {
	if s.err != nil {
		return s.err
	}
	if s.files == nil {
		return os.ErrNotExist
	}
	data, ok := s.files[oldPath]
	if !ok {
		return os.ErrNotExist
	}
	s.files[newPath] = data
	delete(s.files, oldPath)
	if s.stats != nil {
		if info, ok := s.stats[oldPath]; ok {
			s.stats[newPath] = fileInfoStub{name: stdpath.Base(newPath), size: info.Size(), mode: info.Mode(), modTime: time.Now()}
			delete(s.stats, oldPath)
		}
	}
	return nil
}

func (s *stubSFTPClient) Truncate(path string, size int64) error {
	if s.err != nil {
		return s.err
	}
	s.ensureMaps()
	data := s.files[path]
	if int64(len(data)) >= size {
		s.files[path] = data[:size]
	} else {
		extra := make([]byte, size-int64(len(data)))
		s.files[path] = append(data, extra...)
	}
	s.stats[path] = fileInfoStub{name: stdpath.Base(path), size: int64(len(s.files[path])), mode: 0o644, modTime: time.Now()}
	return nil
}

type stubReadableFile struct {
	Reader *bytes.Reader
}

func (s *stubReadableFile) Read(p []byte) (int, error) { return s.Reader.Read(p) }
func (s *stubReadableFile) Close() error               { return nil }
func (s *stubReadableFile) Seek(offset int64, whence int) (int64, error) {
	return s.Reader.Seek(offset, whence)
}

type stubWritableFile struct {
	client *stubSFTPClient
	path   string
	offset int64
}

func (w *stubWritableFile) ensureFile() {
	w.client.ensureMaps()
	if _, ok := w.client.files[w.path]; !ok {
		w.client.files[w.path] = make([]byte, 0)
	}
}

func (w *stubWritableFile) Write(p []byte) (int, error) {
	if _, err := w.Seek(w.offset, io.SeekStart); err != nil {
		return 0, err
	}
	n, err := w.WriteAt(p, w.offset)
	if err != nil {
		return n, err
	}
	w.offset += int64(n)
	return n, nil
}

func (w *stubWritableFile) WriteAt(p []byte, off int64) (int, error) {
	w.ensureFile()
	data := append([]byte(nil), w.client.files[w.path]...)
	required := off + int64(len(p))
	if int64(len(data)) < required {
		expanded := make([]byte, required)
		copy(expanded, data)
		data = expanded
	}
	copy(data[off:off+int64(len(p))], p)
	w.client.files[w.path] = data
	w.client.stats[w.path] = fileInfoStub{name: stdpath.Base(w.path), size: int64(len(data)), mode: 0o644, modTime: time.Now()}
	return len(p), nil
}

func (w *stubWritableFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		w.offset = offset
	case io.SeekCurrent:
		w.offset += offset
	case io.SeekEnd:
		data := w.client.files[w.path]
		w.offset = int64(len(data)) + offset
	default:
		return 0, fmt.Errorf("unsupported whence")
	}
	if w.offset < 0 {
		w.offset = 0
	}
	return w.offset, nil
}

func (w *stubWritableFile) Close() error { return nil }

type fileInfoStub struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

func (f fileInfoStub) Name() string       { return f.name }
func (f fileInfoStub) Size() int64        { return f.size }
func (f fileInfoStub) Mode() os.FileMode  { return f.mode }
func (f fileInfoStub) ModTime() time.Time { return f.modTime }
func (f fileInfoStub) IsDir() bool        { return f.mode.IsDir() }
func (f fileInfoStub) Sys() any           { return nil }
