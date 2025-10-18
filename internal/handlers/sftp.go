package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	stdpath "path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	pkgsftp "github.com/pkg/sftp"

	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/realtime"
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
	hub       *realtime.Hub
	broadcast func(stream string, message realtime.Message)
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
func NewSFTPHandler(channels sftpChannelBorrower, lifecycle sessionAuthorizer, checker resourceChecker, hub *realtime.Hub) *SFTPHandler {
	handler := &SFTPHandler{
		channels:  channels,
		lifecycle: lifecycle,
		checker:   checker,
		hub:       hub,
	}
	if hub != nil {
		handler.broadcast = hub.BroadcastStream
	}
	return handler
}

const maxInlineFileBytes = 5 * 1024 * 1024
const maxUploadChunkBytes = 64 * 1024 * 1024
const transferProgressChunk = 256 * 1024
const maxDeleteDepth = 10

type transferEmitter struct {
	handler      *SFTPHandler
	sessionID    string
	connectionID string
	userID       string
	direction    string
	path         string
	transferID   string
	totalBytes   int64
	progressStep int64
	transferred  int64
	lastProgress int64
	completed    bool
}

func newTransferEmitter(handler *SFTPHandler, session *models.ConnectionSession, userID, direction, path string) *transferEmitter {
	if handler == nil || session == nil {
		return nil
	}
	emitter := &transferEmitter{
		handler:      handler,
		sessionID:    session.ID,
		connectionID: session.ConnectionID,
		userID:       userID,
		direction:    direction,
		path:         path,
		transferID:   uuid.NewString(),
		totalBytes:   -1,
		progressStep: transferProgressChunk,
	}
	return emitter
}

func (e *transferEmitter) setTotal(total int64) {
	if e == nil {
		return
	}
	e.totalBytes = total
}

func (e *transferEmitter) start() {
	if e == nil {
		return
	}
	e.emit("sftp.transfer.started", map[string]any{
		"status":            "started",
		"total_bytes":       e.totalBytes,
		"bytes_transferred": int64(0),
	})
}

func (e *transferEmitter) add(delta int64) {
	if e == nil || delta <= 0 {
		return
	}
	e.transferred += delta
	if e.transferred == e.totalBytes {
		e.lastProgress = e.transferred
		return
	}
	if e.progressStep > 0 && (e.transferred-e.lastProgress >= e.progressStep) {
		e.emit("sftp.transfer.progress", map[string]any{
			"status":            "progress",
			"bytes_transferred": e.transferred,
			"total_bytes":       e.totalBytes,
		})
		e.lastProgress = e.transferred
	}
}

func (e *transferEmitter) complete() {
	if e == nil || e.completed {
		return
	}
	e.completed = true
	e.emit("sftp.transfer.completed", map[string]any{
		"status":            "completed",
		"bytes_transferred": e.transferred,
		"total_bytes":       e.totalBytes,
	})
}

func (e *transferEmitter) fail(err error) {
	if e == nil || e.completed {
		return
	}
	e.completed = true
	message := ""
	if err != nil {
		message = err.Error()
	}
	e.emit("sftp.transfer.failed", map[string]any{
		"status":            "failed",
		"bytes_transferred": e.transferred,
		"total_bytes":       e.totalBytes,
		"error":             message,
	})
}

func (e *transferEmitter) emit(event string, data map[string]any) {
	if e == nil || e.handler == nil {
		return
	}
	payload := map[string]any{
		"session_id":    e.sessionID,
		"connection_id": e.connectionID,
		"user_id":       e.userID,
		"path":          e.path,
		"direction":     e.direction,
		"transfer_id":   e.transferID,
	}
	for k, v := range data {
		payload[k] = v
	}
	e.handler.emitTransferEvent(event, payload)
}

// List enumerates entries within the requested directory path.
func (h *SFTPHandler) List(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	// Resolve to absolute path
	absPath, err := client.RealPath(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
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
		dtos = append(dtos, toSFTPEntryDTO(absPath, entry))
	}

	sort.SliceStable(dtos, func(i, j int) bool {
		if dtos[i].IsDir == dtos[j].IsDir {
			return strings.ToLower(dtos[i].Name) < strings.ToLower(dtos[j].Name)
		}
		return dtos[i].IsDir && !dtos[j].IsDir
	})

	response.Success(c, http.StatusOK, sftpListResponse{
		Path:    absPath,
		Entries: dtos,
	})
}

// Metadata returns stat information for a single file or directory.
func (h *SFTPHandler) Metadata(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	info, err := client.Stat(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}

	entry := toSFTPEntryDTO(stdpath.Dir(cleanPath), info)
	response.Success(c, http.StatusOK, entry)
}

// ReadFile fetches file contents for inline editing subject to a size cap.
func (h *SFTPHandler) ReadFile(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	info, err := client.Stat(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	if info.IsDir() {
		response.Error(c, apperrors.NewBadRequest("requested path is a directory"))
		return
	}
	if info.Size() > maxInlineFileBytes {
		response.Error(c, apperrors.New("sftp.file_too_large", fmt.Sprintf("file exceeds %d bytes inline limit", maxInlineFileBytes), http.StatusRequestEntityTooLarge))
		return
	}

	reader, err := client.Open(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		response.Error(c, apperrors.Wrap(err, "read sftp file"))
		return
	}

	payload := sftpFileContentResponse{
		Entry:    toSFTPEntryDTO(stdpath.Dir(cleanPath), info),
		Encoding: "base64",
		Content:  base64.StdEncoding.EncodeToString(data),
	}

	response.Success(c, http.StatusOK, payload)
}

type saveFileRequest struct {
	Path          string `json:"path"`
	Content       string `json:"content"`
	Encoding      string `json:"encoding"`
	CreateParents bool   `json:"create_parents"`
}

// SaveFile replaces the target file with the provided inline content.
func (h *SFTPHandler) SaveFile(c *gin.Context) {
	client, release, session, userID, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	var payload saveFileRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid save payload"))
		return
	}

	cleanPath, err := sanitizeSFTPPath(payload.Path)
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if isRootPath(cleanPath) {
		response.Error(c, apperrors.NewBadRequest("cannot overwrite root path"))
		return
	}

	data, err := decodeContent(payload.Content, payload.Encoding)
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if int64(len(data)) > maxInlineFileBytes {
		response.Error(c, apperrors.New("sftp.payload_too_large", fmt.Sprintf("file exceeds %d bytes inline limit", maxInlineFileBytes), http.StatusRequestEntityTooLarge))
		return
	}

	if payload.CreateParents {
		parent := stdpath.Dir(cleanPath)
		if parent != "." && parent != "/" {
			if err := client.MkdirAll(parent); err != nil {
				response.Error(c, mapSFTPError(err))
				return
			}
		}
	}

	writer, err := client.OpenFile(cleanPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	defer writer.Close()

	emitter := newTransferEmitter(h, session, userID, "save", cleanPath)
	if emitter != nil {
		emitter.setTotal(int64(len(data)))
		emitter.start()
	}

	var writeErr error
	defer func() {
		if emitter == nil {
			return
		}
		if writeErr != nil {
			emitter.fail(writeErr)
		} else {
			emitter.complete()
		}
	}()

	_, writeErr = copyWithProgress(writer, bytes.NewReader(data), int64(len(data))+1, emitter)
	if writeErr != nil {
		response.Error(c, apperrors.Wrap(writeErr, "write sftp file"))
		return
	}

	response.Success(c, http.StatusOK, map[string]any{
		"path":          cleanPath,
		"bytes_written": len(data),
		"transfer_id":   emitterID(emitter),
	})
}

// Upload streams request payload into the target file with optional resumable offset.
func (h *SFTPHandler) Upload(c *gin.Context) {
	client, release, session, userID, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if isRootPath(cleanPath) {
		response.Error(c, apperrors.NewBadRequest("invalid target path"))
		return
	}

	createParents := parseBoolParam(c.DefaultQuery("create_parents", "false"))
	appendMode := parseBoolParam(c.DefaultQuery("append", "false"))
	offset, err := parseOffset(c.Request.Header.Get("Upload-Offset"), c.Query("offset"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid offset"))
		return
	}
	if offset < 0 {
		response.Error(c, apperrors.NewBadRequest("offset must be >= 0"))
		return
	}

	if createParents {
		parent := stdpath.Dir(cleanPath)
		if parent != "." && parent != "/" {
			if err := client.MkdirAll(parent); err != nil {
				response.Error(c, mapSFTPError(err))
				return
			}
		}
	}

	info, statErr := client.Stat(cleanPath)
	fileExists := statErr == nil
	if statErr != nil && !isNotFoundError(statErr) {
		response.Error(c, mapSFTPError(statErr))
		return
	}
	if fileExists && info.IsDir() {
		response.Error(c, apperrors.NewBadRequest("target path is a directory"))
		return
	}
	if !fileExists && offset > 0 {
		response.Error(c, apperrors.NewBadRequest("cannot resume upload for missing file"))
		return
	}
	if fileExists && offset > info.Size() {
		response.Error(c, apperrors.NewBadRequest("offset exceeds existing file size"))
		return
	}

	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 && !appendMode {
		flags |= os.O_TRUNC
	}
	writer, err := client.OpenFile(cleanPath, flags)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	defer writer.Close()

	if appendMode && fileExists && offset == 0 {
		offset = info.Size()
	}

	if offset > 0 {
		if _, err := writer.Seek(offset, io.SeekStart); err != nil {
			response.Error(c, apperrors.Wrap(err, "seek remote file"))
			return
		}
	}

	contentLength := c.Request.ContentLength
	if contentLength > maxUploadChunkBytes {
		response.Error(c, apperrors.New("sftp.upload_too_large", fmt.Sprintf("chunk exceeds %d bytes", maxUploadChunkBytes), http.StatusRequestEntityTooLarge))
		return
	}

	emitter := newTransferEmitter(h, session, userID, "upload", cleanPath)
	if emitter != nil {
		if contentLength >= 0 {
			emitter.setTotal(contentLength)
		}
		emitter.start()
	}

	var transferErr error
	defer func() {
		if emitter == nil {
			return
		}
		if transferErr != nil {
			emitter.fail(transferErr)
		} else {
			emitter.complete()
		}
	}()

	written, err := copyWithProgress(writer, c.Request.Body, maxUploadChunkBytes, emitter)
	if err != nil {
		transferErr = err
		response.Error(c, apperrors.Wrap(err, "upload chunk"))
		return
	}

	nextOffset := offset + written
	c.Header("Upload-Offset", strconv.FormatInt(nextOffset, 10))
	response.Success(c, http.StatusOK, map[string]any{
		"path":          cleanPath,
		"bytes_written": written,
		"next_offset":   nextOffset,
		"transfer_id":   emitterID(emitter),
	})
}

// Download streams the requested file to the caller.
func (h *SFTPHandler) Download(c *gin.Context) {
	client, release, session, userID, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}

	info, err := client.Stat(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	if info.IsDir() {
		response.Error(c, apperrors.NewBadRequest("requested path is a directory"))
		return
	}

	reader, err := client.Open(cleanPath)
	if err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}
	defer reader.Close()

	totalSize := info.Size()
	start, length, partial, rangeErr := parseRangeHeader(c.GetHeader("Range"), totalSize)
	if rangeErr != nil {
		response.Error(c, apperrors.New("sftp.range_invalid", rangeErr.Error(), http.StatusRequestedRangeNotSatisfiable))
		return
	}
	if length < 0 {
		length = 0
	}

	if seeker, ok := reader.(io.Seeker); ok && start > 0 {
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			response.Error(c, apperrors.Wrap(err, "seek remote file"))
			return
		}
	} else if start > 0 {
		if _, err := io.CopyN(io.Discard, reader, start); err != nil {
			response.Error(c, apperrors.Wrap(err, "discard prefix"))
			return
		}
	}

	emitter := newTransferEmitter(h, session, userID, "download", cleanPath)
	if emitter != nil {
		emitter.setTotal(length)
		emitter.start()
	}

	var transferErr error
	defer func() {
		if emitter == nil {
			return
		}
		if transferErr != nil {
			emitter.fail(transferErr)
		} else {
			emitter.complete()
		}
	}()

	c.Header("Accept-Ranges", "bytes")
	status := http.StatusOK
	if partial {
		status = http.StatusPartialContent
		end := start + length - 1
		if end < start {
			end = start
		}
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, totalSize))
	}
	c.Status(status)

	readSource := io.Reader(reader)
	if partial {
		readSource = io.LimitReader(reader, length)
	}

	filename := info.Name()
	if filename == "" {
		filename = stdpath.Base(cleanPath)
	}
	setDownloadHeaders(c, filename, length, info.ModTime())

	limit := int64(0)
	if partial {
		limit = length
	}
	written, err := copyWithProgress(c.Writer, readSource, limit, emitter)
	if err != nil {
		transferErr = err
		c.Error(fmt.Errorf("sftp download copy: %w", err)) //nolint:errcheck // best effort logging
		return
	}
	if partial && written != length {
		transferErr = fmt.Errorf("partial transfer incomplete: wrote %d of %d", written, length)
		c.Error(fmt.Errorf("sftp download incomplete: wrote %d of %d", written, length)) //nolint:errcheck
	}
}

type renameRequest struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Overwrite bool   `json:"overwrite"`
}

// Rename moves or renames a remote file.
func (h *SFTPHandler) Rename(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	var payload renameRequest
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.Error(c, apperrors.NewBadRequest("invalid rename payload"))
		return
	}

	source, err := sanitizeSFTPPath(payload.Source)
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	target, err := sanitizeSFTPPath(payload.Target)
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if isRootPath(source) || isRootPath(target) {
		response.Error(c, apperrors.NewBadRequest("invalid source or target path"))
		return
	}
	if strings.EqualFold(source, target) {
		response.Error(c, apperrors.NewBadRequest("source and target must differ"))
		return
	}

	if !payload.Overwrite {
		if _, err := client.Stat(target); err == nil {
			response.Error(c, apperrors.NewBadRequest("target already exists"))
			return
		} else if !isNotFoundError(err) {
			response.Error(c, mapSFTPError(err))
			return
		}
	} else {
		if info, err := client.Stat(target); err == nil {
			if info.IsDir() {
				response.Error(c, apperrors.NewBadRequest("overwriting directories is not supported"))
				return
			}
			if err := client.Remove(target); err != nil && !isNotFoundError(err) {
				response.Error(c, mapSFTPError(err))
				return
			}
		} else if !isNotFoundError(err) {
			response.Error(c, mapSFTPError(err))
			return
		}
	}

	if err := client.Rename(source, target); err != nil {
		response.Error(c, mapSFTPError(err))
		return
	}

	response.Success(c, http.StatusOK, map[string]any{
		"source": source,
		"target": target,
	})
}

// DeleteFile removes the specified file.
func (h *SFTPHandler) DeleteFile(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if isRootPath(cleanPath) {
		response.Error(c, apperrors.NewBadRequest("cannot delete root"))
		return
	}

	if err := client.Remove(cleanPath); err != nil {
		if isNotFoundError(err) {
			response.Success(c, http.StatusOK, map[string]any{"path": cleanPath, "deleted": false})
			return
		}
		response.Error(c, mapSFTPError(err))
		return
	}

	response.Success(c, http.StatusOK, map[string]any{
		"path":    cleanPath,
		"deleted": true,
	})
}

// DeleteDirectory removes the specified directory, optionally recursively.
func (h *SFTPHandler) DeleteDirectory(c *gin.Context) {
	client, release, _, _, ok := h.borrowClient(c)
	if !ok {
		return
	}
	defer release()

	cleanPath, err := sanitizeSFTPPath(c.Query("path"))
	if err != nil {
		response.Error(c, apperrors.NewBadRequest(err.Error()))
		return
	}
	if isRootPath(cleanPath) {
		response.Error(c, apperrors.NewBadRequest("cannot delete root"))
		return
	}

	recursive := parseBoolParam(c.DefaultQuery("recursive", "false"))
	if recursive {
		if err := removeRecursive(client, cleanPath, maxDeleteDepth); err != nil {
			response.Error(c, mapSFTPError(err))
			return
		}
	} else {
		if err := client.RemoveDirectory(cleanPath); err != nil {
			response.Error(c, mapSFTPError(err))
			return
		}
	}

	response.Success(c, http.StatusOK, map[string]any{
		"path":      cleanPath,
		"deleted":   true,
		"recursive": recursive,
	})
}

func (h *SFTPHandler) borrowClient(c *gin.Context) (shellsftp.Client, func() error, *models.ConnectionSession, string, bool) {
	if h == nil || h.channels == nil || h.lifecycle == nil {
		response.Error(c, apperrors.ErrNotFound)
		return nil, nil, nil, "", false
	}

	sessionID := strings.TrimSpace(c.Param("sessionID"))
	if sessionID == "" {
		response.Error(c, apperrors.NewBadRequest("session id is required"))
		return nil, nil, nil, "", false
	}

	userID := strings.TrimSpace(c.GetString(middleware.CtxUserIDKey))
	if userID == "" {
		response.Error(c, apperrors.ErrUnauthorized)
		return nil, nil, nil, "", false
	}

	session, err := h.lifecycle.AuthorizeSessionAccess(c.Request.Context(), sessionID, userID)
	if err != nil {
		h.handleLifecycleError(c, err)
		return nil, nil, nil, "", false
	}
	if session.ClosedAt != nil {
		response.Error(c, apperrors.NewBadRequest("session is no longer active"))
		return nil, nil, nil, "", false
	}

	if h.checker != nil {
		allowed, checkErr := h.checker.CheckResource(c.Request.Context(), userID, "connection", session.ConnectionID, "protocol:ssh.sftp")
		if checkErr != nil {
			response.Error(c, apperrors.Wrap(checkErr, "sftp permission check"))
			return nil, nil, nil, "", false
		}
		if !allowed {
			response.Error(c, apperrors.ErrForbidden)
			return nil, nil, nil, "", false
		}
	}

	client, release, err := h.channels.Borrow(sessionID)
	if err != nil {
		h.handleBorrowError(c, err)
		return nil, nil, nil, "", false
	}

	return client, release, session, userID, true
}

func parseBoolParam(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseOffset(headerValue, queryValue string) (int64, error) {
	if v := strings.TrimSpace(headerValue); v != "" {
		return strconv.ParseInt(v, 10, 64)
	}
	if v := strings.TrimSpace(queryValue); v != "" {
		return strconv.ParseInt(v, 10, 64)
	}
	return 0, nil
}

func decodeContent(content, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "base64":
		return base64.StdEncoding.DecodeString(content)
	case "utf-8", "utf8", "plain":
		return []byte(content), nil
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func copyWithProgress(dst io.Writer, src io.Reader, maxBytes int64, emitter *transferEmitter) (int64, error) {
	buffer := make([]byte, 64*1024)
	var total int64
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			if maxBytes > 0 && total+int64(n) > maxBytes {
				return total, fmt.Errorf("stream exceeds permitted limit of %d bytes", maxBytes)
			}
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return total, writeErr
			}
			total += int64(n)
			if emitter != nil {
				emitter.add(int64(n))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return total, readErr
		}
	}
	return total, nil
}

func parseRangeHeader(rangeHeader string, size int64) (int64, int64, bool, error) {
	rangeHeader = strings.TrimSpace(rangeHeader)
	if rangeHeader == "" {
		return 0, size, false, nil
	}
	if !strings.HasPrefix(strings.ToLower(rangeHeader), "bytes=") {
		return 0, 0, false, fmt.Errorf("unsupported range unit")
	}
	spec := rangeHeader[6:]
	parts := strings.SplitN(spec, ",", 2)
	segment := strings.TrimSpace(parts[0])
	if segment == "" {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	values := strings.SplitN(segment, "-", 2)
	if len(values) != 2 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	startStr := strings.TrimSpace(values[0])
	endStr := strings.TrimSpace(values[1])
	if startStr == "" {
		length, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || length < 0 {
			return 0, 0, false, fmt.Errorf("invalid range")
		}
		if length > size {
			length = size
		}
		return size - length, length, true, nil
	}
	start, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}
	if start >= size {
		return 0, 0, false, fmt.Errorf("range start beyond size")
	}
	var length int64
	if endStr == "" {
		length = size - start
	} else {
		end, err := strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, false, fmt.Errorf("invalid range")
		}
		length = end - start + 1
	}
	if start+length > size {
		length = size - start
	}
	return start, length, true, nil
}

func isRootPath(path string) bool {
	path = strings.TrimSpace(path)
	return path == "" || path == "." || path == "/"
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	var statusErr *pkgsftp.StatusError
	if errors.As(err, &statusErr) {
		return statusErr.FxCode() == pkgsftp.ErrSSHFxNoSuchFile
	}
	return false
}

func removeRecursive(client shellsftp.Client, dir string, depth int) error {
	if depth <= 0 {
		return fmt.Errorf("maximum delete depth reached")
	}
	entries, err := client.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		child := stdpath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := removeRecursive(client, child, depth-1); err != nil {
				return err
			}
			if err := client.RemoveDirectory(child); err != nil && !isNotFoundError(err) {
				return err
			}
			continue
		}
		if err := client.Remove(child); err != nil && !isNotFoundError(err) {
			return err
		}
	}
	return client.RemoveDirectory(dir)
}

func emitterID(e *transferEmitter) string {
	if e == nil {
		return ""
	}
	return e.transferID
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

func (h *SFTPHandler) emitTransferEvent(event string, data map[string]any) {
	if h == nil || h.broadcast == nil {
		return
	}
	if data == nil {
		data = make(map[string]any)
	}
	h.broadcast(realtime.StreamSFTPTransfers, realtime.Message{
		Event: event,
		Data:  data,
	})
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

type sftpFileContentResponse struct {
	Entry    sftpEntryDTO `json:"entry"`
	Encoding string       `json:"encoding"`
	Content  string       `json:"content"`
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

func setDownloadHeaders(c *gin.Context, filename string, size int64, modTime time.Time) {
	quoted := url.PathEscape(filename)
	disposition := fmt.Sprintf("attachment; filename*=UTF-8''%s", quoted)
	c.Header("Content-Disposition", disposition)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", size))
	if !modTime.IsZero() {
		c.Header("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	}
}
