package transport_test

import (
	"bytes"
	"testing"

	"github.com/charlesng35/shellcn/internal/transport"
)

func TestStreamTargetRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := transport.WriteStreamTarget(&buf, "tcp", "172.17.0.2:8080"); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Extra payload after the preamble must be left untouched for the proxy copy.
	buf.WriteString("GET / HTTP/1.1\r\n")

	network, addr, err := transport.ReadStreamTarget(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if network != "tcp" || addr != "172.17.0.2:8080" {
		t.Errorf("got %q,%q; want tcp,172.17.0.2:8080", network, addr)
	}
	if rest := buf.String(); rest != "GET / HTTP/1.1\r\n" {
		t.Errorf("preamble over-read; remaining = %q", rest)
	}
}
