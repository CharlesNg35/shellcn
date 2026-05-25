package plugin

import "slices"

// IconType selects how an Icon's Value is interpreted by the renderer.
type IconType string

const (
	IconName   IconType = "name"   // built-in glyph id, e.g. "terminal"
	IconURL    IconType = "url"    // remote image
	IconBase64 IconType = "base64" // inline data URI
	IconEmoji  IconType = "emoji"  // single emoji
	IconSVG    IconType = "svg"    // raw inline SVG markup (sanitized before render)
)

// Icon is a structured icon reference used by every icon field in the manifest.
type Icon struct {
	Type  IconType `json:"type"`
	Value string   `json:"value"`
}

// Capability is a declarative feature tag — for panel selection only, never
// behavior dispatch.
type Capability string

// Layout selects how the connection workspace is arranged.
type Layout string

const (
	LayoutTabs        Layout = "tabs"
	LayoutSidebarTree Layout = "sidebar_tree"
)

// Transport is how a session reaches its target (orthogonal to protocol).
type Transport string

const (
	TransportDirect Transport = "direct"
	TransportAgent  Transport = "agent"
)

// AgentMode is what an enrolled agent proxies on the target side.
type AgentMode string

const (
	AgentTCP  AgentMode = "tcp"
	AgentUnix AgentMode = "unix"
	AgentK8s  AgentMode = "k8s_reverse_proxy"
	AgentHTTP AgentMode = "http_proxy"
)

// ProxyTarget describes the single endpoint an agent exposes back to the gateway.
type ProxyTarget struct {
	Mode    AgentMode
	Address string
	Risk    RiskLevel
}

// InstallArtifact is a launch recipe shown to the user to start an agent.
type InstallArtifact struct {
	Label    string
	Kind     string
	Template string
}

// AgentProfile is required iff a plugin declares TransportAgent.
type AgentProfile struct {
	Proxy   ProxyTarget
	Install []InstallArtifact
}

// Manifest is a plugin's single versioned contract — pure declarative data.
// Route metadata (permission/risk/audit/input) lives on Routes, not here.
type Manifest struct {
	APIVersion  int
	Name        string
	Version     string
	Title       string
	Description string
	Icon        Icon

	Config       Schema
	Capabilities []Capability

	SupportedTransports []Transport
	Agent               *AgentProfile

	Layout    Layout
	Tabs      []Tab
	Tree      []TreeGroup
	Resources []ResourceType
	Actions   []Action
	Streams   []Stream
}

// SupportsTransport reports whether the manifest declares the given transport.
func (m Manifest) SupportsTransport(t Transport) bool {
	return slices.Contains(m.SupportedTransports, t)
}
