# Connection Protocols Management - Implementation Plan

## Overview

This plan implements Phase 3 of the Core Module ([ROADMAP.md:37-41](../../ROADMAP.md)) - dynamic protocol management with config-based and driver-based enablement + permission filtering.

## Protocol Availability Rules

A protocol is **AVAILABLE** to a user when ALL conditions are met:

```
1. ✅ Registered in registry (internal/protocols/core.go)
           ↓
2. ✅ Enabled in config.yaml (modules.*.enabled = true)
           ↓
3. ✅ Driver marked ready (Protocol.Enabled = true)
           ↓
4. ✅ User has permissions (connection.view + {protocol}.connect)
           ↓
      Protocol AVAILABLE
```

### Key Insight

**Config controls admin enable/disable, Driver controls implementation readiness**

- **Config** (`config.yaml`): Admin can turn modules on/off without code changes
- **Driver** (Registry): Developer marks implementation as ready
- **Both must be true** for protocol to be available
- **Permissions**: Filter what each user can see

## Architecture

### Config System (Already Exists!)

From [internal/app/config.go:122-133](../../internal/app/config.go):

```go
type ModuleConfig struct {
    SSH        SSHModuleConfig      `mapstructure:"ssh"`
    Telnet     TelnetModuleConfig   `mapstructure:"telnet"`
    SFTP       SFTPModuleConfig     `mapstructure:"sftp"`
    RDP        DesktopModuleConfig  `mapstructure:"rdp"`
    VNC        DesktopModuleConfig  `mapstructure:"vnc"`
    Docker     SimpleModuleConfig   `mapstructure:"docker"`
    Kubernetes SimpleModuleConfig   `mapstructure:"kubernetes"`
    Database   DatabaseModuleConfig `mapstructure:"database"`
    Proxmox    SimpleModuleConfig   `mapstructure:"proxmox"`
    FileShare  SimpleModuleConfig   `mapstructure:"file_share"`
}
```

Defaults in [config.go:284-314](../../internal/app/config.go):
- SSH: enabled=true, port=22
- Telnet: enabled=true, port=23
- RDP: enabled=true, port=3389
- Docker: enabled=true
- Kubernetes: enabled=false
- etc.

### New Components Needed

1. **Protocol Registry** - Tracks driver implementation status
2. **Protocol Model** - Stores config+driver status in DB
3. **Protocol Service** - Combines config+driver+permissions
4. **API Endpoints** - Exposes available protocols
5. **Frontend** - Dynamic UI based on API

## Implementation

### 1. Database Model

**File:** `internal/models/connection_protocol.go`

```go
package models

type ConnectionProtocol struct {
    BaseModel

    Name        string `gorm:"not null;uniqueIndex" json:"name"`
    ProtocolID  string `gorm:"not null;uniqueIndex" json:"protocol_id"`
    Module      string `gorm:"not null;index" json:"module"`
    Icon        string `json:"icon"`
    Description string `json:"description"`
    Category    string `gorm:"index" json:"category"`
    DefaultPort int    `json:"default_port"`
    SortOrder   int    `gorm:"default:0" json:"sort_order"`
    Features    string `gorm:"type:json" json:"features"`

    // Two-tier enablement
    DriverEnabled bool `gorm:"default:false" json:"driver_enabled"`  // From registry
    ConfigEnabled bool `gorm:"default:false" json:"config_enabled"`  // From config.yaml
}

func (c *ConnectionProtocol) IsAvailable() bool {
    return c.DriverEnabled && c.ConfigEnabled
}
```

### 2. Protocol Registry

**File:** `internal/protocols/registry.go`

```go
package protocols

type Category string

const (
    CategoryTerminal  Category = "terminal"
    CategoryDesktop   Category = "desktop"
    CategoryContainer Category = "container"
    CategoryDatabase  Category = "database"
    CategoryVM        Category = "vm"
)

type Protocol struct {
    ID          string
    Name        string
    Module      string
    Icon        string
    Description string
    Category    Category
    DefaultPort int
    Features    []string
    SortOrder   int
    Enabled     bool  // Driver implementation ready?
}

var globalRegistry = &registry{protocols: make(map[string]*Protocol)}

func Register(proto *Protocol) error { /* ... */ }
func Get(id string) (*Protocol, error) { /* ... */ }
func GetAll() []*Protocol { /* ... */ }
```

**File:** `internal/protocols/core.go`

```go
package protocols

func init() {
    Register(&Protocol{
        ID: "ssh", Name: "SSH / Telnet", Module: "ssh",
        Icon: "Server", Category: CategoryTerminal, DefaultPort: 22,
        Enabled: false,  // Set true when SSH driver implemented
        Features: []string{"terminal", "sftp"}, SortOrder: 1,
    })

    Register(&Protocol{
        ID: "rdp", Name: "RDP", Module: "rdp",
        Icon: "Monitor", Category: CategoryDesktop, DefaultPort: 3389,
        Enabled: false,  // Set true when RDP driver implemented
        Features: []string{"desktop"}, SortOrder: 2,
    })

    // ... all protocols ...
}
```

### 3. Protocol Sync (Registry + Config → Database)

**File:** `internal/protocols/sync.go`

```go
package protocols

import "github.com/charlesng35/shellcn/internal/app"

func Sync(ctx context.Context, db *gorm.DB, cfg *app.Config) error {
    for _, proto := range GetAll() {
        configEnabled := isEnabledInConfig(proto.ID, cfg)

        db.Clauses(clause.OnConflict{
            Columns: []clause.Column{{Name: "id"}},
            DoUpdates: clause.AssignmentColumns([]string{
                "driver_enabled", "config_enabled", /* ... */
            }),
        }).Create(&models.ConnectionProtocol{
            BaseModel: models.BaseModel{ID: proto.ID},
            DriverEnabled: proto.Enabled,
            ConfigEnabled: configEnabled,
            // ... other fields ...
        })
    }
    return nil
}

func isEnabledInConfig(protocolID string, cfg *app.Config) bool {
    switch protocolID {
    case "ssh": return cfg.Modules.SSH.Enabled
    case "rdp": return cfg.Modules.RDP.Enabled
    case "docker": return cfg.Modules.Docker.Enabled
    case "mysql": return cfg.Modules.Database.Enabled && cfg.Modules.Database.MySQL
    // ... all protocols ...
    default: return false
    }
}
```

### 4. Protocol Permissions

**File:** `internal/protocols/permissions.go`

```go
package protocols

func RegisterPermissions() error {
    // Base permissions
    permissions.Register(&permissions.Permission{
        ID: "connection.view", Module: "core",
        Description: "View available protocols",
    })
    permissions.Register(&permissions.Permission{
        ID: "connection.create", Module: "core",
        DependsOn: []string{"connection.view"},
        Description: "Create connections",
    })

    // Protocol-specific permissions
    for _, proto := range GetAll() {
        permissions.Register(&permissions.Permission{
            ID: fmt.Sprintf("%s.connect", proto.ID),
            Module: proto.Module,
            DependsOn: []string{"connection.view"},
            Description: fmt.Sprintf("Use %s protocol", proto.Name),
        })
    }
    return nil
}
```

### 5. Protocol Service

**File:** `internal/services/protocol_service.go`

```go
package services

type ProtocolService struct {
    db      *gorm.DB
    checker *permissions.Checker
}

type ProtocolInfo struct {
    ID string `json:"id"`
    Name string `json:"name"`
    // ... all fields ...
    DriverEnabled bool `json:"driver_enabled"`
    ConfigEnabled bool `json:"config_enabled"`
    Available bool `json:"available"`
}

func (s *ProtocolService) GetAvailableProtocols(ctx context.Context) ([]*ProtocolInfo, error) {
    var dbProtos []models.ConnectionProtocol
    s.db.Find(&dbProtos)

    infos := []*ProtocolInfo{}
    for _, db := range dbProtos {
        if !db.IsAvailable() { continue }  // Skip if driver OR config disabled

        proto, _ := protocols.Get(db.ProtocolID)
        infos = append(infos, &ProtocolInfo{
            ID: proto.ID,
            DriverEnabled: db.DriverEnabled,
            ConfigEnabled: db.ConfigEnabled,
            Available: true,
            // ... copy all fields ...
        })
    }
    return infos, nil
}

func (s *ProtocolService) GetUserProtocols(ctx context.Context, userID string) ([]*ProtocolInfo, error) {
    var user models.User
    s.db.Preload("Roles.Permissions").First(&user, "id = ?", userID)

    if user.IsRoot {
        return s.GetAvailableProtocols(ctx)  // Root gets all
    }

    allProtos, _ := s.GetAvailableProtocols(ctx)
    userProtos := []*ProtocolInfo{}

    for _, proto := range allProtos {
        hasView, _ := s.checker.Check(ctx, userID, "connection.view")
        if !hasView { continue }

        hasProto, _ := s.checker.Check(ctx, userID, proto.ID+".connect")
        if hasProto {
            userProtos = append(userProtos, proto)
        }
    }

    return userProtos, nil
}
```

### 6. API Handler + Routes

**File:** `internal/handlers/protocols.go`

```go
func (h *ProtocolHandler) GetUserProtocols(c *gin.Context) {
    userID, _ := c.Get("userID")
    protos, err := h.service.GetUserProtocols(c.Request.Context(), userID.(string))
    if err != nil {
        response.Error(c, errors.ErrInternalServer)
        return
    }
    response.Success(c, http.StatusOK, gin.H{"protocols": protos, "count": len(protos)})
}
```

**File:** `internal/api/routes_protocols.go`

```go
func registerProtocolRoutes(api *gin.RouterGroup, db *gorm.DB, checker *permissions.Checker) error {
    handler, _ := handlers.NewProtocolHandler(db, checker)

    api.GET("/protocols",
        middleware.RequirePermission(checker, "connection.view"),
        handler.GetAvailableProtocols)
    api.GET("/protocols/available",
        middleware.RequirePermission(checker, "connection.view"),
        handler.GetUserProtocols)
    return nil
}
```

### 7. Database Migration Updates

**Update [internal/database/migrations.go:14](../../internal/database/migrations.go)**

```go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        // ... existing models ...
        &models.ConnectionProtocol{},  // ADD THIS
    )
}
```

**Update [internal/database/migrations.go:31](../../internal/database/migrations.go)**

```go
func SeedData(db *gorm.DB, cfg *app.Config) error {  // ADD cfg param
    // Register protocol permissions
    protocols.RegisterPermissions()

    // Sync permissions
    permissions.Sync(context.Background(), db)

    // Sync protocols with config
    protocols.Sync(context.Background(), db, cfg)  // ADD THIS

    // ... existing role seeding ...
}
```

**Update [internal/database/db.go:44](../../internal/database/db.go)**

```go
func AutoMigrateAndSeed(db *gorm.DB, cfg *app.Config) error {  // ADD cfg param
    AutoMigrate(db)
    SeedData(db, cfg)  // Pass cfg
    return nil
}
```

**Update [cmd/server/main.go:247](../../cmd/server/main.go)**

```go
if err := database.AutoMigrateAndSeed(db, cfg); err != nil {  // Pass cfg
    return nil, fmt.Errorf("auto-migrate database: %w", err)
}
```

**Update [internal/api/router.go:136](../../internal/api/router.go)**

```go
// After permission routes:
if err := registerProtocolRoutes(api, db, checker); err != nil {
    return nil, err
}
```

### 8. Frontend

**File:** `web/src/types/protocols.ts`

```typescript
export interface Protocol {
  id: string
  name: string
  icon: string
  category: string
  driver_enabled: boolean
  config_enabled: boolean
  available: boolean
}
```

**File:** `web/src/lib/api/protocols.ts`

```typescript
export const protocolsApi = {
  getUserProtocols: async (): Promise<Protocol[]> => {
    const response = await apiClient.get('/protocols/available')
    return unwrapResponse(response).protocols
  },
}
```

**File:** `web/src/hooks/useProtocols.ts`

```typescript
export function useUserProtocols() {
  return useQuery({
    queryKey: ['protocols', 'user'],
    queryFn: protocolsApi.getUserProtocols,
    staleTime: 5 * 60 * 1000,
  })
}
```

**Update [web/src/pages/connections/Connections.tsx](../../web/src/pages/connections/Connections.tsx)**

```typescript
const { data: protocols } = useUserProtocols()

const connectionTypes = useMemo(() => {
  return [{id: 'all', label: 'All', icon: 'Server'},
          ...protocols.map(p => ({id: p.id, label: p.name, icon: p.icon}))]
}, [protocols])
```

## Implementation Checklist

### Backend
- [ ] Create `internal/models/connection_protocol.go`
- [ ] Create `internal/protocols/registry.go`
- [ ] Create `internal/protocols/core.go` with all protocols
- [ ] Create `internal/protocols/sync.go` with config integration
- [ ] Create `internal/protocols/permissions.go`
- [ ] Create `internal/services/protocol_service.go`
- [ ] Create `internal/handlers/protocols.go`
- [ ] Create `internal/api/routes_protocols.go`
- [ ] Update `internal/database/migrations.go` (AutoMigrate + SeedData signature)
- [ ] Update `internal/database/db.go` (AutoMigrateAndSeed signature)
- [ ] Update `cmd/server/main.go` (pass config)
- [ ] Update `internal/api/router.go` (register routes)

### Frontend
- [ ] Create `web/src/types/protocols.ts`
- [ ] Create `web/src/lib/api/protocols.ts`
- [ ] Create `web/src/hooks/useProtocols.ts`
- [ ] Update `web/src/pages/connections/Connections.tsx`

### Testing
- [ ] Protocol registry tests
- [ ] Protocol service tests (root vs regular user)
- [ ] Config integration tests
- [ ] API endpoint tests
- [ ] Frontend hook tests

## Config Example

```yaml
# config.yaml
modules:
  ssh:
    enabled: true    # Config: ON
    # Driver in registry: Enabled = false → SSH NOT available

  rdp:
    enabled: false   # Config: OFF
    # Even if driver ready, RDP NOT available

  docker:
    enabled: true    # Config: ON
    # Driver in registry: Enabled = true → Docker AVAILABLE!
```

## API Endpoints

| Endpoint | Auth | Permission | Returns |
|----------|------|------------|---------|
| `GET /api/protocols` | Required | `connection.view` | All available protocols (driver+config enabled) |
| `GET /api/protocols/available` | Required | `connection.view` | User-permitted protocols only |
| `GET /api/protocols/category/:cat` | Required | `connection.view` | Filtered by category |
| `GET /api/protocols/stats` | Required | `connection.view` | Usage statistics |

## Success Criteria

- ✅ Protocol available = DriverEnabled AND ConfigEnabled AND UserHasPermission
- ✅ Admins can disable protocols via config without code changes
- ✅ Developers mark drivers ready via registry `Enabled` flag
- ✅ Root users see all available protocols
- ✅ Regular users see only permitted protocols
- ✅ UI dynamically renders tabs based on API response
- ✅ Follows existing codebase patterns (permission system, service layer, etc.)

## Future Enhancements

1. Hot config reload (no restart needed)
2. Protocol health monitoring
3. Admin UI for toggling protocols
4. Connection CRUD operations
5. Protocol templates

---

**Version:** 2.0 (Config-Integrated)
**Date:** 2025-10-09
**Status:** Ready for Implementation
