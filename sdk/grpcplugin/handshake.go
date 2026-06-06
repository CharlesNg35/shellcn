// Package grpcplugin holds the go-plugin wire wiring shared by the core host
// adapter and the plugin SDK: the handshake and the dispense key.
package grpcplugin

import goplugin "github.com/hashicorp/go-plugin"

// ProtocolVersion is the plugin wire-contract version. Bump it on any breaking
// change to the proto or the manifest JSON shape; the host refuses a mismatch.
const ProtocolVersion = 2

// PluginName is the dispense key under which the Plugin service is served.
const PluginName = "plugin"

// Handshake is the magic-cookie handshake both ends share. It is not a security
// boundary — it only makes go-plugin refuse an unrelated executable with a clear
// error instead of a confusing protocol failure.
var Handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  ProtocolVersion,
	MagicCookieKey:   "SHELLCN_PLUGIN",
	MagicCookieValue: "shellcn-plugin-handshake",
}
