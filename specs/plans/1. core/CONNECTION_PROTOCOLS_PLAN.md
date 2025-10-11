# Connection Protocols & Drivers - Implementation Plan (Phase 3 Core)

## Overview

This plan delivers Phase 3 of the Core module ([ROADMAP.md:37-41](../../ROADMAP.md)). It turns the protocol registry into a complete **Connection Platform** with:

- Canonical **Connection** entities scoped to teams and users.
- A scalable **driver system** that supports native Go clients, Rust FFI modules, and future proxy adapters.
- Deterministic **availability rules** that combine driver readiness, configuration, row-level visibility, and modular permissions.
- Service + API layers to power the `Connections` UI (creation, filtering, launch preview, sharing).
- Auditability and fine-grained permissions aligned with `specs/project/project_spec.md` (remote access, credential vault).

### Goals

1. Model connections as reusable assets that can be owned per team or by individual users.
2. Provide a driver registry/descriptor system with capabilities metadata and health reporting.
3. Keep protocol availability derived from three levers: driver readiness, configuration toggles, and permission grants.
4. Expose REST endpoints and frontend hooks that surface only the protocols/connections a user can access.
5. Ensure every change (driver readiness, protocol sync, connection edits) emits audit records and respects team boundaries.

---

### Terminology

- **Protocol Driver** – the executable implementation that knows how to initiate a connection. Earlier documents referenced these as _modules_; protocol driver is now the canonical term.
- **Protocol Definition** – catalog metadata produced by the driver registry (id, labels, sort order, capabilities).
- **Connection** – a persisted record that references a protocol definition plus team visibility and identity bindings.
- **Capabilities** – feature flags a driver exposes (terminal, desktop, recording, metrics, extras) which guide permissions and UI elements.

Legacy references in specs that still mention “module” should be interpreted as “protocol driver.” Each driver spec must comply with the standards in `specs/project/PROTOCOL_DRIVER_STANDARDS.md`.

---

## Domain Model

### Tables & Relationships

| Table                     | Purpose                                                                    |
| ------------------------- | -------------------------------------------------------------------------- |
| `connection_protocols`    | Snapshot of driver metadata + config enable state (registry mirror).       |
| `connections`             | Base connection definition (name, protocol, org/team ownership, settings). |
| `connection_targets`      | One-to-many endpoints (primary + fallback hosts, labels).                  |
| `connection_visibilities` | Row-level ACL for teams and direct-user grants.                            |
| `connection_labels`       | (Optional) Tagging table for filters/search (future).                      |

### GORM Models

**`internal/models/connection_protocol.go`**

```go
type ConnectionProtocol struct {
    BaseModel
    Name           string `gorm:"not null;uniqueIndex" json:"name"`
    ProtocolID     string `gorm:"not null;uniqueIndex" json:"protocol_id"`
    Module         string `gorm:"not null;index" json:"module"`
    Icon           string `json:"icon"`
    Description    string `json:"description"`
    Category       string `gorm:"index" json:"category"`
    DefaultPort    int    `json:"default_port"`
    SortOrder      int    `gorm:"default:0" json:"sort_order"`
    Capabilities   string `gorm:"type:json" json:"capabilities"`
    Features       string `gorm:"type:json" json:"features"`
    DriverEnabled  bool   `gorm:"default:false" json:"driver_enabled"`
    ConfigEnabled  bool   `gorm:"default:false" json:"config_enabled"`
}

func (c *ConnectionProtocol) IsAvailable() bool {
    return c.DriverEnabled && c.ConfigEnabled
}
```

**`internal/models/connection.go`**

```go
type Connection struct {
    BaseModel
    Name           string  `gorm:"not null;uniqueIndex:conn_team_name" json:"name"`
    Description    string  `json:"description"`
    ProtocolID     string  `gorm:"not null;index" json:"protocol_id"`
    TeamID         *string `gorm:"type:uuid;index;uniqueIndex:conn_team_name" json:"team_id"`
    OwnerUserID    string  `gorm:"type:uuid;index" json:"owner_user_id"`
    Metadata       string  `gorm:"type:json" json:"metadata"`
    Settings       string  `gorm:"type:json" json:"settings"`
    SecretID       *string `gorm:"type:uuid" json:"secret_id"`
    LastUsedAt     *time.Time `json:"last_used_at"`

    Protocol    *ConnectionProtocol   `gorm:"foreignKey:ProtocolID" json:"protocol,omitempty"`
    Targets     []ConnectionTarget    `gorm:"foreignKey:ConnectionID" json:"targets,omitempty"`
    Visibility  []ConnectionVisibility `gorm:"foreignKey:ConnectionID" json:"visibility,omitempty"`
}
```

**`internal/models/connection_target.go`**

```go
type ConnectionTarget struct {
    BaseModel
    ConnectionID string `gorm:"type:uuid;index" json:"connection_id"`
    Host         string `gorm:"not null" json:"host"`
    Port         int    `json:"port"`
    Labels       string `gorm:"type:json" json:"labels"`
    Ordering     int    `gorm:"default:0" json:"ordering"`
}
```

**`internal/models/connection_visibility.go`**

```go
type ConnectionVisibility struct {
    BaseModel
    ConnectionID    string  `gorm:"type:uuid;index" json:"connection_id"`
    TeamID          *string `gorm:"type:uuid;index" json:"team_id"`
    UserID          *string `gorm:"type:uuid;index" json:"user_id"`
    PermissionScope string  `gorm:"type:varchar(32)" json:"permission_scope"` // view|use|manage
}
```

Key points:

- Team-level scoping allows curated sets for squads.
- Visibility table enables sharing with explicit users (similar to vault shares).
- Connections without a team are personal to the owner user.

---

## Driver Specification Artifacts

- Each driver owns a spec file in `specs/project/drivers/<driver>.md` following `PROTOCOL_DRIVER_STANDARDS.md`.
- Driver specs must define:
  - Descriptor metadata (title, icon, category, default sort order).
  - Connection property schema (settings fields, validation, defaults).
  - Capability flags surfaced to frontend.
  - Permission profile (connect/manage/feature/admin scopes).
  - Identity or credential requirements that the upcoming Identity system satisfies.
  - Testing guidance (unit + integration + frontend acceptance criteria).
- The protocol catalog sync will fail CI if a driver registers without its corresponding spec entry.

---

## Driver & Protocol Architecture

### Driver Categories

- **Native** (`internal/drivers/native`): SSH, Telnet, Docker, Kubernetes, databases.
- **FFI** (`internal/drivers/ffi`): RDP, VNC, Serial (Rust static libs via CGO).
- **Proxy** (`internal/drivers/proxy`): HTTP bridges to enterprise gateways (future).

### Driver Interface

**`internal/drivers/driver.go`**

```go
type Driver interface {
    Descriptor() Descriptor
    Capabilities() Capabilities
    ValidateConfig(ctx context.Context, cfg map[string]any) error
    TestConnection(ctx context.Context, cfg map[string]any, secret *vault.Credential) error
    Launch(ctx context.Context, request SessionRequest) (SessionHandle, error)
}
```

```go
type Descriptor struct {
    ID           string
    Module       string
    Title        string
    Category     string
    Version      string
    Icon         string
    SortOrder    int
    ImpliesPerms []string
}

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
```

Drivers may optionally implement:

```go
type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}

type SchemaProvider interface {
    ConfigSchema() map[string]SchemaField
}
```

### Driver Registry

**`internal/drivers/registry.go`**

```go
type Registry interface {
    Register(driver Driver) error
    Must(id string) Driver
    Get(id string) (Driver, bool)
    List() []Driver
    Describe() []Descriptor
}
```

- `internal/drivers/bootstrap.go` instantiates a singleton registry and registers all drivers during `cmd/server/main.go` startup. If a required driver fails registration (missing static lib), the process exits with a descriptive error.
- Health checks run at startup and every hour (`driverwatch.Daemon`). Status updates feed into `connection_protocols.driver_enabled`.

### Protocol Registry (Metadata Layer)

`internal/protocols` consumes driver descriptors and exposes immutable metadata to the rest of the backend.

```go
func Register(proto *Protocol) error
func Get(id string) (*Protocol, error)
func GetAll() []*Protocol
func DescribeCapabilities(id string) (drivers.Capabilities, error)
```

Each protocol entry references a driver ID; registry panics in tests if a driver is missing to catch regressions early.

---

## Availability Pipeline

Protocol availability cascades through four gates:

```
Driver Registered & HealthCheck OK
        AND
Config modules.<protocol>.enabled == true
        AND
permission.Check(user, "connection.view") && permission.Check(user, "{protocol}.connect")
        AND
VisibilityService.Allowed(user, connectionID or protocol scope)
```

- Driver readiness toggles `connection_protocols.driver_enabled`.
- Configuration toggles `connection_protocols.config_enabled` (per tenant; default seeded from config file).
- Permissions drawn from the modular system (see next section).
- Visibility: per-connection access lists and team scoping.

Root users bypass all gates except visibility (they can still manage shares) per `project_spec`.

---

## Permissions

### Registry Setup

**`internal/protocols/permissions.go`**

```go
// Base connection permissions
connection.view
connection.launch        (depends on connection.view)
connection.manage        (depends on connection.view)
connection.share         (depends on connection.manage)
connection.audit         (depends on audit.view)

// Per-driver permissions (auto-registered for each protocol id)
{protocol}.connect       (depends on connection.launch)
{protocol}.manage        (depends on connection.manage, implies {protocol}.connect)
```

Dependencies flow through `permissions.Register`, just like core permissions.

### Driver Permission Profiles

Every protocol driver must declare a permission profile when registering with the registry:

| Scope Type              | Naming                       | Description                                                                       | Dependency          |
| ----------------------- | ---------------------------- | --------------------------------------------------------------------------------- | ------------------- |
| Base usage              | `{protocol}.connect`         | Launch / attach runtime session                                                   | `connection.launch` |
| Advanced settings       | `{protocol}.manage`          | Edit driver-specific settings (namespaces, daemon sockets, tunables)              | `connection.manage` |
| Optional runtime scopes | `{protocol}.use.<feature>`   | Feature toggles such as `kubernetes.exec`, `docker.attach`, `rdp.recording`       | `connection.launch` |
| Optional admin scopes   | `{protocol}.admin.<feature>` | Sensitive operations such as `kubernetes.cluster.admin`, `database.schema.manage` | `connection.manage` |

Driver packages emit these scopes via `protocols.RegisterDriverPermissions(protoID, profile)` and list them in `specs/project/drivers/<protocol>.md`. Kubernetes, Docker, Databases, and File Share drivers use custom subsets from this table. Each new scope must be appended to the in-memory registry before `permissions.Sync` executes so database state stays consistent.

### Roles

Seed two roles (updated `internal/database/seed.go`):

- `connection.viewer` → `connection.view`, selected `{protocol}.connect` for commonly enabled drivers.
- `connection.admin` → all connection + protocol manage permissions.

Team-specific roles can be created later by admins using `/api/permissions/roles`.

---

## Services

### ProtocolService (`internal/services/protocol_service.go`)

Responsibilities:

- Read `connection_protocols` table and merge with driver metadata (capabilities, schema).
- Provide `GetAvailableProtocols(ctx)` (all) and `GetUserProtocols(ctx, userID)` (permission-filtered).
- Offer `TestDriver(ctx, protocolID)` for admins (delegates to driver `TestConnection` with synthetic payload).
- Cache descriptors in-memory for 5 minutes to reduce DB load.

### ConnectionService (`internal/services/connection_service.go`)

Key APIs:

```go
type CreateConnectionInput struct {
    Name           string
    Description    string
    ProtocolID     string
    TeamID         *string
    Metadata       map[string]any
    Settings       map[string]any
    SecretRef      *vault.SecretRef
    Targets        []ConnectionTargetInput
    Visibility     []ConnectionVisibilityInput
}
```

Behaviors:

- Validate driver exists and is available (DriverEnabled + ConfigEnabled) before create.
- Call driver `ValidateConfig` using provided settings.
- Serialize `Settings` and `Metadata` as JSON; encrypt secret payloads via Credential Vault service when inline.
- Manage `ConnectionVisibility` rows (team/user scopes). Enforce at least one scope when connection is team-owned.
- Record audit entries: `connection.create`, `connection.update`, `connection.delete`, `connection.share`, `connection.launch.preview`.
- Provide `ListForUser(ctx, userID, filter)` combining team membership and explicit share rows.

### Visibility & Permission Enforcement

`ConnectionService` checks permissions before mutating:

- Create/Update/Delete require `connection.manage` + `permissions.Check(user, protocolID+".manage")` for driver-specific settings.
- Share updates require `connection.share`.
- Launch preview/test requires `connection.launch` + `{protocol}.connect`.

---

## API Layer

### Protocol Routes (`internal/api/routes_protocols.go`)

```
GET  /api/protocols                   -> list all protocols (needs connection.view)
GET  /api/protocols/available         -> user-filtered list (connection.view)
GET  /api/protocols/:id               -> descriptor incl. capabilities (connection.view)
GET  /api/protocols/:id/permissions   -> permission map for UI (connection.view)
POST /api/protocols/:id/test          -> driver test (connection.manage)
```

### Connection Routes (`internal/api/routes_connections.go`)

```
GET    /api/connections                      (connection.view)
GET    /api/connections/:id                  (connection.view)
POST   /api/connections                      (connection.manage)
PATCH  /api/connections/:id                  (connection.manage)
DELETE /api/connections/:id                  (connection.manage)
POST   /api/connections/:id/share            (connection.share)
POST   /api/connections/:id/preview          (connection.launch)
```

All routes attach `middleware.RequirePermission(checker, <perm>)`. For preview and driver tests, middleware also checks `{protocol}.connect` via `ProtocolPermissionGuard` helper.

### Handler Responsibilities

- `ProtocolHandler` orchestrates ProtocolService methods and handles permission errors gracefully.
- `ConnectionHandler` binds/validates payloads (`internal/handlers/validation.go`), calls ConnectionService, and serializes visibility records.

### Handler Responsibilities

- `ProtocolHandler` orchestrates ProtocolService methods and handles permission errors gracefully.
- `ConnectionHandler` binds/validates payloads (`internal/handlers/validation.go`), calls ConnectionService, and serializes visibility records.

---

## Sync & Bootstrap Flow

1. `cmd/server/main.go`

   - Load config.
   - Initialize DB.
   - Call `drivers.Bootstrap()`.
   - `connection_protocols.Sync(ctx, db, cfg)` stores descriptors and driver availability.
   - `permissions.Sync(ctx, db)` persists global permission registry.
   - `database.SeedData(db, cfg)` seeds roles + default connections (optional sample).

2. Background job `ProtocolWatchdog`

   - Every hour: run driver `HealthCheck`, update DB, emit audit events on changes.
   - Emit websocket event `protocol.updated` for live UI refresh.

3. Launch router + register `/api/protocols` and `/api/connections` groups.

---

## Frontend Plan

### Types & API wrappers

- `web/src/types/protocols.ts` – matches `ProtocolInfo` (id, title, icon, availability, capabilities).
- `web/src/types/connections.ts` – connection payload with targets + visibility summary.
- `web/src/lib/api/protocols.ts` – fetch list/detail/test endpoints.
- `web/src/lib/api/connections.ts` – CRUD for connections + share + preview.
- `web/src/types/identities.ts` (future) – aligns identity payloads with driver credential requirements from driver specs.

### Hooks

- `useUserProtocols(queryOptions?)` – caches for 5 minutes.
- `useConnections(filters)` – includes search, protocol filter, org filter.
- `useConnectionMutations()` – create/update/delete/share wrappers with optimistic cache updates.
- `useDriverCapabilities(protocolID)` – fetches descriptor lazily.

### UI Updates (`web/src/pages/connections/Connections.tsx`)

- Tabs generated from `useUserProtocols` (category icons, capability chips).
- Connection cards show team badges, availability chips, and driver icons.
- "Launch" button disabled when driver not available for user (lack permission or config disabled).
- Share modal lists teams (leveraging `/api/teams`).
- Identity picker surfaces credential suggestions based on driver spec metadata (e.g., show kubeconfigs for Kubernetes drivers).

---

## Database & Migration Updates

1. Extend `internal/database/migrations.go`:
   ```go
   db.AutoMigrate(
       &models.ConnectionProtocol{},
       &models.Connection{},
       &models.ConnectionTarget{},
       &models.ConnectionVisibility{},
   )
   ```
2. Update seeding to call `protocols.RegisterPermissions()`, `permissions.Sync`, and `protocols.Sync(ctx, db, cfg)`.
3. Rename `AutoMigrateAndSeed` to accept `*app.Config` and pass from `cmd/server/main.go`.
4. Add indexes for frequently queried columns (team_id, protocol_id, permission_scope).

---

## Testing Strategy

- **drivers/registry_test.go** – registration, duplicates, descriptor snapshot.
- **protocols/sync_test.go** – driver + config interplay, config toggles, capability JSON output.
- **services/protocol_service_test.go** – root vs. regular user, availability gating.
- **services/connection_service_test.go** – validation, visibility filtering, permission enforcement, audit entries.
- **handlers/protocols_test.go** – API responses, permission middleware integration.
- **handlers/connections_test.go** – CRUD flows and share endpoint.
- **frontend** – React testing library for `Connections` page (filtering, actions) and hooks.

Mock utilities: add `testutil.NewDriverRegistry()` to isolate driver behaviors.

---

## Deployment & Observability

- Emit metrics `protocol_availability_total{protocol, state}` and `connection_launch_total{protocol, result}` via Prometheus collectors.
- Audit events `protocol.health.update`, `connection.create`, etc., already handled via services.
- Provide CLI command `shellcn protocols sync` for on-demand resync in deployments.

---

## Success Criteria

- ✅ Driver registry successfully initializes native + FFI drivers, surfacing capability metadata.
- ✅ Config toggles and driver health status produce consistent availability flags in API responses.
- ✅ Connection CRUD respects team visibility and modular permissions.
- ✅ Protocol and connection endpoints integrate with middleware.Auth + RequirePermission.
- ✅ Audit log entries create traceability for all connection/driver operations.
- ✅ Frontend auto-updates connection tabs and cards based on API results.
- ✅ Root users see/manage everything; team-scoped users only see what they should.

---

## Future Enhancements

1. Hot config reload (watcher) that re-syncs driver availability without restarting server.
2. Protocol health dashboard UI with retry + disable toggles.
3. Driver marketplace (uploadable bundles with signature validation).
4. Session orchestration (shared sessions, recordings, metrics streaming).
5. Connection templates + automation (bulk assign to teams, schedule rotation).

---

**Version:** 3.0 (Driver-Centric Core)
**Date:** 2025-10-09
**Status:** Ready for Implementation
