# ShellCN Core – Configuration Guide

This guide describes how to configure the Core backend for development and production deployments. Configuration can be supplied via YAML files, environment variables, or a combination of both.

---

## 1. Configuration Sources

1. **Runtime defaults** – `internal/app/config.go` seeds sensible defaults (SQLite, built-in modules, metrics enabled).
2. **Configuration file** – `config/config.yaml` (or a custom path via `--config`) overrides defaults.
3. **Environment variables** – any key can be overridden with the prefix `SHELLCN_` and dot notation replaced by underscores.
   - Example: `SHELLCN_DATABASE_DRIVER=postgres`, `SHELLCN_AUTH_JWT_SECRET=super-secret`.

If no secrets are supplied, `app.ApplyRuntimeDefaults` will generate a JWT secret and vault encryption key on first boot. These values are logged as generated; persist them in your configuration after the first run.

---

## 2. Recommended Layout

```
shellcn/
├── config/
│   └── config.yaml        # primary configuration file (loaded by default)
├── data/                  # persistent database and encryption materials
└── ...
```

You can point the server to an alternate directory or file using:

```bash
./shellcn-server --config /etc/shellcn/config.yaml
```

---

## 3. Base Configuration Example

```yaml
# config/config.yaml
server:
  port: 8443
  log_level: info

database:
  driver: postgres
  postgres:
    enabled: true
    host: db.internal
    port: 5432
    database: shellcn
    username: shellcn
    password: ${DB_PASSWORD}

cache:
  redis:
    enabled: true
    address: redis.internal:6379
    username: ""
    password: ${REDIS_PASSWORD}
    db: 0
    tls: true

vault:
  encryption_key: ${VAULT_ENCRYPTION_KEY} # 32 bytes hex
  algorithm: aes-256-gcm
  key_rotation_days: 90

auth:
  jwt:
    secret: ${JWT_SECRET}
    issuer: shellcn
    access_token_ttl: 15m
  session:
    refresh_token_ttl: 720h # 30 days
    refresh_token_length: 48
  local:
    lockout_threshold: 5
    lockout_duration: 15m

email:
  smtp:
    enabled: true
    host: smtp.internal
    port: 587
    username: no-reply
    password: ${SMTP_PASSWORD}
    from: "ShellCN <no-reply@example.com>"
    use_tls: true

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
    ssh_v1_enabled: false
    ssh_v2_enabled: true
  rdp:
    enabled: true
    default_port: 3389
  docker:
    enabled: true
  database:
    enabled: true
    mysql: true
    postgres: true
    redis: true
```

> ℹ️ Values such as `${JWT_SECRET}` rely on the shell to substitute environment variables. When running under systemd or Kubernetes, inject them via environment or secret management.

---

## 4. Database Options

| Driver     | Notes                                                                                                                         |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------- |
| `sqlite`   | Default for development. Stores database in `database.path` (default `./data/shellcn.sqlite`). Requires write access on disk. |
| `postgres` | Recommended for production. Requires `database.postgres.enabled=true` and credentials.                                        |
| `mysql`    | Supported via `database.mysql.enabled=true`. Ensure `server.sql_mode` compatibility in DB.                                    |

For Postgres/MySQL, omit `database.path` and either provide `DSN` or host credentials. If both `DSN` and host fields are present, `DSN` takes precedence.

---

## 5. Cache Layer

- **Redis enabled**: Sessions, rate limiting, and other high-frequency lookups leverage Redis.
- **Redis disabled**: The system transparently falls back to SQL stores (`cache.NewDatabaseStore`).
- Use TLS (`cache.redis.tls=true`) for managed cloud services; combine with `SHELLCN_CACHE_REDIS_PASSWORD` for authentication.

---

## 6. Authentication Providers

Provider configuration payloads are stored encrypted via the vault key. Admin APIs accept the following JSON structures:

- **OIDC** – issuer, client ID, client secret, redirect URL, scopes.
- **SAML** – metadata URL _or_ inline SSO URL, ACS URL, private key, certificate chain, attribute mapping.
- **LDAP** – host, port, bind DN, bind password, TLS settings, search filter `{identifier}` placeholders, attribute mapping.

Configure provider secrets via the admin UI or `POST /api/auth/providers/:type/configure`. Ensure the vault key is stable across restarts or secrets cannot be decrypted.

---

## 7. SMTP Setup

The mailer requires:

- Valid sender (`email.smtp.from`) and host/port.
- Credentials for `AUTH PLAIN` if SMTP server enforces auth.
- TLS recommended (`use_tls=true`). For self-signed certs, terminate TLS at an internal relay.
- Default timeout is 10 seconds; override with `email.smtp.timeout`.

If SMTP is disabled, features like password resets and invite e-mails remain unavailable.

---

## 8. Feature Flags & Modules

- `features.session_sharing.enabled` – enables collaborative terminal sessions. Also configure `max_shared_users`.
- `features.clipboard_sync.max_size_kb` – protect against excessively large clipboard transfers.
- Module toggles in `modules.*.enabled` allow footprint reduction; disabled modules do not register permissions or background jobs.

---

## 9. Deployment Profiles

### 9.1 Local Development

```bash
export SHELLCN_SERVER_LOG_LEVEL=debug
export SHELLCN_AUTH_JWT_SECRET="dev-secret"
export SHELLCN_VAULT_ENCRYPTION_KEY="0123456789abcdef0123456789abcdef"
go run ./cmd/server --config /path/to/dev/config
```

Use SQLite, disable Redis (`cache.redis.enabled=false`), and rely on runtime-generated secrets only in disposable environments.

### 9.2 Production (Kubernetes Example)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: shellcn-secrets
stringData:
  JWT_SECRET: "...random..."
  VAULT_KEY: "...32-byte-hex..."
  DB_PASSWORD: "...postgres..."
  REDIS_PASSWORD: "...redis..."
---
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: shellcn
          image: ghcr.io/acme/shellcn-core:core-v1.0.0
          env:
            - name: SHELLCN_AUTH_JWT_SECRET
              valueFrom:
                { secretKeyRef: { name: shellcn-secrets, key: JWT_SECRET } }
            - name: SHELLCN_VAULT_ENCRYPTION_KEY
              valueFrom:
                { secretKeyRef: { name: shellcn-secrets, key: VAULT_KEY } }
            - name: SHELLCN_DATABASE_POSTGRES_PASSWORD
              valueFrom:
                { secretKeyRef: { name: shellcn-secrets, key: DB_PASSWORD } }
```

Mount `config.yaml` via ConfigMap or bake into the container image for immutable deployments.

### 9.3 Docker Compose

```yaml
version: "3.9"

services:
  shellcn:
    image: ghcr.io/acme/shellcn-core:core-v1.0.0
    ports:
      - "8000:8000"
    environment:
      SHELLCN_AUTH_JWT_SECRET: ${JWT_SECRET}
      SHELLCN_VAULT_ENCRYPTION_KEY: ${VAULT_ENCRYPTION_KEY}
    volumes:
      - shellcn-data:/var/lib/shellcn

volumes:
  shellcn-data:
    driver: local
```

Populate the `.env` file with secrets, and run `docker compose up -d`. The named volume persists the SQLite database and encryption metadata.

---

## 10. Operational Best Practices

- **Rotate secrets** – configure a 90-day rotation for the vault key and SMTP credentials; update config and restart gracefully.
- **Backups** – when using SQLite, snapshot `./data`. For Postgres/MySQL use native backups plus periodic audit log exports.
- **Observability** – collect `/metrics` and `/health`. Restrict `/metrics` via reverse proxy to avoid public exposure.
- **Rate Limiting** – tune rate limiter at `middleware.RateLimit(rateStore, requests, window)` in code if upstream load balancers already enforce limits.
- **Configuration validation** – run `go run ./cmd/server --config ./config --config-check-only` once the CLI flag is added (planned). Until then, use `go test ./internal/app -run TestLoadConfigExamples`.

---

## 11. Reference

- `internal/app/config.go` – master struct definitions and defaults.
- `specs/plans/CORE_MODULE_PLAN_BACKEND.md` – broader implementation roadmap.
- `specs/plans/CORE_MODULE_API.md` – detailed API contract for building clients.
