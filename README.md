# ShellCN

A comprehensive web-based remote client platform for managing enterprise infrastructure access. ShellCN provides secure client connections to external services including SSH servers, Docker hosts, Kubernetes clusters, databases, and other remote infrastructure.

## Features

### Protocol Support

- **SSH/SFTP** - Remote server access with file transfer
- **Telnet** - Legacy device management
- **RDP** - Windows desktop access (via Rust FFI)
- **VNC** - Cross-platform remote desktop (via Rust FFI)
- **Docker** - Container management client
- **Kubernetes** - K8s cluster management
- **Database Clients** - MySQL, PostgreSQL, MongoDB, Redis

### Core Features

- 🔐 **Secure Credential Vault** - AES-256-GCM encrypted credential storage
- 👥 **Multi-User & Organizations** - Team-based access control
- 🔑 **Role-Based Permissions** - Granular permission system with dependencies
- 📊 **Monitoring & Audit** - Prometheus metrics and comprehensive audit logging
- 🖥️ **Terminal Interface** - xterm.js-based terminal with user preferences
- 🎨 **Modern UI** - React 19 + Vite 7 + Tailwind CSS v4

## Tech Stack

### Backend

- **Language:** Go 1.21+
- **Framework:** Gin
- **ORM:** GORM
- **Database:** SQLite (default), PostgreSQL, MySQL
- **Authentication:** JWT
- **Rust FFI:** IronRDP, vnc-rs (static linking via CGO)

### Frontend

- **Framework:** React 19
- **Build Tool:** Vite 7
- **Language:** TypeScript
- **State Management:** TanStack Query + Zustand
- **Forms:** react-hook-form + Zod
- **Styling:** Tailwind CSS v4
- **Terminal:** xterm.js

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Node.js 18+ and PNPM
- Rust (for RDP/VNC modules)

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/shellcn.git
cd shellcn

# Install backend dependencies
go mod download

# Install frontend dependencies
cd web
pnpm install

# Build the application
cd ..
make build

# Run the application
./bin/shellcn
```

### First-Time Setup

1. Navigate to `http://localhost:8080/setup`
2. Create the first admin user
3. Login with your credentials
4. Start adding connections!

## Configuration

ShellCN reads configuration from a YAML file (default: `config/config.yaml`) and environment variables using the `SHELLCN_` prefix. Environment variables override file values automatically.

### Sample `config/config.yaml`

```yaml
server:
  port: 8080

database:
  driver: sqlite # sqlite | postgres | mysql
  path: ./data/shellcn.db # used when driver is sqlite
  postgres:
    enabled: false
    host: localhost
    port: 5432
    database: shellcn
    username: postgres
    password: ${DB_PASSWORD}
  mysql:
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

auth:
  jwt:
    secret: ${JWT_SECRET}
    issuer: shellcn.local
    access_token_ttl: 15m
  session:
    refresh_token_ttl: 720h # 30 days
    refresh_token_length: 48
  local:
    lockout_threshold: 5
    lockout_duration: 15m

modules:
  ssh:
    enabled: true
    default_port: 22
    ssh_v1_enabled: false
    ssh_v2_enabled: true
    auto_reconnect: true
    max_reconnect_attempts: 3
    keepalive_interval: 60
  telnet:
    enabled: true
    default_port: 23
    auto_reconnect: true
  sftp:
    enabled: true
    default_port: 22
  rdp:
    enabled: true
    default_port: 3389
  vnc:
    enabled: true
    default_port: 5900
  docker:
    enabled: true
  kubernetes:
    enabled: false
  database:
    enabled: true
    mysql: true
    postgres: true
    redis: true
    mongodb: true
  proxmox:
    enabled: false
  file_share:
    enabled: false
```

### Environment Overrides

Every field can be overridden via environment variables by replacing dots with underscores and prefixing with `SHELLCN_`. For example:

```bash
export SHELLCN_SERVER_PORT=9090
export SHELLCN_AUTH_JWT_SECRET="dev-secret"
export SHELLCN_DATABASE_DRIVER=postgres
export SHELLCN_DATABASE_POSTGRES_ENABLED=true
export SHELLCN_DATABASE_POSTGRES_PASSWORD="p@ssw0rd"
```

This makes it easy to adapt configuration per environment (development, CI, production) without changing the YAML file.

## Development

### Project Structure

```
shellcn/
├── internal/           # Backend Go code
│   ├── api/           # HTTP handlers & middleware
│   ├── auth/          # Authentication & JWT
│   ├── permissions/   # Permission system
│   ├── vault/         # Credential encryption
│   └── modules/       # Protocol client modules
├── web/               # Frontend React app
│   └── src/
│       ├── pages/     # Page components
│       ├── components/# Reusable components
│       └── hooks/     # Custom React hooks
├── rust-modules/      # Rust FFI modules
│   ├── rdp/          # RDP client (IronRDP)
│   └── vnc/          # VNC client (vnc-rs)
└── docs/              # Documentation
```

### Running in Development

```bash
# Backend (with hot reload)
go run cmd/server/main.go

# Frontend (with Vite dev server)
cd web
pnpm dev
```

### Building

```bash
# Build everything
make build

# Build backend only
make build-backend

# Build frontend only
make build-frontend

# Build Rust FFI modules
make build-rust
```

## Documentation

- [Implementation Guide](IMPLEMENTATION_GUIDE.md) - Step-by-step implementation roadmap
- [Module Implementation](MODULE_IMPLEMENTATION.md) - Detailed module specifications
- [Project Specification](project_spec.md) - Complete technical specifications
- [Frontend Guidelines](specs/project/FRONTEND_GUIDELINES.md) - Frontend development guidelines

## Architecture

ShellCN uses a modular permission-based architecture:

1. **Core Module** - Authentication, users, organizations, permissions
2. **Vault Module** - Encrypted credential storage (AES-256-GCM)
3. **Protocol Modules** - SSH, Telnet, RDP, VNC, Docker, K8s, Databases
4. **Monitoring Module** - Prometheus metrics and health checks

Each module registers its permissions at startup using Go's `init()` pattern, enabling dynamic permission checking with dependency resolution.

## Security

- 🔒 **Encryption:** AES-256-GCM for credential storage
- 🔑 **Key Derivation:** Argon2id for password hashing
- 🛡️ **Authentication:** JWT-based with bcrypt password hashing
- 📝 **Audit Logging:** Comprehensive audit trail of all operations
- 👤 **RBAC:** Role-based access control with permission dependencies

## Contributing

Contributions are welcome! Please read our [Contributing Guidelines](CONTRIBUTING.md) first.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Roadmap

- [x] Core authentication & permission system
- [x] Vault credential management
- [ ] SSH/SFTP module
- [ ] Docker module
- [ ] Kubernetes module
- [ ] Database modules (MySQL, PostgreSQL, Redis, MongoDB)
- [ ] RDP module (Rust FFI)
- [ ] VNC module (Rust FFI)
- [ ] Telnet module
- [ ] Proxmox module
- [ ] File share module (SMB, NFS, S3)

## Support

- 📖 [Documentation](https://github.com/yourusername/shellcn/wiki)
- 🐛 [Issue Tracker](https://github.com/yourusername/shellcn/issues)
- 💬 [Discussions](https://github.com/yourusername/shellcn/discussions)

## Acknowledgments

- [xterm.js](https://xtermjs.org/) - Terminal emulator
- [IronRDP](https://github.com/Devolutions/IronRDP) - RDP protocol implementation
- [vnc-rs](https://github.com/whitequark/rust-vnc) - VNC protocol implementation

---

**Built with ❤️ for the DevOps community**
