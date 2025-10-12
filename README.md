# ShellCN

ShellCN is a web-based remote access platform for teams that need to manage SSH, RDP, VNC, Docker, Kubernetes, Database and more targets from a single control plane. It combines audited session management with an encrypted credential vault powered by a Go backend and a modern React frontend.

## Highlights

- Unified access gateway for shell, desktop, container, and database protocols
- Granular role-based permissions with dependency resolution and auditing
- Secure credential storage backed by AES-256-GCM and team-aware sharing

## Requirements

- Go 1.24 or newer (CGO enabled when building Rust FFI modules)
- Node.js 20 or newer with `pnpm`

## Quick Start

### Local Development

```bash
git clone https://github.com/charlesng35/shellcn.git
cd shellcn
make install
make dev
```

`make install` tidies Go modules and installs the frontend dependencies with `pnpm`. `make dev` launches the full-stack development environment defined in `scripts/dev.mjs`, running the Go API and the Vite dev server together.

### Docker Compose

```yaml
version: "3.9"

services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports:
      - "8000:8000"
    volumes:
      - shellcn-data:/var/lib/shellcn
    environment:
      SHELLCN_AUTH_JWT_SECRET: "replace-with-strong-secret"

volumes:
  shellcn-data:
    driver: local
```

Set `SHELLCN_AUTH_JWT_SECRET` to a strong value so the API can issue and validate tokens.

## Configuration

Configuration defaults to `config/config.yaml`, and every value can be overridden with environment variables prefixed with `SHELLCN_`.

```bash
# Example overrides
export SHELLCN_SERVER_PORT=9090
export SHELLCN_DATABASE_DRIVER=postgres
export SHELLCN_DATABASE_POSTGRES_ENABLED=true
export SHELLCN_DATABASE_POSTGRES_PASSWORD="p@ssw0rd"
```

For the complete list of supported environment variables and Docker deployment guidance, review `docs/dockers.md`.

## Contributing

We welcome pull requests. Before you start, make sure you have Go 1.24+, Node.js 20+, and `pnpm` installed locally.

1. Clone the repository and run `make install` to prepare Go modules and frontend dependencies.
2. Run `make dev` for the full-stack development loop, or `make dev-server` / `make dev-web` if you want to focus on a single service.
3. Create a feature branch, commit your changes, and open a pull request against `main`.

## License

This project is licensed under the MIT License. See `LICENSE` for the full text.
