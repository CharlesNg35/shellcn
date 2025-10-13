package drivers

import "context"

// Driver exposes metadata and capability information for a connection driver.
type Driver interface {
	// Metadata methods
	ID() string
	Name() string
	Module() string
	Category() string
	Icon() string
	Description() string
	DefaultPort() int
	SortOrder() int

	// Capabilities returns the feature flags supported by this driver.
	Capabilities(ctx context.Context) (Capabilities, error)

	// Descriptor returns legacy descriptor format (deprecated, use metadata methods).
	Descriptor() Descriptor
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

// BaseDriver provides a default implementation of Driver metadata methods using a Descriptor.
// Embed this in your driver struct to satisfy the Driver interface.
type BaseDriver struct {
	descriptor Descriptor
}

// NewBaseDriver creates a BaseDriver with the given descriptor.
func NewBaseDriver(desc Descriptor) BaseDriver {
	return BaseDriver{descriptor: desc}
}

func (b BaseDriver) ID() string             { return b.descriptor.ID }
func (b BaseDriver) Name() string           { return b.descriptor.Title }
func (b BaseDriver) Module() string         { return b.descriptor.Module }
func (b BaseDriver) Category() string       { return b.descriptor.Category }
func (b BaseDriver) Icon() string           { return b.descriptor.Icon }
func (b BaseDriver) Description() string    { return "" }
func (b BaseDriver) DefaultPort() int       { return 0 }
func (b BaseDriver) SortOrder() int         { return b.descriptor.SortOrder }
func (b BaseDriver) Descriptor() Descriptor { return b.descriptor }
