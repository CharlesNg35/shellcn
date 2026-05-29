package ftpfs

import (
	"net"
	"testing"
	"time"
)

func TestFTPControlConnClearsStaleDeadlineBeforeWrite(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	conn := ftpControlConn{Conn: client}
	if err := conn.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatalf("set deadline: %v", err)
	}

	readDone := make(chan error, 1)
	go func() {
		buf := make([]byte, 4)
		_, err := server.Read(buf)
		readDone <- err
	}()

	if _, err := conn.Write([]byte("PING")); err != nil {
		t.Fatalf("write after stale deadline: %v", err)
	}
	if err := <-readDone; err != nil {
		t.Fatalf("server read: %v", err)
	}
}
