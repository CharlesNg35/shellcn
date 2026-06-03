package sshsftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

// Session holds all mutable per-connection SSH state.
type Session struct {
	client *ssh.Client
	mu     sync.Mutex
	sftp   *sftp.Client
}

// NewSession wraps an authenticated SSH client.
func NewSession(client *ssh.Client) *Session {
	return &Session{client: client}
}

// Unwrap returns the shared SSH/SFTP session from the core session handle.
func Unwrap(sess plugin.Session) (*Session, error) {
	if s, ok := sess.(*Session); ok {
		return s, nil
	}
	type sessionGetter interface {
		Session() plugin.Session
	}
	if h, ok := sess.(sessionGetter); ok {
		if s, ok := h.Session().(*Session); ok {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w: SSH/SFTP session unavailable", plugin.ErrUnavailable)
}

// Filesystem opens SFTP lazily over the existing SSH client.
func (s *Session) Filesystem() (*sftp.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sftp != nil {
		return s.sftp, nil
	}
	client, err := sftp.NewClient(s.client)
	if err != nil {
		return nil, fmt.Errorf("%w: open sftp subsystem: %v", plugin.ErrUnavailable, err)
	}
	s.sftp = client
	return client, nil
}

func (s *Session) HealthCheck(context.Context) error {
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	return err
}

func (s *Session) OpenChannel(ctx context.Context, req plugin.ChannelRequest) (plugin.Channel, error) {
	switch req.Kind {
	case plugin.StreamTerminal:
		return s.openTerminal(ctx, req.Params)
	default:
		return nil, plugin.ErrNotSupported
	}
}

func (s *Session) RunCommand(ctx context.Context, command string) (string, bool, error) {
	sshSess, err := s.client.NewSession()
	if err != nil {
		return "", false, fmt.Errorf("%w: open command session: %v", plugin.ErrUnavailable, err)
	}
	defer func() { _ = sshSess.Close() }()
	var out limitedBuffer
	sshSess.Stdout = &out
	sshSess.Stderr = &out
	if err := sshSess.Start(command); err != nil {
		return "", false, fmt.Errorf("%w: start command: %v", plugin.ErrUnavailable, err)
	}
	done := make(chan error, 1)
	go func() { done <- sshSess.Wait() }()
	select {
	case <-ctx.Done():
		_ = sshSess.Close()
		return out.String(), out.Truncated(), ctx.Err()
	case err := <-done:
		if err != nil {
			return out.String(), out.Truncated(), fmt.Errorf("%w: command failed: %v", plugin.ErrUnavailable, err)
		}
		return out.String(), out.Truncated(), nil
	}
}

func (s *Session) Close() error {
	s.mu.Lock()
	fs := s.sftp
	s.sftp = nil
	s.mu.Unlock()
	var err error
	if fs != nil {
		err = fs.Close()
	}
	if cerr := s.client.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

func (s *Session) openTerminal(ctx context.Context, params map[string]string) (plugin.Channel, error) {
	sshSess, err := s.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("%w: open terminal: %v", plugin.ErrUnavailable, err)
	}
	stdin, err := sshSess.StdinPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, err
	}
	stdout, err := sshSess.StdoutPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, err
	}
	stderr, err := sshSess.StderrPipe()
	if err != nil {
		_ = sshSess.Close()
		return nil, err
	}
	cols, rows := terminalSize(params)
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := sshSess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("%w: request pty: %v", plugin.ErrUnavailable, err)
	}
	if err := sshSess.Shell(); err != nil {
		_ = sshSess.Close()
		return nil, fmt.Errorf("%w: start shell: %v", plugin.ErrUnavailable, err)
	}
	outR, outW := io.Pipe()
	ch := &terminalChannel{
		ctx:     ctx,
		session: sshSess,
		stdin:   stdin,
		out:     outR,
		outW:    outW,
		done:    make(chan struct{}),
	}
	go ch.copyOutput(stdout, stderr)
	return ch, nil
}

func terminalSize(params map[string]string) (int, int) {
	cols := intParam(params, "cols", 80)
	rows := intParam(params, "rows", 24)
	return cols, rows
}

func intParam(params map[string]string, key string, fallback int) int {
	var n int
	if _, err := fmt.Sscanf(params[key], "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

type terminalChannel struct {
	ctx     context.Context
	session *ssh.Session
	stdin   io.WriteCloser
	out     *io.PipeReader
	outW    *io.PipeWriter
	done    chan struct{}
	once    sync.Once
}

const maxCommandOutput = 1 << 20

type limitedBuffer struct {
	buf       []byte
	truncated atomic.Bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := maxCommandOutput - len(b.buf)
	if remaining <= 0 {
		b.truncated.Store(true)
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf = append(b.buf, p[:remaining]...)
		b.truncated.Store(true)
		return len(p), nil
	}
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *limitedBuffer) String() string { return string(b.buf) }

func (b *limitedBuffer) Truncated() bool { return b.truncated.Load() }

func (c *terminalChannel) Kind() plugin.StreamKind { return plugin.StreamTerminal }

func (c *terminalChannel) Read(p []byte) (int, error) {
	return c.out.Read(p)
}

func (c *terminalChannel) Write(p []byte) (int, error) {
	return c.stdin.Write(p)
}

func (c *terminalChannel) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return c.session.WindowChange(rows, cols)
}

func (c *terminalChannel) Close() error {
	var err error
	c.once.Do(func() {
		_ = c.stdin.Close()
		_ = c.session.Close()
		err = c.out.Close()
		<-c.done
	})
	return err
}

func (c *terminalChannel) copyOutput(readers ...io.Reader) {
	defer close(c.done)
	var wg sync.WaitGroup
	for _, r := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = io.Copy(c.outW, r)
		}()
	}
	wait := make(chan error, 1)
	go func() { wait <- c.session.Wait() }()
	select {
	case <-c.ctx.Done():
	case err := <-wait:
		if err != nil && !errors.Is(err, io.EOF) {
			_ = c.outW.CloseWithError(err)
			wg.Wait()
			return
		}
	}
	wg.Wait()
	_ = c.outW.Close()
}
