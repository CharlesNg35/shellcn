package rfb

import (
	"bytes"
	"compress/zlib"
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

	// fb is RGBX; the red channel sits at byte offset 0 of each pixel.
	topRed := s.fb[(0*s.width+0)*4]
	bottomRed := s.fb[(1*s.width+0)*4]
	if topRed != 10 || bottomRed != 20 {
		t.Fatalf("rows flipped: top red = %d (want 10), bottom red = %d (want 20)", topRed, bottomRed)
	}
}

func TestPushBitmapClipsDirtyRect(t *testing.T) {
	s := NewFramebufferServer(nil, 2, 2)
	data := []byte{
		1, 2, 3, 0, 4, 5, 6, 0,
		7, 8, 9, 0, 10, 11, 12, 0,
	}
	s.PushBitmap(-1, -1, 2, 2, 4, data)

	if len(s.dirty) != 1 {
		t.Fatalf("dirty rects = %d, want 1", len(s.dirty))
	}
	if got, want := s.dirty[0], (Rect{X: 0, Y: 0, W: 1, H: 1}); got != want {
		t.Fatalf("dirty rect = %+v, want %+v", got, want)
	}
	if got := s.fb[0:4]; !bytes.Equal(got, []byte{12, 11, 10, 0}) {
		t.Fatalf("clipped pixel = %v, want [12 11 10 0]", got)
	}
}

func TestEncodeUpdateNativeRawCopiesRows(t *testing.T) {
	rects := []rectSnapshot{{
		Rect:   Rect{X: 1, Y: 2, W: 2, H: 1},
		Pixels: []byte{1, 2, 3, 0, 4, 5, 6, 0},
	}}
	got := encodeUpdate(rects, nativePixelFormat(), nil)
	want := []byte{
		0, 0, 0, 1,
		0, 1, 0, 2, 0, 2, 0, 1,
		0, 0, 0, 0,
		1, 2, 3, 0, 4, 5, 6, 0,
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded update = %v, want %v", got, want)
	}
}

func TestEncodeUpdateNativeRawAllocations(t *testing.T) {
	rects := []rectSnapshot{{
		Rect:   Rect{W: 64, H: 64},
		Pixels: make([]byte, 64*64*4),
	}}
	allocs := testing.AllocsPerRun(100, func() {
		_ = encodeUpdate(rects, nativePixelFormat(), nil)
	})
	if allocs > 2 {
		t.Fatalf("encodeUpdate native allocations = %.2f, want <= 2", allocs)
	}
}

func TestReadSetEncodingsTracksZlib(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 3})
	writeInt32(&buf, encZlib)
	writeInt32(&buf, pseudoEncodingCompressLevel0+3)
	writeInt32(&buf, encCursor)

	s := NewFramebufferServer(&buf, 1, 1)
	if err := s.readSetEncodings(); err != nil {
		t.Fatalf("readSetEncodings: %v", err)
	}
	if !s.useZlib {
		t.Fatal("useZlib = false, want true")
	}
	if s.zlibLevel != 3 {
		t.Fatalf("zlibLevel = %d, want 3", s.zlibLevel)
	}
	if !s.wantCursor {
		t.Fatal("wantCursor = false, want true")
	}
}

func TestEncodeUpdateUsesZlibForNativeLargeRect(t *testing.T) {
	pixels := bytes.Repeat([]byte{10, 20, 30, 0}, 32*32)
	rects := []rectSnapshot{{
		Rect:   Rect{X: 2, Y: 3, W: 32, H: 32},
		Pixels: pixels,
	}}
	got := newUpdateEncoder().encode(rects, nativePixelFormat(), nil, true, zlib.BestSpeed)

	if got[0] != 0 || binary.BigEndian.Uint16(got[2:]) != 1 {
		t.Fatalf("bad update header: %v", got[:4])
	}
	if enc := int32(binary.BigEndian.Uint32(got[12:])); enc != encZlib {
		t.Fatalf("encoding = %d, want %d", enc, encZlib)
	}
	n := int(binary.BigEndian.Uint32(got[16:]))
	if n <= 0 || 20+n != len(got) {
		t.Fatalf("compressed length = %d, payload size = %d", n, len(got))
	}
	zr, err := zlib.NewReader(bytes.NewReader(got[20:]))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	defer func() { _ = zr.Close() }()
	decoded := make([]byte, len(pixels))
	if _, err := io.ReadFull(zr, decoded); err != nil {
		t.Fatalf("inflate: %v", err)
	}
	if !bytes.Equal(decoded, pixels) {
		t.Fatal("inflated pixels do not match original")
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

func writeInt32(w io.Writer, v int32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], uint32(v))
	_, _ = w.Write(b[:])
}
