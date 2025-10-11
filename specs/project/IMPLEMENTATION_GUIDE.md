# ShellCN Platform - Implementation Guide

This guide provides a step-by-step roadmap for implementing the ShellCN platform, including module order, implementation priorities, and practical steps.

---

## Table of Contents

1. [Implementation Philosophy](#implementation-philosophy)
2. [Implementation Phases Overview](#implementation-phases-overview)
3. [Phase 1: Foundation (Weeks 1-3)](#phase-1-foundation-weeks-1-3)
4. [Phase 2: First Protocol - SSH (Weeks 4-5)](#phase-2-first-protocol---ssh-weeks-4-5)
5. [Phase 3: Terminal Protocols (Week 6)](#phase-3-terminal-protocols-week-6)
6. [Phase 4: Rust FFI Modules (Weeks 7-8)](#phase-4-rust-ffi-modules-weeks-7-8)
7. [Phase 5: Container & Cloud (Weeks 9-11)](#phase-5-container--cloud-weeks-9-11)
8. [Phase 6: Databases (Weeks 12-14)](#phase-6-databases-weeks-12-14)
9. [Phase 7: Advanced Modules (Weeks 15+)](#phase-7-advanced-modules-weeks-15)
10. [Week 1 Detailed Breakdown](#week-1-detailed-breakdown)
11. [Success Metrics](#success-metrics)
12. [Best Practices](#best-practices)

---

## Implementation Philosophy

**Build in order of dependency, not complexity.**

The implementation order is designed to:
- ✅ Establish core patterns early
- ✅ Validate architecture decisions quickly
- ✅ Maximize code reuse
- ✅ Deliver value incrementally
- ✅ Test integration points early

**Key Principle:** Once SSH module works end-to-end, you have proven:
- Authentication flow
- Permission system
- Vault integration
- Terminal UI pattern
- WebSocket communication

All subsequent modules will follow these established patterns.

---

## Implementation Phases Overview

```
Phase 1: Foundation (Weeks 1-3)
├── Core Module (Auth, Users, Permissions)
├── Vault Module (Credentials, Encryption)
└── Monitoring Module (Metrics, Health)

Phase 2: First Protocol (Weeks 4-5)
└── SSH Module (validates all patterns)

Phase 3: Terminal Protocols (Week 6)
├── Telnet Module
└── SFTP Module

Phase 4: Rust FFI (Weeks 7-8)
├── RDP Module
└── VNC Module

Phase 5: Container & Cloud (Weeks 9-11)
├── Docker Module
└── Kubernetes Module

Phase 6: Databases (Weeks 12-14)
├── MySQL
├── PostgreSQL
├── Redis
└── MongoDB

Phase 7: Advanced (Weeks 15+)
├── Proxmox Module
└── File Share Module
```

---

## Phase 1: Foundation (Weeks 1-3)

### Week 1: Core Module

**Priority: Critical - Everything depends on this**

#### What to Build:
1. **Project Setup**
   - Initialize Go module
   - Create directory structure
   - Initialize frontend (React + Vite + TypeScript)
   - Install dependencies

2. **Database Layer**
   - Database connection (SQLite/PostgreSQL/MySQL)
   - User model
   - Permission model
   - Auto-migration setup

3. **Authentication System**
   - JWT service
   - Password hashing (bcrypt)
   - Login/logout handlers
   - JWT middleware

4. **Permission Registry**
   - Global permission registry
   - `Register()` function for modules
   - `Check()` function with dependency resolution
   - `ValidateDependencies()` on startup

5. **First-Time Setup**
   - Setup status endpoint
   - Create first user (as root)
   - Setup UI page
   - Login UI page

**Deliverables:**
- ✅ Can create first user via `/setup`
- ✅ Can login and receive JWT token
- ✅ Permission system validates dependencies

---

### Week 2: Vault Module

**Priority: Critical - All protocol modules need this**

#### What to Build:
1. **Encryption Service**
   - AES-256-GCM encryption
   - Master key from environment variable
   - Argon2id key derivation

2. **Identity Management**
   - Create/Read/Update/Delete identities
   - Identity types (SSH, Database, Generic)
   - Encrypted credential storage

3. **SSH Key Management**
   - Upload SSH private keys
   - Generate new SSH key pairs
   - Support RSA, ECDSA, Ed25519
   - Passphrase-protected keys

4. **Vault Permissions**
   - Register all vault permissions in `init()`
   - `vault.view`, `vault.create`, `vault.edit`, etc.

5. **Vault UI**
   - Identity list page (`/settings/identities`)
   - Create/Edit identity forms
   - Identity selector component (reusable)
   - SSH key upload/generation

**Deliverables:**
- ✅ Can store encrypted credentials
- ✅ Can upload/generate SSH keys
- ✅ Identity selector works in forms

---

### Week 3: Monitoring & Preparation

**Priority: Important - Needed for production**

#### What to Build:
1. **Prometheus Metrics**
   - HTTP request metrics
   - Connection metrics by protocol
   - Authentication attempt metrics
   - Permission check metrics

2. **Health Checks**
   - `/health` - Simple up/down
   - `/health/ready` - Ready to serve
   - `/health/live` - Liveness probe

3. **Audit Logging**
   - User action logging
   - Resource access logging
   - Authentication attempts
   - Permission denials

**Deliverables:**
- ✅ Metrics exposed at `/metrics`
- ✅ Health checks working
- ✅ Audit logs stored in database

---

## Phase 2: First Protocol - SSH (Weeks 4-5)

### Why SSH First?

- ✅ No external dependencies (uses Go's `crypto/ssh`)
- ✅ Tests vault integration
- ✅ Establishes terminal UI patterns
- ✅ Validates WebSocket communication
- ✅ Most familiar protocol

### Week 4: SSH Backend

#### What to Build:
1. **SSH Client Wrapper**
   - SSH connection with password/key auth
   - PTY request
   - Shell execution
   - Session management

2. **WebSocket Handler**
   - Upgrade HTTP to WebSocket
   - Pipe WebSocket ↔ SSH
   - Handle resize events
   - Connection lifecycle

3. **Auto-Reconnection**
   - Detect connection drops
   - Exponential backoff retry
   - State restoration
   - User notifications

4. **SSH Permissions**
   - Register permissions in `init()`
   - `ssh.connect`, `ssh.execute`, `ssh.session.share`, etc.

**Deliverables:**
- ✅ SSH client connects successfully
- ✅ WebSocket communication works
- ✅ Auto-reconnection tested

---

### Week 5: SSH Frontend

#### What to Build:
1. **Terminal Component**
   - xterm.js integration
   - FitAddon for responsive sizing
   - User preferences (font, theme, cursor)
   - WebSocket connection

2. **Connection Management UI**
   - Connection list page
   - Create connection form
   - Identity selector integration
   - Connection settings

3. **Session Features**
   - Session recording (optional)
   - Session sharing (optional)
   - Clipboard sync (optional)

**Deliverables:**
- ✅ Can connect to SSH server via UI
- ✅ Terminal works with xterm.js
- ✅ User preferences applied
- ✅ Credentials from vault work

---

## Phase 3: Terminal Protocols (Week 6)

### Telnet Module

**Reuses SSH patterns:**
- Terminal component (same UI)
- WebSocket handler (similar pattern)
- Simpler authentication (no keys)

#### What to Build:
1. Telnet client wrapper
2. WebSocket handler (copy from SSH)
3. Telnet permissions registration
4. Connection UI (reuse SSH UI)

**Deliverables:**
- ✅ Telnet connections work
- ✅ Terminal UI reused successfully

---

### SFTP Module

**Extends SSH module:**

#### What to Build:
1. SFTP client wrapper
2. File operations (list, upload, download, delete, chmod)
3. Dual-pane file manager UI
4. Drag & drop upload
5. Progress tracking
6. SFTP permissions registration

**Deliverables:**
- ✅ Can browse remote files
- ✅ Upload/download works
- ✅ File manager UI complete

---

## Phase 4: Rust FFI Modules (Weeks 7-8)

### Week 7: RDP Module

**Setup Rust FFI:**

#### What to Build:
1. **Rust FFI Setup**
   - Create `rust-modules/rdp/` directory
   - Configure Cargo.toml with ironrdp
   - Setup cbindgen for C bindings
   - Build script to generate headers

2. **Go CGO Integration**
   - Import generated C headers
   - Create Go wrapper functions
   - Handle memory management

3. **RDP Session Handler**
   - WebSocket handler for RDP
   - Canvas-based desktop viewer
   - Mouse/keyboard events

4. **RDP Permissions**
   - Register permissions in `init()`

**Deliverables:**
- ✅ Rust FFI builds successfully
- ✅ RDP connections work
- ✅ Desktop viewer functional

---

### Week 8: VNC Module

**Reuse RDP patterns:**

#### What to Build:
1. Same FFI setup with vnc-rs
2. Go CGO wrapper
3. VNC session handler
4. Reuse desktop viewer UI
5. VNC permissions registration

**Deliverables:**
- ✅ VNC connections work
- ✅ Desktop viewer reused

---

## Phase 5: Container & Cloud (Weeks 9-11)

### Week 9-10: Docker Module

#### What to Build:
1. **Docker SDK Integration**
   - Docker client wrapper
   - Connection management (TCP, socket, SSH tunnel)

2. **Container Management**
   - List, create, start, stop, delete containers
   - Container logs (real-time)
   - Container stats (CPU, memory, network)
   - Container exec (reuse terminal UI!)

3. **Image/Volume/Network Management**
   - List, pull, push, delete images
   - Volume operations
   - Network operations

4. **Docker Permissions**
   - Register 30+ permissions in `init()`

**Deliverables:**
- ✅ Docker connections work
- ✅ Container management functional
- ✅ Container exec reuses terminal

---

### Week 11: Kubernetes Module

#### What to Build:
1. **K8s Client Integration**
   - client-go setup
   - kubeconfig support
   - Context/namespace management

2. **Pod Management**
   - List, view, delete pods
   - Pod logs (real-time)
   - Pod exec (reuse terminal UI!)
   - Pod metrics

3. **Workload Resources**
   - Deployments (list, scale, rollback)
   - StatefulSets, DaemonSets
   - Jobs, CronJobs

4. **Configuration & Storage**
   - ConfigMaps, Secrets
   - PV/PVC, StorageClass

5. **YAML Editor**
   - Monaco editor integration
   - Syntax validation
   - Apply changes

6. **K8s Permissions**
   - Register 40+ permissions in `init()`

**Deliverables:**
- ✅ K8s connections work
- ✅ Pod management functional
- ✅ YAML editor works

---

## Phase 6: Databases (Weeks 12-14)

### Implementation Order

1. **MySQL** (most common)
2. **PostgreSQL** (similar to MySQL)
3. **Redis** (simpler, different pattern)
4. **MongoDB** (most different)

### What to Build (for each database):

#### MySQL/PostgreSQL:
1. Database client wrapper
2. SQL query editor (Monaco editor)
3. Table browser with pagination
4. Result table viewer
5. Import/export (CSV, JSON, SQL)
6. User management
7. Server management

#### Redis:
1. Redis client wrapper
2. Key browser
3. Data type operations (String, Hash, List, Set, etc.)
4. Redis CLI
5. Pub/Sub monitoring

#### MongoDB:
1. MongoDB client wrapper
2. Collection browser
3. Document editor (JSON)
4. Aggregation pipeline builder (visual)
5. Import/export (JSON, CSV)

### Database Permissions:
- Register 25+ permissions in `init()`

**Deliverables:**
- ✅ All 4 databases work
- ✅ Query editors functional
- ✅ Import/export works

---

## Phase 7: Advanced Modules (Weeks 15+)

### Proxmox Module (Optional)

#### What to Build:
1. Proxmox API client
2. VM management (list, start, stop, delete)
3. LXC container management
4. noVNC integration
5. Storage management

---

### File Share Module (Optional)

#### What to Build:
1. **Multiple Protocol Support**
   - SMB/CIFS client
   - NFS client
   - FTP/FTPS client
   - S3 client
   - WebDAV client

2. **File Manager**
   - Reuse SFTP file manager UI
   - Protocol-specific operations

---

## Week 1 Detailed Breakdown

### Day 1: Monday - Project Initialization

**Morning (4 hours):**
- Create project structure
- Initialize Go module
- Create all directories
- Initialize frontend (React + Vite)

**Afternoon (4 hours):**
- Install Go dependencies (gin, gorm, jwt-go, etc.)
- Install frontend dependencies (React Query, Zustand, etc.)
- Setup Tailwind CSS
- Create `.env` and `.env.example`

**Deliverable:** Project structure ready

---

### Day 2: Tuesday - Database Layer

**Morning:**
- `internal/database/db.go` - Database connection
- `internal/models/user.go` - User model
- `internal/models/permission.go` - Permission model

**Afternoon:**
- `internal/database/repositories/user_repository.go`
- Auto-migration setup
- Test database connection

**Deliverable:** Database layer working

---

### Day 3: Wednesday - Authentication

**Morning:**
- `internal/auth/jwt.go` - JWT service
- `internal/auth/password.go` - Password hashing

**Afternoon:**
- `internal/api/handlers/auth.go` - Login/logout
- `internal/api/middleware/auth.go` - JWT middleware
- Test authentication flow

**Deliverable:** Authentication working

---

### Day 4: Thursday - Permission System

**Morning:**
- `internal/permissions/registry.go` - Permission registry
- `internal/permissions/core.go` - Core permissions registration

**Afternoon:**
- `internal/permissions/checker.go` - Permission checker
- `internal/api/middleware/permission.go` - Permission middleware
- Test permission checks

**Deliverable:** Permission system working

---

### Day 5: Friday - First-Time Setup

**Morning:**
- `internal/api/handlers/setup.go` - Setup handler
- Test creating first user via API

**Afternoon:**
- `web/src/pages/Setup.tsx` - Setup UI
- `web/src/pages/Login.tsx` - Login UI
- End-to-end test

**Deliverable:** Can create first user and login

---

### Weekend: Integration & Testing

- End-to-end test: Setup → Login → Dashboard
- Fix any integration issues
- Prepare for Week 2 (Vault module)

---

## Success Metrics

### After Week 1:
- ✅ Can create first user via `/setup`
- ✅ Can login and receive JWT token
- ✅ Permission system validates dependencies
- ✅ All core permissions registered

### After Week 5 (SSH Complete):
- ✅ Can connect to SSH server
- ✅ Terminal works with xterm.js
- ✅ WebSocket communication stable
- ✅ Credentials stored in vault
- ✅ User preferences applied to terminal

### After Week 11 (Container Modules):
- ✅ Docker module works
- ✅ Kubernetes module works
- ✅ Container exec reuses terminal UI
- ✅ YAML editor functional

### After Week 14 (Core Modules Complete):
- ✅ All default modules working
- ✅ Permission system tested across modules
- ✅ Vault integration proven
- ✅ UI patterns established
- ✅ Ready for optional modules

---

## Best Practices

### Development Practices:
1. **Test as you build** - Don't wait for Phase 7 to test Phase 1
2. **Document permissions** - Every new module must register permissions in `init()`
3. **Validate dependencies** - Run `permissions.ValidateDependencies()` on startup
4. **Reuse UI components** - Terminal, file browser, forms should be reusable

### Security Practices:
5. **User preferences first** - Never hardcode terminal/UI settings
6. **Root bypass** - Always check `isRoot` before permission checks
7. **Audit everything** - Log all sensitive operations
8. **Encrypt credentials** - Use AES-256-GCM for vault storage

### Code Practices:
9. **Module isolation** - Each module in separate directory
10. **Permission registration** - Each module has `permissions.go` with `init()`
11. **Consistent patterns** - Follow SSH module patterns for other protocols
12. **Import modules** - Import modules in `app.go` to trigger `init()`

---

## Module Priority Summary

**Must Build First (Critical Path):**
1. Core Module (Week 1)
2. Vault Module (Week 2)
3. SSH Module (Week 4-5) ← **Start here for first protocol**

**Then Build (High Value):**
4. Docker Module (Week 9-10)
5. Kubernetes Module (Week 11)
6. Database Modules (Week 12-14)

**Finally Build (Optional):**
7. Telnet, SFTP (Week 6)
8. RDP, VNC (Week 7-8)
9. Proxmox, File Share (Week 15+)

---

## Quick Start Commands

```bash
# Week 1, Day 1
mkdir -p shellcn/{internal,web,rust-modules,docs}
cd shellcn
go mod init shellcn

mkdir -p internal/{app,api/{handlers,middleware},auth,permissions,models,database/{repositories},vault,monitoring}
mkdir -p internal/modules/{ssh,telnet,rdp,vnc,docker,kubernetes,database,proxmox,fileshare}
mkdir -p web/src/{pages,components/{terminal,file-manager,vault},hooks,lib/{api,stores}}

cd web
pnpm create vite@latest . --template react-ts
pnpm install
pnpm add @tanstack/react-query zustand react-hook-form zod tailwindcss xterm xterm-addon-fit

# Install Go dependencies
cd ..
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/sqlite
go get github.com/golang-jwt/jwt/v5
go get golang.org/x/crypto/bcrypt
go get golang.org/x/crypto/ssh
go get github.com/prometheus/client_golang/prometheus
```

---

**End of Implementation Guide**

**Next Steps:**
1. Review [MODULE_IMPLEMENTATION.md](MODULE_IMPLEMENTATION.md) for detailed module specifications and code examples
2. Review [project_spec.md](project_spec.md) for complete technical specifications
3. Start with Week 1, Day 1 - Project Initialization
4. Follow the critical path: Core → Vault → SSH
