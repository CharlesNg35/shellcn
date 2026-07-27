package rfb

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"io"
	"testing"
)

// zrleDecodeRect is a faithful port of noVNC's core/decoders/zrle.js. If a
// framebuffer survives our encoder -> this decoder round trip, noVNC decodes it
// identically. Pixels are returned as R|G<<8|B<<16 keys, row-major.
func zrleDecodeRect(t *testing.T, r io.Reader, w, h int) []uint32 {
	t.Helper()
	pixels := make([]uint32, w*h)

	readByte := func() byte {
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			t.Fatalf("read byte: %v", err)
		}
		return b[0]
	}
	readCPixels := func(n int) []uint32 {
		buf := make([]byte, 3*n)
		if _, err := io.ReadFull(r, buf); err != nil {
			t.Fatalf("read %d cpixels: %v", n, err)
		}
		out := make([]uint32, n)
		for i := 0; i < n; i++ {
			out[i] = uint32(buf[i*3]) | uint32(buf[i*3+1])<<8 | uint32(buf[i*3+2])<<16
		}
		return out
	}
	readRLELen := func() int {
		length := 0
		for {
			c := readByte()
			length += int(c)
			if c != 255 {
				break
			}
		}
		return length + 1
	}

	for ty := 0; ty < h; ty += zrleTile {
		th := min(zrleTile, h-ty)
		for tx := 0; tx < w; tx += zrleTile {
			tw := min(zrleTile, w-tx)
			tileSize := tw * th
			tile := make([]uint32, tileSize)

			switch sub := readByte(); {
			case sub == 0: // raw
				copy(tile, readCPixels(tileSize))
			case sub == 1: // solid
				c := readCPixels(1)[0]
				for i := range tile {
					tile[i] = c
				}
			case sub >= 2 && sub <= 16: // packed palette
				palette := readCPixels(int(sub))
				bpp := paletteBits(int(sub))
				mask := byte((1 << bpp) - 1)
				off := 0
				encoded := readByte()
				for y := 0; y < th; y++ {
					shift := 8 - bpp
					for x := 0; x < tw; x++ {
						if shift < 0 {
							shift = 8 - bpp
							encoded = readByte()
						}
						tile[off] = palette[(encoded>>uint(shift))&mask]
						off++
						shift -= bpp
					}
					if shift < 8-bpp && y < th-1 {
						encoded = readByte()
					}
				}
			case sub == 128: // plain RLE
				i := 0
				for i < tileSize {
					c := readCPixels(1)[0]
					length := readRLELen()
					for j := 0; j < length; j++ {
						tile[i] = c
						i++
					}
				}
			case sub >= 130: // palette RLE
				palette := readCPixels(int(sub) - 128)
				off := 0
				for off < tileSize {
					idx := int(readByte())
					length := 1
					if idx >= 128 {
						idx -= 128
						length = readRLELen()
					}
					for j := 0; j < length; j++ {
						tile[off] = palette[idx]
						off++
					}
				}
			default:
				t.Fatalf("unknown subencoding %d", sub)
			}

			for row := 0; row < th; row++ {
				for col := 0; col < tw; col++ {
					pixels[(ty+row)*w+tx+col] = tile[row*tw+col]
				}
			}
		}
	}
	return pixels
}

// zrleEncodeAndDecode runs a rect through the real encoder and the noVNC-port
// decoder, returning the reconstructed keys. It also asserts the wire framing.
func zrleEncodeAndDecode(t *testing.T, pixels []byte, w, h int) []uint32 {
	t.Helper()
	rects := []rectSnapshot{{Rect: Rect{X: 0, Y: 0, W: w, H: h}, Pixels: pixels}}
	got := newUpdateEncoder().encode(rects, nativePixelFormat(), nil, false, true, zlib.BestSpeed)
	if enc := int32(binary.BigEndian.Uint32(got[12:])); enc != encZRLE {
		t.Fatalf("encoding = %d, want ZRLE %d", enc, encZRLE)
	}
	n := int(binary.BigEndian.Uint32(got[16:]))
	if 20+n != len(got) {
		t.Fatalf("payload length %d, framing left %d", n, len(got)-20)
	}
	zr, err := zlib.NewReader(bytes.NewReader(got[20:]))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	defer func() { _ = zr.Close() }()
	return zrleDecodeRect(t, zr, w, h)
}

func originalKeys(pixels []byte, w, h int) []uint32 {
	keys := make([]uint32, w*h)
	for i := 0; i < w*h; i++ {
		keys[i] = uint32(pixels[i*4]) | uint32(pixels[i*4+1])<<8 | uint32(pixels[i*4+2])<<16
	}
	return keys
}

func setPixel(pixels []byte, w, x, y int, r, g, b byte) {
	i := (y*w + x) * 4
	pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = r, g, b, 0
}

func assertRoundTrip(t *testing.T, name string, pixels []byte, w, h int) {
	t.Helper()
	got := zrleEncodeAndDecode(t, pixels, w, h)
	want := originalKeys(pixels, w, h)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: pixel %d (%d,%d) = %06x, want %06x", name, i, i%w, i/w, got[i], want[i])
		}
	}
}

func TestZRLERoundTripSolid(t *testing.T) {
	w, h := 70, 66 // spans 2x2 tiles with partial edges
	pixels := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pixels[i*4], pixels[i*4+1], pixels[i*4+2] = 0x30, 0x60, 0x90
	}
	assertRoundTrip(t, "solid", pixels, w, h)
}

func TestZRLERoundTripPackedPalette(t *testing.T) {
	// A few colours in a fine checkerboard: short runs favour packed palette.
	w, h := 64, 64
	pixels := make([]byte, w*h*4)
	colors := [][3]byte{{10, 10, 10}, {200, 30, 30}, {30, 200, 30}, {30, 30, 200}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := colors[(x+y*3)%len(colors)]
			setPixel(pixels, w, x, y, c[0], c[1], c[2])
		}
	}
	assertRoundTrip(t, "packed-palette", pixels, w, h)
}

func TestZRLERoundTripPaletteRLE(t *testing.T) {
	// A few colours in wide horizontal bands: long runs favour palette RLE.
	w, h := 100, 80
	pixels := make([]byte, w*h*4)
	colors := [][3]byte{{5, 5, 5}, {250, 250, 250}, {120, 40, 200}}
	for y := 0; y < h; y++ {
		c := colors[(y/7)%len(colors)]
		for x := 0; x < w; x++ {
			setPixel(pixels, w, x, y, c[0], c[1], c[2])
		}
	}
	assertRoundTrip(t, "palette-rle", pixels, w, h)
}

func TestZRLERoundTripPlainRLE(t *testing.T) {
	// >128 distinct colours but with runs: forces plain RLE over palette modes.
	w, h := 64, 64
	pixels := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		v := byte((i / 16) % 200) // runs of 16, 200 distinct colours
		pixels[i*4], pixels[i*4+1], pixels[i*4+2] = v, byte(255-int(v)), v/2
	}
	assertRoundTrip(t, "plain-rle", pixels, w, h)
}

func TestZRLERoundTripRawAllUnique(t *testing.T) {
	// Every pixel distinct: no runs, no palette -> raw tiles.
	w, h := 64, 48
	pixels := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pixels[i*4], pixels[i*4+1], pixels[i*4+2] = byte(i), byte(i>>8), byte(i>>4)
	}
	assertRoundTrip(t, "raw", pixels, w, h)
}

func TestZRLERoundTripMixedLargeFrame(t *testing.T) {
	// A larger frame with distinct regions so several subencodings coexist and
	// partial edge tiles are exercised.
	w, h := 200, 150
	pixels := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < 64: // solid
				setPixel(pixels, w, x, y, 20, 40, 60)
			case x < 130: // gradient (many colours, runs by row)
				setPixel(pixels, w, x, y, byte(x), byte(y), byte(x^y))
			default: // few-colour bands
				c := byte(40 * ((y / 5) % 4))
				setPixel(pixels, w, x, y, c, 255-c, 128)
			}
		}
	}
	assertRoundTrip(t, "mixed", pixels, w, h)
}

func TestZRLEPersistentStreamAcrossRects(t *testing.T) {
	// The zlib stream is continuous across rects; the client keeps one inflate
	// context. Encode two rects on one encoder and decode both from one stream.
	w, h := 64, 64
	mk := func(base byte) []byte {
		p := make([]byte, w*h*4)
		for i := 0; i < w*h; i++ {
			v := base + byte((i/16)%50)
			p[i*4], p[i*4+1], p[i*4+2] = v, v, v
		}
		return p
	}
	e := newUpdateEncoder()
	// The returned slice aliases the encoder's buffer, so copy each payload
	// before the next call reuses it (production copies via append immediately).
	a, okA := e.encodeZRLERect(rectSnapshot{Rect: Rect{W: w, H: h}, Pixels: mk(10)}, zlib.BestSpeed)
	stream := append([]byte{}, a...)
	b, okB := e.encodeZRLERect(rectSnapshot{Rect: Rect{W: w, H: h}, Pixels: mk(120)}, zlib.BestSpeed)
	stream = append(stream, b...)
	if !okA || !okB {
		t.Fatal("encode failed")
	}
	zr, err := zlib.NewReader(bytes.NewReader(stream))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	defer func() { _ = zr.Close() }()
	got1 := zrleDecodeRect(t, zr, w, h)
	got2 := zrleDecodeRect(t, zr, w, h)
	if want := originalKeys(mk(10), w, h); !equalKeys(got1, want) {
		t.Fatal("rect 1 mismatch")
	}
	if want := originalKeys(mk(120), w, h); !equalKeys(got2, want) {
		t.Fatal("rect 2 mismatch")
	}
}

func equalKeys(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestReadSetEncodingsPrefersZRLE(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0, 0, 2})
	writeInt32(&buf, encZRLE)
	writeInt32(&buf, encZlib)

	s := NewFramebufferServer(&buf, 1, 1)
	if err := s.readSetEncodings(); err != nil {
		t.Fatalf("readSetEncodings: %v", err)
	}
	if !s.useZRLE {
		t.Fatal("useZRLE = false, want true")
	}
	// The encode dispatch prefers ZRLE over Zlib when both are offered.
	pixels := bytes.Repeat([]byte{10, 20, 30, 0}, 32*32)
	rects := []rectSnapshot{{Rect: Rect{W: 32, H: 32}, Pixels: pixels}}
	got := newUpdateEncoder().encode(rects, nativePixelFormat(), nil, s.useZlib, s.useZRLE, zlib.BestSpeed)
	if enc := int32(binary.BigEndian.Uint32(got[12:])); enc != encZRLE {
		t.Fatalf("encoding = %d, want ZRLE %d", enc, encZRLE)
	}
}

// Guard against an accidental change to the run-length wire format.
func TestZRLERunLengthEncoding(t *testing.T) {
	cases := map[int][]byte{1: {0}, 2: {1}, 255: {254}, 256: {255, 0}, 511: {255, 255, 0}, 512: {255, 255, 1}}
	for length, want := range cases {
		if got := appendRLELen(nil, length); !bytes.Equal(got, want) {
			t.Errorf("appendRLELen(%d) = %v, want %v", length, got, want)
		}
	}
}
