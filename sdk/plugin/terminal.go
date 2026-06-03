package plugin

import "encoding/json"

// Resizer is implemented by channels whose upstream can change terminal size.
type Resizer interface {
	Resize(cols, rows int) error
}

// CopyTerminalInput forwards browser keystrokes from client to ch, handling the
// terminal panel's in-band control frames: a frame beginning with 0x00 carries
// JSON ({"type":"resize","cols":N,"rows":N}) and is applied to ch when it
// implements Resizer instead of being written upstream. Use it as the
// browser→upstream half of a terminal StreamHandler:
//
//	go func() { errc <- plugin.CopyTerminalInput(ch, client) }()
func CopyTerminalInput(ch Channel, client ClientStream) error {
	buf := make([]byte, 32<<10)
	for {
		n, err := client.Read(buf)
		if n > 0 {
			frame := buf[:n]
			if len(frame) > 1 && frame[0] == 0 {
				applyTerminalControl(ch, frame[1:])
			} else if _, werr := ch.Write(frame); werr != nil {
				return werr
			}
		}
		if err != nil {
			return err
		}
	}
}

func applyTerminalControl(ch Channel, frame []byte) {
	cols, rows, ok := ParseResizeControl(frame)
	if !ok {
		return
	}
	if r, ok := ch.(Resizer); ok {
		_ = r.Resize(cols, rows)
	}
}

// ParseResizeControl decodes a terminal control frame's JSON payload (the bytes
// after the 0x00 prefix); ok is false for anything that is not a resize.
func ParseResizeControl(frame []byte) (cols, rows int, ok bool) {
	var msg struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if json.Unmarshal(frame, &msg) != nil || msg.Type != "resize" {
		return 0, 0, false
	}
	return msg.Cols, msg.Rows, true
}
