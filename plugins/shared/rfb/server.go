package rfb

import (
	"bytes"
	"compress/zlib"
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

const (
	encRaw  = 0
	encZlib = 6

	pseudoEncodingCompressLevel9 = -247
	pseudoEncodingCompressLevel0 = -256
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
// one advertised in ServerInit: 32bpp little-endian, red in the low byte. This
// matches noVNC's preferred 24-bit true-colour format, so Raw updates can copy
// rows without repacking each pixel.
func nativePixelFormat() PixelFormat {
	return PixelFormat{
		BitsPerPixel: 32, Depth: 24, TrueColor: true,
		RedMax: 255, GreenMax: 255, BlueMax: 255,
		RedShift: 0, GreenShift: 8, BlueShift: 16,
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

	mu         sync.Mutex
	cond       *sync.Cond
	clientPF   PixelFormat
	fb         []byte // width*height*4, native RGBX32
	dirty      []Rect
	requested  bool
	full       bool
	closed     bool
	useZlib    bool
	zlibLevel  int
	wantCursor bool // client advertised the Cursor pseudo-encoding
	cursorSent bool
}

// NewFramebufferServer creates a server for a width×height desktop writing to rw.
func NewFramebufferServer(rw io.ReadWriter, width, height int) *FramebufferServer {
	s := &FramebufferServer{
		rw: rw, width: width, height: height,
		clientPF:  nativePixelFormat(),
		fb:        make([]byte, width*height*4),
		zlibLevel: zlib.BestSpeed,
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
	rc, ok := clipRect(Rect{X: x, Y: y, W: w, H: h}, s.width, s.height)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for dy := rc.Y; dy < rc.Y+rc.H; dy++ {
		row := dy - y
		for dx := rc.X; dx < rc.X+rc.W; dx++ {
			col := dx - x
			si := (row*w + col) * srcBytesPerPixel
			if si+srcBytesPerPixel > len(data) {
				continue
			}
			di := (dy*s.width + dx) * 4
			switch srcBytesPerPixel {
			case 4, 3: // BGR(A/X)
				s.fb[di], s.fb[di+1], s.fb[di+2], s.fb[di+3] = data[si+2], data[si+1], data[si], 0
			case 2: // RGB565, little-endian
				v := binary.LittleEndian.Uint16(data[si:])
				s.fb[di], s.fb[di+1], s.fb[di+2], s.fb[di+3] = uint8((v>>11)&0x1f)<<3, uint8((v>>5)&0x3f)<<2, uint8(v&0x1f)<<3, 0
			default:
				s.fb[di], s.fb[di+1], s.fb[di+2], s.fb[di+3] = data[si], data[si], data[si], 0
			}
		}
	}
	s.dirty = append(s.dirty, rc)
	s.cond.Signal()
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
	if n == 0 {
		return nil
	}
	body := make([]byte, n*4)
	if _, err := io.ReadFull(s.rw, body); err != nil {
		return err
	}
	s.mu.Lock()
	s.useZlib = false
	for i := 0; i < n; i++ {
		enc := int32(binary.BigEndian.Uint32(body[i*4:]))
		switch {
		case enc == encCursor:
			s.wantCursor = true
		case enc == encZlib:
			s.useZlib = true
		case enc >= pseudoEncodingCompressLevel0 && enc <= pseudoEncodingCompressLevel9:
			s.zlibLevel = int(enc - pseudoEncodingCompressLevel0)
		}
	}
	s.mu.Unlock()
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
	enc := newUpdateEncoder()
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
			rects = coalesceRects(s.dirty, s.width, s.height)
		}
		s.dirty = nil
		s.full = false
		s.requested = false
		pf := s.clientPF
		useZlib := s.useZlib
		zlibLevel := s.zlibLevel
		cursor := s.takeCursorLocked(pf)
		update := s.snapshotUpdateLocked(rects)
		s.mu.Unlock()

		payload := enc.encode(update, pf, cursor, useZlib, zlibLevel)
		if _, err := s.rw.Write(payload); err != nil {
			s.close()
			return
		}
	}
}

type rectSnapshot struct {
	Rect
	Pixels []byte
}

func (s *FramebufferServer) takeCursorLocked(pf PixelFormat) []byte {
	if !s.wantCursor || s.cursorSent {
		return nil
	}
	s.cursorSent = true
	return encodeCursorRect(pf)
}

// snapshotUpdateLocked copies dirty native framebuffer rows while holding the
// mutex. The slower client-format packing and socket write happen after unlock.
func (s *FramebufferServer) snapshotUpdateLocked(rects []Rect) []rectSnapshot {
	out := make([]rectSnapshot, 0, len(rects))
	for _, rc := range rects {
		clipped, ok := clipRect(rc, s.width, s.height)
		if !ok {
			continue
		}
		pixels := make([]byte, clipped.W*clipped.H*4)
		for row := 0; row < clipped.H; row++ {
			src := ((clipped.Y+row)*s.width + clipped.X) * 4
			dst := row * clipped.W * 4
			copy(pixels[dst:dst+clipped.W*4], s.fb[src:src+clipped.W*4])
		}
		out = append(out, rectSnapshot{Rect: clipped, Pixels: pixels})
	}
	return out
}

type updateEncoder struct {
	zlibBuf    bytes.Buffer
	zlibWriter *zlib.Writer
	zlibLevel  int
}

func newUpdateEncoder() *updateEncoder {
	return &updateEncoder{zlibLevel: zlib.BestSpeed}
}

// encodeUpdate builds a FramebufferUpdate of Raw rectangles.
func encodeUpdate(rects []rectSnapshot, pf PixelFormat, cursor []byte) []byte {
	return newUpdateEncoder().encode(rects, pf, cursor, false, zlib.BestSpeed)
}

func (e *updateEncoder) encode(rects []rectSnapshot, pf PixelFormat, cursor []byte, useZlib bool, zlibLevel int) []byte {
	bpp := int(pf.BitsPerPixel) / 8
	if bpp == 0 {
		bpp = 4
	}
	extra := 0
	if len(cursor) > 0 {
		extra = 1
	}
	out := make([]byte, 0, updatePayloadSize(rects, bpp, len(cursor)))
	out = append(out, 0, 0) // message-type 0 + padding
	var count [2]byte
	binary.BigEndian.PutUint16(count[:], uint16(len(rects)+extra))
	out = append(out, count[:]...)
	out = append(out, cursor...)
	for _, rc := range rects {
		if useZlib && isNativePixelFormat(pf) && len(rc.Pixels) >= 1024 {
			if compressed, ok := e.compress(rc.Pixels, zlibLevel); ok {
				out = appendRectHeader(out, rc.Rect, encZlib)
				var length [4]byte
				binary.BigEndian.PutUint32(length[:], uint32(len(compressed)))
				out = append(out, length[:]...)
				out = append(out, compressed...)
				continue
			}
		}
		out = appendRectHeader(out, rc.Rect, encRaw)
		if isNativePixelFormat(pf) {
			out = append(out, rc.Pixels...)
			continue
		}
		for i := 0; i+3 < len(rc.Pixels); i += 4 {
			out = appendPixel(out, pf, [3]uint8{rc.Pixels[i], rc.Pixels[i+1], rc.Pixels[i+2]}, bpp)
		}
	}
	return out
}

func (e *updateEncoder) compress(pixels []byte, level int) ([]byte, bool) {
	if level < zlib.NoCompression || level > zlib.BestCompression {
		level = zlib.BestSpeed
	}
	if e.zlibWriter == nil {
		e.zlibBuf.Reset()
		zw, err := zlib.NewWriterLevel(&e.zlibBuf, level)
		if err != nil {
			return nil, false
		}
		e.zlibWriter = zw
		e.zlibLevel = level
	} else {
		e.zlibBuf.Reset()
	}
	if _, err := e.zlibWriter.Write(pixels); err != nil {
		return nil, false
	}
	if err := e.zlibWriter.Flush(); err != nil {
		return nil, false
	}
	return e.zlibBuf.Bytes(), true
}

func appendRectHeader(out []byte, rc Rect, encoding int32) []byte {
	var hdr [12]byte
	binary.BigEndian.PutUint16(hdr[0:], uint16(rc.X))
	binary.BigEndian.PutUint16(hdr[2:], uint16(rc.Y))
	binary.BigEndian.PutUint16(hdr[4:], uint16(rc.W))
	binary.BigEndian.PutUint16(hdr[6:], uint16(rc.H))
	binary.BigEndian.PutUint32(hdr[8:], uint32(encoding))
	return append(out, hdr[:]...)
}

func updatePayloadSize(rects []rectSnapshot, bpp, cursorLen int) int {
	size := 4 + cursorLen
	for _, rc := range rects {
		size += 12 + rc.W*rc.H*bpp
	}
	return size
}

func coalesceRects(rects []Rect, width, height int) []Rect {
	if len(rects) == 0 {
		return nil
	}
	clipped := make([]Rect, 0, len(rects))
	totalArea := 0
	for _, rc := range rects {
		c, ok := clipRect(rc, width, height)
		if !ok {
			continue
		}
		clipped = append(clipped, c)
		totalArea += c.W * c.H
	}
	if len(clipped) == 0 {
		return nil
	}
	if len(clipped) <= 64 && totalArea < (width*height*3)/4 {
		return clipped
	}
	minX, minY := width, height
	maxX, maxY := 0, 0
	for _, rc := range clipped {
		if rc.X < minX {
			minX = rc.X
		}
		if rc.Y < minY {
			minY = rc.Y
		}
		if x2 := rc.X + rc.W; x2 > maxX {
			maxX = x2
		}
		if y2 := rc.Y + rc.H; y2 > maxY {
			maxY = y2
		}
	}
	return []Rect{{X: minX, Y: minY, W: maxX - minX, H: maxY - minY}}
}

func clipRect(rc Rect, width, height int) (Rect, bool) {
	x1, y1 := rc.X, rc.Y
	x2, y2 := rc.X+rc.W, rc.Y+rc.H
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	if x2 > width {
		x2 = width
	}
	if y2 > height {
		y2 = height
	}
	if x2 <= x1 || y2 <= y1 {
		return Rect{}, false
	}
	return Rect{X: x1, Y: y1, W: x2 - x1, H: y2 - y1}, true
}

func isNativePixelFormat(pf PixelFormat) bool {
	return pf.BitsPerPixel == 32 &&
		pf.Depth == 24 &&
		!pf.BigEndian &&
		pf.TrueColor &&
		pf.RedMax == 255 &&
		pf.GreenMax == 255 &&
		pf.BlueMax == 255 &&
		pf.RedShift == 0 &&
		pf.GreenShift == 8 &&
		pf.BlueShift == 16
}

func appendPixel(out []byte, pf PixelFormat, rgb [3]uint8, bpp int) []byte {
	v := scale(uint32(rgb[0]), pf.RedMax)<<pf.RedShift |
		scale(uint32(rgb[1]), pf.GreenMax)<<pf.GreenShift |
		scale(uint32(rgb[2]), pf.BlueMax)<<pf.BlueShift
	for i := 0; i < bpp; i++ {
		if pf.BigEndian {
			out = append(out, byte(v>>(8*(bpp-1-i))))
		} else {
			out = append(out, byte(v>>(8*i)))
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
