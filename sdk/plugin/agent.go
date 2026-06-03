package plugin

// Gateway-owned identifiers a plugin embeds in its manifest/config. They live in
// the SDK so an out-of-tree plugin can reference them without reaching into the
// core (internal/app mirrors these for server-side use).
const (
	// DefaultClientName is the default client/application identifier a plugin
	// reports to its target when the user configures none.
	DefaultClientName = "shellcn"

	// AgentBinary is the gateway's plugin-agnostic agent binary, referenced by
	// agent-transport install artifacts.
	AgentBinary = "shellcn-agent"

	// AgentImageLatest is the agent's published container image.
	AgentImageLatest = "ghcr.io/charlesng35/" + AgentBinary + ":latest"

	// AgentInternalAddress is the in-tunnel address the gateway dials an enrolled
	// agent's proxy on.
	AgentInternalAddress = AgentBinary + ".internal:443"
)
