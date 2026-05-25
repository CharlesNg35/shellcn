package sshsftp

import (
	"context"
	"net"
	"sync"
	"testing"

	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/charlesng/shellcn/internal/plugin"
)

func TestOpenTerminalAndSFTPLazilyShareClient(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()
	cfg := srv.config()
	cfg["known_hosts"] = knownhosts.Line([]string{knownhosts.Normalize(net.JoinHostPort(srv.Host, srv.Port))}, srv.PublicKey)

	sess, err := Connect(context.Background(), plugin.ConnectConfig{Config: cfg, Net: pluginNet{}})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	sshSess := sess.(*Session)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		ch, err := sshSess.OpenChannel(context.Background(), plugin.ChannelRequest{Kind: plugin.StreamTerminal})
		if err != nil {
			errs <- err
			return
		}
		defer func() { _ = ch.Close() }()
		buf := make([]byte, 16)
		if _, err := ch.Read(buf); err != nil {
			errs <- err
		}
	}()
	go func() {
		defer wg.Done()
		fs, err := sshSess.Filesystem()
		if err != nil {
			errs <- err
			return
		}
		if _, err := fs.ReadDir("/"); err != nil {
			errs <- err
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent terminal+sftp: %v", err)
		}
	}
}

func TestFilesystemIsLazyAndReused(t *testing.T) {
	srv := newSSHServer(t)
	defer srv.Close()
	cfg := srv.config()
	cfg["known_hosts"] = knownhosts.Line([]string{knownhosts.Normalize(net.JoinHostPort(srv.Host, srv.Port))}, srv.PublicKey)

	sess, err := Connect(context.Background(), plugin.ConnectConfig{Config: cfg, Net: pluginNet{}})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = sess.Close() }()
	sshSess := sess.(*Session)
	if sshSess.sftp != nil {
		t.Fatal("sftp opened before first filesystem use")
	}
	first, err := sshSess.Filesystem()
	if err != nil {
		t.Fatalf("Filesystem first: %v", err)
	}
	second, err := sshSess.Filesystem()
	if err != nil {
		t.Fatalf("Filesystem second: %v", err)
	}
	if first != second {
		t.Fatal("filesystem client was not reused")
	}
}
