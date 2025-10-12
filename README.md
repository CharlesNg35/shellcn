# ShellCN

A web-based platform for managing remote infrastructure access. Connect to SSH servers, Docker hosts, Kubernetes clusters, databases, and object storage from a single interface. Features encrypted credential vault, team collaboration, session recording, and comprehensive audit logging.

- Web-based interface accessible from any browser
- Single binary deployment with embedded frontend
- Encrypted credential vault for passwords and SSH keys
- Fine-grained permissions and role-based access control
- Session recording and audit logs
- Team collaboration with connection sharing

## Features

### Supported Protocols

- **Terminal**: SSH (v1/v2), Telnet
- **Remote Desktop**: RDP, VNC
- **Containers**: Docker, Kubernetes
- **Databases**: MySQL, PostgreSQL, MongoDB, Redis
- **Object Storage**: S3, MinIO, Google Cloud Storage, Azure Blob

### Security

- **Credential Vault**: Store and share credentials securely with AES-256-GCM encryption
- **Reusable Identities**: Create credential profiles that work across multiple connections
- **Multi-Factor Authentication**: TOTP support for additional security
- **Enterprise Authentication**: OIDC, SAML, LDAP/Active Directory integration
- **Audit Trail**: Every action is logged for compliance and security review
- **Session Recording**: Record terminal and desktop sessions for audit purposes

### Collaboration

- **Team Management**: Organize users into teams with shared access to connections
- **Connection Sharing**: Share specific connections with team members or individuals
- **Session Sharing**: Collaborate in real-time by sharing live terminal or desktop sessions
- **Notifications**: Real-time alerts for shared sessions, permission changes, and system events

## Quick Start

### Prerequisites

- **Go**: 1.24 or newer
- **Node.js**: 20+ with pnpm
- **Docker**: For containerized deployment (optional)

### Local Development

```bash
# Clone the repository
git clone https://github.com/charlesng35/shellcn.git
cd shellcn

# Install dependencies
make install

# Start development environment (backend + frontend)
make dev
```

Open `http://localhost:8000` in your browser. You'll be prompted to create the first administrator account.

### Production Build

```bash
# Build the complete application
make build

# Run the binary
./bin/shellcn
```

The application will:

- Create a `./data/` directory for the SQLite database
- Start the server on `http://localhost:8000`
- Prompt you to create the first admin user on first launch

## Docker Deployment

### Docker Compose (Recommended)

Create a `docker-compose.yml`:

```yaml
services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports:
      - "8000:8000"
    volumes:
      - shellcn-data:/var/lib/shellcn
    environment:
      # Required: JWT secret for token signing
      SHELLCN_AUTH_JWT_SECRET: "change-this-to-a-strong-secret"

      # Optional: Vault encryption key
      SHELLCN_VAULT_ENCRYPTION_KEY: "change-this-to-a-strong-key"

      # Optional: Use PostgreSQL instead of SQLite
      # SHELLCN_DATABASE_DRIVER: postgres
      # SHELLCN_DATABASE_POSTGRES_ENABLED: true
      # SHELLCN_DATABASE_POSTGRES_HOST: postgres
      # SHELLCN_DATABASE_POSTGRES_DATABASE: shellcn
      # SHELLCN_DATABASE_POSTGRES_USERNAME: shellcn
      # SHELLCN_DATABASE_POSTGRES_PASSWORD: your-password

volumes:
  shellcn-data:
```

Start the application:

```bash
docker-compose up -d
```

### Standalone Docker

```bash
docker run -d \
  --name shellcn \
  -p 8000:8000 \
  -v shellcn-data:/var/lib/shellcn \
  -e SHELLCN_AUTH_JWT_SECRET="your-strong-secret-here" \
  -e SHELLCN_VAULT_ENCRYPTION_KEY="your-vault-key-here" \
  ghcr.io/charlesng35/shellcn:latest
```

### Access the Application

1. Open `http://localhost:8000` in your browser
2. Complete the first-time setup wizard to create your admin account
3. Start adding connections and managing your infrastructure

## Configuration

All configuration can be managed through environment variables with the `SHELLCN_` prefix:

```bash
# Server
SHELLCN_SERVER_PORT=8000

# Database (default: SQLite)
SHELLCN_DATABASE_DRIVER=sqlite
SHELLCN_DATABASE_SQLITE_PATH=./data/database.sqlite

# Vault encryption
SHELLCN_VAULT_ENCRYPTION_KEY=your-32-byte-encryption-key

# Authentication
SHELLCN_AUTH_JWT_SECRET=your-jwt-secret
SHELLCN_AUTH_JWT_EXPIRY=24h

# Protocol toggles
SHELLCN_PROTOCOLS_SSH_ENABLED=true
SHELLCN_PROTOCOLS_RDP_ENABLED=true
SHELLCN_PROTOCOLS_VNC_ENABLED=true
SHELLCN_PROTOCOLS_DOCKER_ENABLED=true
SHELLCN_PROTOCOLS_DATABASE_ENABLED=true

# Optional: Redis for caching
SHELLCN_CACHE_REDIS_ENABLED=false
SHELLCN_CACHE_REDIS_ADDRESS=localhost:6379
```

See `config/config.yaml` for all available options.

## Development

### Available Commands

```bash
# Development
make dev            # Start full-stack development (hot reload)
make dev-server     # Backend only
make dev-web        # Frontend only

# Building
make build          # Build production binary
make build-go       # Build backend only
make build-web      # Build frontend only

# Testing
make test           # Run all tests
make test-web       # Frontend tests only
make test-cover     # Go tests with coverage

# Code Quality
make lint           # Run linters
make fmt            # Format code
make clean          # Remove build artifacts
```

## Contributing

Contributions are welcome! Fork the repository, make your changes, and submit a pull request.

## License

MIT License - see `LICENSE` file for details.

---

Built for teams that need secure, audited access to their infrastructure.
