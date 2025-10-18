package terminal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	readLimitBytes = int64(256 << 10) // 256 KiB
	heartbeatEvery = 30 * time.Second
)

// Streams exposes the stdin/stdout/stderr handles for a terminal session.
type Streams interface {
	Stdin() io.WriteCloser
	Stdout() io.Reader
	Stderr() io.Reader
	Resize(columns, rows int) error
}

// Callbacks allows callers to react to events produced by the bridge.
type Callbacks struct {
	OnHeartbeat func()
	OnEvent     func(event string, payload any)
	OnError     func(error)
}

// Config describes the runtime parameters required to run the terminal bridge.
type Config struct {
	Conn      *websocket.Conn
	SessionID string
	Streams   Streams
	Callbacks Callbacks
}

type outboundMessage struct {
	messageType int
	payload     []byte
}

// Run connects a websocket connection to the provided terminal streams until one side exits.
func Run(ctx context.Context, cfg Config) error {
	if cfg.Conn == nil {
		return errors.New("terminal: websocket connection is required")
	}
	if cfg.Streams == nil {
		return errors.New("terminal: streams implementation is required")
	}

	conn := cfg.Conn
	sessionID := cfg.SessionID

	conn.SetReadLimit(readLimitBytes)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		if cfg.Callbacks.OnHeartbeat != nil {
			cfg.Callbacks.OnHeartbeat()
		}
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	out := make(chan outboundMessage, 32)
	errCh := make(chan error, 4)
	stop := make(chan struct{})

	go writePump(conn, out, errCh, stop)
	go readPump(conn, cfg.Streams, sessionID, out, errCh, stop, cfg.Callbacks)
	if stdout := cfg.Streams.Stdout(); stdout != nil {
		go streamPump(stdout, websocket.BinaryMessage, sessionID, "stdout", out, errCh, stop, cfg.Callbacks)
	}
	if stderr := cfg.Streams.Stderr(); stderr != nil {
		go streamPump(stderr, websocket.BinaryMessage, sessionID, "stderr", out, errCh, stop, cfg.Callbacks)
	}
	go heartbeatLoop(stop, cfg.Callbacks)

	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
			case <-stop:
			}
		}()
	}

	var result error
	select {
	case err := <-errCh:
		result = err
	case <-stop:
	}

	close(stop)
	close(out)

	if result != nil && !errors.Is(result, io.EOF) {
		if cfg.Callbacks.OnError != nil {
			cfg.Callbacks.OnError(result)
		}
		msg := map[string]any{"type": "error", "message": result.Error(), "session_id": sessionID}
		if b, err := json.Marshal(msg); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}
	}

	return result
}

func writePump(conn *websocket.Conn, outbound <-chan outboundMessage, errCh chan<- error, stop <-chan struct{}) {
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	for {
		select {
		case <-stop:
			return
		case msg, ok := <-outbound:
			if !ok {
				return
			}
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				errCh <- err
				return
			}
			if err := conn.WriteMessage(msg.messageType, msg.payload); err != nil {
				errCh <- err
				return
			}
		case <-pingTicker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				errCh <- err
				return
			}
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func readPump(conn *websocket.Conn, streams Streams, sessionID string, outbound chan<- outboundMessage, errCh chan<- error, stop <-chan struct{}, callbacks Callbacks) {
	stdin := streams.Stdin()
	for {
		select {
		case <-stop:
			return
		default:
		}

		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}

		if callbacks.OnHeartbeat != nil {
			callbacks.OnHeartbeat()
		}

		if messageType == websocket.TextMessage {
			if handled := processControlMessage(streams, sessionID, payload, callbacks); handled {
				continue
			}
			// Don't trim raw terminal input - xterm sends proper control characters
		}

		if len(payload) == 0 || stdin == nil {
			continue
		}

		if _, err := stdin.Write(payload); err != nil {
			errCh <- err
			return
		}
	}
}

func processControlMessage(streams Streams, sessionID string, payload []byte, callbacks Callbacks) bool {
	var ctrl struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := json.Unmarshal(payload, &ctrl); err != nil {
		return false
	}

	switch strings.ToLower(ctrl.Type) {
	case "resize":
		if err := streams.Resize(ctrl.Cols, ctrl.Rows); err != nil {
			return false
		}
		if callbacks.OnEvent != nil {
			callbacks.OnEvent("resize", map[string]any{"session_id": sessionID, "cols": ctrl.Cols, "rows": ctrl.Rows})
		}
		return true
	case "heartbeat":
		if callbacks.OnHeartbeat != nil {
			callbacks.OnHeartbeat()
		}
		return true
	default:
		return false
	}
}

func streamPump(reader io.Reader, messageType int, sessionID, event string, outbound chan<- outboundMessage, errCh chan<- error, stop <-chan struct{}, callbacks Callbacks) {
	buf := make([]byte, 32*1024)
	for {
		select {
		case <-stop:
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			select {
			case outbound <- outboundMessage{messageType: messageType, payload: chunk}:
			case <-stop:
				return
			}
			if callbacks.OnEvent != nil {
				callbacks.OnEvent(event, map[string]any{"session_id": sessionID, "payload": chunk})
			}
		}
		if err != nil {
			if err != io.EOF {
				errCh <- err
			} else {
				errCh <- nil
			}
			return
		}
	}
}

func heartbeatLoop(stop <-chan struct{}, callbacks Callbacks) {
	ticker := time.NewTicker(heartbeatEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if callbacks.OnHeartbeat != nil {
				callbacks.OnHeartbeat()
			}
		case <-stop:
			return
		}
	}
}
