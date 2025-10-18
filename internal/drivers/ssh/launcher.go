package ssh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	pkgsftp "github.com/pkg/sftp"
	gossh "golang.org/x/crypto/ssh"

	"github.com/charlesng35/shellcn/internal/drivers"
	shellsftp "github.com/charlesng35/shellcn/internal/sftp"
)

const (
	defaultTerminalType = "xterm-256color"
	defaultTermHeight   = 24
	defaultTermWidth    = 80
	defaultDialTimeout  = 10 * time.Second
)

type sftpClientWrapper struct {
	client *pkgsftp.Client
}

func (w *sftpClientWrapper) ReadDir(path string) ([]os.FileInfo, error) {
	if w == nil || w.client == nil {
		return nil, errors.New("ssh: sftp client unavailable")
	}
	return w.client.ReadDir(path)
}

func (w *sftpClientWrapper) Stat(path string) (os.FileInfo, error) {
	if w == nil || w.client == nil {
		return nil, errors.New("ssh: sftp client unavailable")
	}
	return w.client.Stat(path)
}

func (w *sftpClientWrapper) Open(path string) (shellsftp.ReadableFile, error) {
	if w == nil || w.client == nil {
		return nil, errors.New("ssh: sftp client unavailable")
	}
	f, err := w.client.Open(path)
	if err != nil {
		return nil, err
	}
	return &sftpFileAdapter{File: f}, nil
}

func (w *sftpClientWrapper) OpenFile(path string, flag int) (shellsftp.WritableFile, error) {
	if w == nil || w.client == nil {
		return nil, errors.New("ssh: sftp client unavailable")
	}
	f, err := w.client.OpenFile(path, flag)
	if err != nil {
		return nil, err
	}
	return &sftpFileAdapter{File: f}, nil
}

func (w *sftpClientWrapper) Create(path string) (shellsftp.WritableFile, error) {
	if w == nil || w.client == nil {
		return nil, errors.New("ssh: sftp client unavailable")
	}
	f, err := w.client.Create(path)
	if err != nil {
		return nil, err
	}
	return &sftpFileAdapter{File: f}, nil
}

func (w *sftpClientWrapper) MkdirAll(path string) error {
	if w == nil || w.client == nil {
		return errors.New("ssh: sftp client unavailable")
	}
	return w.client.MkdirAll(path)
}

func (w *sftpClientWrapper) Remove(path string) error {
	if w == nil || w.client == nil {
		return errors.New("ssh: sftp client unavailable")
	}
	return w.client.Remove(path)
}

func (w *sftpClientWrapper) RemoveDirectory(path string) error {
	if w == nil || w.client == nil {
		return errors.New("ssh: sftp client unavailable")
	}
	return w.client.RemoveDirectory(path)
}

func (w *sftpClientWrapper) Rename(oldPath, newPath string) error {
	if w == nil || w.client == nil {
		return errors.New("ssh: sftp client unavailable")
	}
	return w.client.Rename(oldPath, newPath)
}

func (w *sftpClientWrapper) Truncate(path string, size int64) error {
	if w == nil || w.client == nil {
		return errors.New("ssh: sftp client unavailable")
	}
	return w.client.Truncate(path, size)
}

func (w *sftpClientWrapper) RealPath(path string) (string, error) {
	if w == nil || w.client == nil {
		return "", errors.New("ssh: sftp client unavailable")
	}
	return w.client.RealPath(path)
}

type sftpFileAdapter struct {
	File *pkgsftp.File
}

func (a *sftpFileAdapter) Read(p []byte) (int, error) {
	if a == nil || a.File == nil {
		return 0, errors.New("ssh: sftp file unavailable")
	}
	return a.File.Read(p)
}

func (a *sftpFileAdapter) Close() error {
	if a == nil || a.File == nil {
		return nil
	}
	return a.File.Close()
}

func (a *sftpFileAdapter) Seek(offset int64, whence int) (int64, error) {
	if a == nil || a.File == nil {
		return 0, errors.New("ssh: sftp file unavailable")
	}
	return a.File.Seek(offset, whence)
}

func (a *sftpFileAdapter) Write(p []byte) (int, error) {
	if a == nil || a.File == nil {
		return 0, errors.New("ssh: sftp file unavailable")
	}
	return a.File.Write(p)
}

func (a *sftpFileAdapter) WriteAt(p []byte, off int64) (int, error) {
	if a == nil || a.File == nil {
		return 0, errors.New("ssh: sftp file unavailable")
	}
	return a.File.WriteAt(p, off)
}

var (
	_ drivers.SessionHandle = (*Handle)(nil)
)

// Handle exposes SSH session streams for higher-level orchestration.
type Handle struct {
	id      string
	client  *gossh.Client
	session *gossh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader

	sftpMu     sync.Mutex
	sftpClient *pkgsftp.Client
	sftpUsers  int
	closed     bool

	closeOnce sync.Once
	closeErr  error
}

// ID returns the session identifier associated with the handle.
func (h *Handle) ID() string {
	return h.id
}

// Stdin returns a writer connected to the remote shell stdin.
func (h *Handle) Stdin() io.WriteCloser {
	return h.stdin
}

// Stdout returns a reader for the remote shell stdout.
func (h *Handle) Stdout() io.Reader {
	return h.stdout
}

// Stderr returns a reader for the remote shell stderr.
func (h *Handle) Stderr() io.Reader {
	return h.stderr
}

// Resize adjusts the PTY dimensions for the active SSH session.
func (h *Handle) Resize(columns, rows int) error {
	if h.session == nil {
		return errors.New("ssh: session not initialised")
	}
	if columns <= 0 {
		columns = defaultTermWidth
	}
	if rows <= 0 {
		rows = defaultTermHeight
	}
	return h.session.WindowChange(rows, columns)
}

// Close terminates the SSH session and underlying client connection.
func (h *Handle) Close(ctx context.Context) error {
	if h == nil {
		return nil
	}
	h.closeOnce.Do(func() {
		h.sftpMu.Lock()
		if h.sftpClient != nil {
			_ = h.sftpClient.Close()
			h.sftpClient = nil
		}
		h.sftpUsers = 0
		h.closed = true
		h.sftpMu.Unlock()

		if h.session != nil {
			_ = h.session.Close()
		}
		if h.client != nil {
			h.closeErr = h.client.Close()
			h.client = nil
		}
	})
	return h.closeErr
}

// AcquireSFTP returns a pooled SFTP client associated with the underlying SSH connection.
// Callers must invoke the returned release function once they finish using the client.
func (h *Handle) AcquireSFTP() (shellsftp.Client, func() error, error) {
	if h == nil {
		return nil, nil, errors.New("ssh: handle is nil")
	}

	h.sftpMu.Lock()
	defer h.sftpMu.Unlock()

	if h.closed {
		return nil, nil, errors.New("ssh: session already closed")
	}
	if h.client == nil {
		return nil, nil, errors.New("ssh: client not initialised")
	}

	if h.sftpClient == nil {
		client, err := pkgsftp.NewClient(h.client, pkgsftp.MaxPacket(1<<15))
		if err != nil {
			return nil, nil, fmt.Errorf("ssh: create sftp client: %w", err)
		}
		h.sftpClient = client
	}

	h.sftpUsers++
	acquired := &sftpClientWrapper{client: h.sftpClient}
	released := false

	release := func() error {
		h.sftpMu.Lock()
		defer h.sftpMu.Unlock()
		if released {
			return nil
		}
		released = true
		if h.sftpUsers > 0 {
			h.sftpUsers--
		}
		if h.sftpUsers == 0 && (h.closed || h.client == nil) {
			if h.sftpClient != nil {
				err := h.sftpClient.Close()
				h.sftpClient = nil
				return err
			}
		}
		return nil
	}

	return acquired, release, nil
}

// Launch establishes an interactive SSH session and returns a session handle.
func (d *Driver) Launch(ctx context.Context, req drivers.SessionRequest) (drivers.SessionHandle, error) {
	cfg, err := parseLaunchConfig(req.Settings, req.Secret)
	if err != nil {
		return nil, err
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultDialTimeout
	}

	dialer := net.Dialer{
		Timeout: cfg.Timeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
	if err != nil {
		return nil, fmt.Errorf("ssh: dial %s:%d: %w", cfg.Host, cfg.Port, err)
	}

	clientConn, chans, reqs, err := gossh.NewClientConn(conn, net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), cfg.ClientConfig)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh: client handshake: %w", err)
	}

	client := gossh.NewClient(clientConn, chans, reqs)

	isSFTPOnly := strings.EqualFold(strings.TrimSpace(req.ProtocolID), DriverIDSFTP)

	var session *gossh.Session
	var stdin io.WriteCloser
	var stdout io.Reader
	var stderr io.Reader

	cleanup := func() {
		if session != nil {
			_ = session.Close()
		}
		_ = client.Close()
	}

	if !isSFTPOnly {
		session, err = client.NewSession()
		if err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("ssh: create session: %w", err)
		}

		stdin, err = session.StdinPipe()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("ssh: stdin pipe: %w", err)
		}

		stdout, err = session.StdoutPipe()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("ssh: stdout pipe: %w", err)
		}

		stderr, err = session.StderrPipe()
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("ssh: stderr pipe: %w", err)
		}

		columns := cfg.TerminalWidth
		if columns <= 0 {
			columns = defaultTermWidth
		}
		rows := cfg.TerminalHeight
		if rows <= 0 {
			rows = defaultTermHeight
		}
		modes := gossh.TerminalModes{
			gossh.ECHO:          1,
			gossh.TTY_OP_ISPEED: 14400,
			gossh.TTY_OP_OSPEED: 14400,
		}
		if err := session.RequestPty(cfg.TerminalType, rows, columns, modes); err != nil {
			cleanup()
			return nil, fmt.Errorf("ssh: request pty: %w", err)
		}

		if err := session.Shell(); err != nil {
			cleanup()
			return nil, fmt.Errorf("ssh: start shell: %w", err)
		}
	}

	handle := &Handle{
		id:      cfg.SessionID,
		client:  client,
		session: session,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
	}

	return handle, nil
}

type launchConfig struct {
	SessionID      string
	Host           string
	Port           int
	Username       string
	AuthMethod     string
	Password       string
	PrivateKey     []byte
	Passphrase     string
	Timeout        time.Duration
	TerminalType   string
	TerminalWidth  int
	TerminalHeight int
	ClientConfig   *gossh.ClientConfig
}

func parseLaunchConfig(settings map[string]any, secret map[string]any) (launchConfig, error) {
	cfg := launchConfig{
		Port:         22,
		TerminalType: defaultTerminalType,
	}

	if id, ok := stringValue(secret, "session_id"); ok {
		cfg.SessionID = id
	}

	if host, ok := stringValue(settings, "host"); ok {
		cfg.Host = host
	}

	if port, ok := intValue(settings, "port"); ok && port > 0 {
		cfg.Port = port
	}

	if timeoutStr, ok := stringValue(settings, "timeout"); ok {
		if parsed, err := time.ParseDuration(timeoutStr); err == nil {
			cfg.Timeout = parsed
		}
	}

	if username, ok := stringValue(secret, "username"); ok {
		cfg.Username = username
	}

	if authMethod, ok := stringValue(secret, "auth_method"); ok {
		cfg.AuthMethod = strings.ToLower(authMethod)
	}

	if password, ok := stringValue(secret, "password"); ok {
		cfg.Password = password
	}

	if privateKey, ok := stringValue(secret, "private_key"); ok {
		cfg.PrivateKey = []byte(privateKey)
	}

	if passphrase, ok := stringValue(secret, "passphrase"); ok {
		cfg.Passphrase = passphrase
	}

	if termType, ok := stringValue(settings, "terminal_type"); ok {
		cfg.TerminalType = termType
	}

	if width, ok := intValue(settings, "terminal_width"); ok {
		cfg.TerminalWidth = width
	}
	if height, ok := intValue(settings, "terminal_height"); ok {
		cfg.TerminalHeight = height
	}

	if cfg.Host == "" {
		return cfg, errors.New("ssh: host is required")
	}
	if cfg.Username == "" {
		return cfg, errors.New("ssh: username is required")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("ssh: invalid port %d", cfg.Port)
	}
	if cfg.AuthMethod == "" {
		return cfg, errors.New("ssh: auth_method is required")
	}

	clientConfig, err := buildClientConfig(cfg)
	if err != nil {
		return cfg, err
	}
	cfg.ClientConfig = clientConfig

	return cfg, nil
}

func buildClientConfig(cfg launchConfig) (*gossh.ClientConfig, error) {
	authMethods := []gossh.AuthMethod{}
	switch cfg.AuthMethod {
	case "private_key", "publickey", "key":
		if len(cfg.PrivateKey) == 0 {
			return nil, errors.New("ssh: private key is required for key authentication")
		}
		var signer gossh.Signer
		var err error
		if cfg.Passphrase != "" {
			signer, err = gossh.ParsePrivateKeyWithPassphrase(cfg.PrivateKey, []byte(cfg.Passphrase))
		} else {
			signer, err = gossh.ParsePrivateKey(cfg.PrivateKey)
		}
		if err != nil {
			return nil, fmt.Errorf("ssh: parse private key: %w", err)
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
		if cfg.Password != "" {
			authMethods = append(authMethods, gossh.Password(cfg.Password))
		}
	case "password":
		if cfg.Password == "" {
			return nil, errors.New("ssh: password is required for password authentication")
		}
		authMethods = append(authMethods, gossh.Password(cfg.Password))
	default:
		return nil, fmt.Errorf("ssh: unsupported auth_method %q", cfg.AuthMethod)
	}

	if len(authMethods) == 0 {
		return nil, errors.New("ssh: no authentication methods configured")
	}

	clientConfig := &gossh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	}
	return clientConfig, nil
}

func stringValue(source map[string]any, key string) (string, bool) {
	if source == nil {
		return "", false
	}
	value, ok := source[key]
	if !ok || value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v), true
	case fmt.Stringer:
		return strings.TrimSpace(v.String()), true
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v)), true
	}
}

func intValue(source map[string]any, key string) (int, bool) {
	if source == nil {
		return 0, false
	}
	value, ok := source[key]
	if !ok || value == nil {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
		if parsed, err := strconv.Atoi(v.String()); err == nil {
			return parsed, true
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
