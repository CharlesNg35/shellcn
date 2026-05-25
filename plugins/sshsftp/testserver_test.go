package sshsftp

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sshTestServer struct {
	Host      string
	Port      string
	PublicKey ssh.PublicKey

	ln           net.Listener
	serverConfig *ssh.ServerConfig
	done         chan struct{}
	once         sync.Once
}

func newSSHServer(t *testing.T) *sshTestServer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if meta.User() == "u" && string(pass) == "p" {
				return nil, nil
			}
			return nil, errors.New("bad credentials")
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	s := &sshTestServer{
		Host: "127.0.0.1", Port: port, PublicKey: signer.PublicKey(),
		ln: ln, serverConfig: cfg, done: make(chan struct{}),
	}
	go s.serve()
	return s
}

func (s *sshTestServer) Close() {
	s.once.Do(func() {
		_ = s.ln.Close()
		close(s.done)
	})
}

func (s *sshTestServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *sshTestServer) handleConn(conn net.Conn) {
	sc, chans, reqs, err := ssh.NewServerConn(conn, s.serverConfig)
	if err != nil {
		_ = conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() {
		<-s.done
		_ = sc.Close()
	}()
	for ch := range chans {
		if ch.ChannelType() != "session" {
			_ = ch.Reject(ssh.UnknownChannelType, "session only")
			continue
		}
		channel, requests, err := ch.Accept()
		if err != nil {
			continue
		}
		go s.handleSession(channel, requests)
	}
}

func (s *sshTestServer) handleSession(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer func() { _ = ch.Close() }()
	for req := range reqs {
		switch req.Type {
		case "pty-req", "window-change":
			_ = req.Reply(true, nil)
		case "shell":
			_ = req.Reply(true, nil)
			_, _ = io.WriteString(ch, "ready\n")
			_, _ = io.Copy(ch, ch)
			return
		case "subsystem":
			var payload struct{ Name string }
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil || payload.Name != "sftp" {
				_ = req.Reply(false, nil)
				continue
			}
			_ = req.Reply(true, nil)
			server, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory("/"))
			if err != nil {
				return
			}
			_ = server.Serve()
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func (s *sshTestServer) config() map[string]any {
	port, _ := strconv.Atoi(s.Port)
	return map[string]any{"host": s.Host, "port": port, "user": "u", "auth": "password", "password": "p"}
}
