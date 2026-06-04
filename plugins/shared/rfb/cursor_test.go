package rfb

import (
	"encoding/binary"
	"testing"
)

func TestEncodeCursorRect(t *testing.T) {
	w, h := cursorSize()
	rect := encodeCursorRect(nativePixelFormat())

	wantLen := 12 + w*h*4 + ((w+7)/8)*h
	if len(rect) != wantLen {
		t.Fatalf("cursor rect len = %d, want %d", len(rect), wantLen)
	}
	if got := int16(binary.BigEndian.Uint16(rect[4:])); int(got) != w {
		t.Errorf("width = %d, want %d", got, w)
	}
	if enc := int32(binary.BigEndian.Uint32(rect[8:])); enc != encCursor {
		t.Errorf("encoding = %d, want %d", enc, encCursor)
	}
}
