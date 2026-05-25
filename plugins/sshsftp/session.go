package sshsftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/charlesng/shellcn/internal/plugin"
)

// Session holds all mutable per-connection SSH state.
type Session struct {
	client  *ssh.Client
	mu      sync.Mutex
	sftp    *sftp.Client
	tunnels map[string]*Tunnel
}

// NewSession wraps an authenticated SSH client.
func NewSession(client *ssh.Client) *Session {
	return &Session{client: client, tunnels: map[string]*Tunnel{}}
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

func (s *Session) Close() error {
	s.mu.Lock()
	fs := s.sftp
	s.sftp = nil
	s.mu.Unlock()
	var err error
	if fs != nil {
		err = fs.Close()
	}
	for _, t := range s.tunnelsSnapshot() {
		if cerr := t.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	if cerr := s.client.Close(); cerr != nil && err == nil {
		err = cerr
	}
	return err
}

// Tunnel is a local TCP forward through the SSH connection.
type Tunnel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Listen    string `json:"listen"`
	Target    string `json:"target"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`

	ln   net.Listener
	done chan struct{}
	once sync.Once
}

// OpenTunnel starts a local TCP listener that forwards accepted connections over SSH.
func (s *Session) OpenTunnel(id, name, listen, target string) (*Tunnel, error) {
	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, fmt.Errorf("%w: listen tunnel: %v", plugin.ErrUnavailable, err)
	}
	t := &Tunnel{
		ID: id, Name: name, Listen: ln.Addr().String(), Target: target,
		Status: "active", CreatedAt: nowUTC(), ln: ln, done: make(chan struct{}),
	}
	s.mu.Lock()
	s.tunnels[id] = t
	s.mu.Unlock()
	go s.serveTunnel(t)
	return t, nil
}

// ListTunnels returns active tunnel summaries.
func (s *Session) ListTunnels() []Tunnel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Tunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		out = append(out, Tunnel{ID: t.ID, Name: t.Name, Listen: t.Listen, Target: t.Target, Status: t.Status, CreatedAt: t.CreatedAt})
	}
	return out
}

// CloseTunnel stops a tunnel by id.
func (s *Session) CloseTunnel(id string) error {
	s.mu.Lock()
	t, ok := s.tunnels[id]
	if ok {
		delete(s.tunnels, id)
	}
	s.mu.Unlock()
	if !ok {
		return plugin.ErrNotFound
	}
	return t.Close()
}

func (s *Session) tunnelsSnapshot() []*Tunnel {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Tunnel, 0, len(s.tunnels))
	for id, t := range s.tunnels {
		out = append(out, t)
		delete(s.tunnels, id)
	}
	return out
}

func (s *Session) serveTunnel(t *Tunnel) {
	for {
		conn, err := t.ln.Accept()
		if err != nil {
			return
		}
		go s.handleTunnelConn(conn, t.Target)
	}
}

func (s *Session) handleTunnelConn(local net.Conn, target string) {
	defer func() { _ = local.Close() }()
	remote, err := s.client.Dial("tcp", target)
	if err != nil {
		return
	}
	defer func() { _ = remote.Close() }()
	errc := make(chan error, 2)
	go func() {
		_, err := io.Copy(remote, local)
		errc <- err
	}()
	go func() {
		_, err := io.Copy(local, remote)
		errc <- err
	}()
	<-errc
}

func (t *Tunnel) Close() error {
	var err error
	t.once.Do(func() {
		t.Status = "closed"
		err = t.ln.Close()
		close(t.done)
	})
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

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
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
