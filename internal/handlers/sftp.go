package handlers

import (
	"context"
	"errors"
	"net/http"
	"os"
	stdpath "path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	pkgsftp "github.com/pkg/sftp"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/services"
	shellsftp "github.com/charlesng35/shellcn/internal/sftp"
	apperrors "github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// SFTPHandler exposes active-session SFTP operations.
type SFTPHandler struct {
	channels  sftpChannelBorrower
	lifecycle sessionAuthorizer
	checker   resourceChecker
}

type sftpChannelBorrower interface {
	Borrow(sessionID string) (shellsftp.Client, func() error, error)
}

type sessionAuthorizer interface {
	AuthorizeSessionAccess(ctx context.Context, sessionID, userID string) (*models.ConnectionSession, error)
}

type resourceChecker interface {
	CheckResource(ctx context.Context, userID, resourceType, resourceID, permission string) (bool, error)
}

// NewSFTPHandler constructs a handler when dependencies are supplied.
func NewSFTPHandler(channels sftpChannelBorrower, lifecycle sessionAuthorizer, checker resourceChecker) *SFTPHandler {
	return &SFTPHandler{
		channels:  channels,
		lifecycle: lifecycle,
		checker:   checker,
	}
}

// List enumerates entries within the requested directory path.
func (h *SFTPHandler) List(c *gin.Context) {
	if h == nil || h.channels == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return
	}

	session, err := h.lifecycle.AuthorizeSessionAccess(c.Request.Context(), sessionID, userID)
	if err != nil {
		h.handleLifecycleError(c, err)
		return
	}
	if session.ClosedAt != nil {
		response.Error(c, apperrors.NewBadRequest("session is no longer active"))
		return
	}

	if h.checker != nil {
		allowed, checkErr := h.checker.CheckResource(c.Request.Context(), userID, "connection", session.ConnectionID, "protocol:ssh.sftp")
		if checkErr != nil {
			response.Error(c, apperrors.Wrap(checkErr, "sftp permission check"))
			return
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return
		}
	}

	client, release, err := h.channels.Borrow(sessionID)
	if err != nil {
		h.handleBorrowError(c, err)
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	entries, err := client.ReadDir(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}

	dtos := make([]sftpEntryDTO, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		dtos = append(dtos, toSFTPEntryDTO(cleanPath, entry))
	}

	sort.SliceStable(dtos, func(i, j int) bool {
		if dtos[i].IsDir == dtos[j].IsDir {
			return strings.ToLower(dtos[i].Name) < strings.ToLower(dtos[j].Name)
		}
		return dtos[i].IsDir && !dtos[j].IsDir
	})

	response.Success(c, http.StatusOK, sftpListResponse{
		Path:    cleanPath,
		Entries: dtos,
	})
}

func (h *SFTPHandler) handleLifecycleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrSessionNotFound):
		response.Error(c, apperrors.ErrNotFound)
	case errors.Is(err, services.ErrSessionAccessDenied):
		response.Error(c, apperrors.ErrForbidden)
	default:
		response.Error(c, apperrors.Wrap(err, "session authorization failed"))
	}
}

func (h *SFTPHandler) handleBorrowError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrSFTPSessionNotFound):
		response.Error(c, apperrors.New("session.sftp_unavailable", "SFTP channel is not ready", http.StatusConflict).WithInternal(err))
	case errors.Is(err, services.ErrSFTPProviderInvalid):
		response.Error(c, apperrors.NewBadRequest("SFTP channel is not available"))
	default:
		response.Error(c, apperrors.Wrap(err, "acquire sftp channel"))
	}
}

func sanitizeSFTPPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ".", nil
	}
	if !utf8.ValidString(trimmed) {
		return "", errors.New("path must be valid UTF-8")
	}
	if strings.ContainsRune(trimmed, '\x00') {
		return "", errors.New("path contains invalid characters")
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", errors.New("parent directory segments are not allowed")
		}
	}
	cleaned := stdpath.Clean(trimmed)
	if strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", errors.New("path escapes session root")
	}
	if cleaned == "" {
		return ".", nil
	}
	return cleaned, nil
}

type sftpListResponse struct {
	Path    string         `json:"path"`
	Entries []sftpEntryDTO `json:"entries"`
}

type sftpEntryDTO struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Type       string    `json:"type"`
	IsDir      bool      `json:"is_dir"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModifiedAt time.Time `json:"modified_at"`
}

func toSFTPEntryDTO(base string, info os.FileInfo) sftpEntryDTO {
	entryPath := info.Name()
	switch base {
	case "", ".":
		entryPath = info.Name()
	case "/":
		entryPath = stdpath.Clean("/" + info.Name())
	default:
		entryPath = stdpath.Clean(stdpath.Join(base, info.Name()))
	}

	mode := info.Mode()
	entryType := "file"
	if mode.IsDir() {
		entryType = "directory"
	} else if mode&os.ModeSymlink != 0 {
		entryType = "symlink"
	}

	return sftpEntryDTO{
		Name:       info.Name(),
		Path:       entryPath,
		Type:       entryType,
		IsDir:      mode.IsDir(),
		Size:       info.Size(),
		Mode:       mode.String(),
		ModifiedAt: info.ModTime(),
	}
}

func mapSFTPError(err error) error {
	if err == nil {
		return nil
	}
	var statusErr *pkgsftp.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.FxCode() {
		case pkgsftp.ErrSSHFxNoSuchFile:
			return apperrors.ErrNotFound
		case pkgsftp.ErrSSHFxPermissionDenied:
			return apperrors.ErrForbidden
		default:
			return apperrors.New("sftp.error", statusErr.Error(), http.StatusBadGateway).WithInternal(err)
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return apperrors.ErrNotFound
	}
	if errors.Is(err, os.ErrPermission) {
		return apperrors.ErrForbidden
	}
	return apperrors.Wrap(err, "sftp operation failed")
}
