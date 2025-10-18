package ssh_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/charlesng35/shellcn/internal/drivers"
	sshdriver "github.com/charlesng35/shellcn/internal/drivers/ssh"
)

func TestLaunchWithPasswordAuthentication(t *testing.T) {
	server, cleanup := startMockSSHServer(t, mockCredentials{
		Username: "tester",
		Password: "secret",
	})
	t.Cleanup(cleanup)

	driver := sshdriver.NewSSHDriver()

	handle, err := driver.Launch(context.Background(), drivers.SessionRequest{
		ConnectionID: "conn-1",
		ProtocolID:   sshdriver.DriverIDSSH,
		UserID:       "user-1",
		Settings: map[string]any{
			"host":            "127.0.0.1",
			"port":            server.Port,
			"terminal_width":  80,
			"terminal_height": 24,
			"timeout":         "5s",
		},
		Secret: map[string]any{
			"session_id":  "session-123",
			"username":    server.Creds.Username,
			"auth_method": "password",
			"password":    server.Creds.Password,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, handle.Close(context.Background()))
	})

	h, ok := handle.(*sshdriver.Handle)
	require.True(t, ok, "expected SSH handle type assertion to succeed")

	message := []byte("hello world\n")
	_, err = h.Stdin().Write(message)
	require.NoError(t, err)

	resp := make([]byte, len(message))
	_, err = io.ReadFull(h.Stdout(), resp)
	require.NoError(t, err)
	require.Equal(t, message, resp)

	require.NoError(t, h.Close(context.Background()))
}

type mockCredentials struct {
	Username string
	Password string
}

type mockSSHServer struct {
	Address string
	Port    int
	Creds   mockCredentials
}

func startMockSSHServer(t *testing.T, creds mockCredentials) (*mockSSHServer, func()) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := gossh.NewSignerFromKey(privateKey)
	require.NoError(t, err)

	serverConfig := &gossh.ServerConfig{
		PasswordCallback: func(conn gossh.ConnMetadata, password []byte) (*gossh.Permissions, error) {
			if conn.User() == creds.Username && string(password) == creds.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("permission denied")
		},
	}
	serverConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := &mockSSHServer{
		Address: listener.Addr().String(),
		Creds:   creds,
	}
	_, portStr, err := net.SplitHostPort(server.Address)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	server.Port = port

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleSSHConnection(conn, serverConfig)
		}
	}()

	return server, func() {
		_ = listener.Close()
	}
}

func handleSSHConnection(conn net.Conn, config *gossh.ServerConfig) {
	defer conn.Close()

	sshConn, chans, reqs, err := gossh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go gossh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(gossh.UnknownChannelType, "unsupported channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go func(in <-chan *gossh.Request) {
			for req := range in {
				switch req.Type {
				case "pty-req":
					_ = req.Reply(true, nil)
				case "shell":
					_ = req.Reply(true, nil)
					go echoLoop(channel)
				default:
					_ = req.Reply(false, nil)
				}
			}
		}(requests)
	}
}

func echoLoop(channel gossh.Channel) {
	defer channel.Close()
	buffer := make([]byte, 1024)

	for {
		n, err := channel.Read(buffer)
		if n > 0 {
			if _, werr := channel.Write(buffer[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}
