// Package rfb implements the minimal RFB (VNC) protocol pieces ShellCN needs: a
// gateway-side handshake toward the browser using Security None (shared by the
// VNC and RDP plugins) plus upstream VNC authentication. The browser renders the
// stream with noVNC; ShellCN never exposes the upstream password to the client.
package rfb

import (
	"fmt"
	"io"
)

// protocolVersion is the RFB version ShellCN speaks on both sides of the bridge.
const protocolVersion = "RFB 003.008\n"

const (
	secNone    = 1
	secVNCAuth = 2
)

// ServerHandshakeNone performs the gateway-side RFB 3.8 handshake toward the
// browser using Security type None — the gateway has already authenticated
// upstream, so the client never sees a password challenge. After it returns the
// caller must write the ServerInit message; from then on the connection carries
// raw RFB bytes.
func ServerHandshakeNone(client io.ReadWriter) error {
	if _, err := io.WriteString(client, protocolVersion); err != nil {
		return fmt.Errorf("write version: %w", err)
	}
	if _, err := io.ReadFull(client, make([]byte, 12)); err != nil {
		return fmt.Errorf("read client version: %w", err)
	}
	if _, err := client.Write([]byte{1, secNone}); err != nil {
		return fmt.Errorf("write security types: %w", err)
	}
	chosen := make([]byte, 1)
	if _, err := io.ReadFull(client, chosen); err != nil {
		return fmt.Errorf("read security choice: %w", err)
	}
	if chosen[0] != secNone {
		return fmt.Errorf("client selected unsupported security type %d", chosen[0])
	}
	if _, err := client.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("write security result: %w", err)
	}
	if _, err := io.ReadFull(client, make([]byte, 1)); err != nil {
		return fmt.Errorf("read client init: %w", err)
	}
	return nil
}
