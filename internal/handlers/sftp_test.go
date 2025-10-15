package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
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

	h := NewSFTPHandler(channels, lifecycle, checker)

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
	h := NewSFTPHandler(&stubSFTPChannel{}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "c"}}, &stubResourceChecker{allowed: true})

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

	h := NewSFTPHandler(channels, lifecycle, checker)

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

	h := NewSFTPHandler(channels, lifecycle, &stubResourceChecker{allowed: true})

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
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true})

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
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true})

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
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true})

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
	h := NewSFTPHandler(&stubSFTPChannel{client: client}, &stubSessionAuthorizer{session: &models.ConnectionSession{ConnectionID: "conn"}}, &stubResourceChecker{allowed: true})

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

func (s *stubSFTPClient) Open(path string) (io.ReadCloser, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.files == nil {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	data, ok := s.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

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
