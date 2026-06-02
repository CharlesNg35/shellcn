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

// Capability is a declarative feature tag, not behavior dispatch.
type Capability string

// Layout selects how the connection workspace is arranged.
type Layout string

const (
	LayoutTabs        Layout = "tabs"         // flat top tab bar, one panel at a time
	LayoutSidebarTree Layout = "sidebar_tree" // left resource tree + detail pane
	LayoutDashboard   Layout = "dashboard"    // grid of panels (from Tabs) shown at once
	LayoutSingle      Layout = "single"       // one full-bleed panel, no tab bar (a desktop/terminal/file screen)
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
	AgentTCP         AgentMode = "tcp"
	AgentUnix        AgentMode = "unix"
	AgentHTTP        AgentMode = "http_proxy"
	AgentHostMonitor AgentMode = "host_monitor"
)

// ProxyTarget describes the endpoint an agent exposes back to the gateway.
type ProxyTarget struct {
	Mode      AgentMode
	Address   string
	Risk      RiskLevel
	TokenFile string
	CAFile    string
	// Forward allows per-stream target-side addresses instead of only Address.
	Forward bool
}

// ArtifactDelivery selects how an install artifact reaches the target.
type ArtifactDelivery string

const (
	// DeliveryInline injects the token directly into Template (the default).
	DeliveryInline ArtifactDelivery = ""
	// DeliveryURL serves Content from a single-use signed-ticket URL.
	DeliveryURL ArtifactDelivery = "url"
)

// InstallArtifact is a launch recipe shown to start an agent.
type InstallArtifact struct {
	Label      string
	Kind       string
	Template   string
	Content    string
	Filename   string
	Delivery   ArtifactDelivery
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

// Manifest is a plugin's single versioned declarative contract.
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
	Tabs      []Panel
	Tree      []TreeGroup
	Resources []ResourceType
	Actions   []Action
	Streams   []Stream

	// HeaderActions reference Actions shown in the connection workspace header.
	HeaderActions []string

	// Scope declares global selectors injected into every request.
	Scope []ScopeFilter

	// Recording declares which stream classes this plugin can record.
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
