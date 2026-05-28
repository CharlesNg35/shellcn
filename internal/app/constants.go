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

	DefaultDatabaseDSN = Name + ".db"
	DefaultClientName  = Name

	SessionIssuer     = Name
	JWTSigningContext = Name + " auth jwt:"

	AgentUsername        = AgentBinary
	AgentSlugFallback    = AgentBinary
	AgentInternalHost    = AgentBinary + ".internal"
	AgentInternalAddress = AgentInternalHost + ":443"
)
