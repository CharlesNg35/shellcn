// Package app centralizes ShellCN product identifiers and published artifact
// names used across the backend.
package app

const (
	Name         = "shellcn"
	ServerBinary = Name
	AgentBinary  = Name + "-agent"

	ServerImageRepository = "ghcr.io/charlesng35/" + ServerBinary
	AgentImageRepository  = "ghcr.io/charlesng35/" + AgentBinary
	ServerImageLatest     = ServerImageRepository + ":latest"
	AgentImageLatest      = AgentImageRepository + ":latest"

	// Repository is the canonical GitHub repo; LatestReleaseURL points at the
	// published server/agent binaries.
	Repository       = "CharlesNg35/" + Name
	LatestReleaseURL = "https://github.com/" + Repository + "/releases/latest"

	DefaultDatabaseDSN = Name + ".db"
	DefaultClientName  = Name

	// DisplayName is the human-facing product name (e.g. the 2FA issuer shown in
	// authenticator apps), as opposed to the lowercase slug Name.
	DisplayName = "ShellCN"

	SessionIssuer     = Name
	JWTSigningContext = Name + " auth jwt:"

	AgentUsername        = AgentBinary
	AgentSlugFallback    = AgentBinary
	AgentInternalHost    = AgentBinary + ".internal"
	AgentInternalAddress = AgentInternalHost + ":443"
)
