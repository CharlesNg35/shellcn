package rfb

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"testing"
)

func TestReverseBits(t *testing.T) {
	cases := map[byte]byte{0x00: 0x00, 0xff: 0xff, 0x01: 0x80, 0x80: 0x01, 0xf0: 0x0f}
	for in, want := range cases {
		if got := reverseBits(in); got != want {
			t.Errorf("reverseBits(%#02x) = %#02x, want %#02x", in, got, want)
		}
	}
}

func TestServerHandshakeNone(t *testing.T) {
	srv, cli := net.Pipe()
	defer func() { _ = srv.Close() }()
	defer func() { _ = cli.Close() }()

	errc := make(chan error, 1)
	go func() { errc <- ServerHandshakeNone(srv) }()

	ver := make([]byte, 12)
	if _, err := io.ReadFull(cli, ver); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if string(ver) != protocolVersion {
		t.Fatalf("version = %q", ver)
	}
	if _, err := cli.Write([]byte(protocolVersion)); err != nil {
		t.Fatalf("write version: %v", err)
	}
	sec := make([]byte, 2)
	if _, err := io.ReadFull(cli, sec); err != nil {
		t.Fatalf("read security types: %v", err)
	}
	if sec[0] != 1 || sec[1] != secNone {
		t.Fatalf("security types = %v", sec)
	}
	if _, err := cli.Write([]byte{secNone}); err != nil {
		t.Fatalf("write choice: %v", err)
	}
	res := make([]byte, 4)
	if _, err := io.ReadFull(cli, res); err != nil {
		t.Fatalf("read result: %v", err)
	}
	if binary.BigEndian.Uint32(res) != 0 {
		t.Fatalf("security result = %v", res)
	}
	if _, err := cli.Write([]byte{1}); err != nil { // ClientInit shared flag
		t.Fatalf("write client init: %v", err)
	}
	if err := <-errc; err != nil {
		t.Fatalf("ServerHandshakeNone: %v", err)
	}
}

func TestDialVNCNoAuth(t *testing.T) {
	srv, cli := net.Pipe()
	defer func() { _ = srv.Close() }()
	defer func() { _ = cli.Close() }()

	want := buildServerInit(800, 600, "screen")
	go func() {
		_, _ = srv.Write([]byte(protocolVersion))
		_, _ = io.ReadFull(srv, make([]byte, 12))
		_, _ = srv.Write([]byte{1, secNone})     // offer only None
		_, _ = io.ReadFull(srv, make([]byte, 1)) // client choice
		_, _ = srv.Write([]byte{0, 0, 0, 0})     // SecurityResult OK
		_, _ = io.ReadFull(srv, make([]byte, 1)) // ClientInit
		_, _ = srv.Write(want)
	}()

	got, err := DialVNC(cli, "")
	if err != nil {
		t.Fatalf("DialVNC: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("server init mismatch:\n got %v\nwant %v", got, want)
	}
}

// TestPushBitmapTopDown guards against the framebuffer being rendered upside
// down: grdp's decoders emit top-down rows, so source row 0 must land on the top
// framebuffer row, not the bottom.
func TestPushBitmapTopDown(t *testing.T) {
	s := NewFramebufferServer(nil, 1, 2)
	// 1px-wide, 2px-tall BGRA source, top-down: row 0 has R=10, row 1 has R=20.
	data := []byte{0, 0, 10, 0, 0, 0, 20, 0}
	s.PushBitmap(0, 0, 1, 2, 4, data)

	// fb is BGRX; the red channel sits at byte offset 2 of each pixel.
	topRed := s.fb[(0*s.width+0)*4+2]
	bottomRed := s.fb[(1*s.width+0)*4+2]
	if topRed != 10 || bottomRed != 20 {
		t.Fatalf("rows flipped: top red = %d (want 10), bottom red = %d (want 20)", topRed, bottomRed)
	}
}

func buildServerInit(w, h int, name string) []byte {
	b := make([]byte, 24+len(name))
	binary.BigEndian.PutUint16(b[0:], uint16(w))
	binary.BigEndian.PutUint16(b[2:], uint16(h))
	binary.BigEndian.PutUint32(b[20:], uint32(len(name)))
	copy(b[24:], name)
	return b
}
