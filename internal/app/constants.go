// Package app centralizes ShellCN product identifiers and published artifact
// names used across the backend. Plugin-facing identifiers are owned by the SDK
// (sdk/plugin) and mirrored here for server-side use.
package app

import "github.com/charlesng35/shellcn/sdk/plugin"

const (
	Name         = "shellcn"
	ServerBinary = Name
	AgentBinary  = plugin.AgentBinary

	ServerImageRepository = "ghcr.io/charlesng35/" + ServerBinary
	AgentImageRepository  = "ghcr.io/charlesng35/" + AgentBinary
	ServerImageLatest     = ServerImageRepository + ":latest"
	AgentImageLatest      = plugin.AgentImageLatest

	// Repository is the canonical GitHub repo; LatestReleaseURL points at the
	// published server/agent binaries.
	Repository       = "CharlesNg35/" + Name
	LatestReleaseURL = "https://github.com/" + Repository + "/releases/latest"

	DefaultDatabaseDSN = Name + ".db"
	DefaultClientName  = plugin.DefaultClientName

	// DisplayName is the human-facing product name (e.g. the 2FA issuer shown in
	// authenticator apps), as opposed to the lowercase slug Name.
	DisplayName = "ShellCN"

	SessionIssuer     = Name
	JWTSigningContext = Name + " auth jwt:"

	AgentUsername        = AgentBinary
	AgentSlugFallback    = AgentBinary
	AgentInternalHost    = AgentBinary + ".internal"
	AgentInternalAddress = plugin.AgentInternalAddress
)
