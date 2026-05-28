package rfb

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// RFB client→server message types.
const (
	msgSetPixelFormat           = 0
	msgSetEncodings             = 2
	msgFramebufferUpdateRequest = 3
	msgKeyEvent                 = 4
	msgPointerEvent             = 5
	msgClientCutText            = 6
)

// InputSink receives the browser's input events, translated by the consuming
// plugin (e.g. RDP) into its own protocol.
type InputSink interface {
	KeyEvent(down bool, keysym uint32)
	PointerEvent(buttonMask uint8, x, y int)
}

// Rect is a framebuffer region in pixels.
type Rect struct{ X, Y, W, H int }

// PixelFormat mirrors the 16-byte RFB true-color pixel-format structure.
type PixelFormat struct {
	BitsPerPixel uint8
	Depth        uint8
	BigEndian    bool
	TrueColor    bool
	RedMax       uint16
	GreenMax     uint16
	BlueMax      uint16
	RedShift     uint8
	GreenShift   uint8
	BlueShift    uint8
}

// nativePixelFormat is the format the backing framebuffer is stored in and the
// one advertised in ServerInit: 32bpp little-endian, blue in the low byte.
func nativePixelFormat() PixelFormat {
	return PixelFormat{
		BitsPerPixel: 32, Depth: 24, TrueColor: true,
		RedMax: 255, GreenMax: 255, BlueMax: 255,
		RedShift: 16, GreenShift: 8, BlueShift: 0,
	}
}

func (pf PixelFormat) marshal() []byte {
	b := make([]byte, 16)
	b[0], b[1] = pf.BitsPerPixel, pf.Depth
	if pf.BigEndian {
		b[2] = 1
	}
	if pf.TrueColor {
		b[3] = 1
	}
	binary.BigEndian.PutUint16(b[4:], pf.RedMax)
	binary.BigEndian.PutUint16(b[6:], pf.GreenMax)
	binary.BigEndian.PutUint16(b[8:], pf.BlueMax)
	b[10], b[11], b[12] = pf.RedShift, pf.GreenShift, pf.BlueShift
	return b
}

func parsePixelFormat(b []byte) PixelFormat {
	return PixelFormat{
		BitsPerPixel: b[0], Depth: b[1], BigEndian: b[2] != 0, TrueColor: b[3] != 0,
		RedMax: binary.BigEndian.Uint16(b[4:]), GreenMax: binary.BigEndian.Uint16(b[6:]), BlueMax: binary.BigEndian.Uint16(b[8:]),
		RedShift: b[10], GreenShift: b[11], BlueShift: b[12],
	}
}

// FramebufferServer renders a synthetic RFB session to the browser. A protocol
// plugin (RDP) feeds decoded bitmaps via PushBitmap and receives input through
// an InputSink. Updates are coalesced and flushed only when the client has an
// outstanding FramebufferUpdateRequest, as the RFB protocol requires.
type FramebufferServer struct {
	rw     io.ReadWriter
	width  int
	height int
	sink   InputSink

	mu        sync.Mutex
	cond      *sync.Cond
	clientPF  PixelFormat
	fb        []byte // width*height*4, native BGRX32
	dirty     []Rect
	requested bool
	full      bool
	closed    bool
}

// NewFramebufferServer creates a server for a width×height desktop writing to rw.
func NewFramebufferServer(rw io.ReadWriter, width, height int) *FramebufferServer {
	s := &FramebufferServer{
		rw: rw, width: width, height: height,
		clientPF: nativePixelFormat(),
		fb:       make([]byte, width*height*4),
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

// Serve performs the gateway handshake, sends ServerInit, then runs the read and
// write pumps until the client disconnects or an error occurs.
func (s *FramebufferServer) Serve(sink InputSink) error {
	s.sink = sink
	if err := ServerHandshakeNone(s.rw); err != nil {
		return err
	}
	if err := s.writeServerInit(); err != nil {
		return err
	}
	done := make(chan struct{})
	go s.writePump(done)
	err := s.readLoop()
	s.close()
	<-done
	return err
}

func (s *FramebufferServer) writeServerInit() error {
	name := []byte("ShellCN")
	buf := make([]byte, 0, 24+len(name))
	hdr := make([]byte, 8)
	binary.BigEndian.PutUint16(hdr[0:], uint16(s.width))
	binary.BigEndian.PutUint16(hdr[2:], uint16(s.height))
	buf = append(buf, hdr[0:4]...)
	buf = append(buf, nativePixelFormat().marshal()...)
	nl := make([]byte, 4)
	binary.BigEndian.PutUint32(nl, uint32(len(name)))
	buf = append(buf, nl...)
	buf = append(buf, name...)
	_, err := s.rw.Write(buf)
	return err
}

// PushBitmap copies a decoded rectangle into the backing framebuffer (converting
// from the source bytes-per-pixel) and marks it dirty. grdp's bitmap decoders
// already emit top-down rows — they un-flip RDP's bottom-up wire format during
// decode — so a source row maps straight to the destination row.
func (s *FramebufferServer) PushBitmap(x, y, w, h, srcBytesPerPixel int, data []byte) {
	if w <= 0 || h <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for row := 0; row < h; row++ {
		dy := y + row
		if dy < 0 || dy >= s.height {
			continue
		}
		for col := 0; col < w; col++ {
			dx := x + col
			if dx < 0 || dx >= s.width {
				continue
			}
			si := (row*w + col) * srcBytesPerPixel
			if si+srcBytesPerPixel > len(data) {
				continue
			}
			r, g, b := decodeSource(data[si:], srcBytesPerPixel)
			di := (dy*s.width + dx) * 4
			s.fb[di], s.fb[di+1], s.fb[di+2], s.fb[di+3] = b, g, r, 0
		}
	}
	s.dirty = append(s.dirty, Rect{X: x, Y: y, W: w, H: h})
	s.cond.Signal()
}

// decodeSource extracts 8-bit R,G,B from one source pixel.
func decodeSource(p []byte, bytesPerPixel int) (r, g, b uint8) {
	switch bytesPerPixel {
	case 4, 3: // BGR(A/X)
		return p[2], p[1], p[0]
	case 2: // RGB565, little-endian
		v := binary.LittleEndian.Uint16(p)
		r = uint8((v>>11)&0x1f) << 3
		g = uint8((v>>5)&0x3f) << 2
		b = uint8(v&0x1f) << 3
		return r, g, b
	default:
		return p[0], p[0], p[0]
	}
}

func (s *FramebufferServer) readLoop() error {
	header := make([]byte, 1)
	for {
		if _, err := io.ReadFull(s.rw, header); err != nil {
			return ignoreEOF(err)
		}
		var err error
		switch header[0] {
		case msgSetPixelFormat:
			err = s.readSetPixelFormat()
		case msgSetEncodings:
			err = s.readSetEncodings()
		case msgFramebufferUpdateRequest:
			err = s.readUpdateRequest()
		case msgKeyEvent:
			err = s.readKeyEvent()
		case msgPointerEvent:
			err = s.readPointerEvent()
		case msgClientCutText:
			err = s.readClientCutText()
		default:
			err = fmt.Errorf("unknown rfb client message %d", header[0])
		}
		if err != nil {
			return ignoreEOF(err)
		}
	}
}

func (s *FramebufferServer) readSetPixelFormat() error {
	buf := make([]byte, 19) // 3 padding + 16 pixel-format
	if _, err := io.ReadFull(s.rw, buf); err != nil {
		return err
	}
	pf := parsePixelFormat(buf[3:])
	s.mu.Lock()
	s.clientPF = pf
	s.mu.Unlock()
	return nil
}

func (s *FramebufferServer) readSetEncodings() error {
	head := make([]byte, 3) // 1 padding + 2 count
	if _, err := io.ReadFull(s.rw, head); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint16(head[1:]))
	if n > 0 {
		if _, err := io.ReadFull(s.rw, make([]byte, n*4)); err != nil {
			return err
		}
	}
	return nil
}

func (s *FramebufferServer) readUpdateRequest() error {
	buf := make([]byte, 9) // incremental + x + y + w + h
	if _, err := io.ReadFull(s.rw, buf); err != nil {
		return err
	}
	incremental := buf[0] != 0
	s.mu.Lock()
	s.requested = true
	if !incremental {
		s.full = true
	}
	s.cond.Signal()
	s.mu.Unlock()
	return nil
}

func (s *FramebufferServer) readKeyEvent() error {
	buf := make([]byte, 7) // down-flag + 2 padding + 4 keysym
	if _, err := io.ReadFull(s.rw, buf); err != nil {
		return err
	}
	if s.sink != nil {
		s.sink.KeyEvent(buf[0] != 0, binary.BigEndian.Uint32(buf[3:]))
	}
	return nil
}

func (s *FramebufferServer) readPointerEvent() error {
	buf := make([]byte, 5) // button-mask + 2 x + 2 y
	if _, err := io.ReadFull(s.rw, buf); err != nil {
		return err
	}
	if s.sink != nil {
		s.sink.PointerEvent(buf[0], int(binary.BigEndian.Uint16(buf[1:])), int(binary.BigEndian.Uint16(buf[3:])))
	}
	return nil
}

func (s *FramebufferServer) readClientCutText() error {
	head := make([]byte, 7) // 3 padding + 4 length
	if _, err := io.ReadFull(s.rw, head); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint32(head[3:]))
	if n > 0 {
		if _, err := io.ReadFull(s.rw, make([]byte, n)); err != nil {
			return err
		}
	}
	return nil
}

func (s *FramebufferServer) writePump(done chan<- struct{}) {
	defer close(done)
	for {
		s.mu.Lock()
		for {
			ready := s.closed || (s.requested && (s.full || len(s.dirty) > 0))
			if ready {
				break
			}
			s.cond.Wait()
		}
		if s.closed {
			s.mu.Unlock()
			return
		}
		var rects []Rect
		if s.full {
			rects = []Rect{{X: 0, Y: 0, W: s.width, H: s.height}}
		} else {
			rects = s.dirty
		}
		s.dirty = nil
		s.full = false
		s.requested = false
		pf := s.clientPF
		payload := s.encodeUpdate(rects, pf)
		s.mu.Unlock()

		if _, err := s.rw.Write(payload); err != nil {
			s.close()
			return
		}
	}
}

// encodeUpdate builds a FramebufferUpdate of Raw rectangles. The caller holds
// the mutex so the backing framebuffer is read consistently.
func (s *FramebufferServer) encodeUpdate(rects []Rect, pf PixelFormat) []byte {
	bpp := int(pf.BitsPerPixel) / 8
	if bpp == 0 {
		bpp = 4
	}
	out := []byte{0, 0} // message-type 0 + padding
	count := make([]byte, 2)
	binary.BigEndian.PutUint16(count, uint16(len(rects)))
	out = append(out, count...)
	for _, rc := range rects {
		hdr := make([]byte, 12)
		binary.BigEndian.PutUint16(hdr[0:], uint16(rc.X))
		binary.BigEndian.PutUint16(hdr[2:], uint16(rc.Y))
		binary.BigEndian.PutUint16(hdr[4:], uint16(rc.W))
		binary.BigEndian.PutUint16(hdr[6:], uint16(rc.H))
		// encoding 0 = Raw (hdr[8:12] already zero)
		out = append(out, hdr...)
		for row := 0; row < rc.H; row++ {
			for col := 0; col < rc.W; col++ {
				px := s.pixelAt(rc.X+col, rc.Y+row)
				out = append(out, packPixel(pf, px, bpp)...)
			}
		}
	}
	return out
}

func (s *FramebufferServer) pixelAt(x, y int) (rgb [3]uint8) {
	if x < 0 || x >= s.width || y < 0 || y >= s.height {
		return rgb
	}
	i := (y*s.width + x) * 4
	return [3]uint8{s.fb[i+2], s.fb[i+1], s.fb[i]} // r,g,b from BGRX
}

func packPixel(pf PixelFormat, rgb [3]uint8, bpp int) []byte {
	v := scale(uint32(rgb[0]), pf.RedMax)<<pf.RedShift |
		scale(uint32(rgb[1]), pf.GreenMax)<<pf.GreenShift |
		scale(uint32(rgb[2]), pf.BlueMax)<<pf.BlueShift
	out := make([]byte, bpp)
	for i := 0; i < bpp; i++ {
		if pf.BigEndian {
			out[i] = byte(v >> (8 * (bpp - 1 - i)))
		} else {
			out[i] = byte(v >> (8 * i))
		}
	}
	return out
}

func scale(v8 uint32, maxv uint16) uint32 {
	if maxv == 0 {
		return 0
	}
	return v8 * uint32(maxv) / 255
}

func (s *FramebufferServer) close() {
	s.mu.Lock()
	s.closed = true
	s.cond.Signal()
	s.mu.Unlock()
}

func ignoreEOF(err error) error {
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return nil
	}
	return err
}
