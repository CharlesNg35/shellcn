package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"

	iauth "github.com/charlesng35/shellcn/internal/auth"
	"github.com/charlesng35/shellcn/internal/middleware"
	"github.com/charlesng35/shellcn/internal/realtime"
	"github.com/charlesng35/shellcn/pkg/errors"
	"github.com/charlesng35/shellcn/pkg/response"
)

// RealtimeHandler upgrades HTTP connections into authenticated WebSocket streams.
type RealtimeHandler struct {
	hub            *realtime.Hub
	jwt            *iauth.JWTService
	allowedStreams map[string]struct{}
	ssh            *SSHSessionHandler
}

// NewRealtimeHandler constructs a realtime handler and optionally restricts allowed streams.
// If no streams are provided, any stream name is accepted.
func NewRealtimeHandler(hub *realtime.Hub, jwt *iauth.JWTService, ssh *SSHSessionHandler, streams ...string) *RealtimeHandler {
	allowed := make(map[string]struct{}, len(streams))
	for _, stream := range streams {
		stream = normalizeStream(stream)
		if stream == "" {
			continue
		}
		allowed[stream] = struct{}{}
	}

	return &RealtimeHandler{
		hub:            hub,
		jwt:            jwt,
		allowedStreams: allowed,
		ssh:            ssh,
	}
}

// Stream validates the caller and either tunnels SSH traffic or upgrades the
// request to the realtime hub for broadcast streams. This keeps a single
// websocket entry point while still supporting protocol-specific tunnels.
func (h *RealtimeHandler) Stream(c *gin.Context) {
	if h.jwt == nil || h.hub == nil {
		response.Error(c, errors.ErrNotFound)
		return
	}

	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		token = strings.TrimSpace(c.Query("access_token"))
	}
	if token == "" {
		authz := c.GetHeader("Authorization")
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token = strings.TrimSpace(authz[7:])
		}
	}

	if token == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	claims, err := h.jwt.ValidateAccessToken(token)
	if err != nil {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	userID := strings.TrimSpace(claims.UserID)
	if userID == "" {
		response.Error(c, errors.ErrUnauthorized)
		return
	}

	// `tunnel=ssh` uses the SSH session handler instead of the hub multiplexer.
	tunnel := strings.ToLower(strings.TrimSpace(c.Query("tunnel")))
	if tunnel == "ssh" {
		if h.ssh == nil {
			response.Error(c, errors.ErrNotFound)
			return
		}
		c.Set(middleware.CtxUserIDKey, userID)
		h.ssh.ServeTunnel(c, claims)
		return
	}

	streams := gatherStreams(c)
	if len(streams) == 0 {
		streams = []string{realtime.StreamNotifications}
	}

	if len(h.allowedStreams) > 0 {
		for _, stream := range streams {
			if _, ok := h.allowedStreams[stream]; !ok {
				response.Error(c, errors.ErrNotFound)
				return
			}
		}
	}

	var allowed map[string]struct{}
	if len(h.allowedStreams) > 0 {
		allowed = h.allowedStreams
	}
	h.hub.Serve(userID, streams, allowed, c.Writer, c.Request)
}

func gatherStreams(c *gin.Context) []string {
	var streams []string

	if pathStream := normalizeStream(c.Param("stream")); pathStream != "" {
		streams = append(streams, pathStream)
	}

	for _, queryStream := range c.QueryArray("stream") {
		if normalized := normalizeStream(queryStream); normalized != "" {
			streams = append(streams, normalized)
		}
	}

	raw := c.Query("streams")
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if normalized := normalizeStream(part); normalized != "" {
				streams = append(streams, normalized)
			}
		}
	}

	return uniqueStreams(streams)
}

func normalizeStream(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func uniqueStreams(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
