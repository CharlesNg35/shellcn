<p align="center">
  <img src="web/public/favicon.svg" width="72" alt="ShellCN logo" />
</p>

<h1 align="center">ShellCN</h1>

<p align="center">One self-hosted gateway for everything you log into.</p>

<p align="center">
  <a href="https://github.com/CharlesNg35/shellcn/actions/workflows/ci.yml"><img src="https://github.com/CharlesNg35/shellcn/actions/workflows/ci.yml/badge.svg" alt="CI" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-GPL--3.0-blue.svg" alt="License: GPL-3.0" /></a>
</p>

---

ShellCN is a single Go binary that gives you one secure, audited place to reach all your
servers, containers, databases, and desktops. Run it, open a browser, and you're in.

_Demo (coming soon):_

It speaks **20 protocols** out of the box today:

- **Shells**: SSH
- **File transfer**: SFTP, FTP/FTPS, SMB, WebDAV, S3
- **Containers & clusters**: Docker, Swarm, Podman, Kubernetes, Proxmox
- **Remote desktops**: VNC, RDP
- **Databases**: PostgreSQL, MySQL, MongoDB, Redis
- **Observability**: server monitoring
- **Directory**: LDAP

## Quick start

Each needs a master key (it encrypts stored credentials) and a first admin login, then open
**http://localhost:8081** and sign in. Reuse the same master key on restart, or stored
credentials can't be decrypted.

### Docker, one command

Data and session recordings live in `/data`, so mount a volume there.

```sh
docker run -d --name shellcn -p 8081:8081 \
  -v shellcn-data:/data \
  -e SHELLCN_MASTER_KEY="$(openssl rand -base64 32)" \
  -e SHELLCN_BOOTSTRAP_ADMIN_USERNAME=admin \
  -e SHELLCN_BOOTSTRAP_ADMIN_PASSWORD=change-me \
  ghcr.io/charlesng35/shellcn:latest
```

### Docker Compose

Save this as `docker-compose.yml`:

```yaml
services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports:
      - "8081:8081"
    environment:
      SHELLCN_MASTER_KEY: ${SHELLCN_MASTER_KEY}
      SHELLCN_BOOTSTRAP_ADMIN_USERNAME: admin
      SHELLCN_BOOTSTRAP_ADMIN_PASSWORD: change-me
    volumes:
      - shellcn-data:/data
    restart: unless-stopped

volumes:
  shellcn-data:
```

Generate a key once with `openssl rand -base64 32`, put it in a `.env` file beside the
compose file as `SHELLCN_MASTER_KEY=...`, then run `docker compose up -d`.

The `/data` volume also persists private plugins placed in `/data/plugins.d`.

### Single binary

Grab the build for your platform from the
[releases page](https://github.com/CharlesNg35/shellcn/releases), then run it with the same
three settings:

```sh
export SHELLCN_MASTER_KEY="$(openssl rand -base64 32)"
export SHELLCN_BOOTSTRAP_ADMIN_USERNAME=admin
export SHELLCN_BOOTSTRAP_ADMIN_PASSWORD=change-me
./shellcn
```

It serves on `:8081` and keeps its data in the working directory.

> **Early days.** ShellCN is in active development, so expect a few rough edges. Please feel
> free to take it for a spin and tell us what breaks. Issues and feedback are very welcome.

## Extending it

ShellCN keeps the built-in protocol set small. Extra protocols are external plugins:

- Install ShellCN-maintained plugins from the Marketplace.
- Build your own with the
  [plugin starter](https://github.com/CharlesNg35/shellcn-plugin-starter).
- To publish a plugin, release its binary and submit a manifest to
  [shellcn-plugin-registry](https://github.com/CharlesNg35/shellcn-plugin-registry).
- For private plugins, skip the registry and drop the binary in the gateway plugin
  directory.

The default plugin directory is `/data/plugins.d` in Docker and `./plugins.d` when running
the single binary. You can override it with `plugins.dir` or `SHELLCN_PLUGINS_DIR`.

## Build from source

Requires Go 1.26+ and Node 24+.

```sh
make build    # single binary at bin/shellcn
make dev      # live-reloading dev server
```

## Related repositories

ShellCN is developed across a small family of repos we maintain:

| Repository                                                                        | What it is                                                             |
| --------------------------------------------------------------------------------- | ---------------------------------------------------------------------- |
| [shellcn](https://github.com/CharlesNg35/shellcn)                                 | The gateway: core, SDK, and the built-in protocol set.                 |
| [shellcn-plugin-registry](https://github.com/CharlesNg35/shellcn-plugin-registry) | The plugin registry that powers the in-app Marketplace.                |
| [shellcn-contrib](https://github.com/CharlesNg35/shellcn-contrib)                 | ShellCN-maintained external plugins, installable from the Marketplace. |
| [shellcn-plugin-starter](https://github.com/CharlesNg35/shellcn-plugin-starter)   | Template + docs for writing your own plugin.                           |

## License

[GPL-3.0](LICENSE)
