package drivers

import "context"

// Driver exposes descriptor and capability metadata for a connection driver.
type Driver interface {
	Descriptor() Descriptor
	Capabilities(ctx context.Context) (Capabilities, error)
}

// HealthReporter allows drivers to report readiness/health signals.
type HealthReporter interface {
	HealthCheck(ctx context.Context) error
}

// Validator allows a driver to validate protocol-specific configuration maps.
type Validator interface {
	ValidateConfig(ctx context.Context, cfg map[string]any) error
}

// Tester allows a driver to run a lightweight connection diagnostic.
type Tester interface {
	TestConnection(ctx context.Context, cfg map[string]any) error
}

// Launcher is implemented by drivers that can initiate live connection sessions.
type Launcher interface {
	Launch(ctx context.Context, req SessionRequest) (SessionHandle, error)
}

// Descriptor summarises driver metadata for registry and protocol catalogues.
type Descriptor struct {
	ID        string
	Module    string
	Title     string
	Category  string
	Icon      string
	Version   string
	SortOrder int
}

// Capabilities list the feature flags surfaced by a driver.
type Capabilities struct {
	Terminal         bool
	Desktop          bool
	FileTransfer     bool
	Clipboard        bool
	SessionRecording bool
	Metrics          bool
	Reconnect        bool
	Extras           map[string]bool
}

// SessionRequest provides the runtime context for creating a driver session.
type SessionRequest struct {
	ConnectionID string
	ProtocolID   string
	UserID       string
	Settings     map[string]any
	Secret       map[string]any
}

// SessionHandle represents an established connection handle returned by drivers.
type SessionHandle interface {
	Close(ctx context.Context) error
}
