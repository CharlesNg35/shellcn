# ShellCN

A self-hosted **infrastructure access gateway / operations cockpit**: a single Go
binary with an embedded Vue 3 frontend that brokers secure, audited access to SSH,
SFTP, Docker, Kubernetes, Proxmox, databases, remote desktops, and more.

Every protocol is a **first-party, compiled-in Go plugin** that declares a
**versioned manifest**. The core owns rendering, security, sessions, transport, and
audit; the **frontend is a universal renderer** driven entirely by the manifest
projection — adding a plugin requires zero frontend changes.

## Status

Early development. Work proceeds **UI-first** through the phased plan — see
[`checklist.md`](checklist.md) for current progress.

## Documentation

- [`specs/v2.md`](specs/v2.md) — architecture (source of truth)
- [`specs/plans/`](specs/plans/) — phased, numbered build steps
- [`AGENTS.md`](AGENTS.md) — working agreement (conventions, invariants)

## Develop

Requires Go 1.25+ and Node 24.11+.

```sh
make build      # vite build → embed → single binary (bin/shellcn)
make dev-web    # Vite dev server (HMR)
make dev-api    # Go API server (--dev; paired with Vite /api proxy)
make test       # Go + frontend tests
make lint       # golangci-lint + gofumpt + frontend lint
```

## License

[MIT](LICENSE)
