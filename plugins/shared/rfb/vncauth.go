package rfb

import (
	"crypto/des" //nolint:gosec // VNC authentication is defined in terms of DES.
	"encoding/binary"
	"fmt"
	"io"
	"slices"
)

// DialVNC performs the client-side RFB 3.8 handshake and authentication against
// an upstream VNC server reachable over conn. On success conn is positioned
// immediately after ServerInit, and the returned bytes are the ServerInit
// message to forward to the browser verbatim. Only the count-prefixed security
// list of RFB 3.7+/3.8 is supported.
func DialVNC(conn io.ReadWriter, password string) ([]byte, error) {
	if _, err := io.ReadFull(conn, make([]byte, 12)); err != nil {
		return nil, fmt.Errorf("read server version: %w", err)
	}
	if _, err := io.WriteString(conn, protocolVersion); err != nil {
		return nil, fmt.Errorf("write client version: %w", err)
	}

	count := make([]byte, 1)
	if _, err := io.ReadFull(conn, count); err != nil {
		return nil, fmt.Errorf("read security count: %w", err)
	}
	if count[0] == 0 {
		return nil, fmt.Errorf("vnc server rejected connection: %s", readReason(conn))
	}
	types := make([]byte, count[0])
	if _, err := io.ReadFull(conn, types); err != nil {
		return nil, fmt.Errorf("read security types: %w", err)
	}
	chosen, err := chooseSecurity(types, password)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte{chosen}); err != nil {
		return nil, fmt.Errorf("write security choice: %w", err)
	}

	if chosen == secVNCAuth {
		challenge := make([]byte, 16)
		if _, err := io.ReadFull(conn, challenge); err != nil {
			return nil, fmt.Errorf("read vnc challenge: %w", err)
		}
		resp, err := vncAuthResponse(password, challenge)
		if err != nil {
			return nil, err
		}
		if _, err := conn.Write(resp); err != nil {
			return nil, fmt.Errorf("write vnc response: %w", err)
		}
	}

	result := make([]byte, 4)
	if _, err := io.ReadFull(conn, result); err != nil {
		return nil, fmt.Errorf("read security result: %w", err)
	}
	if binary.BigEndian.Uint32(result) != 0 {
		return nil, fmt.Errorf("vnc authentication failed: %s", readReason(conn))
	}

	// ClientInit: shared flag set so other viewers stay connected.
	if _, err := conn.Write([]byte{1}); err != nil {
		return nil, fmt.Errorf("write client init: %w", err)
	}

	// ServerInit: 2 width, 2 height, 16 pixel-format, 4 name-length, then name.
	head := make([]byte, 24)
	if _, err := io.ReadFull(conn, head); err != nil {
		return nil, fmt.Errorf("read server init: %w", err)
	}
	name := make([]byte, binary.BigEndian.Uint32(head[20:24]))
	if _, err := io.ReadFull(conn, name); err != nil {
		return nil, fmt.Errorf("read server name: %w", err)
	}
	return append(head, name...), nil
}

func chooseSecurity(types []byte, password string) (byte, error) {
	has := func(t byte) bool { return slices.Contains(types, t) }
	if password != "" && has(secVNCAuth) {
		return secVNCAuth, nil
	}
	if has(secNone) {
		return secNone, nil
	}
	if has(secVNCAuth) {
		return 0, fmt.Errorf("vnc server requires a password")
	}
	return 0, fmt.Errorf("vnc server offered no supported security type")
}

func readReason(conn io.Reader) string {
	length := make([]byte, 4)
	if _, err := io.ReadFull(conn, length); err != nil {
		return "unknown"
	}
	msg := make([]byte, binary.BigEndian.Uint32(length))
	if _, err := io.ReadFull(conn, msg); err != nil {
		return "unknown"
	}
	return string(msg)
}

// vncAuthResponse encrypts the 16-byte challenge with the VNC password using DES
// in ECB mode. VNC reverses the bit order of each key byte; the password is
// truncated or zero-padded to the 8-byte DES key length.
func vncAuthResponse(password string, challenge []byte) ([]byte, error) {
	key := make([]byte, 8)
	copy(key, password)
	for i := range key {
		key[i] = reverseBits(key[i])
	}
	block, err := des.NewCipher(key) //nolint:gosec // DES is mandated by the VNC auth scheme.
	if err != nil {
		return nil, fmt.Errorf("vnc des key: %w", err)
	}
	out := make([]byte, 16)
	block.Encrypt(out[0:8], challenge[0:8])
	block.Encrypt(out[8:16], challenge[8:16])
	return out, nil
}

func reverseBits(b byte) byte {
	var r byte
	for range 8 {
		r = (r << 1) | (b & 1)
		b >>= 1
	}
	return r
}
