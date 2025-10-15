# ShellCN Platform - Module Implementation Guide

This document provides a detailed breakdown of all modules, their features, implementation requirements, and dependencies.

> **Terminology update:** The platform now refers to these runtime components as **protocol drivers** (formerly “modules”). Existing section titles retain the historical naming for continuity, but new specs and code should prefer the driver terminology described in `specs/project/PROTOCOL_DRIVER_STANDARDS.md`.

---

## Table of Contents

1. [Permission System](#permission-system)
   - [Permission Registry Design](#permission-registry-design)
   - [Module Permission Registration](#module-permission-registration)
   - [Permission Checking](#permission-checking)
2. [Core Module](#1-core-module)
3. [Vault Module](#2-vault-module-credential-management)
4. [SSH Module](#3-ssh-module)
5. [Telnet Module](#4-telnet-module)
6. [RDP Module](#5-rdp-module-rust-ffi)
7. [VNC Module](#6-vnc-module-rust-ffi)
8. [SFTP Module](#7-sftp-module)
9. [Docker Module](#8-docker-module)
10. [Kubernetes Module](#9-kubernetes-module)
11. [Database Module](#10-database-module)
12. [Proxmox Module](#11-proxmox-module)
13. [Object Storage Module](#12-object-storage-module)
14. [Monitoring Module](#13-monitoring-module)
15. [Notification Module](#14-notification-module)
16. [Module Dependency Graph](#module-dependency-graph)
17. [Implementation Checklist](#implementation-checklist)

---

## Permission System

The platform uses a centralized permission registry system where all modules register their permissions at startup. This enables dynamic permission checking with dependency resolution and module isolation.

### Permission Registry Design

**Architecture:** Global registry with thread-safe access, dependency tracking, and validation.

**Location:** `internal/permissions/registry.go`

```go
package permissions

import (
    "fmt"
    "sync"
)

type Permission struct {
    ID          string
    Module      string
    DependsOn   []string
    Description string
}

type PermissionRegistry struct {
    permissions map[string]*Permission
    mu          sync.RWMutex
}

var globalRegistry = &PermissionRegistry{
    permissions: make(map[string]*Permission),
}

// Register a permission - called by modules in init()
func Register(perm *Permission) error {
    globalRegistry.mu.Lock()
    defer globalRegistry.mu.Unlock()

    if _, exists := globalRegistry.permissions[perm.ID]; exists {
        return fmt.Errorf("permission %s already registered", perm.ID)
    }

    globalRegistry.permissions[perm.ID] = perm
    return nil
}

// Get all registered permissions
func GetAll() map[string]*Permission {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    result := make(map[string]*Permission)
    for k, v := range globalRegistry.permissions {
        result[k] = v
    }
    return result
}

// Get permissions by module
func GetByModule(module string) []*Permission {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    var result []*Permission
    for _, perm := range globalRegistry.permissions {
        if perm.Module == module {
            result = append(result, perm)
        }
    }
    return result
}

// Check if user has permission (with dependency resolution)
func Check(userID, permissionID string) (bool, error) {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    perm, exists := globalRegistry.permissions[permissionID]
    if !exists {
        return false, fmt.Errorf("permission %s not found", permissionID)
    }

    // Check dependencies first
    for _, dep := range perm.DependsOn {
        hasDepPerm, err := checkUserPermission(userID, dep)
        if err != nil {
            return false, err
        }
        if !hasDepPerm {
            return false, nil // Missing dependency
        }
    }

    // Check the permission itself
    return checkUserPermission(userID, permissionID)
}

// Validate all permission dependencies on startup
func ValidateDependencies() error {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    for _, perm := range globalRegistry.permissions {
        for _, dep := range perm.DependsOn {
            if _, exists := globalRegistry.permissions[dep]; !exists {
                return fmt.Errorf("permission %s depends on non-existent permission %s",
                    perm.ID, dep)
            }
        }
    }
    return nil
}
```

### Module Permission Registration

Each module registers its permissions using `init()` functions that are automatically called when the package is imported.

**Pattern:** Every module has a `permissions.go` file that registers its permissions.

**Example - Core Permissions:**

**Location:** `internal/permissions/core.go`

```go
package permissions

func init() {
    // User Management
    Register(&Permission{
        ID:          "user.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View users",
    })

    Register(&Permission{
        ID:          "user.create",
        Module:      "core",
        DependsOn:   []string{"user.view"},
        Description: "Create new users",
    })

    Register(&Permission{
        ID:          "user.edit",
        Module:      "core",
        DependsOn:   []string{"user.view"},
        Description: "Edit user details",
    })

    Register(&Permission{
        ID:          "user.delete",
        Module:      "core",
        DependsOn:   []string{"user.view", "user.edit"},
        Description: "Delete users",
    })

    // Team Management
    Register(&Permission{
        ID:          "team.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View teams",
    })

    // Permission Management
    Register(&Permission{
        ID:          "permission.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View permissions",
    })

    Register(&Permission{
        ID:          "permission.manage",
        Module:      "core",
        DependsOn:   []string{"permission.view"},
        Description: "Assign/revoke permissions",
    })

    // Audit
    Register(&Permission{
        ID:          "audit.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View audit logs",
    })

    Register(&Permission{
        ID:          "audit.export",
        Module:      "core",
        DependsOn:   []string{"audit.view"},
        Description: "Export audit logs",
    })
}
```

**Example - Module Permissions (SSH):**

**Location:** `internal/modules/ssh/permissions.go`

```go
package ssh

import "shellcn/internal/permissions"

func init() {
    permissions.Register(&permissions.Permission{
        ID:          "ssh.connect",
        Module:      "ssh",
        DependsOn:   []string{"vault.view"},
        Description: "Connect to SSH servers",
    })

    permissions.Register(&permissions.Permission{
        ID:          "ssh.execute",
        Module:      "ssh",
        DependsOn:   []string{"ssh.connect"},
        Description: "Execute commands",
    })

    permissions.Register(&permissions.Permission{
        ID:          "ssh.session.share",
        Module:      "ssh",
        DependsOn:   []string{"ssh.connect"},
        Description: "Share SSH sessions",
    })

    permissions.Register(&permissions.Permission{
        ID:          "ssh.clipboard.sync",
        Module:      "ssh",
        DependsOn:   []string{"ssh.connect"},
        Description: "Enable clipboard sync",
    })

    permissions.Register(&permissions.Permission{
        ID:          "ssh.session.record",
        Module:      "ssh",
        DependsOn:   []string{"ssh.connect"},
        Description: "Record SSH sessions",
    })
}
```

**Example - Vault Permissions:**

**Location:** `internal/vault/permissions.go`

```go
package vault

import "shellcn/internal/permissions"

func init() {
    permissions.Register(&permissions.Permission{
        ID:          "vault.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View own identities",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.create",
        Module:      "core",
        DependsOn:   []string{"vault.view"},
        Description: "Create new identities",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.edit",
        Module:      "core",
        DependsOn:   []string{"vault.view"},
        Description: "Edit own identities",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.delete",
        Module:      "core",
        DependsOn:   []string{"vault.view"},
        Description: "Delete own identities",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.share",
        Module:      "core",
        DependsOn:   []string{"vault.view", "vault.edit"},
        Description: "Share identities with others",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.use_shared",
        Module:      "core",
        DependsOn:   []string{"vault.view"},
        Description: "Use identities shared by others",
    })

    permissions.Register(&permissions.Permission{
        ID:          "vault.manage_all",
        Module:      "core",
        DependsOn:   []string{"vault.view", "vault.edit", "vault.delete"},
        Description: "Manage all identities (admin)",
    })
}
```

### Permission Checking

**Application Startup:**

**Location:** `internal/app/app.go`

```go
package app

import (
    "log"

    // Import modules to trigger init() registration
    _ "shellcn/internal/permissions"
    _ "shellcn/internal/vault"
    _ "shellcn/internal/modules/ssh"
    _ "shellcn/internal/modules/telnet"
    _ "shellcn/internal/modules/rdp"
    _ "shellcn/internal/modules/vnc"
    _ "shellcn/internal/modules/docker"
    _ "shellcn/internal/modules/kubernetes"
    _ "shellcn/internal/modules/database"
)

func (a *App) Start() error {
    // Validate all permission dependencies
    if err := permissions.ValidateDependencies(); err != nil {
        log.Fatal("Permission dependency validation failed:", err)
    }

    log.Printf("Loaded %d permissions from %d modules",
        len(permissions.GetAll()),
        getModuleCount(),
    )

    return a.server.Run()
}
```

**API Middleware:**

**Location:** `internal/api/middleware/permission.go`

```go
package middleware

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "shellcn/internal/permissions"
)

func RequirePermission(perm string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        isRoot := c.GetBool("is_root")

        // Root user bypasses all permission checks
        if isRoot {
            c.Next()
            return
        }

        // Check permission with dependency resolution
        hasPermission, err := permissions.Check(userID, perm)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{
                "error": "Permission check failed",
            })
            c.Abort()
            return
        }

        if !hasPermission {
            c.JSON(http.StatusForbidden, gin.H{
                "error": "Permission denied",
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

**Usage Example:**

```go
// Apply permission middleware to routes
router.POST("/api/users",
    middleware.AuthRequired(),
    middleware.RequirePermission("user.create"),
    handlers.CreateUser,
)

router.POST("/api/ssh/connections",
    middleware.AuthRequired(),
    middleware.RequirePermission("ssh.connect"),
    handlers.CreateSSHConnection,
)

router.POST("/api/docker/containers/:id/exec",
    middleware.AuthRequired(),
    middleware.RequirePermission("docker.container.exec"),
    handlers.DockerExec,
)
```

**Frontend Permission Checking:**

**Location:** `web/src/hooks/usePermissions.ts`

```typescript
import { useAuth } from './useAuth';

export function useHasPermission(permission: string): boolean {
  const { user } = useAuth();

  // Root user has all permissions
  if (user?.isRoot) return true;

  // Check user's permission list
  return user?.permissions?.includes(permission) || false;
}

export function useHasAnyPermission(permissions: string[]): boolean {
  const { user } = useAuth();
  if (user?.isRoot) return true;
  return permissions.some(perm => user?.permissions?.includes(perm));
}

export function useHasAllPermissions(permissions: string[]): boolean {
  const { user } = useAuth();
  if (user?.isRoot) return true;
  return permissions.every(perm => user?.permissions?.includes(perm));
}
```

**Usage in React Components:**

```typescript
import { useHasPermission } from '@/hooks/usePermissions';

export function UserManagement() {
  const canCreate = useHasPermission('user.create');
  const canDelete = useHasPermission('user.delete');

  return (
    <div>
      {canCreate && (
        <button onClick={handleCreate}>Create User</button>
      )}
      {canDelete && (
        <button onClick={handleDelete}>Delete User</button>
      )}
    </div>
  );
}
```

---

## 1. Core Module

**Status:** Required (Always Enabled)

### Features

#### 1.1 Authentication & Authorization
- **Local Authentication**
  - Username/password login
  - Bcrypt password hashing
  - JWT token generation
  - Session management

- **External Authentication** (Optional)
  - OpenID Connect (OIDC)
  - SAML 2.0
  - LDAP/Active Directory

- **Multi-Factor Authentication** (Optional)
  - TOTP (Time-based One-Time Password)
  - QR code generation
  - Backup codes

#### 1.2 User Management
- Create/Read/Update/Delete users
- User profile management
- Password reset
- Email verification
- User activation/deactivation
- Root/superuser management

#### 1.3 First-Time Setup
- **UI-based first user creation**
  - No default credentials
  - Setup wizard at `/setup`
  - Auto-redirect when no users exist
  - First user created as superuser/root
  - Password encryption with bcrypt

#### 1.4 Team Management
- Create teams
- Assign users to teams
- Team-based access control

#### 1.5 Permission System
- Role-Based Access Control (RBAC)
- Permission dependencies
- Permission inheritance
- Dynamic permission checking
- Root user bypass (superuser has all permissions)
- Module-specific permission registration

#### 1.6 Audit Logging
- User action logging
- Resource access logging
- Authentication attempts
- Permission denials
- System events
- Log retention policies
- Log export (CSV, JSON)

#### 1.7 Session Management
- Active session tracking
- Session expiry
- Session revocation
- Multi-device sessions
- Session sharing between users
- Session recording (optional)

### Backend Implementation

**Location:** `internal/`

```
internal/
├── app/
│   ├── app.go              # Application initialization
│   ├── config.go           # Configuration management
│   └── server.go           # HTTP server setup
│
├── api/
│   ├── router.go           # Route definitions
│   ├── middleware/
│   │   ├── auth.go         # JWT authentication
│   │   ├── cors.go         # CORS handling
│   │   ├── logger.go       # Request logging
│   │   ├── ratelimit.go    # Rate limiting
│   │   └── recovery.go     # Panic recovery
│   │
│   └── handlers/
│       ├── auth.go         # Login, logout, refresh
│       ├── setup.go        # First user setup
│       ├── users.go        # User CRUD
│       ├── teams.go
│       ├── permissions.go
│       ├── sessions.go
│       ├── health.go       # Health checks
│       └── websocket.go    # WebSocket handler
│
├── auth/
│   ├── auth.go             # Auth interface
│   ├── jwt.go              # JWT implementation
│   ├── providers/
│   │   ├── local.go        # Local auth
│   │   ├── oidc.go         # OIDC provider
│   │   ├── saml.go         # SAML provider
│   │   └── ldap.go         # LDAP provider
│   │
│   └── mfa/
│       └── totp.go         # TOTP implementation
│
├── permissions/
│   ├── checker.go          # Permission checker
│   ├── dependencies.go     # Dependency resolver
│   ├── core.go             # Core permissions
│   └── registry.go         # Module registration
│
├── models/
│   ├── user.go
│   ├── team.go
│   ├── role.go
│   ├── permission.go
│   ├── session.go
│   └── audit_log.go
│
├── database/
│   ├── db.go
│   ├── sqlite.go
│   ├── postgres.go
│   ├── mysql.go
│   └── repositories/
│       ├── user_repository.go
│       ├── session_repository.go
│       └── audit_repository.go
│
└── services/
    ├── user_service.go
    ├── auth_service.go
    ├── permission_service.go
    └── audit_service.go
```

### Frontend Implementation

**Location:** `web/src/`

```
web/src/
├── pages/
│   ├── Login.tsx
│   ├── Setup.tsx           # First user setup
│   ├── Dashboard.tsx
│   │
│   └── settings/
│       ├── Profile.tsx
│       ├── Security.tsx
│       ├── Teams.tsx
│       └── Users.tsx       # Admin only
│
├── components/
│   ├── auth/
│   │   ├── LoginForm.tsx
│   │   ├── SetupForm.tsx
│   │   ├── MFASetup.tsx
│   │   └── PasswordReset.tsx
│   │
│   └── admin/
│       ├── UserTable.tsx
│       ├── RoleManager.tsx
│       └── PermissionMatrix.tsx
│
├── hooks/
│   ├── useAuth.ts
│   ├── usePermissions.ts
│   ├── useCurrentUser.ts
│   └── useTeams.ts
│
└── lib/
    └── api/
        ├── auth.ts
        ├── users.ts
        ├── teams.ts
        └── permissions.ts
```

### Core Permissions

```go
CORE_PERMISSIONS = {
    // User Management
    "user.view": {
        "module": "core",
        "depends_on": [],
        "description": "View users",
    },
    "user.create": {
        "module": "core",
        "depends_on": ["user.view"],
        "description": "Create new users",
    },
    "user.edit": {
        "module": "core",
        "depends_on": ["user.view"],
        "description": "Edit user details",
    },
    "user.delete": {
        "module": "core",
        "depends_on": ["user.view", "user.edit"],
        "description": "Delete users",
    },

    // Team Management
    "team.view": {
        "module": "core",
        "depends_on": [],
        "description": "View teams",
    },
    "team.create": {
        "module": "core",
        "depends_on": ["team.view"],
        "description": "Create teams",
    },
    "team.manage": {
        "module": "core",
        "depends_on": ["team.view"],
        "description": "Manage teams",
    },

    // Permission Management
    "permission.view": {
        "module": "core",
        "depends_on": [],
        "description": "View permissions",
    },
    "permission.manage": {
        "module": "core",
        "depends_on": ["permission.view"],
        "description": "Assign/revoke permissions",
    },

    // Audit
    "audit.view": {
        "module": "core",
        "depends_on": [],
        "description": "View audit logs",
    },
    "audit.export": {
        "module": "core",
        "depends_on": ["audit.view"],
        "description": "Export audit logs",
    },
}
```

---

## 2. Vault Module (Credential Management)

**Status:** Core Module (Always Enabled)

### Features

#### 2.1 Identity Management
- Create reusable identities (credentials)
- Identity types: SSH, Database, Generic
- Store usernames and passwords (encrypted)
- Store SSH private keys (encrypted)
- Store SSH key passphrases (encrypted)
- Identity metadata and notes
- Identity versioning

#### 2.2 SSH Key Management
- Upload SSH private keys
- Generate new SSH key pairs
- Support RSA, ECDSA, Ed25519
- Encrypted key storage (AES-256-GCM)
- Key fingerprint tracking
- Public key export
- Passphrase-protected keys

#### 2.3 Credential Sharing
- Share identities with users
- Share identities with teams
- Permission levels: Read, Use, Edit
- Audit trail for credential access
- Revoke shared access

#### 2.4 Encryption
- AES-256-GCM encryption
- Master key from `VAULT_ENCRYPTION_KEY`
- Argon2id key derivation
- Unique nonce per credential
- Key rotation support
- Zero-knowledge option (user passphrase)

#### 2.5 Integration with Connections
- Identity selector in connection forms
- "Custom Identity" option for ad-hoc credentials
- Link to `/settings/identities` from connection forms
- Auto-fill credentials when identity selected
- List connections using each identity

### Backend Implementation

**Location:** `internal/vault/`

```
internal/vault/
├── vault.go                # Vault interface
├── encryption.go           # AES-256-GCM encryption
├── identity.go             # Identity management
├── credentials.go          # Credential storage
├── keys.go                 # SSH key management
├── sharing.go              # Identity sharing
├── permissions.go          # Vault permissions
└── handler.go              # API handlers

internal/models/
├── identity.go             # Identity model
├── ssh_key.go              # SSH key model
├── identity_share.go       # Sharing model
└── vault_key.go            # Encryption key metadata

internal/services/
└── vault_service.go        # Vault business logic
```

### Frontend Implementation

**Location:** `web/src/`

```
web/src/
├── pages/
│   └── settings/
│       ├── Identities.tsx      # /settings/identities
│       ├── NewIdentity.tsx
│       ├── EditIdentity.tsx
│       └── SSHKeys.tsx
│
├── components/
│   └── vault/
│       ├── IdentitySelector.tsx    # Reusable dropdown
│       ├── IdentityTable.tsx
│       ├── IdentityForm.tsx
│       ├── SSHKeyUpload.tsx
│       ├── SSHKeyGenerator.tsx
│       ├── ShareIdentityDialog.tsx
│       └── IdentityUsage.tsx       # Show connections using identity
│
├── hooks/
│   ├── useIdentities.ts
│   ├── useIdentity.ts
│   ├── useSSHKeys.ts
│   └── useIdentitySharing.ts
│
└── lib/
    └── api/
        ├── vault.ts
        └── ssh-keys.ts
```

### Vault Permissions

```go
VAULT_PERMISSIONS = {
    "vault.view": {
        "module": "core",
        "depends_on": [],
        "description": "View own identities",
    },
    "vault.create": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Create new identities",
    },
    "vault.edit": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Edit own identities",
    },
    "vault.delete": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Delete own identities",
    },
    "vault.share": {
        "module": "core",
        "depends_on": ["vault.view", "vault.edit"],
        "description": "Share identities with others",
    },
    "vault.use_shared": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Use identities shared by others",
    },
    "vault.manage_all": {
        "module": "core",
        "depends_on": ["vault.view", "vault.edit", "vault.delete"],
        "description": "Manage all identities (admin)",
    },
}
```

### Database Schema

```sql
-- Identities
CREATE TABLE identities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,                -- 'ssh', 'database', 'generic'
    user_id TEXT REFERENCES users(id),
    username TEXT,
    password_encrypted TEXT,
    private_key_encrypted TEXT,
    passphrase_encrypted TEXT,
    metadata TEXT,                      -- JSON metadata
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- SSH Keys
CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    name TEXT NOT NULL,
    key_type TEXT,                      -- 'rsa', 'ecdsa', 'ed25519'
    private_key_encrypted TEXT,
    public_key TEXT,
    passphrase_encrypted TEXT,
    fingerprint TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Identity Sharing
CREATE TABLE identity_shares (
    id TEXT PRIMARY KEY,
    identity_id TEXT REFERENCES identities(id),
    shared_with_user_id TEXT REFERENCES users(id),
    permission_level TEXT DEFAULT 'read',  -- 'read', 'use', 'edit'
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Vault Encryption Keys (metadata)
CREATE TABLE vault_keys (
    id TEXT PRIMARY KEY,
    key_version INTEGER NOT NULL,
    algorithm TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    rotated_at TEXT
);
```

---

## 3. SSH Module

**Status:** Default Enabled (Can be disabled)

### Features

#### 3.1 SSH Connection
- SSH v2 support (enabled by default)
- Auto protocol detection
- Multiple authentication methods:
  - Password authentication
  - Public key authentication
  - Keyboard-interactive authentication
  - Agent forwarding (optional)
- Connection timeout configuration
- Keepalive support

#### 3.2 Terminal Emulator
- xterm.js-based terminal
- Full ANSI/VT100 support
- Terminal resizing (dynamic fit)
- Copy/paste support
- Search in terminal
- Scrollback buffer
- Download session logs
- **User-configurable settings:**
  - Font family (from user preferences)
  - Font size (from user preferences)
  - Cursor style (from user preferences)
  - Theme/colors (from user preferences)

#### 3.3 Encoding & Keyboard
- Receive encoding (UTF-8, ISO-8859-1, etc.)
- Terminal encoding (UTF-8, ISO-2022, etc.)
- AltGr mode (auto, ctrl-alt, right-alt)
- Alt key modifier (escape, 8-bit, browser-key)
- Backspace behavior
- Ctrl+C/Ctrl+V behavior
- Meta key handling

#### 3.4 Scrolling
- Scroll on keystroke
- Scroll on output
- Scrollbar visibility
- Arrow key scroll emulation

#### 3.5 Auto-Reconnection
- Enable/disable auto-reconnect
- Max reconnect attempts (default: 3)
- Reconnect delay (seconds)
- Exponential backoff
- Session state restoration
- User notification on reconnection

#### 3.6 Session Features
- Session recording (optional)
- Session sharing with other users
- Clipboard synchronization (permission-based)
- Session history
- Multiple simultaneous sessions

### Backend Implementation

**Location:** `internal/modules/ssh/`

```
internal/modules/ssh/
├── ssh.go                  # SSH client wrapper
├── session.go              # SSH session management
├── terminal.go             # Terminal emulation
├── auth.go                 # Authentication methods
├── reconnect.go            # Auto-reconnection logic
├── recorder.go             # Session recording
├── permissions.go          # SSH permissions
└── handler.go              # WebSocket handler

internal/models/
└── ssh_connection.go       # SSH connection model
```

### Frontend Implementation

**Location:** `web/src/`

```
web/src/
├── pages/
│   └── ssh/
│       ├── ConnectionList.tsx
│       ├── NewConnection.tsx
│       ├── EditConnection.tsx
│       └── Terminal.tsx
│
├── components/
│   └── terminal/
│       ├── SSHTerminal.tsx
│       ├── TerminalToolbar.tsx
│       ├── TerminalSearch.tsx
│       └── ConnectionStatus.tsx
│
├── hooks/
│   ├── useSSHConnection.ts
│   ├── useTerminal.ts
│   └── useSSHWebSocket.ts
│
└── lib/
    ├── api/
    │   └── ssh.ts
    └── terminal-themes.ts      # Theme definitions from user prefs
```

### SSH Permissions

```go
SSH_PERMISSIONS = {
    "ssh.connect": {
        "module": "ssh",
        "depends_on": ["vault.view"],
        "description": "Connect to SSH servers",
    },
    "ssh.execute": {
        "module": "ssh",
        "depends_on": ["ssh.connect"],
        "description": "Execute commands",
    },
    "ssh.session.share": {
        "module": "ssh",
        "depends_on": ["ssh.connect"],
        "description": "Share SSH sessions",
    },
    "ssh.clipboard.sync": {
        "module": "ssh",
        "depends_on": ["ssh.connect"],
        "description": "Enable clipboard sync",
    },
    "ssh.session.record": {
        "module": "ssh",
        "depends_on": ["ssh.connect"],
        "description": "Record SSH sessions",
    },
}
```

### Configuration

```go
type SSHConnectionConfig struct {
    // Basic
    Name        string
    Protocol    string  // "ssh", "auto"
    Icon        string

    // Connection
    Host        string
    Port        int     // Default: 22

    // Authentication
    IdentityID  *string // Reference to vault identity
    AuthMethod  string  // "password", "publickey", "keyboard-interactive"
    Username    string  // If custom identity
    Password    string  // Encrypted (if custom identity)
    PrivateKey  string  // Encrypted (if custom identity)
    Passphrase  string  // Encrypted (if custom identity)

    // Encoding
    ReceiveEncoding  string
    TerminalEncoding string

    // Keyboard
    AltGrMode         string
    AltKeyModifier    string
    BackspaceAsCtrlH  bool
    AltKeyAsMeta      bool
    CtrlCCopyBehavior bool
    CtrlVPasteBehavior bool

    // Scrolling
    ScrollOnKeystroke    bool
    ScrollOnOutput       bool
    ScrollbarVisible     bool
    EmulateArrowWithScroll bool

    // Reconnection
    EnableReconnect      bool
    ReconnectAttempts    int
    ReconnectDelay       int
    ConnectionTimeout    int
    KeepAliveInterval    int

    // Features
    ClipboardEnabled     bool
    SessionRecording     bool

    Notes               string
}
```

---

## 4. Telnet Module

**Status:** Optional (Can be disabled)

### Features

#### 4.1 Telnet Connection
- Standard Telnet protocol
- Default port: 23
- Connection timeout
- Legacy device support

#### 4.2 Terminal Features
- Same terminal features as SSH
- xterm.js integration
- Encoding support
- Keyboard customization
- Scrolling options
- **User preferences applied** (font, theme, cursor)

#### 4.3 Auto-Reconnection
- Auto-reconnect support
- Configurable retry logic

### Backend Implementation

**Location:** `internal/modules/telnet/`

```
internal/modules/telnet/
├── telnet.go
├── session.go
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/telnet/`

```
web/src/pages/telnet/
├── ConnectionList.tsx
├── NewConnection.tsx
└── Terminal.tsx
```

### Telnet Permissions

```go
TELNET_PERMISSIONS = {
    "telnet.connect": {
        "module": "telnet",
        "depends_on": [],
        "description": "Connect to Telnet servers",
    },
    "telnet.session.share": {
        "module": "telnet",
        "depends_on": ["telnet.connect"],
        "description": "Share Telnet sessions",
    },
}
```

---

## 5. RDP Module (Rust FFI)

**Status:** Default Enabled (Rust FFI)

### Features

#### 5.1 RDP Connection
- Remote Desktop Protocol (Windows)
- IronRDP library (Rust)
- Static linking via CGO
- Authentication with credentials
- Domain support

#### 5.2 Display Features
- Screen resolution configuration
- Color depth selection (8, 16, 24, 32-bit)
- Full-screen mode
- Scaling options

#### 5.3 Audio & Clipboard
- Audio redirection
- Clipboard sharing (bidirectional)
- Permission-based clipboard

#### 5.4 Session Features
- Session sharing
- Recording support
- Multi-monitor support (optional)

### Rust FFI Implementation

**Location:** `rust-modules/rdp/`

```
rust-modules/rdp/
├── Cargo.toml              # Dependencies: ironrdp
├── cbindgen.toml           # C binding config
├── build.rs                # Header generation
├── rdp_ffi.h               # Generated C header
└── src/
    └── lib.rs              # FFI functions

# Cargo.toml
[dependencies]
ironrdp = "0.1"  # Check latest on crates.io
tokio = { version = "1.35", features = ["full"] }

[lib]
crate-type = ["staticlib"]

[build-dependencies]
cbindgen = "0.29"
```

### Backend (Go CGO)

**Location:** `internal/modules/rdp/`

```
internal/modules/rdp/
├── ffi.go                  # CGO bindings
├── session.go              # RDP session wrapper
├── permissions.go
└── handler.go              # WebSocket handler
```

### Frontend Implementation

**Location:** `web/src/pages/rdp/`

```
web/src/pages/rdp/
├── ConnectionList.tsx
├── NewConnection.tsx
└── Desktop.tsx             # RDP viewer (canvas-based)
```

### RDP Permissions

```go
RDP_PERMISSIONS = {
    "rdp.connect": {
        "module": "rdp",
        "depends_on": ["vault.view"],
        "description": "Connect to RDP servers",
    },
    "rdp.clipboard.sync": {
        "module": "rdp",
        "depends_on": ["rdp.connect"],
        "description": "Enable clipboard sync",
    },
    "rdp.session.share": {
        "module": "rdp",
        "depends_on": ["rdp.connect"],
        "description": "Share RDP sessions",
    },
}
```

---

## 6. VNC Module (Rust FFI)

**Status:** Default Enabled (Rust FFI)

### Features

#### 6.1 VNC Connection
- Virtual Network Computing
- vnc-rs library (Rust)
- Static linking via CGO
- Password authentication

#### 6.2 Display Features
- Color quality settings
- Compression options
- Scaling
- Full-screen mode

#### 6.3 Session Features
- Session sharing
- Recording support

### Rust FFI Implementation

**Location:** `rust-modules/vnc/`

```
rust-modules/vnc/
├── Cargo.toml              # Dependencies: vnc
├── cbindgen.toml
├── build.rs
├── vnc_ffi.h
└── src/
    └── lib.rs

# Cargo.toml
[dependencies]
vnc = "0.4"  # Check latest on crates.io
tokio = { version = "1.35", features = ["full"] }

[lib]
crate-type = ["staticlib"]

[build-dependencies]
cbindgen = "0.29"
```

### Backend (Go CGO)

**Location:** `internal/modules/vnc/`

```
internal/modules/vnc/
├── ffi.go
├── session.go
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/vnc/`

```
web/src/pages/vnc/
├── ConnectionList.tsx
├── NewConnection.tsx
└── Desktop.tsx
```

### VNC Permissions

```go
VNC_PERMISSIONS = {
    "vnc.connect": {
        "module": "vnc",
        "depends_on": [],
        "description": "Connect to VNC servers",
    },
    "vnc.session.share": {
        "module": "vnc",
        "depends_on": ["vnc.connect"],
        "description": "Share VNC sessions",
    },
}
```

---

## 7. SFTP Module

**Status:** Part of SSH Module

### Features

#### 7.1 File Transfer
- SFTP over SSH
- Upload files
- Download files
- Drag & drop support
- Progress tracking

#### 7.2 File Manager
- Dual-pane browser (local/remote)
- Directory navigation
- File operations:
  - Create directory
  - Rename
  - Delete
  - Copy
  - Move
  - Change permissions
- File search
- Sorting and filtering
- **User preferences:**
  - Show hidden files (from settings)
  - Default view (list/grid)
  - Sort order

#### 7.3 File Viewing
- Text file preview
- Image preview
- Syntax highlighting (code files)
- Download before edit

### Backend Implementation

**Location:** `internal/modules/sftp/`

```
internal/modules/sftp/
├── sftp.go                 # SFTP client wrapper
├── transfer.go             # File transfer
├── operations.go           # File operations
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/components/file-manager/`

```
web/src/components/file-manager/
├── FileManager.tsx
├── FileExplorer.tsx
├── FileTable.tsx
├── FileUpload.tsx
├── TransferProgress.tsx
└── FilePreview.tsx
```

### SFTP Permissions

```go
SFTP_PERMISSIONS = {
    "sftp.browse": {
        "module": "ssh",
        "depends_on": ["ssh.connect"],
        "description": "Browse remote files",
    },
    "sftp.download": {
        "module": "ssh",
        "depends_on": ["sftp.browse"],
        "description": "Download files",
    },
    "sftp.upload": {
        "module": "ssh",
        "depends_on": ["sftp.browse"],
        "description": "Upload files",
    },
    "sftp.delete": {
        "module": "ssh",
        "depends_on": ["sftp.browse"],
        "description": "Delete files",
    },
    "sftp.chmod": {
        "module": "ssh",
        "depends_on": ["sftp.browse"],
        "description": "Change file permissions",
    },
}
```

---

## 8. Docker Module

**Status:** Default Enabled

### Features

#### 8.1 Docker Host Connection
- Connect to Docker daemon
- TCP connection
- Unix socket connection
- SSH tunnel support
- TLS authentication (client certificates)
- Docker context support
- Connection health monitoring

#### 8.2 Container Management

**Container Operations:**
- List containers (all, running, stopped, paused)
- Create new containers
- Start containers
- Stop containers
- Restart containers
- Pause/Unpause containers
- Kill containers (SIGKILL)
- Rename containers
- Delete containers
- Prune stopped containers

**Container Inspection:**
- View container details
- Container logs (real-time, follow, tail, timestamps)
- Container stats (CPU, memory, network, disk I/O)
- Container processes (top)
- Container file system changes
- Container port mappings
- Container environment variables
- Container labels

**Container Interaction:**
- Execute commands in container (docker exec)
- Attach to container
- Container terminal (interactive shell)
- Copy files to/from container
- Export container filesystem
- Commit container to image
- Update container configuration (resource limits)

**Container Networking:**
- View container networks
- Connect container to network
- Disconnect container from network
- Inspect network settings

#### 8.3 Image Management

**Image Operations:**
- List images (all, dangling)
- Pull images from registry
- Push images to registry
- Build images from Dockerfile
- Tag images
- Delete images
- Prune unused images
- Save images to tar
- Load images from tar
- Import filesystem as image

**Image Inspection:**
- View image details
- Image history (layers)
- Image size
- Image labels
- Image environment variables

**Image Registry:**
- Login to registry
- Logout from registry
- Search images in registry
- Private registry support

#### 8.4 Volume Management

**Volume Operations:**
- List volumes
- Create volumes
- Delete volumes
- Prune unused volumes
- Volume driver support

**Volume Inspection:**
- View volume details
- Volume mount points
- Volume size
- Volume driver options
- Volume labels

#### 8.5 Network Management

**Network Operations:**
- List networks
- Create networks (bridge, host, overlay, macvlan)
- Delete networks
- Prune unused networks
- Connect containers to network
- Disconnect containers from network

**Network Inspection:**
- View network details
- Network driver
- Network subnet/gateway
- Connected containers
- Network options
- IPAM configuration

#### 8.6 System & Administration

**System Information:**
- Docker version
- System info (OS, architecture, CPU, memory)
- Disk usage
- Data root directory
- Storage driver
- Logging driver
- Runtime information

**System Operations:**
- Docker events (real-time monitoring)
- System-wide prune (containers, images, volumes, networks)
- Docker swarm status (if enabled)

**Resource Management:**
- Set container resource limits (CPU, memory)
- View resource usage across all containers
- Container restart policies

#### 8.7 Docker Compose (Optional)

**Compose Operations:**
- List compose projects
- Deploy compose stack
- Stop compose stack
- Remove compose stack
- Scale services
- View compose logs
- Restart compose services

#### 8.8 Docker Swarm (Optional)

**Swarm Management:**
- Initialize swarm
- Join swarm
- Leave swarm
- View swarm nodes
- Promote/Demote nodes

**Service Management:**
- List services
- Create services
- Scale services
- Update services
- Delete services
- View service logs
- Service tasks/replicas

### Backend Implementation

**Location:** `internal/modules/docker/`

```
internal/modules/docker/
├── client.go               # Docker SDK client
├── container.go            # Container operations
├── image.go                # Image operations
├── volume.go               # Volume operations
├── network.go              # Network operations
├── exec.go                 # Container exec
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/docker/`

```
web/src/pages/docker/
├── HostList.tsx
├── NewHost.tsx
├── ContainerList.tsx
├── ContainerDetails.tsx
├── ContainerLogs.tsx
├── ContainerExec.tsx       # Terminal in container
├── ImageList.tsx
├── VolumeList.tsx
└── NetworkList.tsx
```

### Docker Permissions

```go
DOCKER_PERMISSIONS = {
    // Connection
    "docker.connect": {
        "module": "docker",
        "depends_on": [],
        "description": "Connect to Docker hosts",
    },

    // Container - Read
    "docker.container.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List containers",
    },
    "docker.container.view": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container details",
    },
    "docker.container.logs": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container logs",
    },
    "docker.container.stats": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "View container stats",
    },

    // Container - Execute
    "docker.container.exec": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Execute in containers",
    },
    "docker.container.attach": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Attach to containers",
    },
    "docker.container.copy": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Copy files to/from containers",
    },

    // Container - Manage
    "docker.container.create": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Create containers",
    },
    "docker.container.start": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Start containers",
    },
    "docker.container.stop": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Stop containers",
    },
    "docker.container.restart": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Restart containers",
    },
    "docker.container.pause": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Pause/Unpause containers",
    },
    "docker.container.kill": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Kill containers",
    },
    "docker.container.delete": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Delete containers",
    },
    "docker.container.update": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
        "description": "Update container config",
    },
    "docker.container.prune": {
        "module": "docker",
        "depends_on": ["docker.container.delete"],
        "description": "Prune stopped containers",
    },

    // Image - Read
    "docker.image.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List images",
    },
    "docker.image.view": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "View image details",
    },
    "docker.image.history": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "View image history",
    },

    // Image - Manage
    "docker.image.pull": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Pull images",
    },
    "docker.image.push": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Push images",
    },
    "docker.image.build": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Build images",
    },
    "docker.image.tag": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Tag images",
    },
    "docker.image.delete": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Delete images",
    },
    "docker.image.prune": {
        "module": "docker",
        "depends_on": ["docker.image.delete"],
        "description": "Prune unused images",
    },
    "docker.image.import": {
        "module": "docker",
        "depends_on": ["docker.image.list"],
        "description": "Import/Export images",
    },

    // Volume
    "docker.volume.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List volumes",
    },
    "docker.volume.view": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "View volume details",
    },
    "docker.volume.create": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "Create volumes",
    },
    "docker.volume.delete": {
        "module": "docker",
        "depends_on": ["docker.volume.list"],
        "description": "Delete volumes",
    },
    "docker.volume.prune": {
        "module": "docker",
        "depends_on": ["docker.volume.delete"],
        "description": "Prune unused volumes",
    },

    // Network
    "docker.network.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "List networks",
    },
    "docker.network.view": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "View network details",
    },
    "docker.network.create": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "Create networks",
    },
    "docker.network.delete": {
        "module": "docker",
        "depends_on": ["docker.network.list"],
        "description": "Delete networks",
    },
    "docker.network.connect": {
        "module": "docker",
        "depends_on": ["docker.network.list", "docker.container.list"],
        "description": "Connect containers to networks",
    },
    "docker.network.disconnect": {
        "module": "docker",
        "depends_on": ["docker.network.list", "docker.container.list"],
        "description": "Disconnect containers from networks",
    },
    "docker.network.prune": {
        "module": "docker",
        "depends_on": ["docker.network.delete"],
        "description": "Prune unused networks",
    },

    // System
    "docker.system.info": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "View system info",
    },
    "docker.system.events": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Monitor system events",
    },
    "docker.system.df": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "View disk usage",
    },
    "docker.system.prune": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "System-wide prune",
    },

    // Registry
    "docker.registry.login": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Login to registry",
    },
    "docker.registry.search": {
        "module": "docker",
        "depends_on": ["docker.connect"],
        "description": "Search registry",
    },
}
```

---

## 9. Kubernetes Module

**Status:** Optional

### Features

#### 9.1 Cluster Connection
- Connect to K8s cluster
- kubeconfig support (upload or paste)
- Token authentication
- Certificate authentication
- Service account authentication
- Multiple context management
- Namespace selection
- Cluster switching
- Connection pooling

#### 9.2 Workload Resources

**Pod Management:**
- List pods (all namespaces or specific)
- View pod details
- Pod logs (real-time, follow, tail, timestamps)
- Execute in pod (kubectl exec)
- Pod terminal (multi-container support)
- Delete pods
- Pod describe (YAML/JSON)
- Pod events
- Pod metrics (CPU, Memory, Network)
- Restart pod
- Port forward to pod
- Copy files to/from pod

**Deployment Management:**
- List deployments
- Create deployment (YAML editor)
- Edit deployment (rolling update)
- Scale deployments (manual or autoscale)
- Update deployment
- Delete deployment
- Rollback deployment (revision history)
- Pause/Resume deployment
- Restart deployment
- View deployment status
- Deployment events
- Replica set management

**StatefulSet Management:**
- List StatefulSets
- Create StatefulSet
- Edit StatefulSet
- Scale StatefulSet
- Delete StatefulSet
- View StatefulSet status
- Rolling update management

**DaemonSet Management:**
- List DaemonSets
- Create DaemonSet
- Edit DaemonSet
- Delete DaemonSet
- View DaemonSet status
- Node selector management

**Job & CronJob Management:**
- List Jobs
- Create Job
- Delete Job
- View Job logs
- Job completion status
- List CronJobs
- Create CronJob
- Edit CronJob schedule
- Suspend/Resume CronJob
- Trigger CronJob manually

**ReplicaSet Management:**
- List ReplicaSets
- View ReplicaSet details
- Scale ReplicaSet
- Delete ReplicaSet

#### 9.3 Service & Networking

**Service Management:**
- List services
- Create service (ClusterIP, NodePort, LoadBalancer)
- Edit service
- Delete service
- Service inspection (endpoints, selectors)
- Service port mapping
- Service type conversion
- External name services

**Ingress Management:**
- List Ingresses
- Create Ingress
- Edit Ingress rules
- Delete Ingress
- TLS certificate management
- Ingress class selection
- Path-based routing
- Host-based routing

**NetworkPolicy Management:**
- List NetworkPolicies
- Create NetworkPolicy
- Edit NetworkPolicy
- Delete NetworkPolicy
- Ingress/Egress rules
- Pod selector configuration

**Endpoints Management:**
- List Endpoints
- View Endpoint details
- Endpoint subset inspection

#### 9.4 Configuration & Storage

**ConfigMap Management:**
- List ConfigMaps
- Create ConfigMap (from file, literal, YAML)
- Edit ConfigMap
- Delete ConfigMap
- View ConfigMap data
- ConfigMap versioning
- Use as environment variables
- Mount as volumes

**Secret Management:**
- List Secrets
- Create Secret (generic, docker-registry, TLS)
- Edit Secret
- Delete Secret
- View Secret data (base64 decoded)
- Secret types (Opaque, TLS, Docker config)
- Use in pods

**PersistentVolume Management:**
- List PersistentVolumes
- Create PersistentVolume
- Edit PersistentVolume
- Delete PersistentVolume
- View PV status and capacity
- Storage class association
- Reclaim policy management

**PersistentVolumeClaim Management:**
- List PersistentVolumeClaims
- Create PersistentVolumeClaim
- Edit PersistentVolumeClaim
- Delete PersistentVolumeClaim
- View PVC status
- Resize PVC
- Bind status

**StorageClass Management:**
- List StorageClasses
- Create StorageClass
- Edit StorageClass
- Delete StorageClass
- Provisioner configuration
- Volume binding mode

#### 9.5 Cluster Resources

**Node Management:**
- List nodes
- View node details
- Node metrics (CPU, Memory, Disk)
- Node conditions
- Node labels and taints
- Cordon/Uncordon node
- Drain node
- Node capacity and allocatable resources
- Node events

**Namespace Management:**
- List namespaces
- Create namespace
- Delete namespace
- Set resource quotas
- Set limit ranges
- Namespace labels

**ServiceAccount Management:**
- List ServiceAccounts
- Create ServiceAccount
- Delete ServiceAccount
- Token management
- RBAC binding

**ResourceQuota Management:**
- List ResourceQuotas
- Create ResourceQuota
- Edit ResourceQuota
- Delete ResourceQuota
- View quota usage

**LimitRange Management:**
- List LimitRanges
- Create LimitRange
- Edit LimitRange
- Delete LimitRange

#### 9.6 RBAC (Role-Based Access Control)

**Role Management:**
- List Roles
- Create Role
- Edit Role
- Delete Role
- View Role permissions

**ClusterRole Management:**
- List ClusterRoles
- Create ClusterRole
- Edit ClusterRole
- Delete ClusterRole

**RoleBinding Management:**
- List RoleBindings
- Create RoleBinding
- Edit RoleBinding
- Delete RoleBinding

**ClusterRoleBinding Management:**
- List ClusterRoleBindings
- Create ClusterRoleBinding
- Edit ClusterRoleBinding
- Delete ClusterRoleBinding

#### 9.7 Advanced Features

**HorizontalPodAutoscaler (HPA):**
- List HPAs
- Create HPA
- Edit HPA (metrics, min/max replicas)
- Delete HPA
- View HPA status and metrics

**VerticalPodAutoscaler (VPA):**
- List VPAs
- Create VPA
- Edit VPA
- Delete VPA

**PodDisruptionBudget:**
- List PodDisruptionBudgets
- Create PodDisruptionBudget
- Edit PodDisruptionBudget
- Delete PodDisruptionBudget

**Custom Resource Definitions (CRD):**
- List CRDs
- View CRD details
- Create custom resources
- Edit custom resources
- Delete custom resources

**Events:**
- View cluster events
- Filter events by namespace/resource
- Event streaming (real-time)

**Port Forwarding:**
- Forward pod ports to local
- Forward service ports
- Multiple port forwards
- Port forward management
- Auto-reconnect

**Resource Metrics:**
- Cluster-wide metrics
- Node metrics
- Pod metrics
- Container metrics
- Integration with Metrics Server

**YAML/JSON Editor:**
- Edit any resource as YAML/JSON
- Syntax validation
- Apply changes
- Dry-run mode

### Backend Implementation

**Location:** `internal/modules/kubernetes/`

```
internal/modules/kubernetes/
├── client.go               # K8s client-go
├── pod.go                  # Pod operations
├── deployment.go           # Deployment operations
├── service.go              # Service operations
├── configmap.go            # ConfigMap operations
├── secret.go               # Secret operations
├── exec.go                 # Pod exec
├── portforward.go          # Port forwarding
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/kubernetes/`

```
web/src/pages/kubernetes/
├── ClusterList.tsx
├── NewCluster.tsx
├── PodList.tsx
├── PodDetails.tsx
├── PodLogs.tsx
├── PodExec.tsx             # Terminal in pod
├── DeploymentList.tsx
├── ServiceList.tsx
├── ConfigMapList.tsx
├── SecretList.tsx
└── PortForward.tsx
```

### Kubernetes Permissions

```go
K8S_PERMISSIONS = {
    // Connection
    "k8s.connect": {
        "module": "kubernetes",
        "depends_on": [],
        "description": "Connect to K8s clusters",
    },

    // Pods
    "k8s.pod.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List pods",
    },
    "k8s.pod.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "View pod details",
    },
    "k8s.pod.exec": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Execute in pods",
    },
    "k8s.pod.logs": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "View pod logs",
    },
    "k8s.pod.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Delete pods",
    },
    "k8s.pod.portforward": {
        "module": "kubernetes",
        "depends_on": ["k8s.pod.list"],
        "description": "Port forward to pods",
    },

    // Deployments
    "k8s.deployment.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List deployments",
    },
    "k8s.deployment.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Create deployments",
    },
    "k8s.deployment.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Edit deployments",
    },
    "k8s.deployment.scale": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Scale deployments",
    },
    "k8s.deployment.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.list"],
        "description": "Delete deployments",
    },
    "k8s.deployment.rollback": {
        "module": "kubernetes",
        "depends_on": ["k8s.deployment.edit"],
        "description": "Rollback deployments",
    },

    // StatefulSets
    "k8s.statefulset.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List StatefulSets",
    },
    "k8s.statefulset.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.statefulset.list"],
        "description": "Manage StatefulSets",
    },

    // DaemonSets
    "k8s.daemonset.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List DaemonSets",
    },
    "k8s.daemonset.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.daemonset.list"],
        "description": "Manage DaemonSets",
    },

    // Jobs & CronJobs
    "k8s.job.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List Jobs",
    },
    "k8s.job.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.job.list"],
        "description": "Manage Jobs",
    },
    "k8s.cronjob.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List CronJobs",
    },
    "k8s.cronjob.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.cronjob.list"],
        "description": "Manage CronJobs",
    },

    // Services
    "k8s.service.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List services",
    },
    "k8s.service.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Create services",
    },
    "k8s.service.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Edit services",
    },
    "k8s.service.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.service.list"],
        "description": "Delete services",
    },

    // Ingress
    "k8s.ingress.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List Ingresses",
    },
    "k8s.ingress.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.ingress.list"],
        "description": "Manage Ingresses",
    },

    // NetworkPolicy
    "k8s.networkpolicy.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List NetworkPolicies",
    },
    "k8s.networkpolicy.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.networkpolicy.list"],
        "description": "Manage NetworkPolicies",
    },

    // ConfigMaps
    "k8s.configmap.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List ConfigMaps",
    },
    "k8s.configmap.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Create ConfigMaps",
    },
    "k8s.configmap.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Edit ConfigMaps",
    },
    "k8s.configmap.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.configmap.list"],
        "description": "Delete ConfigMaps",
    },

    // Secrets
    "k8s.secret.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List secrets",
    },
    "k8s.secret.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "View secret data",
    },
    "k8s.secret.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "Create secrets",
    },
    "k8s.secret.edit": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.view"],
        "description": "Edit secrets",
    },
    "k8s.secret.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.secret.list"],
        "description": "Delete secrets",
    },

    // Storage
    "k8s.pv.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List PersistentVolumes",
    },
    "k8s.pv.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.pv.list"],
        "description": "Manage PersistentVolumes",
    },
    "k8s.pvc.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List PersistentVolumeClaims",
    },
    "k8s.pvc.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.pvc.list"],
        "description": "Manage PersistentVolumeClaims",
    },
    "k8s.storageclass.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List StorageClasses",
    },
    "k8s.storageclass.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.storageclass.list"],
        "description": "Manage StorageClasses",
    },

    // Nodes
    "k8s.node.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List nodes",
    },
    "k8s.node.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.node.list"],
        "description": "View node details",
    },
    "k8s.node.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.node.list"],
        "description": "Manage nodes (cordon, drain)",
    },

    // Namespaces
    "k8s.namespace.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List namespaces",
    },
    "k8s.namespace.create": {
        "module": "kubernetes",
        "depends_on": ["k8s.namespace.list"],
        "description": "Create namespaces",
    },
    "k8s.namespace.delete": {
        "module": "kubernetes",
        "depends_on": ["k8s.namespace.list"],
        "description": "Delete namespaces",
    },

    // RBAC
    "k8s.rbac.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View RBAC resources",
    },
    "k8s.rbac.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.rbac.view"],
        "description": "Manage RBAC resources",
    },

    // Advanced
    "k8s.hpa.list": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "List HPAs",
    },
    "k8s.hpa.manage": {
        "module": "kubernetes",
        "depends_on": ["k8s.hpa.list"],
        "description": "Manage HPAs",
    },
    "k8s.events.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View cluster events",
    },
    "k8s.metrics.view": {
        "module": "kubernetes",
        "depends_on": ["k8s.connect"],
        "description": "View resource metrics",
    },
}
```

---

## 10. Database Module

**Status:** Default Enabled

### Features

#### 10.1 MySQL Features

**Connection Management:**
- Connect to MySQL server
- Use vault identities for credentials
- SSL/TLS support
- SSH tunnel support
- Connection pooling
- Multiple database connections

**Query & Execution:**
- SQL query editor with syntax highlighting
- Execute SELECT queries
- Execute INSERT/UPDATE/DELETE
- Execute DDL (CREATE/ALTER/DROP)
- Multiple query execution
- Query timeout configuration
- Explain query execution plan
- Query profiling

**Database Browser:**
- List databases
- Create database
- Drop database
- Switch database
- View database size
- Character set and collation

**Table Management:**
- List tables
- Create table
- Alter table structure
- Drop table
- Rename table
- Truncate table
- View table structure (columns, types, attributes)
- View table indexes
- View table foreign keys
- View table triggers
- View table size
- Table statistics

**Data Browser:**
- Browse table data with pagination
- Filter rows (WHERE conditions)
- Sort columns
- Search across columns
- Edit rows inline
- Insert new rows
- Delete rows
- Bulk operations

**Schema Tools:**
- View indexes (create, drop, analyze)
- View foreign keys
- View triggers (create, drop, enable/disable)
- View stored procedures
- View functions
- View views (create, drop)
- View events

**Import/Export:**
- Export results to CSV
- Export results to JSON
- Export results to Excel
- Export results to SQL
- Import data from CSV
- SQL dump export
- SQL dump import

**User & Permissions:**
- List MySQL users
- Create users
- Grant/Revoke privileges
- View user permissions
- Change user password

**Server Management:**
- View server status
- View server variables
- View processlist (kill queries)
- View slow query log
- View binary logs
- Flush logs/privileges/tables

#### 10.2 PostgreSQL Features

**Connection Management:**
- Connect to PostgreSQL server
- Use vault identities
- SSL/TLS modes (disable, require, verify-ca, verify-full)
- SSH tunnel support
- Connection pooling
- Multiple schemas

**Query & Execution:**
- SQL query editor with PostgreSQL syntax
- Execute queries
- Transaction management (BEGIN, COMMIT, ROLLBACK)
- Savepoints
- Prepared statements
- Query explain/analyze
- Query planner visualization

**Database Browser:**
- List databases
- Create database
- Drop database
- Database templates
- View database encoding
- Database statistics

**Schema Management:**
- List schemas
- Create schema
- Drop schema
- Set search path
- Schema permissions

**Table Management:**
- List tables (public and all schemas)
- Create table with constraints
- Alter table
- Drop table
- Table inheritance
- Partitioned tables
- View table statistics
- Analyze table

**Data Types:**
- Support for PostgreSQL-specific types (ARRAY, JSON, JSONB, HSTORE, UUID, etc.)
- Custom types/domains
- Enum types
- Range types

**Advanced Features:**
- Sequences (create, alter, drop, currval, nextval)
- Views (create, drop, materialized views)
- Functions (PL/pgSQL, SQL, etc.)
- Stored procedures
- Triggers
- Rules
- Constraints (check, unique, primary key, foreign key)

**Extensions:**
- List installed extensions
- Install extensions
- View available extensions
- PostGIS support (if installed)

**User & Roles:**
- List roles/users
- Create role
- Grant/Revoke privileges
- Role membership
- Row-level security policies

**Import/Export:**
- Export to CSV/JSON/Excel
- pg_dump integration
- COPY command support
- pg_restore integration

**Server Management:**
- View server settings
- View active connections
- Kill connections
- View locks
- View statistics (pg_stat views)
- Vacuum/Analyze
- Reindex

#### 10.3 MongoDB Features

**Connection Management:**
- Connect to MongoDB server/cluster
- Use vault identities
- Replica set support
- Sharded cluster support
- SSL/TLS support
- SSH tunnel support
- Authentication mechanisms (SCRAM, X.509, LDAP)

**Database Browser:**
- List databases
- Create database
- Drop database
- Database statistics
- Storage engine info

**Collection Management:**
- List collections
- Create collection
- Drop collection
- Rename collection
- Collection statistics
- Capped collections
- View collection indexes
- Create/Drop indexes (single, compound, text, geospatial)
- Index statistics

**Document Operations:**
- Browse documents with pagination
- Insert document (JSON editor)
- Update document
- Delete document
- Bulk operations
- Find with query (MongoDB query syntax)
- Sort/Limit/Skip
- Projection

**Query Features:**
- MongoDB query editor
- Aggregation pipeline builder (visual)
- Find operations
- Count documents
- Distinct values
- Map-Reduce operations
- Text search

**Aggregation:**
- Visual pipeline builder
- Stage-by-stage execution
- Pipeline templates
- Export pipeline as code
- Aggregation explain

**Schema Tools:**
- Schema analyzer (infer schema from documents)
- Schema validation rules
- JSON schema support

**Import/Export:**
- Export to JSON
- Export to CSV
- Import from JSON
- Import from CSV
- mongodump/mongorestore support

**User & Security:**
- List users
- Create user
- Update user roles
- Drop user
- Built-in roles
- Custom roles

**Server Management:**
- Server status
- Current operations
- Kill operations
- Profiler
- Server logs

#### 10.4 Redis Features

**Connection Management:**
- Connect to Redis server
- Use vault identities
- Redis Sentinel support
- Redis Cluster support
- SSL/TLS support
- SSH tunnel support
- Connection pooling

**Key Browser:**
- List keys (with pattern matching)
- Search keys (SCAN command)
- Key count
- Key type detection
- Key TTL display
- Key memory usage
- Database selector (DB 0-15)

**Key Operations:**
- Get key value
- Set key value
- Delete key
- Rename key
- Set TTL/Expire
- Persist key (remove TTL)
- Type-specific operations

**Data Type Support:**
- String (GET, SET, APPEND, INCR, DECR)
- Hash (HGET, HSET, HDEL, HGETALL, HINCRBY)
- List (LPUSH, RPUSH, LPOP, RPOP, LRANGE, LINDEX)
- Set (SADD, SREM, SMEMBERS, SINTER, SUNION, SDIFF)
- Sorted Set (ZADD, ZREM, ZRANGE, ZRANK, ZSCORE)
- Bitmap operations
- HyperLogLog
- Geospatial indexes
- Stream (XADD, XREAD, XRANGE)

**Command Execution:**
- Redis CLI (execute any Redis command)
- Command history
- Command auto-complete
- Command documentation

**Pub/Sub:**
- Subscribe to channels
- Publish messages
- Pattern subscriptions
- Monitor pub/sub activity

**Server Management:**
- Server info
- Memory stats
- CPU stats
- Keyspace statistics
- Replication info
- Client list
- Kill client connections
- Config get/set
- Slow log
- Monitor command execution

**Persistence:**
- View RDB/AOF status
- Trigger BGSAVE
- Trigger BGREWRITEAOF
- View last save time

**Import/Export:**
- Export keys to JSON
- Import keys from JSON
- RDB file download/upload

### Backend Implementation

**Location:** `internal/modules/database/`

```
internal/modules/database/
├── client.go               # Database client factory
├── mysql.go                # MySQL client
├── postgres.go             # PostgreSQL client
├── mongodb.go              # MongoDB client
├── redis.go                # Redis client
├── query.go                # Query execution
├── schema.go               # Schema inspection
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/databases/`

```
web/src/pages/databases/
├── ConnectionList.tsx
├── NewConnection.tsx
├── QueryEditor.tsx         # SQL editor
├── TableBrowser.tsx
├── SchemaBrowser.tsx
├── ResultTable.tsx
├── RedisClient.tsx
└── MongoClient.tsx
```

### Database Permissions

```go
DATABASE_PERMISSIONS = {
    // Connection
    "database.connect": {
        "module": "database",
        "depends_on": ["vault.view"],
        "description": "Connect to databases",
    },

    // Query - Read
    "database.query.read": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Execute SELECT queries",
    },
    "database.query.explain": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Explain query execution",
    },

    // Query - Write
    "database.query.write": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Execute INSERT/UPDATE/DELETE",
    },
    "database.query.transaction": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Manage transactions",
    },

    // Query - DDL
    "database.query.ddl": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Execute DDL (CREATE/ALTER/DROP)",
    },

    // Schema - View
    "database.schema.view": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View database schema",
    },
    "database.table.list": {
        "module": "database",
        "depends_on": ["database.schema.view"],
        "description": "List tables",
    },
    "database.table.structure": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "View table structure",
    },

    // Schema - Manage
    "database.table.create": {
        "module": "database",
        "depends_on": ["database.schema.view"],
        "description": "Create tables",
    },
    "database.table.alter": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Alter tables",
    },
    "database.table.drop": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "Drop tables",
    },
    "database.index.manage": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Manage indexes",
    },

    // Data Browser
    "database.data.browse": {
        "module": "database",
        "depends_on": ["database.table.list"],
        "description": "Browse table data",
    },
    "database.data.edit": {
        "module": "database",
        "depends_on": ["database.data.browse", "database.query.write"],
        "description": "Edit data inline",
    },
    "database.data.delete": {
        "module": "database",
        "depends_on": ["database.data.browse", "database.query.write"],
        "description": "Delete data",
    },

    // Import/Export
    "database.export": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Export query results/data",
    },
    "database.import": {
        "module": "database",
        "depends_on": ["database.query.write"],
        "description": "Import data",
    },
    "database.dump": {
        "module": "database",
        "depends_on": ["database.schema.view", "database.query.read"],
        "description": "Database dump (backup)",
    },
    "database.restore": {
        "module": "database",
        "depends_on": ["database.query.ddl", "database.query.write"],
        "description": "Database restore",
    },

    // User Management
    "database.user.list": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "List database users",
    },
    "database.user.create": {
        "module": "database",
        "depends_on": ["database.user.list"],
        "description": "Create database users",
    },
    "database.user.grant": {
        "module": "database",
        "depends_on": ["database.user.list"],
        "description": "Grant/Revoke privileges",
    },

    // Server Management
    "database.server.status": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View server status",
    },
    "database.server.variables": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "View server variables",
    },
    "database.server.processes": {
        "module": "database",
        "depends_on": ["database.server.status"],
        "description": "View/Kill processes",
    },
    "database.server.logs": {
        "module": "database",
        "depends_on": ["database.server.status"],
        "description": "View server logs",
    },

    // MongoDB-specific
    "database.mongodb.aggregate": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Execute aggregation pipelines",
    },
    "database.mongodb.index": {
        "module": "database",
        "depends_on": ["database.table.structure"],
        "description": "Manage MongoDB indexes",
    },

    // Redis-specific
    "database.redis.cli": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Execute Redis commands",
    },
    "database.redis.pubsub": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Pub/Sub operations",
    },
    "database.redis.keys": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Manage Redis keys",
    },
}
```

---

## 11. Proxmox Module

**Status:** Optional

### Features

#### 11.1 Proxmox Connection
- Connect to Proxmox VE hosts
- API token authentication
- Username/password authentication
- Realm selection (PAM, PVE, etc.)
- Node selection

#### 11.2 Virtual Machine Management
- List VMs
- Start/Stop/Restart VMs
- VM console (noVNC)
- VM configuration
- Create VMs
- Delete VMs
- VM snapshots

#### 11.3 LXC Container Management
- List containers
- Start/Stop containers
- Container console
- Create containers
- Delete containers

#### 11.4 Storage Management
- List storage
- Storage usage
- ISO management

### Backend Implementation

**Location:** `internal/modules/proxmox/`

```
internal/modules/proxmox/
├── client.go               # Proxmox API client
├── vm.go                   # VM operations
├── container.go            # LXC operations
├── storage.go              # Storage operations
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/proxmox/`

```
web/src/pages/proxmox/
├── HostList.tsx
├── NewHost.tsx
├── VMList.tsx
├── VMConsole.tsx           # noVNC viewer
├── ContainerList.tsx
└── StorageList.tsx
```

---

## 12. Object Storage Module

**Status:** Optional

### Features

#### 12.1 Supported Protocols
- AWS S3
- MinIO
- Google Cloud Storage
- Azure Blob Storage
- DigitalOcean Spaces

#### 12.2 Object Operations
- Browse buckets/containers
- Upload objects
- Download objects
- Create buckets
- Delete objects/buckets
- Object metadata management
- Object search

#### 12.3 Multi-Cloud Support
- S3-compatible storage
- Cloud provider integration
- Unified object browser

### Backend Implementation

**Location:** `internal/modules/objectstorage/`

```
internal/modules/objectstorage/
├── s3.go
├── gcs.go
├── azure.go
├── minio.go
├── permissions.go
└── handler.go
```

### Frontend Implementation

**Location:** `web/src/pages/objectstorage/`

```
web/src/pages/objectstorage/
├── ConnectionList.tsx
├── S3Browser.tsx
├── ObjectBrowser.tsx
└── ObjectManager.tsx
```

---

## 13. Monitoring Module

**Status:** Core (Always Enabled)

### Features

#### 13.1 Prometheus Metrics
- HTTP request metrics
- WebSocket connection metrics
- Active session count
- Connection count by protocol
- Authentication attempts
- Permission check metrics
- Session duration
- Clipboard sync events

#### 13.2 Health Checks
- `/health` - Simple up/down
- `/health/ready` - Ready to serve
- `/health/live` - Liveness probe
- Database connectivity
- Module status

#### 13.3 System Monitoring
- Active connections
- Resource usage
- Error rates
- Response times

### Backend Implementation

**Location:** `internal/monitoring/`

```
internal/monitoring/
├── metrics.go              # Prometheus metrics
├── health.go               # Health checks
└── collector.go            # Metric collection
```

### Frontend Implementation

**Location:** `web/src/pages/admin/`

```
web/src/pages/admin/
└── Monitoring.tsx          # Metrics dashboard
```

---

## 14. Notification Module

**Status:** Core (Always Enabled)

### Features

#### 14.1 Notification Types
- Session shared with you
- Session ended
- Permission granted/revoked
- Connection failed
- Security alerts
- System updates

#### 14.2 Delivery Methods
- Real-time via WebSocket
- In-app notification center
- Persistent storage
- Mark as read/unread

#### 14.3 User Preferences
- Notification preferences
- Email notifications (optional)
- Desktop notifications

### Backend Implementation

**Location:** `internal/monitoring/notifications.go`

### Frontend Implementation

**Location:** `web/src/components/notifications/`

```
web/src/components/notifications/
├── NotificationCenter.tsx
├── NotificationItem.tsx
└── NotificationBell.tsx
```

---

## Module Dependency Graph

```
Core Module (Required)
├── Vault Module (Required)
│   └── Used by: SSH, RDP, VNC, Database, Proxmox
│
├── SSH Module
│   └── SFTP (included with SSH)
│
├── Telnet Module
│
├── RDP Module (Rust FFI)
│
├── VNC Module (Rust FFI)
│
├── Docker Module
│
├── Kubernetes Module
│
├── Database Module
│   ├── MySQL
│   ├── PostgreSQL
│   ├── MongoDB
│   └── Redis
│
├── Proxmox Module
│
├── Object Storage Module
│   ├── S3
│   ├── MinIO
│   ├── Google Cloud Storage
│   ├── Azure Blob Storage
│   └── DigitalOcean Spaces
│
├── Monitoring Module (Required)
│
└── Notification Module (Required)
```

---

## Implementation Checklist

### Phase 1: Core Foundation
- [ ] Core module (auth, users, teams, permissions)
- [ ] Vault module (identities, SSH keys, encryption)
- [ ] First-time setup UI
- [ ] Database schema (SQLite)
- [ ] JWT authentication
- [ ] Permission system with dependencies
- [ ] Audit logging
- [ ] Monitoring (Prometheus)
- [ ] Health checks
- [ ] Notification system

### Phase 2: Terminal Protocols
- [ ] SSH module (v1, v2, auto-detect)
- [ ] SFTP module (file browser)
- [ ] Terminal component (xterm.js with user preferences)
- [ ] Auto-reconnection logic
- [ ] Session recording
- [ ] Telnet module

### Phase 3: Remote Desktop
- [ ] Rust FFI build system (cbindgen, static linking)
- [ ] RDP module (IronRDP + Rust FFI)
- [ ] VNC module (vnc-rs + Rust FFI)
- [ ] Remote desktop viewer component

### Phase 4: Container & Cloud
- [ ] Docker module
- [ ] Kubernetes module
- [ ] Database module (MySQL, PostgreSQL, Redis)
- [ ] MongoDB support

### Phase 5: Advanced Features
- [ ] Proxmox module
- [ ] Object storage module (S3, MinIO, GCS, Azure)
- [ ] Session sharing
- [ ] Clipboard synchronization
- [ ] Advanced monitoring dashboard

---

**End of Module Implementation Guide**
