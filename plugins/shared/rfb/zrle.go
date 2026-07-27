package rfb

import "compress/zlib"

// ZRLE (RFC 6143 §7.7.6) tiles a rectangle into 64×64 blocks and, per tile,
// picks the smallest of solid / packed-palette / RLE / palette-RLE / raw before
// running the whole rectangle through the connection's persistent zlib stream.
// It is lossless and compresses far better than plain Zlib for desktop content
// (text, flat UI, gradients). CPIXELs are 3 bytes (R,G,B) for our native 32bpp
// format. The byte layout is written to match noVNC's ZRLE decoder exactly.
const (
	encZRLE  = 16
	zrleTile = 64
)

type zrleRun struct {
	color  uint32
	length int
}

// encodeZRLERect builds the tile stream for one native rect and compresses it
// through the persistent ZRLE zlib stream. The caller writes the rect header and
// the 4-byte length prefix.
func (e *updateEncoder) encodeZRLERect(rc rectSnapshot, level int) ([]byte, bool) {
	return e.zrleCompress(buildZRLETiles(rc), level)
}

func buildZRLETiles(rc rectSnapshot) []byte {
	w, h := rc.W, rc.H
	out := make([]byte, 0, w*h*3/2+64)
	tp := make([]uint32, 0, zrleTile*zrleTile)
	for ty := 0; ty < h; ty += zrleTile {
		th := min(zrleTile, h-ty)
		for tx := 0; tx < w; tx += zrleTile {
			tw := min(zrleTile, w-tx)
			tp = tp[:0]
			for row := 0; row < th; row++ {
				base := ((ty+row)*w + tx) * 4
				for col := 0; col < tw; col++ {
					p := base + col*4
					tp = append(tp, uint32(rc.Pixels[p])|uint32(rc.Pixels[p+1])<<8|uint32(rc.Pixels[p+2])<<16)
				}
			}
			out = appendZRLETile(out, tp, tw, th)
		}
	}
	return out
}

// appendZRLETile emits one tile using whichever subencoding is smallest.
func appendZRLETile(out []byte, tp []uint32, tw, th int) []byte {
	palette := make([]uint32, 0, 16)
	index := make(map[uint32]int, 16)
	overflow := false
	for _, c := range tp {
		if _, ok := index[c]; !ok {
			if len(palette) < 128 {
				index[c] = len(palette)
				palette = append(palette, c)
			} else {
				overflow = true
			}
		}
	}
	pcount := len(palette)

	// Solid: a single colour for the whole tile.
	if !overflow && pcount == 1 {
		out = append(out, 1)
		return appendCPixel(out, palette[0])
	}

	runs := tileRuns(tp)

	rawSize := 1 + len(tp)*3
	best, choice := rawSize, 0 // 0 = raw

	if !overflow {
		if pcount >= 2 && pcount <= 16 {
			bpp := paletteBits(pcount)
			sz := 1 + pcount*3 + th*((tw*bpp+7)/8)
			if sz < best {
				best, choice = sz, 2 // packed palette
			}
		}
		if pcount >= 2 && pcount <= 127 {
			sz := 1 + pcount*3
			for _, r := range runs {
				sz++
				if r.length > 1 {
					sz += rleLenBytes(r.length)
				}
			}
			if sz < best {
				best, choice = sz, 3 // palette RLE
			}
		}
	}
	{
		sz := 1
		for _, r := range runs {
			sz += 3 + rleLenBytes(r.length)
		}
		if sz < best {
			choice = 1 // plain RLE
		}
	}

	switch choice {
	case 2: // packed palette (subencoding = palette size, 2..16)
		out = append(out, byte(pcount))
		for _, c := range palette {
			out = appendCPixel(out, c)
		}
		out = appendPackedIndices(out, tp, tw, th, index, paletteBits(pcount))
	case 3: // palette RLE (subencoding = 128 + palette size, 130..255)
		out = append(out, byte(pcount+128))
		for _, c := range palette {
			out = appendCPixel(out, c)
		}
		for _, r := range runs {
			idx := index[r.color]
			if r.length == 1 {
				out = append(out, byte(idx))
			} else {
				out = append(out, byte(idx|128))
				out = appendRLELen(out, r.length)
			}
		}
	case 1: // plain RLE
		out = append(out, 128)
		for _, r := range runs {
			out = appendCPixel(out, r.color)
			out = appendRLELen(out, r.length)
		}
	default: // raw
		out = append(out, 0)
		for _, c := range tp {
			out = appendCPixel(out, c)
		}
	}
	return out
}

func tileRuns(tp []uint32) []zrleRun {
	runs := make([]zrleRun, 0, 16)
	for i := 0; i < len(tp); {
		c := tp[i]
		j := i + 1
		for j < len(tp) && tp[j] == c {
			j++
		}
		runs = append(runs, zrleRun{color: c, length: j - i})
		i = j
	}
	return runs
}

func paletteBits(size int) int {
	switch {
	case size <= 2:
		return 1
	case size <= 4:
		return 2
	default:
		return 4
	}
}

// appendPackedIndices packs each row's palette indices MSB-first, byte-aligned
// per row (the last byte of a row is zero-padded), matching noVNC's decoder.
func appendPackedIndices(out []byte, tp []uint32, tw, th int, index map[uint32]int, bpp int) []byte {
	for row := 0; row < th; row++ {
		var cur byte
		bits := 0
		for col := 0; col < tw; col++ {
			cur = cur<<bpp | byte(index[tp[row*tw+col]])
			bits += bpp
			if bits == 8 {
				out = append(out, cur)
				cur, bits = 0, 0
			}
		}
		if bits > 0 {
			out = append(out, cur<<(8-bits))
		}
	}
	return out
}

// appendRLELen encodes a run length as noVNC reads it: bytes summing to length-1
// with 0xFF as the continuation marker.
func appendRLELen(out []byte, length int) []byte {
	length--
	for length >= 255 {
		out = append(out, 255)
		length -= 255
	}
	return append(out, byte(length))
}

func rleLenBytes(length int) int {
	return (length-1)/255 + 1
}

func appendCPixel(out []byte, key uint32) []byte {
	return append(out, byte(key), byte(key>>8), byte(key>>16))
}

func (e *updateEncoder) zrleCompress(data []byte, level int) ([]byte, bool) {
	if level < zlib.NoCompression || level > zlib.BestCompression {
		level = zlib.BestSpeed
	}
	if e.zrleWriter == nil {
		e.zrleBuf.Reset()
		zw, err := zlib.NewWriterLevel(&e.zrleBuf, level)
		if err != nil {
			return nil, false
		}
		e.zrleWriter = zw
	} else {
		e.zrleBuf.Reset()
	}
	if _, err := e.zrleWriter.Write(data); err != nil {
		return nil, false
	}
	if err := e.zrleWriter.Flush(); err != nil {
		return nil, false
	}
	return e.zrleBuf.Bytes(), true
}
