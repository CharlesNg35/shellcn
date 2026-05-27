package plugin

import "slices"

// IconType selects how an Icon's Value is interpreted by the renderer.
type IconType string

const (
	IconLucide IconType = "lucide" // Lucide icon name, kebab-case e.g. "ellipsis-vertical"
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
	LayoutTabs        Layout = "tabs"         // flat top tab bar, one panel at a time
	LayoutSidebarTree Layout = "sidebar_tree" // left resource tree + detail pane
	LayoutDashboard   Layout = "dashboard"    // grid of panels (from Tabs) shown at once
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
	Label      string
	Kind       string
	Template   string
	ConnectURL ArtifactConnectURL
}

type ArtifactConnectURL struct {
	LocalhostHost string
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
	Category    Category

	Config       Schema
	Capabilities []Capability
	// CredentialKinds declares reusable credential kinds owned by this plugin.
	// Shared cross-protocol kinds may still come from the core catalog.
	CredentialKinds []CredentialKindInfo

	SupportedTransports []Transport
	Agent               *AgentProfile

	Layout    Layout
	Tabs      []Tab
	Tree      []TreeGroup
	Resources []ResourceType
	Actions   []Action
	Streams   []Stream

	// Recording declares which stream classes this plugin can record. Empty means
	// the plugin supports no recording (the default).
	Recording []RecordingCapability
}

// SupportsTransport reports whether the manifest declares the given transport.
func (m Manifest) SupportsTransport(t Transport) bool {
	return slices.Contains(m.SupportedTransports, t)
}

// StreamByRoute returns the declared stream served by a WS route, if any.
func (m Manifest) StreamByRoute(routeID string) (Stream, bool) {
	for _, s := range m.Streams {
		if s.RouteID == routeID {
			return s, true
		}
	}
	return Stream{}, false
}
