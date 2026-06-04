package rfb

import "encoding/binary"

// encCursor is the RFB "Cursor" pseudo-encoding (-239).
const encCursor = -239

// arrowCursor is the default pointer noVNC renders when the host's own cursor
// can't be forwarded: 'X' white outline, '.' black fill, ' ' transparent.
// Hotspot is the tip at (0,0).
var arrowCursor = []string{
	"X           ",
	"XX          ",
	"X.X         ",
	"X..X        ",
	"X...X       ",
	"X....X      ",
	"X.....X     ",
	"X......X    ",
	"X.......X   ",
	"X........X  ",
	"X.........X ",
	"X.....XXXXX ",
	"X..X..X     ",
	"X.X X..X    ",
	"XX  X..X    ",
	"X    X..X   ",
	"     X..X   ",
	"      X..X  ",
	"      XXX   ",
}

// encodeCursorRect builds a FramebufferUpdate rectangle carrying the default
// cursor in the client's pixel format: the rect header, the colour pixels, then
// the 1-bpp transparency mask (1 = opaque, MSB first, scanline-padded).
func encodeCursorRect(pf PixelFormat) []byte {
	w, h := cursorSize()
	bpp := int(pf.BitsPerPixel) / 8
	if bpp == 0 {
		bpp = 4
	}
	hdr := make([]byte, 12)
	// hotspot x/y stay 0 (hdr[0:4]); width/height + encoding follow.
	binary.BigEndian.PutUint16(hdr[4:], uint16(w))
	binary.BigEndian.PutUint16(hdr[6:], uint16(h))
	enc := int32(encCursor)
	binary.BigEndian.PutUint32(hdr[8:], uint32(enc))
	out := hdr

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rgb := [3]uint8{0, 0, 0}
			if cursorAt(x, y) == 'X' {
				rgb = [3]uint8{255, 255, 255}
			}
			out = append(out, packPixel(pf, rgb, bpp)...)
		}
	}

	scanline := (w + 7) / 8
	for y := 0; y < h; y++ {
		row := make([]byte, scanline)
		for x := 0; x < w; x++ {
			if cursorAt(x, y) != ' ' {
				row[x/8] |= 0x80 >> uint(x%8)
			}
		}
		out = append(out, row...)
	}
	return out
}

func cursorSize() (w, h int) {
	h = len(arrowCursor)
	for _, r := range arrowCursor {
		if len(r) > w {
			w = len(r)
		}
	}
	return w, h
}

func cursorAt(x, y int) byte {
	if y < 0 || y >= len(arrowCursor) {
		return ' '
	}
	r := arrowCursor[y]
	if x < 0 || x >= len(r) {
		return ' '
	}
	return r[x]
}
