# Real-World Driver Implementation Guide

This guide shows you **step-by-step** how to implement a new protocol driver using the simplified driver registry architecture.

## Example: Implementing a VNC Driver

Let's implement a VNC (Virtual Network Computing) driver that allows users to connect to remote desktops.

---

## Step 1: Create Driver Package Structure

```
internal/drivers/vnc/
├── driver.go          # Main driver implementation
├── driver_test.go     # Unit tests
├── launcher.go        # Session launcher (optional)
└── permissions.go     # Permission registration
```

---

## Step 2: Implement the Driver Interface

**File: `internal/drivers/vnc/driver.go`**

```go
package vnc

import (
	"context"
	"errors"

	"github.com/charlesng35/shellcn/internal/drivers"
)

// Driver implements the drivers.Driver interface for VNC protocol.
type Driver struct {
	drivers.BaseDriver  // Embed BaseDriver for automatic metadata methods
}

// NewDriver creates and returns a new VNC driver instance.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: drivers.NewBaseDriver(drivers.Descriptor{
			ID:        "vnc",           // Unique driver identifier
			Module:    "vnc",           // Maps to config.protocols.vnc
			Title:     "VNC",           // Display name in UI
			Category:  "desktop",       // Category for grouping
			Icon:      "monitor",       // Icon identifier for UI
			Version:   "1.0.0",         // Driver version
			SortOrder: 5,               // Display order (lower = higher priority)
		}),
	}
}

// Capabilities returns the feature flags supported by VNC.
func (d *Driver) Capabilities(ctx context.Context) (drivers.Capabilities, error) {
	return drivers.Capabilities{
		Terminal:         false,  // VNC doesn't provide terminal
		Desktop:          true,   // VNC provides remote desktop
		FileTransfer:     false,  // Basic VNC doesn't support file transfer
		Clipboard:        true,   // VNC supports clipboard sharing
		SessionRecording: true,   // We can record VNC sessions
		Metrics:          true,   // We can collect connection metrics
		Reconnect:        true,   // VNC supports reconnection
		Extras: map[string]bool{
			"screen_scaling": true,  // Custom capability
			"color_depth":    true,  // Custom capability
		},
	}, nil
}

// Description returns a human-readable description of the driver.
func (d *Driver) Description() string {
	return "Virtual Network Computing (VNC) protocol for remote desktop access"
}

// DefaultPort returns the standard VNC port.
func (d *Driver) DefaultPort() int {
	return 5900
}

// HealthCheck verifies that VNC driver dependencies are available.
func (d *Driver) HealthCheck(ctx context.Context) error {
	// Check if VNC client libraries are available
	// Check if required system dependencies exist
	// Return error if driver cannot function

	// For this example, we'll assume everything is ready
	return nil
}

// ValidateConfig validates VNC-specific connection settings.
func (d *Driver) ValidateConfig(ctx context.Context, cfg map[string]any) error {
	// Validate required fields
	host, ok := cfg["host"].(string)
	if !ok || host == "" {
		return errors.New("vnc: host is required")
	}

	// Validate port if provided
	if port, ok := cfg["port"].(float64); ok {
		if port < 1 || port > 65535 {
			return errors.New("vnc: invalid port number")
		}
	}

	// Validate display number
	if display, ok := cfg["display"].(float64); ok {
		if display < 0 || display > 99 {
			return errors.New("vnc: display must be between 0 and 99")
		}
	}

	return nil
}

// TestConnection performs a lightweight connection test.
func (d *Driver) TestConnection(ctx context.Context, cfg map[string]any) error {
	// Attempt to connect to VNC server
	// Verify authentication works
	// Close connection immediately

	// For this example, we'll return success
	return nil
}
```

---

## Step 3: Register Permissions

**File: `internal/drivers/vnc/permissions.go`**

```go
package vnc

import (
	"github.com/charlesng35/shellcn/internal/permissions"
)

// init registers VNC-specific permissions during package initialization.
func init() {
	// Base connection permission
	must(permissions.RegisterProtocolPermission("vnc", "connect", &permissions.Permission{
		DisplayName:  "VNC Connect",
		Description:  "Initiate VNC desktop sessions",
		DefaultScope: "resource",
		DependsOn:    []string{"connection.launch"},
	}))

	// Desktop control permission
	must(permissions.RegisterProtocolPermission("vnc", "control", &permissions.Permission{
		DisplayName:  "VNC Remote Control",
		Description:  "Send keyboard and mouse input to remote desktop",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:vnc.connect"},
	}))

	// View-only mode permission
	must(permissions.RegisterProtocolPermission("vnc", "view", &permissions.Permission{
		DisplayName:  "VNC View Only",
		Description:  "View remote desktop without input control",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:vnc.connect"},
	}))

	// Screen recording permission
	must(permissions.RegisterProtocolPermission("vnc", "record", &permissions.Permission{
		DisplayName:  "VNC Record Sessions",
		Description:  "Record VNC sessions for audit purposes",
		DefaultScope: "resource",
		DependsOn:    []string{"protocol:vnc.connect", "connection.manage"},
	}))
}

// must panics if there's an error during permission registration.
func must(err error) {
	if err != nil {
		panic(err)
	}
}
```

---

## Step 4: Implement Session Launcher (Optional)

**File: `internal/drivers/vnc/launcher.go`**

```go
package vnc

import (
	"context"
	"fmt"

	"github.com/charlesng35/shellcn/internal/drivers"
)

// Ensure Driver implements Launcher interface.
var _ drivers.Launcher = (*Driver)(nil)

// Launch establishes a VNC connection and returns a session handle.
func (d *Driver) Launch(ctx context.Context, req drivers.SessionRequest) (drivers.SessionHandle, error) {
	// Extract connection settings
	host, ok := req.Settings["host"].(string)
	if !ok {
		return nil, fmt.Errorf("vnc: missing host in settings")
	}

	port := d.DefaultPort()
	if p, ok := req.Settings["port"].(float64); ok {
		port = int(p)
	}

	// Extract credentials from secrets
	password := ""
	if pwd, ok := req.Secret["password"].(string); ok {
		password = pwd
	}

	// Create VNC session
	session, err := d.createVNCSession(ctx, host, port, password)
	if err != nil {
		return nil, fmt.Errorf("vnc: failed to create session: %w", err)
	}

	return session, nil
}

// createVNCSession establishes the actual VNC connection.
func (d *Driver) createVNCSession(ctx context.Context, host string, port int, password string) (*Session, error) {
	// Connect to VNC server
	// Perform authentication
	// Start VNC protocol handshake
	// Return session handle

	return &Session{
		host:     host,
		port:     port,
		protocol: "vnc",
	}, nil
}

// Session represents an active VNC connection.
type Session struct {
	host     string
	port     int
	protocol string
	// Add VNC client connection, channels, etc.
}

// Close terminates the VNC session.
func (s *Session) Close(ctx context.Context) error {
	// Clean up VNC connection
	// Release resources
	// Unregister from ActiveSessionService
	return nil
}
```

---

## Step 5: Write Unit Tests

**File: `internal/drivers/vnc/driver_test.go`**

```go
package vnc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriver_Metadata(t *testing.T) {
	drv := NewDriver()

	assert.Equal(t, "vnc", drv.ID())
	assert.Equal(t, "VNC", drv.Name())
	assert.Equal(t, "vnc", drv.Module())
	assert.Equal(t, "desktop", drv.Category())
	assert.Equal(t, 5900, drv.DefaultPort())
	assert.Equal(t, 5, drv.SortOrder())
}

func TestDriver_Capabilities(t *testing.T) {
	drv := NewDriver()
	ctx := context.Background()

	caps, err := drv.Capabilities(ctx)
	require.NoError(t, err)

	assert.False(t, caps.Terminal, "VNC should not support terminal")
	assert.True(t, caps.Desktop, "VNC should support desktop")
	assert.True(t, caps.Clipboard, "VNC should support clipboard")
	assert.True(t, caps.SessionRecording, "VNC should support recording")
	assert.True(t, caps.Extras["screen_scaling"])
	assert.True(t, caps.Extras["color_depth"])
}

func TestDriver_ValidateConfig(t *testing.T) {
	drv := NewDriver()
	ctx := context.Background()

	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"host":    "192.168.1.100",
				"port":    float64(5901),
				"display": float64(1),
			},
			wantErr: false,
		},
		{
			name:    "missing host",
			config:  map[string]any{},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: map[string]any{
				"host": "192.168.1.100",
				"port": float64(99999),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := drv.ValidateConfig(ctx, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDriver_Descriptor(t *testing.T) {
	drv := NewDriver()
	desc := drv.Descriptor()

	assert.Equal(t, "vnc", desc.ID)
	assert.Equal(t, "vnc", desc.Module)
	assert.Equal(t, "VNC", desc.Title)
	assert.Equal(t, "desktop", desc.Category)
}
```

---

## Step 6: Register Driver in Bootstrap

**File: `cmd/server/main.go` (or wherever you bootstrap)**

```go
package main

import (
	"context"
	"log"

	"github.com/charlesng35/shellcn/internal/app"
	"github.com/charlesng35/shellcn/internal/drivers"
	"github.com/charlesng35/shellcn/internal/drivers/vnc"  // Import VNC driver
	"github.com/charlesng35/shellcn/internal/services"
)

func main() {
	// Load configuration
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	// Create driver registry
	driverRegistry := drivers.NewRegistry()

	// Register VNC driver
	driverRegistry.MustRegister(vnc.NewDriver())

	// Register other drivers...
	// driverRegistry.MustRegister(ssh.NewDriver())
	// driverRegistry.MustRegister(docker.NewDriver())

	// Initialize database
	db, err := setupDatabase(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Sync drivers to database
	catalogService, err := services.NewProtocolCatalogService(db)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	if err := catalogService.Sync(ctx, driverRegistry, cfg); err != nil {
		log.Fatal(err)
	}

	// Start server...
}
```

---

## Step 7: Add Configuration Support

**File: `internal/app/config.go`**

```go
// ProtocolConfig enables individual protocol drivers.
type ProtocolConfig struct {
	SSH           SimpleProtocolConfig   `mapstructure:"ssh"`
	VNC           SimpleProtocolConfig   `mapstructure:"vnc"`  // Add VNC
	Docker        SimpleProtocolConfig   `mapstructure:"docker"`
	// ... other protocols
}

// In setDefaults function:
func setDefaults(v *viper.Viper) {
	// ... other defaults
	v.SetDefault("protocols.vnc.enabled", true)  // Enable VNC by default
}
```

**File: `internal/services/protocol_catalog_service.go`**

```go
func protocolEnabled(cfg *app.Config, module string, protocolID string) bool {
	if cfg == nil {
		return true
	}

	switch strings.TrimSpace(module) {
	case "ssh":
		return cfg.Protocols.SSH.Enabled
	case "vnc":
		return cfg.Protocols.VNC.Enabled  // Add VNC check
	case "docker":
		return cfg.Protocols.Docker.Enabled
	// ... other cases
	default:
		return true
	}
}
```

---

## Step 8: Configuration File

**File: `config/config.yaml`**

```yaml
protocols:
  ssh:
    enabled: true
  vnc:
    enabled: true  # Enable VNC protocol
  docker:
    enabled: true
```

**Or via Environment Variables:**

```bash
export SHELLCN_PROTOCOLS_VNC_ENABLED=true
```

---

## Step 9: How It All Works Together

### 1. **Application Startup Flow**

```
main.go
  ├─> Load Config (protocols.vnc.enabled = true)
  ├─> Create Driver Registry
  ├─> Register VNC Driver (driverRegistry.MustRegister(vnc.NewDriver()))
  ├─> VNC permissions auto-registered (via init())
  ├─> Sync to Database (catalogService.Sync())
  │     ├─> Reads metadata from vnc.Driver methods
  │     ├─> Checks cfg.Protocols.VNC.Enabled
  │     ├─> Creates ConnectionProtocol record in database
  └─> Start HTTP server
```

### 2. **Frontend Fetches Protocols**

```
GET /api/protocols
  ├─> Query ConnectionProtocol table
  └─> Returns:
      {
        "id": "vnc",
        "name": "VNC",
        "module": "vnc",
        "category": "desktop",
        "default_port": 5900,
        "config_enabled": true,
        "driver_enabled": true,
        "capabilities": {
          "desktop": true,
          "clipboard": true,
          "session_recording": true
        },
        "features": ["desktop", "clipboard", "session_recording"]
      }
```

### 3. **User Launches VNC Connection**

```
POST /api/connections/:id/launch
  ├─> Permission check: protocol:vnc.connect
  ├─> Get driver from registry: driverRegistry.Get("vnc")
  ├─> Call driver.Launch(ctx, sessionRequest)
  ├─> Register active session
  └─> Return session handle to frontend
```

---

## Benefits of This Architecture

### ✅ **Single Source of Truth**
- Driver defines ALL metadata (no duplication)
- No intermediate protocol registry needed
- Direct driver → database sync

### ✅ **Type-Safe Metadata**
- Driver interface enforces all required methods
- Compile-time guarantee of metadata completeness

### ✅ **Easy Implementation**
- Use `drivers.BaseDriver` helper (no boilerplate)
- Only implement `Capabilities()` and optional interfaces

### ✅ **Flexible Permissions**
- Drivers own their permission definitions
- Auto-registered during package init
- Permission dependencies clearly declared

### ✅ **Config Integration**
- Module field maps to config namespace
- Enable/disable protocols via config
- Environment variable support

### ✅ **Database Caching**
- Fast API responses (no driver calls needed)
- Config-aware enablement state
- Driver health status tracked

---

## Quick Reference: Required vs Optional

### Required (All Drivers Must Implement)
- ✅ Embed `drivers.BaseDriver` with descriptor
- ✅ Implement `Capabilities(ctx) (Capabilities, error)`
- ✅ Register permissions in `init()`
- ✅ Add to `ProtocolConfig` struct
- ✅ Add to `protocolEnabled()` switch

### Optional (Implement as Needed)
- `Description() string` - Override BaseDriver default
- `DefaultPort() int` - Override BaseDriver default
- `HealthCheck(ctx) error` - Implement `HealthReporter`
- `ValidateConfig(ctx, cfg) error` - Implement `Validator`
- `TestConnection(ctx, cfg) error` - Implement `Tester`
- `Launch(ctx, req) (SessionHandle, error)` - Implement `Launcher`

---

## Summary

This architecture makes driver implementation **simple and consistent**:

1. Create driver package
2. Embed `BaseDriver` with metadata
3. Implement `Capabilities()`
4. Register permissions in `init()`
5. Add config mapping
6. Register in bootstrap

The driver registry handles the rest automatically!
