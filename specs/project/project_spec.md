# Enterprise Remote Access Platform - Business Specification

## 📋 Table of Contents

1. [Project Overview](#project-overview)
2. [Technology Stack](#technology-stack)
3. [Project Structure](#project-structure)
4. [Architecture](#architecture)
5. [Module Specifications](#module-specifications)
6. [Database Schema](#database-schema)
7. [Permission System](#permission-system)
8. [Configuration](#configuration)
9. [Deployment Model](#deployment-model)

---

## 1. Project Overview

### 1.1 Description

A comprehensive web-based **remote client platform** for managing enterprise infrastructure access. This is a centralized gateway that provides secure client connections to external services - NOT a service provider itself. Users connect through this platform to access their Docker hosts, SSH servers, Kubernetes clusters, databases, and other remote infrastructure.

### 1.2 Key Features

**Remote Client Capabilities:**
- Multi-protocol terminal clients (SSH, Telnet, Serial)
- Remote desktop clients (RDP, VNC)
- File transfer clients (SFTP, FTP, SMB, NFS, WebDAV, Cloud Storage)
- Container management clients (Docker, Kubernetes)
- Virtual machine clients (Proxmox)
- Database clients (MySQL, PostgreSQL, MongoDB, Redis)

**Enterprise Features:**
- Fine-grained permission system with dependencies
- Multi-tenancy with organizations and teams
- **Enterprise authentication (OIDC, SAML, LDAP, OAuth2, Local)** - All providers configured via UI by admins
- **Secret management (Credential Vault)** - Store SSH keys, passwords, database credentials
- **Connection profiles with reusable identities**
- Session recording and audit logging
- **Auto-reconnection support** for dropped connections
- **Prometheus metrics and monitoring**
- **Health checks and status monitoring**
- **Session sharing between users**
- **Clipboard synchronization**
- **Real-time notification system**
- Single binary deployment with embedded frontend

### 1.3 Target Users

- DevOps Engineers accessing remote infrastructure
- System Administrators managing multiple servers
- Database Administrators connecting to databases
- Security Teams monitoring access and sessions
- Enterprise IT Departments requiring centralized access control

---

## 2. Technology Stack

### 2.1 Backend

#### Core Framework
- **Language**: Go 1.21+
- **Web Framework**: Gin (github.com/gin-gonic/gin)
- **WebSocket**: gorilla/websocket

#### Rust FFI Modules (Static Linking via C-bindings)
- **Language**: Rust 1.75+
- **RDP**: IronRDP
- **VNC**: vnc-rs
- **Build**: Cargo with `staticlib` output for C-compatible static linking
- **C-Bindings**: cbindgen for generating C header files
- **Integration**: CGO with static linking (no dynamic libraries)

**IMPORTANT: Always check for latest library versions before implementation!**
- Check crates.io for latest Rust crate versions
- Check Go package documentation for latest versions
- Verify compatibility between library versions

#### Database
- **Primary ORM**: GORM (gorm.io/gorm)
- **Embedded Database**: SQLite 3.x (default, stored in ./data/)
- **Enterprise Databases**: PostgreSQL 14+, MySQL 8.0+ (optional)
- **Optional Cache**: Redis 7.x (sessions/cache)
- **Migrations**: GORM AutoMigrate
- **Secret Storage**: Encrypted credentials in database (AES-256-GCM)

#### Authentication & Authorization
```go
// Core Auth
"github.com/golang-jwt/jwt/v5"           // JWT
"golang.org/x/crypto/bcrypt"             // Password hashing
"github.com/google/uuid"                 // UUID generation

// OIDC
"github.com/coreos/go-oidc/v3/oidc"      // OpenID Connect
"golang.org/x/oauth2"                    // OAuth 2.0

// SAML
"github.com/crewjam/saml"                // SAML 2.0

// LDAP
"github.com/go-ldap/ldap/v3"             // LDAP/Active Directory

// MFA
"github.com/pquerna/otp"                 // TOTP
"github.com/skip2/go-qrcode"             // QR code generation
```

#### Protocol Libraries

**Pure Go (Excellent Libraries)**
```go
// SSH & SFTP
"golang.org/x/crypto/ssh"                // Official SSH
"github.com/pkg/sftp"                    // SFTP

// Telnet
"github.com/ziutek/telnet"               // Telnet client

// Docker
"github.com/docker/docker/client"        // Official Docker SDK

// Kubernetes
"k8s.io/client-go"                       // Official K8s client
"k8s.io/api"                             // K8s API types
"k8s.io/apimachinery"                    // K8s utilities

// Databases
"github.com/go-sql-driver/mysql"         // MySQL driver
"github.com/lib/pq"                      // PostgreSQL driver
"go.mongodb.org/mongo-driver"            // MongoDB
"github.com/redis/go-redis/v9"           // Redis

// File Sharing
"github.com/hirochachacha/go-smb2"       // SMB/CIFS
"github.com/aws/aws-sdk-go-v2"           // AWS S3
"google.golang.org/api/drive/v3"         // Google Drive
```

**Rust FFI (Better Libraries with Static Linking)**

**NOTE: Check for latest versions on crates.io before implementation!**

```toml
# rust-modules/rdp/Cargo.toml
[package]
name = "rdp-ffi"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["staticlib"]

[dependencies]
ironrdp = "0.1"  # Check latest: https://crates.io/crates/ironrdp
tokio = { version = "1.35", features = ["full"] }

[build-dependencies]
cbindgen = "0.29"  # Check latest: https://crates.io/crates/cbindgen
```

```toml
# rust-modules/vnc/Cargo.toml
[package]
name = "vnc-ffi"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["staticlib"]

[dependencies]
vnc = "0.4"  # Check latest: https://crates.io/crates/vnc
tokio = { version = "1.35", features = ["full"] }

[build-dependencies]
cbindgen = "0.29"  # Check latest: https://crates.io/crates/cbindgen
```

**cbindgen Configuration:**

```toml
# rust-modules/rdp/cbindgen.toml
language = "C"
include_guard = "RDP_FFI_H"
autogen_warning = "/* Warning, this file is autogenerated by cbindgen. Don't modify this manually. */"
no_includes = true
sys_includes = ["stdint.h", "stdbool.h"]

[export]
include = ["RDP"]

[export.rename]
"RDPSession" = "rdp_session_t"
```

**build.rs (C Header Generation):**

```rust
// rust-modules/rdp/build.rs
use std::env;
use std::path::PathBuf;

fn main() {
    let crate_dir = env::var("CARGO_MANIFEST_DIR").unwrap();

    cbindgen::Builder::new()
        .with_crate(crate_dir)
        .with_config(cbindgen::Config::from_file("cbindgen.toml").unwrap())
        .generate()
        .expect("Unable to generate C bindings")
        .write_to_file("rdp_ffi.h");

    println!("cargo:rerun-if-changed=src/lib.rs");
    println!("cargo:rerun-if-changed=cbindgen.toml");
}
```

**FFI Example (Rust Side):**

```rust
// rust-modules/rdp/src/lib.rs
use std::ffi::{CStr, CString};
use std::os::raw::c_char;

#[repr(C)]
pub struct RDPSession {
    host: *const c_char,
    port: u16,
    // ... internal state
}

#[no_mangle]
pub extern "C" fn rdp_session_new(host: *const c_char, port: u16) -> *mut RDPSession {
    let host_str = unsafe { CStr::from_ptr(host).to_str().unwrap() };

    let session = Box::new(RDPSession {
        host,
        port,
    });

    Box::into_raw(session)
}

#[no_mangle]
pub extern "C" fn rdp_session_connect(session: *mut RDPSession) -> bool {
    let session = unsafe { &mut *session };
    // Connection logic...
    true
}

#[no_mangle]
pub extern "C" fn rdp_session_free(session: *mut RDPSession) {
    if !session.is_null() {
        unsafe { drop(Box::from_raw(session)) };
    }
}
```

**Go CGO Integration:**

```go
// internal/modules/rdp/ffi.go
package rdp

/*
#cgo CFLAGS: -I${SRCDIR}/../../../rust-modules/rdp
#cgo LDFLAGS: -L${SRCDIR}/../../../rust-modules/rdp/target/release -lrdp_ffi
#include "rdp_ffi.h"
*/
import "C"
import "unsafe"

type RDPSession struct {
    ptr *C.rdp_session_t
}

func NewRDPSession(host string, port uint16) *RDPSession {
    cHost := C.CString(host)
    defer C.free(unsafe.Pointer(cHost))

    ptr := C.rdp_session_new(cHost, C.uint16_t(port))
    return &RDPSession{ptr: ptr}
}

func (s *RDPSession) Connect() bool {
    return bool(C.rdp_session_connect(s.ptr))
}

func (s *RDPSession) Close() {
    if s.ptr != nil {
        C.rdp_session_free(s.ptr)
        s.ptr = nil
    }
}
```

#### Monitoring & Observability
```go
// Prometheus
"github.com/prometheus/client_golang/prometheus"
"github.com/prometheus/client_golang/prometheus/promhttp"

// Structured Logging
"go.uber.org/zap"                        // Structured logging
```

#### Secret Management & Encryption
```go
"golang.org/x/crypto/chacha20poly1305"   // Encryption for secrets
"golang.org/x/crypto/argon2"             // Key derivation
"github.com/google/tink/go"              // Cryptographic library (optional)
```

#### Utilities
```go
"github.com/spf13/viper"                 // Configuration
"golang.org/x/sync/errgroup"             // Concurrent operations
"github.com/robfig/cron/v3"              // Scheduled tasks
```

### 2.2 Frontend

#### Core Framework (Latest Versions - 2025)
```json
{
  "react": "^19.0.0",
  "react-dom": "^19.0.0",
  "vite": "^7.0.0",
  "typescript": "^5.7.0"
}
```

**IMPORTANT: User Preferences & Customization**

All user-facing settings (terminal appearance, UI themes, file manager preferences, etc.) must be configurable through the Settings page. **Never hardcode user preferences** like terminal font family, font size, cursor style, themes, or other visual/behavioral settings in components.

**Key Principles:**
- ✅ All terminal settings (font, theme, cursor, etc.) stored in user preferences
- ✅ Settings persisted using Zustand with localStorage
- ✅ Components read from settings store, never use hardcoded values
- ✅ Provide sensible defaults but allow full customization
- ✅ Re-render components when preferences change
- ✅ Settings accessible via `/settings` page

**User Preference Categories:**
1. **Terminal Preferences** (`/settings/terminal`)
   - Font family (user-selectable, not hardcoded)
   - Font size (adjustable)
   - Cursor style (block, underline, bar)
   - Cursor blink (on/off)
   - Theme (dark, light, custom with color picker)
   - Custom color schemes

2. **UI Preferences** (`/settings/appearance`)
   - Application theme (dark, light, system)
   - Sidebar behavior (collapsed by default, etc.)
   - Language/locale

3. **File Manager Preferences** (`/settings/file-manager`)
   - Show hidden files
   - Default view (list, grid)
   - Sort order

4. **Session Preferences** (`/settings/sessions`)
   - Auto-reconnect behavior
   - Clipboard sync defaults
   - Session recording preferences

**Implementation Pattern:**
```typescript
// ❌ BAD: Hardcoded terminal settings
const terminal = new Terminal({
  fontSize: 14,
  fontFamily: 'Menlo, Monaco',
  cursorStyle: 'block'
});

// ✅ GOOD: Use user preferences
const { preferences } = useSettingsStore();
const terminal = new Terminal({
  fontSize: preferences.terminal.fontSize,
  fontFamily: preferences.terminal.fontFamily,
  cursorStyle: preferences.terminal.cursorStyle,
  theme: getTerminalTheme(preferences.terminal.theme)
});
```

#### UI Framework (Tailwind CSS 4)
```json
{
  "@radix-ui/react-*": "latest",         // Headless UI primitives
  "tailwindcss": "^4.1.0",               // Utility CSS (v4)
  "class-variance-authority": "^0.7.0",  // Component variants
  "clsx": "^2.1.0",                      // Conditional classes
  "lucide-react": "^0.460.0"             // Icons
}
```

#### Terminal & Remote Desktop
```json
{
  "xterm": "^5.5.0",                     // Terminal emulator
  "xterm-addon-fit": "^0.10.0",          // Terminal fit
  "xterm-addon-web-links": "^0.11.0",    // Clickable links
  "xterm-addon-search": "^0.15.0",       // Search in terminal
  "react-dropzone": "^14.3.0",           // File upload
  "@tanstack/react-table": "^8.20.0"     // File browser table
}
```

#### State Management & API
```json
{
  "@tanstack/react-query": "^5.59.0",    // Server state
  "zustand": "^5.0.0",                   // Client state
  "axios": "^1.7.0",                     // HTTP client
  "socket.io-client": "^4.8.0"           // WebSocket (for notifications)
}
```

#### Forms & Validation
```json
{
  "react-hook-form": "^7.53.0",          // Form handling
  "zod": "^3.23.0",                      // Schema validation
  "@hookform/resolvers": "^3.9.0"        // Form + Zod integration
}
```

#### Routing & Utils
```json
{
  "react-router": "^7.6.2",              // Routing (v7)
  "date-fns": "^4.1.0",                  // Date utilities
  "sonner": "^1.7.0"                     // Toast notifications (modern)
}
```

### 2.3 Build & Development Tools

```json
{
  "vite": "^7.0.0",
  "@vitejs/plugin-react": "^4.3.0",
  "eslint": "^9.15.0",
  "@typescript-eslint/parser": "^8.15.0",
  "prettier": "^3.3.0",
  "vitest": "^2.1.0",                    // Unit testing
  "@testing-library/react": "^16.0.0"    // Component testing (React 19)
}
```

---

## 3. Project Structure

```
shellcn/
├── cmd/
│   └── server/
│       └── main.go                      # Application entry point
│
├── internal/                            # Private application code
│   ├── app/
│   │   ├── app.go                       # Application initialization
│   │   ├── config.go                    # Configuration management
│   │   └── server.go                    # HTTP server setup
│   │
│   ├── api/                             # API layer
│   │   ├── router.go                    # Route definitions
│   │   ├── middleware/
│   │   │   ├── auth.go                  # Authentication middleware
│   │   │   ├── cors.go                  # CORS middleware
│   │   │   ├── logger.go                # Logging middleware
│   │   │   ├── ratelimit.go             # Rate limiting
│   │   │   ├── metrics.go               # Prometheus metrics
│   │   │   └── recovery.go              # Panic recovery
│   │   │
│   │   └── handlers/                    # HTTP handlers
│   │       ├── auth.go                  # Auth endpoints
│   │       ├── users.go                 # User management
│   │       ├── organizations.go         # Organization management
│   │       ├── permissions.go           # Permission management
│   │       ├── websocket.go             # WebSocket handler
│   │       ├── health.go                # Health check
│   │       └── metrics.go               # Metrics endpoint
│   │
│   ├── auth/                            # Authentication system
│   │   ├── auth.go                      # Core auth interface
│   │   ├── jwt.go                       # JWT implementation
│   │   ├── providers/
│   │   │   ├── local.go                 # Local auth
│   │   │   ├── oidc.go                  # OpenID Connect
│   │   │   ├── saml.go                  # SAML 2.0
│   │   │   └── ldap.go                  # LDAP/AD
│   │   │
│   │   └── mfa/
│   │       └── totp.go                  # TOTP implementation
│   │
│   ├── permissions/                     # Permission system
│   │   ├── checker.go                   # Permission checker
│   │   ├── dependencies.go              # Dependency definitions
│   │   ├── resolver.go                  # Dependency resolver
│   │   ├── enforcer.go                  # Permission enforcement
│   │   └── constraints.go               # Dynamic constraints
│   │
│   ├── vault/                           # Secret Management (Credential Vault)
│   │   ├── vault.go                     # Vault interface
│   │   ├── encryption.go                # AES-256-GCM encryption
│   │   ├── identity.go                  # Reusable identity management
│   │   ├── credentials.go               # Credential storage
│   │   ├── keys.go                      # SSH key management
│   │   ├── permissions.go               # Vault permission definitions
│   │   └── handler.go                   # Vault API handler
│   │
│   ├── monitoring/                      # Monitoring & observability
│   │   ├── metrics.go                   # Prometheus metrics
│   │   ├── health.go                    # Health checks
│   │   └── notifications.go             # Notification system
│   │
│   ├── modules/                         # Protocol client modules
│   │   ├── common/
│   │   │   ├── session.go               # Common session interface
│   │   │   ├── recorder.go              # Session recording
│   │   │   ├── sharing.go               # Session sharing
│   │   │   ├── clipboard.go             # Clipboard sync
│   │   │   └── pool.go                  # Connection pooling
│   │   │
│   │   ├── ssh/                         # SSH client module
│   │   ├── sftp/                        # SFTP client module
│   │   ├── telnet/                      # Telnet client module
│   │   ├── rdp/                         # RDP client (Rust FFI)
│   │   ├── vnc/                         # VNC client (Rust FFI)
│   │   ├── docker/                      # Docker client module
│   │   ├── kubernetes/                  # K8s client module
│   │   ├── proxmox/                     # Proxmox client module
│   │   ├── database/                    # Database client module
│   │   └── fileshare/                   # File sharing clients
│   │
│   ├── models/                          # Data models
│   │   ├── user.go
│   │   ├── organization.go
│   │   ├── role.go
│   │   ├── permission.go
│   │   ├── identity.go                  # Vault identity
│   │   ├── connection.go
│   │   ├── session.go
│   │   └── audit_log.go
│   │
│   ├── database/                        # Database layer
│   │   ├── db.go
│   │   ├── sqlite.go
│   │   ├── postgres.go
│   │   ├── mysql.go
│   │   └── repositories/
│   │
│   ├── services/                        # Business logic
│   │   ├── user_service.go
│   │   ├── auth_service.go
│   │   ├── vault_service.go             # Credential management
│   │   ├── permission_service.go
│   │   └── audit_service.go
│   │
│   └── utils/                           # Utilities
│       ├── crypto.go
│       ├── validator.go
│       └── errors.go
│
├── rust-modules/                        # Rust FFI modules (Static Linking)
│   ├── rdp/
│   │   ├── Cargo.toml                   # Dependencies + cbindgen
│   │   ├── cbindgen.toml                # C binding config
│   │   ├── build.rs                     # Build script (generates C headers)
│   │   ├── rdp_ffi.h                    # Generated C header (auto-generated)
│   │   └── src/
│   │       └── lib.rs                   # FFI implementation
│   │
│   └── vnc/
│       ├── Cargo.toml
│       ├── cbindgen.toml
│       ├── build.rs
│       ├── vnc_ffi.h                    # Generated C header
│       └── src/
│           └── lib.rs
│
├── web/                                 # Frontend application
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   │
│   │   ├── pages/                       # Page components
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Login.tsx
│   │   │   ├── vault/                   # Credential Vault Pages
│   │   │   │   ├── IdentityList.tsx     # /settings/identities
│   │   │   │   ├── NewIdentity.tsx
│   │   │   │   └── EditIdentity.tsx
│   │   │   ├── connections/
│   │   │   │   ├── ConnectionList.tsx
│   │   │   │   ├── NewConnection.tsx    # With identity dropdown
│   │   │   │   └── ConnectionDetails.tsx
│   │   │   ├── docker/
│   │   │   ├── kubernetes/
│   │   │   └── settings/
│   │   │
│   │   ├── components/                  # Reusable components
│   │   │   ├── ui/                      # Base UI (shadcn)
│   │   │   ├── terminal/
│   │   │   ├── file-manager/
│   │   │   └── vault/                   # Vault-specific components
│   │   │       ├── IdentitySelector.tsx # Dropdown for selecting identity
│   │   │       └── SSHKeyUpload.tsx
│   │   │
│   │   ├── hooks/
│   │   │   ├── useAuth.ts
│   │   │   ├── useVault.ts              # Vault hooks
│   │   │   └── useIdentity.ts
│   │   │
│   │   ├── lib/
│   │   ├── store/
│   │   └── types/
│   │
│   ├── package.json
│   ├── vite.config.ts                   # Vite 7 config
│   ├── tailwind.config.js               # Tailwind 4 config
│   └── tsconfig.json
│
├── go.mod
├── Makefile
└── README.md
```

---

## 4. Architecture

### 4.1 System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                   Single Go Binary                           │
│  ┌────────────────────────────────────────────────────────┐ │
│  │             Embedded Frontend (Vite 7 Build)           │ │
│  │  - React 19 SPA served at /                            │ │
│  │  - Static assets embedded via go:embed                 │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                  HTTP/WebSocket Server                 │ │
│  │  - Gin framework                                       │ │
│  │  - REST API: /api/*                                    │ │
│  │  - WebSocket: /ws/*                                    │ │
│  │  - Metrics: /metrics (Prometheus)                      │ │
│  │  - Health: /health                                     │ │
│  └────────────────────────────────────────────────────────┘ │
│                           │                                  │
│  ┌────────────────────────┴────────────────────────────┐   │
│  │              Middleware Layer                        │   │
│  │  - Authentication (JWT)                              │   │
│  │  - Authorization (Permission Checker)                │   │
│  │  - CORS                                              │   │
│  │  - Rate Limiting                                     │   │
│  │  - Logging & Audit                                   │   │
│  │  - Prometheus Metrics Collection                     │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                          │                                   │
│  ┌───────────────────────▼──────────────────────────────┐   │
│  │              Business Logic Layer                    │   │
│  │  - User Management                                   │   │
│  │  - Permission Management                             │   │
│  │  - Session Management & Sharing                      │   │
│  │  - Notification Service                              │   │
│  │  - Audit Logging                                     │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                          │                                   │
│  ┌───────────────────────▼──────────────────────────────┐   │
│  │         Protocol Client Module Layer (Pluggable)     │   │
│  │  ┌────────────────────────────────────────────────┐ │   │
│  │  │ Golang Clients         │ Rust FFI Clients      │ │   │
│  │  ├──────────────────────────┼─────────────────────┤ │   │
│  │  │ • SSH/SFTP              │ • RDP (IronRDP)      │ │   │
│  │  │ • Telnet                │ • VNC (vnc-rs)       │ │   │
│  │  │ • Docker Client         │ [Statically Linked]  │ │   │
│  │  │ • Kubernetes Client     │                      │ │   │
│  │  │ • Proxmox Client        │                      │ │   │
│  │  │ • Database Clients      │                      │ │   │
│  │  │ • File Sharing Clients  │                      │ │   │
│  │  └────────────────────────────────────────────────┘ │   │
│  └──────────────────────┬───────────────────────────────┘   │
│                          │                                   │
│  ┌───────────────────────▼──────────────────────────────┐   │
│  │              Data Access Layer                       │   │
│  │  - GORM ORM                                          │   │
│  │  - Repository Pattern                                │   │
│  │  - Database Abstraction                              │   │
│  └──────────────────────┬───────────────────────────────┘   │
└─────────────────────────┼───────────────────────────────────┘
                          │
         ┌────────────────┴────────────────┐
         │                                  │
┌────────▼────────┐              ┌─────────▼─────────┐
│  SQLite 3.x     │              │      Redis        │
│  ./data/        │              │    (Optional)     │
│ database.sqlite │              │  (Cache/Session)  │
└─────────────────┘              └───────────────────┘
```

### 4.2 Remote Client Architecture

**This platform is a CLIENT, not a SERVER:**

```
┌─────────────────────────────────────────────────────────┐
│                  ShellCN Platform                        │
│                  (Client Gateway)                        │
└───┬─────────────┬─────────────┬─────────────┬──────────┘
    │             │             │             │
    │ SSH Client  │ Docker API  │ K8s Client  │ DB Client
    │             │             │             │
    ▼             ▼             ▼             ▼
┌────────┐   ┌────────┐   ┌────────┐   ┌────────┐
│  SSH   │   │ Docker │   │  K8s   │   │ MySQL  │
│ Server │   │  Host  │   │Cluster │   │ Server │
│(Remote)│   │(Remote)│   │(Remote)│   │(Remote)│
└────────┘   └────────┘   └────────┘   └────────┘

Users → ShellCN Platform → External Services
```

---

## 5. Module Specifications

### 5.1 Core Modules (Minimum Default Enabled)

**Default enabled modules:**

1. **SSH/SFTP** - Essential remote server access (SSH v1 & v2 support)
2. **Telnet** - Legacy device management
3. **RDP** - Windows desktop access (Rust FFI)
4. **VNC** - Cross-platform remote desktop (Rust FFI)
5. **Docker** - Container management client
6. **Database Clients** - MySQL, PostgreSQL, Redis

**Optional modules (can be disabled):**
- Kubernetes client
- Proxmox client
- Advanced file sharing (SMB, NFS, S3)
- MongoDB client

### 5.2 SSH Module (Detailed Configuration)

#### SSH Protocol Support
- **SSH v1** - Legacy protocol support (disabled by default for security)
- **SSH v2** - Primary protocol (enabled by default)
- **Auto-detection** - Detect protocol version during handshake

#### SSH Connection Settings

**Basic Configuration:**
```go
type SSHConnectionConfig struct {
    // Basic
    Name        string
    Protocol    string  // "ssh-v1", "ssh-v2", "auto"
    Icon        string  // UI icon selection

    // Connection
    Address     string
    Port        int     // Default: 22

    // Authentication
    IdentityID  *string // Reference to reusable identity
    AuthMethod  string  // "password", "publickey", "keyboard-interactive"
    Username    string
    Password    string  // Encrypted in database
    PrivateKey  string  // Encrypted SSH private key
    Passphrase  string  // Encrypted key passphrase

    // Advanced - Encoding
    ReceiveEncoding  string  // "UTF-8", "ISO-8859-1", etc.
    TerminalEncoding string  // "UTF-8", "ISO-2022", "UTF-8-locked"

    // Advanced - Keyboard
    AltGrMode         string  // "auto", "ctrl-alt", "right-alt"
    AltKeyModifier    string  // "escape", "8-bit", "browser-key"
    BackspaceAsCtrlH  bool
    AltKeyAsMeta      bool
    CtrlCCopyBehavior bool
    CtrlVPasteBehavior bool

    // Advanced - Scrolling
    ScrollOnKeystroke    bool
    ScrollOnOutput       bool
    ScrollbarVisible     bool
    EmulateArrowWithScroll bool

    // Connection Behavior
    EnableReconnect      bool   // Auto-reconnect on disconnect
    ReconnectAttempts    int    // Max reconnect attempts (default: 3)
    ReconnectDelay       int    // Delay between reconnects in seconds
    ConnectionTimeout    int    // Connection timeout in seconds
    KeepAlive           int    // Keepalive interval in seconds

    // Session
    ClipboardEnabled     bool
    SessionRecording     bool

    // Notes
    Notes               string  // Rich text notes
}
```

**Connection Profiles (Reusable Identities):**
```go
type Identity struct {
    ID          string
    Name        string
    Type        string  // "ssh", "database", "generic"

    // SSH-specific
    Username    string
    Password    string  // Encrypted
    PrivateKey  string  // Encrypted SSH key
    Passphrase  string  // Encrypted passphrase

    // Metadata
    CreatedAt   time.Time
    UpdatedAt   time.Time
    UserID      string  // Owner
    SharedWith  []string // Shared user IDs
}
```

**Auto-Reconnection Logic:**
```go
type ReconnectionManager struct {
    Enabled         bool
    MaxAttempts     int
    RetryDelay      time.Duration
    BackoffMultiplier float64  // Exponential backoff
}

// Reconnection flow:
// 1. Detect connection drop
// 2. Store current session state
// 3. Attempt reconnection with exponential backoff
// 4. Restore session state on successful reconnection
// 5. Notify user of reconnection status
```

#### SSH Authentication Methods

1. **Password Authentication**
   - Plain password (encrypted in vault)
   - TOTP/MFA support

2. **Public Key Authentication**
   - RSA, ECDSA, Ed25519 keys
   - Encrypted private key storage
   - Passphrase-protected keys
   - Multiple keys per identity

3. **Keyboard-Interactive**
   - Challenge-response authentication
   - PAM support

4. **Agent Forwarding** (optional)
   - SSH agent support
   - Key forwarding to remote hosts

### 5.3 Telnet Module (Configuration)

**Telnet Connection Settings:**
```go
type TelnetConnectionConfig struct {
    Name        string
    Address     string
    Port        int  // Default: 23

    // Encoding
    ReceiveEncoding  string
    TerminalEncoding string

    // Keyboard & Scrolling (same as SSH)
    // ... (similar to SSH settings)

    // Reconnection
    EnableReconnect   bool
    ReconnectAttempts int

    Notes string
}
```

### 5.4 Secret Management (Credential Vault)

**Dedicated Vault Page:** `/settings/vault` or `/credentials`

The vault is a dedicated page for managing all credentials used across connections. Users can create reusable identities (credentials) and reference them when creating SSH, database, or other protocol connections.

**Vault Architecture:**
```go
// Encrypted storage for sensitive credentials
type VaultService interface {
    // Identity Management
    CreateIdentity(identity *Identity) error
    GetIdentity(id string) (*Identity, error)
    ListIdentities(userID string) ([]*Identity, error)
    UpdateIdentity(identity *Identity) error
    DeleteIdentity(id string) error

    // Credential Encryption
    EncryptCredential(plaintext string) (string, error)
    DecryptCredential(ciphertext string) (string, error)

    // SSH Key Management
    StoreSSHKey(name string, privateKey []byte, passphrase string) error
    GetSSHKey(name string) ([]byte, error)
    ListSSHKeys(userID string) ([]*SSHKey, error)
}
```

**Vault UI Flow:**

1. **Manage Identities Page** (`/settings/identities`)
   - List all saved identities (own + shared)
   - Create new identity
   - Edit/Delete existing identities
   - Share identities with team members
   - Upload SSH keys

2. **Creating Connection** (SSH/Database/etc.)
   - **Basic Tab:**
     - Name (e.g., "Production Server")
     - Protocol (dropdown: SSH v2, SSH v1, Telnet, etc.)
     - Icon (select from icon library)
     - Address (hostname or IP)
     - Port (default based on protocol)
     - **Identity** (dropdown with saved identities + "Custom Identity")
       - When identity selected: credentials auto-filled
       - When "Custom Identity" selected: show username/password fields
       - Link: "(Manage)" → navigates to `/settings/identities`
     - Authentication (dropdown: Password, Public Key, Keyboard-Interactive)
     - Username (if custom identity)
     - Password (if custom identity)
     - Notes (rich text editor)

   - **Advanced Tab:**
     - Encoding settings (Receive, Terminal)
     - Keyboard settings (AltGr, Alt modifier, Ctrl+C/V behavior, etc.)
     - Scrolling settings (scroll on keystroke, output, etc.)
     - Connection behavior (reconnect, timeout, keepalive)

**UI Component: Identity Selector**
```tsx
// components/vault/IdentitySelector.tsx
<div className="form-group">
  <label>Identity
    <Tooltip>
      Identities allow you to use the same credentials on multiple servers.
    </Tooltip>
    <Link href="/settings/identities">(Manage)</Link>
  </label>

  <Select value={selectedIdentity} onChange={handleIdentityChange}>
    <SelectItem value="custom">
      <i>Custom Identity</i>
    </SelectItem>
    {identities.map(identity => (
      <SelectItem key={identity.id} value={identity.id}>
        {identity.name}
      </SelectItem>
    ))}
  </Select>
</div>

{selectedIdentity === 'custom' && (
  <>
    <Input label="Username" {...} />
    <Input label="Password" type="password" {...} />
  </>
)}
```

**Encryption Strategy:**

**All credentials are encrypted before storage:**

1. **User Passwords**:
   - Hashed with bcrypt (cost factor 10+)
   - NEVER stored as plaintext
   - One-way encryption (cannot be reversed)

2. **Vault Credentials** (SSH passwords, DB passwords, etc.):
   - Encrypted with AES-256-GCM
   - Master key derived from `VAULT_ENCRYPTION_KEY` env var using Argon2
   - Unique nonce per credential
   - Encrypted at rest in database

3. **SSH Private Keys**:
   - Encrypted with AES-256-GCM before storage
   - Passphrases encrypted separately
   - Decrypted only when needed for connection

4. **Key Derivation**:
   - Argon2id (memory-hard, GPU-resistant)
   - Salt stored per credential
   - Key rotation supported (with re-encryption)

5. **Zero-Knowledge Option** (Optional):
   - User provides master passphrase
   - Credentials encrypted with user-specific key
   - System cannot decrypt without user passphrase
   - Higher security, user must remember passphrase

**Credential Sharing:**
- Share identities with team members
- Role-based access to shared credentials
- Audit trail for credential access

**Vault Permissions (Core Module):**
```go
// internal/vault/permissions.go
VAULT_PERMISSIONS = {
    "vault.view": {
        "module": "core",
        "depends_on": [],
        "description": "View saved identities (own identities only)",
    },
    "vault.create": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Create new identities/credentials",
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
        "description": "Share identities with other users",
    },
    "vault.use_shared": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Use identities shared by others",
    },
    "vault.manage_all": {
        "module": "core",
        "depends_on": ["vault.view", "vault.edit", "vault.delete"],
        "description": "Manage all identities (admin only)",
    },
}
```

### 5.5 Monitoring Integration (Prometheus)

#### Exposed Metrics
```go
// Connection metrics
connection_total{protocol="ssh|rdp|vnc|docker|k8s"}
connection_active{protocol="ssh|rdp|vnc|docker|k8s"}
connection_duration_seconds{protocol="ssh|rdp|vnc|docker|k8s"}

// Session metrics
session_active_users
session_shared_count
session_duration_seconds

// System metrics
http_requests_total{method,path,status}
http_request_duration_seconds{method,path}
websocket_connections_active
websocket_messages_total{direction="in|out"}

// Business metrics
auth_attempts_total{provider,status="success|failure"}
permission_checks_total{permission,result="allowed|denied"}
audit_logs_total
clipboard_sync_total
notifications_sent_total{type}
```

### 5.6 Health Check System

```go
// Health check endpoints
GET /health         // Simple up/down
GET /health/ready   // Ready to serve traffic
GET /health/live    // Liveness probe
```

### 5.7 Docker Module

**Comprehensive Container Management Client**

#### Docker Host Connection
- Connect to Docker daemon (TCP, Unix socket, SSH tunnel)
- TLS authentication with client certificates
- Docker context support
- Connection health monitoring

#### Container Management

**Container Operations:**
- List containers (all, running, stopped, paused)
- Create, start, stop, restart, pause/unpause, kill containers
- Rename, delete, prune stopped containers
- Update container configuration (resource limits)

**Container Inspection:**
- View container details, logs (real-time, follow, tail, timestamps)
- Container stats (CPU, memory, network, disk I/O)
- Container processes (top), file system changes
- Port mappings, environment variables, labels

**Container Interaction:**
- Execute commands (docker exec), attach to container
- Interactive terminal shell, copy files to/from container
- Export container filesystem, commit container to image

**Container Networking:**
- View/manage container networks
- Connect/disconnect containers to/from networks

#### Image Management
- List, pull, push, build, tag, delete images
- Prune unused images
- Save/load images to/from tar
- View image details, history, size, labels
- Registry operations (login, logout, search)
- Private registry support

#### Volume Management
- List, create, delete, prune volumes
- View volume details, mount points, size
- Volume driver support and options

#### Network Management
- List, create, delete networks (bridge, host, overlay, macvlan)
- Prune unused networks
- View network driver, subnet/gateway, connected containers
- IPAM configuration

#### System & Administration
- Docker version, system info (OS, architecture, CPU, memory)
- Disk usage, storage/logging driver
- Docker events (real-time monitoring)
- System-wide prune
- Container resource limits and restart policies

#### Docker Compose (Optional)
- List compose projects
- Deploy, stop, remove compose stacks
- Scale, restart services
- View compose logs

#### Docker Swarm (Optional)
- Initialize, join, leave swarm
- View, promote/demote nodes
- Manage services (list, create, scale, update, delete)
- View service logs and tasks/replicas

### 5.8 Kubernetes Module

**Comprehensive Cluster Management Client**

#### Cluster Connection
- kubeconfig support (upload or paste)
- Token, certificate, service account authentication
- Multiple context management, namespace selection
- Cluster switching, connection pooling

#### Workload Resources

**Pod Management:**
- List pods (all namespaces or specific)
- View pod details, logs (real-time, follow, tail, timestamps)
- Execute in pod (kubectl exec), pod terminal (multi-container)
- Delete pods, pod describe (YAML/JSON)
- Pod events, metrics (CPU, Memory, Network)
- Restart pod, port forward, copy files to/from pod

**Deployment Management:**
- List, create (YAML editor), edit deployments
- Scale (manual or autoscale), update, delete
- Rollback (revision history), pause/resume, restart
- View deployment status, events
- Replica set management

**StatefulSet, DaemonSet, Job & CronJob Management:**
- List, create, edit, scale, delete
- View status, rolling update management
- Node selector, schedule management
- Suspend/resume, trigger CronJob manually

**ReplicaSet Management:**
- List, view details, scale, delete

#### Service & Networking

**Service Management:**
- List, create (ClusterIP, NodePort, LoadBalancer), edit, delete
- Service inspection (endpoints, selectors), port mapping
- Service type conversion, external name services

**Ingress Management:**
- List, create, edit rules, delete Ingresses
- TLS certificate management, ingress class selection
- Path-based and host-based routing

**NetworkPolicy Management:**
- List, create, edit, delete NetworkPolicies
- Ingress/Egress rules, pod selector configuration

**Endpoints Management:**
- List, view details, endpoint subset inspection

#### Configuration & Storage

**ConfigMap Management:**
- List, create (from file, literal, YAML), edit, delete
- View ConfigMap data, versioning
- Use as environment variables, mount as volumes

**Secret Management:**
- List, create (generic, docker-registry, TLS), edit, delete
- View secret data (base64 decoded)
- Secret types (Opaque, TLS, Docker config)

**PersistentVolume & PersistentVolumeClaim:**
- List, create, edit, delete PV/PVC
- View status, capacity, resize PVC, bind status
- Storage class association, reclaim policy

**StorageClass Management:**
- List, create, edit, delete StorageClasses
- Provisioner configuration, volume binding mode

#### Cluster Resources

**Node Management:**
- List nodes, view details, metrics (CPU, Memory, Disk)
- Node conditions, labels, taints
- Cordon/uncordon, drain node
- Node capacity, allocatable resources, events

**Namespace Management:**
- List, create, delete namespaces
- Set resource quotas, limit ranges, labels

**ServiceAccount, ResourceQuota, LimitRange:**
- List, create, delete ServiceAccounts
- Token management, RBAC binding
- Manage quotas and limits

#### RBAC (Role-Based Access Control)
- List, create, edit, delete Roles/ClusterRoles
- List, create, edit, delete RoleBindings/ClusterRoleBindings
- View role permissions

#### Advanced Features

**HorizontalPodAutoscaler (HPA):**
- List, create, edit, delete HPAs
- Configure metrics, min/max replicas
- View HPA status and metrics

**VerticalPodAutoscaler (VPA):**
- List, create, edit, delete VPAs

**PodDisruptionBudget:**
- List, create, edit, delete PodDisruptionBudgets

**Custom Resource Definitions (CRD):**
- List CRDs, view details
- Create, edit, delete custom resources

**Events:**
- View cluster events
- Filter by namespace/resource
- Event streaming (real-time)

**Port Forwarding:**
- Forward pod/service ports to local
- Multiple port forwards, auto-reconnect

**Resource Metrics:**
- Cluster-wide, node, pod, container metrics
- Integration with Metrics Server

**YAML/JSON Editor:**
- Edit any resource as YAML/JSON
- Syntax validation, apply changes, dry-run mode

### 5.9 Database Module

**Multi-Database Client Platform**

#### MySQL Features

**Connection & Query:**
- Connect with vault identities, SSL/TLS, SSH tunnel
- SQL query editor with syntax highlighting
- Execute SELECT, INSERT/UPDATE/DELETE, DDL
- Multiple query execution, timeout config
- Explain query execution plan, query profiling

**Database & Table Management:**
- List, create, drop databases
- View database size, character set, collation
- List, create, alter, drop, rename, truncate tables
- View table structure, indexes, foreign keys, triggers
- Table statistics, size

**Data Browser:**
- Browse table data with pagination
- Filter rows (WHERE), sort columns, search
- Edit rows inline, insert, delete rows
- Bulk operations

**Schema Tools:**
- View/manage indexes, foreign keys, triggers
- View stored procedures, functions, views, events

**Import/Export:**
- Export to CSV, JSON, Excel, SQL
- Import from CSV
- SQL dump export/import

**User & Server Management:**
- List users, create, grant/revoke privileges
- View server status, variables, processlist
- View slow query log, binary logs
- Flush logs/privileges/tables

#### PostgreSQL Features

**Connection & Query:**
- Connect with SSL/TLS modes, SSH tunnel
- SQL editor with PostgreSQL syntax
- Transaction management (BEGIN, COMMIT, ROLLBACK)
- Savepoints, prepared statements
- Query explain/analyze, planner visualization

**Database & Schema:**
- List, create, drop databases
- Database templates, encoding, statistics
- List, create, drop schemas
- Set search path, schema permissions

**Table Management:**
- List tables (public and all schemas)
- Create table with constraints, alter, drop
- Table inheritance, partitioned tables
- Table statistics, analyze

**Data Types & Advanced Features:**
- PostgreSQL-specific types (ARRAY, JSON, JSONB, HSTORE, UUID)
- Custom types/domains, enum, range types
- Sequences (create, alter, drop, currval, nextval)
- Views (create, drop, materialized views)
- Functions (PL/pgSQL, SQL), stored procedures
- Triggers, rules, constraints

**Extensions:**
- List installed/available extensions
- Install extensions, PostGIS support

**User & Server:**
- List roles/users, create, grant/revoke
- Role membership, row-level security policies
- View server settings, active connections
- Kill connections, view locks, statistics
- Vacuum/Analyze, Reindex

**Import/Export:**
- Export to CSV/JSON/Excel
- pg_dump/pg_restore integration
- COPY command support

#### MongoDB Features

**Connection:**
- Connect with replica set, sharded cluster
- SSL/TLS, SSH tunnel
- Authentication (SCRAM, X.509, LDAP)

**Database & Collections:**
- List, create, drop databases
- Database statistics, storage engine
- List, create, drop, rename collections
- Collection statistics, capped collections
- Create/drop indexes (single, compound, text, geospatial)

**Document Operations:**
- Browse documents with pagination
- Insert (JSON editor), update, delete documents
- Bulk operations
- Find with MongoDB query syntax
- Sort/Limit/Skip, projection

**Query & Aggregation:**
- MongoDB query editor
- Aggregation pipeline builder (visual)
- Stage-by-stage execution, pipeline templates
- Export pipeline as code, aggregation explain
- Count, distinct, map-reduce, text search

**Schema Tools:**
- Schema analyzer (infer from documents)
- Schema validation rules, JSON schema

**User & Server:**
- List users, create, update roles, drop users
- Built-in and custom roles
- Server status, current operations
- Kill operations, profiler, server logs

**Import/Export:**
- Export/import JSON, CSV
- mongodump/mongorestore support

#### Redis Features

**Connection:**
- Connect with Sentinel, Cluster support
- SSL/TLS, SSH tunnel, connection pooling

**Key Browser:**
- List keys (pattern matching), search (SCAN)
- Key count, type detection, TTL, memory usage
- Database selector (DB 0-15)

**Key Operations:**
- Get, set, delete, rename keys
- Set TTL/Expire, persist key
- Type-specific operations

**Data Type Support:**
- String (GET, SET, APPEND, INCR, DECR)
- Hash (HGET, HSET, HDEL, HGETALL, HINCRBY)
- List (LPUSH, RPUSH, LPOP, RPOP, LRANGE, LINDEX)
- Set (SADD, SREM, SMEMBERS, SINTER, SUNION, SDIFF)
- Sorted Set (ZADD, ZREM, ZRANGE, ZRANK, ZSCORE)
- Bitmap, HyperLogLog, Geospatial, Stream

**Command Execution:**
- Redis CLI (execute any command)
- Command history, auto-complete, documentation

**Pub/Sub:**
- Subscribe to channels, publish messages
- Pattern subscriptions, monitor activity

**Server Management:**
- Server info, memory/CPU stats
- Keyspace statistics, replication info
- Client list, kill connections
- Config get/set, slow log, monitor

**Persistence:**
- View RDB/AOF status
- Trigger BGSAVE, BGREWRITEAOF
- View last save time

**Import/Export:**
- Export/import keys to/from JSON
- RDB file download/upload

### 5.10 Session Sharing

```go
// Session sharing capabilities
- Owner has full control
- Share with read-only access
- Share with interactive access
- Real-time collaboration
- Broadcast terminal output to multiple users
```

### 5.11 Clipboard Synchronization

```go
// Clipboard sync features
- Bidirectional sync (browser ↔ remote session)
- Permission-based (requires explicit permission)
- Size limits (configurable max size)
- Disabled by default for security
```

### 5.12 Notification System

```go
// Notification types
- Session shared with you
- Session ended
- Permission granted/revoked
- Connection failed
- Security alerts
- System updates

// Delivery
- Real-time via WebSocket
- In-app notification center
- Persistent storage
```

---

## 6. Database Schema

### 6.1 Core Tables (SQLite by default)

```sql
-- Users
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT,
    is_active INTEGER DEFAULT 1,
    is_superuser INTEGER DEFAULT 0,     -- Root/Admin flag (bypasses all permissions)
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Sessions
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    token_hash TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Shared Sessions
CREATE TABLE shared_sessions (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    owner_user_id TEXT REFERENCES users(id),
    shared_with_user_id TEXT REFERENCES users(id),
    permission_level TEXT DEFAULT 'read_only',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Notifications
CREATE TABLE notifications (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    type TEXT NOT NULL,
    message TEXT,
    is_read INTEGER DEFAULT 0,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Identities (Reusable Credentials)
CREATE TABLE identities (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    user_id TEXT REFERENCES users(id),
    username TEXT,
    password_encrypted TEXT,
    private_key_encrypted TEXT,
    passphrase_encrypted TEXT,
    metadata TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Identity Sharing
CREATE TABLE identity_shares (
    id TEXT PRIMARY KEY,
    identity_id TEXT REFERENCES identities(id),
    shared_with_user_id TEXT REFERENCES users(id),
    permission_level TEXT DEFAULT 'read',
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- SSH Keys (Vault)
CREATE TABLE ssh_keys (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    name TEXT NOT NULL,
    key_type TEXT,
    private_key_encrypted TEXT,
    public_key TEXT,
    passphrase_encrypted TEXT,
    fingerprint TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Connections
CREATE TABLE connections (
    id TEXT PRIMARY KEY,
    user_id TEXT REFERENCES users(id),
    identity_id TEXT REFERENCES identities(id),
    protocol TEXT NOT NULL,
    ssh_version TEXT,
    host TEXT NOT NULL,
    port INTEGER,
    name TEXT,
    icon TEXT,
    config TEXT,

    -- Advanced settings (JSON)
    encoding_settings TEXT,
    keyboard_settings TEXT,
    scrolling_settings TEXT,
    reconnect_settings TEXT,

    clipboard_enabled INTEGER DEFAULT 0,
    notes TEXT,
    created_at TEXT DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT CURRENT_TIMESTAMP
);

-- Audit Logs
CREATE TABLE audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT REFERENCES users(id),
    action TEXT NOT NULL,
    resource_type TEXT,
    resource_id TEXT,
    details TEXT,
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

-- Indexes
CREATE INDEX idx_identities_user ON identities(user_id);
CREATE INDEX idx_connections_user ON connections(user_id);
CREATE INDEX idx_connections_identity ON connections(identity_id);
CREATE INDEX idx_ssh_keys_user ON ssh_keys(user_id);
CREATE INDEX idx_identity_shares_identity ON identity_shares(identity_id);
```

---

## 7. Permission System

### 7.1 Modular Permission Architecture

Permissions are organized by module, with core permissions available globally and module-specific permissions only when that module is enabled.

**Permission Structure:**
```go
type Permission struct {
    ID          string
    Name        string   // e.g., "vault.create", "ssh.connect"
    Module      string   // "core", "ssh", "docker", "vault", etc.
    Description string
    DependsOn   []string // Permission dependencies
    Implies     []string // Permissions automatically granted
}
```

### 7.2 Root/Superuser Concept

**Root users bypass all permission checks:**

```go
type User struct {
    ID          string
    Username    string
    Email       string
    IsSuperuser bool  // Root/Admin flag
    IsActive    bool
    // ...
}

// Permission checker
func (pc *PermissionChecker) Check(userID, permission string) bool {
    user := pc.GetUser(userID)

    // Root users have ALL permissions (including future ones)
    if user.IsSuperuser {
        return true  // Bypass all checks
    }

    // Regular permission check for non-root users
    return pc.checkRegularPermission(userID, permission)
}
```

**First User Setup (UI-based):**

The system does NOT create a default admin automatically. Instead:

```go
// internal/api/handlers/setup.go
func (h *SetupHandler) CheckSetupRequired(c *gin.Context) {
    var count int64
    h.db.Model(&User{}).Count(&count)

    c.JSON(200, gin.H{
        "setup_required": count == 0,  // True if no users exist
    })
}

func (h *SetupHandler) CreateFirstUser(c *gin.Context) {
    var count int64
    h.db.Model(&User{}).Count(&count)

    if count > 0 {
        c.JSON(400, gin.H{"error": "Users already exist"})
        return
    }

    var req struct {
        Username string `json:"username" binding:"required,min=3"`
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required,min=8"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // Hash password with bcrypt
    hashedPassword, _ := bcrypt.GenerateFromPassword(
        []byte(req.Password),
        bcrypt.DefaultCost,
    )

    // Create first user as root admin
    admin := &User{
        ID:           uuid.New().String(),
        Username:     req.Username,
        Email:        req.Email,
        PasswordHash: string(hashedPassword),  // Encrypted!
        IsSuperuser:  true,  // First user = ROOT
        IsActive:     true,
    }

    h.db.Create(admin)

    c.JSON(200, gin.H{
        "message": "First admin user created successfully",
        "user": gin.H{
            "id":       admin.ID,
            "username": admin.Username,
            "email":    admin.Email,
        },
    })
}
```

**First Access Flow:**

1. User opens browser → `http://localhost:8080`
2. System detects no users exist
3. Redirects to `/setup` page
4. User fills out form:
   - Username
   - Email
   - Password (min 8 chars)
5. System creates first user with `IsSuperuser=true`
6. Password hashed with bcrypt (NOT stored as plaintext)
7. User automatically logged in
8. Redirected to dashboard

**Root User Capabilities:**
- ✅ Access to ALL permissions (no explicit grants needed)
- ✅ Access to future permissions (when new modules added)
- ✅ Bypass all permission dependency checks
- ✅ Cannot be deleted (system protection)
- ✅ Cannot have `IsSuperuser` flag removed by other users
- ✅ Full access to admin panel, audit logs, system settings
- ✅ Can create other admin users
- ✅ Can manage all organizations, teams, users

**Non-Root Users:**
- ❌ Must have explicit permission grants
- ❌ Subject to permission dependency validation
- ❌ Cannot access admin-only features without grants
- ✅ Can be assigned roles with permission sets
- ✅ Can be promoted to admin by existing admin

### 7.3 Core Permissions

**Core permissions are always available (for non-root users):**

```go
// internal/permissions/core.go
CORE_PERMISSIONS = {
    // User Management
    "user.view": {
        "module": "core",
        "depends_on": [],
    },
    "user.create": {
        "module": "core",
        "depends_on": ["user.view"],
    },
    "user.edit": {
        "module": "core",
        "depends_on": ["user.view"],
    },
    "user.delete": {
        "module": "core",
        "depends_on": ["user.view", "user.edit"],
    },

    // Organization Management
    "org.view": {
        "module": "core",
        "depends_on": [],
    },
    "org.manage": {
        "module": "core",
        "depends_on": ["org.view"],
    },

    // Vault/Credential Management (CORE)
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
        "description": "Share identities with team",
    },
    "vault.use_shared": {
        "module": "core",
        "depends_on": ["vault.view"],
        "description": "Use shared identities",
    },
    "vault.manage_all": {
        "module": "core",
        "depends_on": ["vault.view", "vault.edit", "vault.delete"],
        "description": "Manage all identities (admin)",
    },
}
```

### 7.4 Module-Specific Permissions

**SSH Module Permissions:**
```go
// internal/modules/ssh/permissions.go
SSH_PERMISSIONS = {
    "ssh.connect": {
        "module": "ssh",
        "depends_on": ["vault.view"],  // Can use identities
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
}
```

**Docker Module Permissions:**
```go
// internal/modules/docker/permissions.go
DOCKER_PERMISSIONS = {
    "docker.connect": {
        "module": "docker",
        "depends_on": [],
        "description": "Connect to Docker hosts",
    },
    "docker.container.list": {
        "module": "docker",
        "depends_on": ["docker.connect"],
    },
    "docker.container.exec": {
        "module": "docker",
        "depends_on": ["docker.container.list"],
    },
}
```

**Database Module Permissions:**
```go
// internal/modules/database/permissions.go
DATABASE_PERMISSIONS = {
    "database.connect": {
        "module": "database",
        "depends_on": ["vault.view"],  // Can use stored credentials
        "description": "Connect to databases",
    },
    "database.query.read": {
        "module": "database",
        "depends_on": ["database.connect"],
        "description": "Execute SELECT queries",
    },
    "database.query.write": {
        "module": "database",
        "depends_on": ["database.query.read"],
        "description": "Execute INSERT/UPDATE/DELETE",
    },
}
```

### 7.5 Permission Registration

**Each module registers its permissions on initialization:**
```go
// internal/modules/ssh/permissions.go
package ssh

import "github.com/your-org/shellcn/internal/permissions"

func RegisterPermissions() {
    permissions.Register(SSH_PERMISSIONS)
}

// internal/app/app.go
func (app *App) InitializeModules() {
    // Core permissions always registered
    core.RegisterPermissions()
    vault.RegisterPermissions()

    // Module permissions registered conditionally
    if app.Config.Modules.SSH.Enabled {
        ssh.RegisterPermissions()
    }
    if app.Config.Modules.Docker.Enabled {
        docker.RegisterPermissions()
    }
    // ... etc
}
```

---

## 8. Configuration

```yaml
# config.yaml
server:
  port: 8080

database:
  driver: sqlite  # sqlite, postgres, mysql
  sqlite:
    path: ./data/database.sqlite
  postgres:  # Optional
    enabled: false
    host: localhost
    port: 5432
    database: shellcn
    username: postgres
    password: ${DB_PASSWORD}
  mysql:  # Optional
    enabled: false
    host: localhost
    port: 3306
    database: shellcn
    username: root
    password: ${DB_PASSWORD}

vault:
  encryption_key: ${VAULT_ENCRYPTION_KEY}
  algorithm: aes-256-gcm
  key_rotation_days: 90

monitoring:
  prometheus:
    enabled: true
    endpoint: /metrics
  health_check:
    enabled: true

features:
  session_sharing:
    enabled: true
    max_shared_users: 5
  clipboard_sync:
    enabled: true
    max_size_kb: 1024
  notifications:
    enabled: true

modules:
  ssh:
    enabled: true
    default_port: 22
    ssh_v1_enabled: false  # Disabled by default (security)
    ssh_v2_enabled: true
    auto_reconnect: true
    max_reconnect_attempts: 3
    keepalive_interval: 60
  telnet:
    enabled: true
    default_port: 23
    auto_reconnect: true
  rdp:
    enabled: true
    default_port: 3389
  vnc:
    enabled: true
    default_port: 5900
  docker:
    enabled: true
  database:
    enabled: true
    mysql: true
    postgres: true
    redis: true
```

---

## 9. Deployment Model

### 9.1 Single Binary Deployment

**Build Process:**

**Prerequisites:**
- ⚠️ **IMPORTANT**: Check for latest library versions before building!
  - Rust crates: https://crates.io
  - Go packages: https://pkg.go.dev
  - pnpm packages: https://npmjs.com

1. **Build Rust FFI modules** → Static libraries (.a files)
   ```bash
   cd rust-modules/rdp
   cargo build --release  # Generates rdp_ffi.h + librdp_ffi.a

   cd ../vnc
   cargo build --release  # Generates vnc_ffi.h + libvnc_ffi.a
   ```

2. **Build frontend** (Vite 7 + React 19 + Tailwind 4) → Static assets
   ```bash
   cd web
   pnpm install
   pnpm run build  # Output: dist/
   ```

3. **Embed frontend in Go binary**
   ```go
   //go:embed web/dist/*
   var staticFiles embed.FS
   ```

4. **Link Rust static libraries with Go (CGO)**
   ```bash
   CGO_ENABLED=1 go build -o shellcn ./cmd/server
   ```

5. **Output**: Single executable (shellcn)

**First Run:**
```bash
./shellcn

# Output:
# ShellCN Platform v1.0.0
# ✓ Created data directory: ./data
# ✓ Initialized SQLite database: ./data/database.sqlite
# ✓ Server started on http://localhost:8080
#
# → Open http://localhost:8080 to create your first admin user
#
# ✓ Metrics: http://localhost:8080/metrics
# ✓ Health: http://localhost:8080/health
```

**First Access (Browser):**
1. Navigate to `http://localhost:8080`
2. Auto-redirected to `/setup` (no users exist)
3. Fill out first admin user form:
   - Username: `your-username`
   - Email: `your-email@example.com`
   - Password: `your-secure-password` (min 8 chars)
4. Click "Create Admin User"
5. Password encrypted with bcrypt before storage
6. User created with root/superuser privileges
7. Auto-login and redirect to dashboard

**Directory Structure:**
```
./shellcn                 # Single binary
./data/                   # Auto-created
  ├── database.sqlite
  └── recordings/
```

---

## 10. UI Routing & Page Structure

### 10.1 Application Routes

```
/                               # Dashboard (all connections overview)
/setup                          # First user setup (only if no users exist)
/login                          # Login page

# Core - Connections & Sessions
/connections
  /                             # All connections list
  /new                          # New connection (with identity selector)
  /:id                          # Connection details
  /:id/edit                     # Edit connection

# Terminal Protocols (SSH, Telnet)
/ssh
  /connections                  # SSH connection list
  /connections/new              # New SSH connection
  /connections/:id/terminal     # SSH terminal session
  /connections/:id/sftp         # SFTP file manager

/telnet
  /connections                  # Telnet connection list
  /connections/new              # New Telnet connection
  /connections/:id/terminal     # Telnet terminal session

# Remote Desktop (RDP, VNC)
/rdp
  /connections                  # RDP connection list
  /connections/new              # New RDP connection
  /connections/:id/desktop      # RDP remote desktop session

/vnc
  /connections                  # VNC connection list
  /connections/new              # New VNC connection
  /connections/:id/desktop      # VNC remote desktop session

# Container Management
/docker
  /hosts                        # Docker host connection list
  /hosts/new                    # Add new Docker host
  /hosts/:id
    /containers                 # Container list
    /containers/:cid/exec       # Execute in container
    /containers/:cid/logs       # Container logs
    /images                     # Image list
    /volumes                    # Volume list
    /networks                   # Network list

/kubernetes
  /clusters                     # K8s cluster connection list
  /clusters/new                 # Add new K8s cluster
  /clusters/:id
    /pods                       # Pod list
    /pods/:pid/exec             # Execute in pod
    /pods/:pid/logs             # Pod logs
    /deployments                # Deployment list
    /services                   # Service list
    /configmaps                 # ConfigMap list
    /secrets                    # Secret list
    /portforward                # Port forwarding

# Virtual Machines
/proxmox
  /hosts                        # Proxmox host connection list
  /hosts/new                    # Add new Proxmox host
  /hosts/:id
    /vms                        # Virtual machine list
    /vms/:vid/console           # VM console
    /containers                 # LXC container list
    /storage                    # Storage list

# Databases
/databases
  /connections                  # Database connection list
  /connections/new              # New database connection (with identity)
  /connections/:id
    /query                      # Query editor
    /tables                     # Table browser
    /schema                     # Schema viewer
  /mysql
    /connections                # MySQL-specific connections
  /postgres
    /connections                # PostgreSQL-specific connections
  /mongodb
    /connections                # MongoDB-specific connections
  /redis
    /connections                # Redis-specific connections

# File Sharing
/fileshare
  /connections                  # File share connection list
  /connections/new              # New file share connection
  /smb
    /connections/:id/browser    # SMB file browser
  /nfs
    /connections/:id/browser    # NFS file browser
  /ftp
    /connections/:id/browser    # FTP file browser
  /s3
    /buckets                    # S3 bucket list
    /buckets/:id/browser        # S3 file browser

# Settings & Administration
/settings
  /identities                   # Credential Vault (Manage Identities)
    /                           # List all identities
    /new                        # Create new identity
    /:id/edit                   # Edit identity
  /ssh-keys                     # SSH Key Management
  /profile                      # User profile
  /security                     # Security settings (MFA, sessions)
  /organizations                # Organization management
    /                           # Organization list
    /:id/teams                  # Team management
    /:id/members                # Member management
  /notifications                # Notification preferences

# Session Management
/sessions
  /active                       # Active sessions list
  /:id                          # Session details
  /:id/share                    # Share session dialog
  /recordings                   # Session recordings

# Administration (Admin Only)
/admin
  /users                        # User management
  /users/new                    # Create user
  /users/:id/edit               # Edit user
  /roles                        # Role management
  /permissions                  # Permission management
  /audit                        # Audit logs
  /monitoring                   # System monitoring (Prometheus)
  /health                       # Health check status
```

### 10.2 Key UI Components

**1. Connection Creation Forms (Protocol-Specific):**

**SSH Connection Form:**
- Name, Icon
- Protocol (SSH v1, SSH v2, Auto)
- Address, Port
- **Identity Selector** (saved identities + "Custom Identity")
- Authentication method (Password, Public Key, Keyboard-Interactive)
- Username/Password (if custom)
- SSH key selection (if public key auth)
- Basic Tab: Connection info
- Advanced Tab: Encoding, Keyboard, Scrolling, Reconnection
- Notes (rich text)

**Telnet Connection Form:**
- Name, Icon, Address, Port
- Identity Selector (optional)
- Encoding settings
- Advanced settings (keyboard, scrolling, reconnection)

**RDP Connection Form:**
- Name, Icon, Address, Port
- Identity Selector
- Domain (optional)
- Screen resolution settings
- Color depth
- Audio redirection
- Clipboard sharing

**VNC Connection Form:**
- Name, Icon, Address, Port
- Password (VNC password)
- Color quality
- Compression settings

**Docker Host Form:**
- Name, Icon
- Connection type (TCP, Unix Socket, SSH tunnel)
- Host address (if TCP)
- TLS settings
- Identity (if SSH tunnel)

**Kubernetes Cluster Form:**
- Name, Icon
- API Server URL
- Authentication (kubeconfig, token, certificate)
- Namespace selection
- Context selection

**Database Connection Form:**
- Name, Icon
- Database type (MySQL, PostgreSQL, MongoDB, Redis)
- Host, Port
- **Identity Selector** (saved credentials)
- Database name
- SSL/TLS settings
- Connection pool settings

**Proxmox Host Form:**
- Name, Icon
- Host URL
- Identity (username/password or API token)
- Node selection
- Realm (PAM, PVE, etc.)

**File Share Connection Form:**
- Name, Icon
- Protocol (SMB, NFS, FTP, S3, WebDAV)
- Host, Port
- Identity Selector
- Share/Bucket name
- Mount options (NFS)
- Encryption (if supported)

**2. Identity/Vault Management:**

**Identity List Table:**
- Columns: Name, Type, Shared With, Created, Actions
- Filter: Own / Shared / All
- Actions: Edit, Delete, Share, View Usage
- Bulk actions: Delete selected, Share selected

**Create/Edit Identity Form:**
- Name
- Type (SSH, Database, Generic)
- Username
- Password
- SSH Private Key (upload or paste)
- SSH Key Passphrase
- Notes
- Share with users/teams

**SSH Key Manager:**
- List of uploaded SSH keys
- Key type, Fingerprint
- Upload new key
- Generate new key pair
- Delete key
- Export public key

**3. Session Components:**

**Terminal Emulator (xterm.js):**
- Terminal window with xterm.js
- Toolbar: Font size, Search, Clear, Download logs
- Status bar: Connection status, Latency
- Session sharing button
- Clipboard sync toggle

**RDP/VNC Viewer:**
- Canvas-based remote desktop viewer
- Toolbar: Fullscreen, Ctrl+Alt+Del, Screenshot
- Keyboard/Mouse input handling
- Clipboard sync

**File Manager (SFTP/File Share):**
- Dual-pane file browser (local/remote)
- Upload/Download with progress
- File operations: Copy, Move, Delete, Rename
- Context menu
- Breadcrumb navigation

**4. Session Sharing Dialog:**
- User/Team selector
- Permission level: Read-only / Interactive
- Expiry time
- Active shared sessions list
- Revoke access button

**5. Database Query Editor:**
- SQL editor with syntax highlighting
- Execute button
- Result table viewer
- Query history
- Save query
- Export results (CSV, JSON)

**6. Docker/Kubernetes Management:**

**Container List:**
- Table: Name, Image, Status, Ports, CPU, Memory
- Actions: Start, Stop, Restart, Delete, Exec, Logs
- Filters: Running, Stopped, All

**Pod List (K8s):**
- Table: Name, Namespace, Status, Restarts, Age
- Actions: Exec, Logs, Port Forward, Delete
- Namespace selector

**7. Permission-Based UI:**
- UI elements hidden/disabled based on user permissions
- Real-time permission validation
- Tooltip explanations for disabled features
- Error messages for denied actions
- Permission requirement indicators

**8. Notification Center:**
- Notification bell icon with badge
- Dropdown list of recent notifications
- Mark as read/unread
- Clear all
- Navigate to related resource

---

## 11. Security Checklist

- ✅ **All passwords hashed with bcrypt** (never stored as plaintext)
- ✅ **User credentials encrypted in vault** (AES-256-GCM)
- ✅ **SSH keys encrypted at rest** (AES-256-GCM)
- ✅ **Master key derived from app secret** (Argon2 KDF)
- ✅ **Database credentials encrypted** (stored in vault)
- ✅ **Zero-knowledge encryption option** (user passphrase)
- ✅ JWT tokens with short expiry
- ✅ HTTPS only in production
- ✅ Rate limiting on all endpoints
- ✅ Permission checks on every operation
- ✅ Session recording for audit
- ✅ Clipboard sync permission-based
- ✅ Session sharing with explicit permissions
- ✅ Prometheus metrics without sensitive data
- ✅ SSH v1 disabled by default (security risk)
- ✅ **First user created via UI** (no default credentials)

---

## 12. License

MIT License

---

**End of Specification**
