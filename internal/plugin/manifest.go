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
	AgentTCP         AgentMode = "tcp"
	AgentUnix        AgentMode = "unix"
	AgentHTTP        AgentMode = "http_proxy"
	AgentHostMonitor AgentMode = "host_monitor"
)

// ProxyTarget describes the single endpoint an agent exposes back to the gateway.
// For http_proxy mode, TokenFile/CAFile are generic target-side paths the agent
// uses to inject a bearer token (empty = none) and verify TLS (empty = system
// roots) — no protocol vocabulary, so any private-HTTP-API plugin can reuse it.
type ProxyTarget struct {
	Mode      AgentMode
	Address   string
	Risk      RiskLevel
	TokenFile string
	CAFile    string
	// Forward lets the gateway dial arbitrary target-side addresses through the
	// agent per-stream (e.g. a container's network), not just Address. Opt-in.
	Forward bool
}

// ArtifactDelivery selects how an install artifact reaches the target.
type ArtifactDelivery string

const (
	// DeliveryInline injects the token directly into Template (the default).
	DeliveryInline ArtifactDelivery = ""
	// DeliveryURL serves Content from a single-use signed-ticket URL; Template
	// becomes the fetch command ({{.ArtifactURL}}), so the token appears only in
	// the fetched body. Generic — any plugin may use it.
	DeliveryURL ArtifactDelivery = "url"
)

// InstallArtifact is a launch recipe shown to the user to start an agent. An
// inline Content (e.g. a Compose YAML) renders directly in the panel to save/copy
// under Filename; Template is the shown command instead.
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
	Tabs      []Panel
	Tree      []TreeGroup
	Resources []ResourceType
	Actions   []Action
	Streams   []Stream

	// HeaderActions reference Actions by ID; the renderer shows them in the
	// connection workspace header, centered, for connection-wide affordances that
	// aren't tied to a selected resource.
	HeaderActions []string

	// Scope declares global header selectors whose chosen value the renderer
	// injects into every request, so all resources share one scope.
	Scope []ScopeFilter

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
