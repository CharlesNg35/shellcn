# Docker Deployment Reference

ShellCN can be configured entirely through environment variables when running under Docker or Docker Compose. Every setting in `internal/app/config.go` maps to a `SHELLCN_*` variable, and the server falls back to sensible defaults when a value is not provided. Environment variables always take precedence over settings loaded from `config/config.yaml`.

## Configuration Reference

### Required Secrets

⚠️ **These must be set before running in production:**

- `SHELLCN_AUTH_JWT_SECRET`: Secret used to sign and verify API tokens. Use a strong, unpredictable value (32+ characters).
- `SHELLCN_VAULT_ENCRYPTION_KEY`: 32-byte key for encrypting stored credentials. Required for production.

---

## Server Configuration

| Environment Variable          | YAML Path             | Default | Description                                      |
| ----------------------------- | --------------------- | ------- | ------------------------------------------------ |
| `SHELLCN_SERVER_PORT`         | `server.port`         | `8000`  | HTTP listen port                                 |
| `SHELLCN_SERVER_LOG_LEVEL`    | `server.log_level`    | `info`  | Logging level (`debug`, `info`, `warn`, `error`) |
| `SHELLCN_SERVER_CSRF_ENABLED` | `server.csrf.enabled` | `false` | Enable CSRF protection middleware                |

**YAML Example:**

```yaml
server:
  port: 8000
  log_level: info
  csrf:
    enabled: false
```

---

## Database Configuration

| Environment Variable      | YAML Path         | Default                 | Description                                                          |
| ------------------------- | ----------------- | ----------------------- | -------------------------------------------------------------------- |
| `SHELLCN_DATABASE_DRIVER` | `database.driver` | `sqlite`                | Storage backend (`sqlite`, `postgres`, `mysql`)                      |
| `SHELLCN_DATABASE_PATH`   | `database.path`   | `./data/shellcn.sqlite` | SQLite file path (container default: `/var/lib/shellcn/data.sqlite`) |
| `SHELLCN_DATABASE_DSN`    | `database.dsn`    | _(empty)_               | Override connection string for Postgres or MySQL                     |

### PostgreSQL Options

| Environment Variable                 | YAML Path                    | Default   | Description                      |
| ------------------------------------ | ---------------------------- | --------- | -------------------------------- |
| `SHELLCN_DATABASE_POSTGRES_ENABLED`  | `database.postgres.enabled`  | `false`   | Toggle Postgres configuration    |
| `SHELLCN_DATABASE_POSTGRES_HOST`     | `database.postgres.host`     | _(empty)_ | Postgres host                    |
| `SHELLCN_DATABASE_POSTGRES_PORT`     | `database.postgres.port`     | `0`       | Postgres port (typically `5432`) |
| `SHELLCN_DATABASE_POSTGRES_DATABASE` | `database.postgres.database` | _(empty)_ | Postgres database name           |
| `SHELLCN_DATABASE_POSTGRES_USERNAME` | `database.postgres.username` | _(empty)_ | Postgres username                |
| `SHELLCN_DATABASE_POSTGRES_PASSWORD` | `database.postgres.password` | _(empty)_ | Postgres password                |

### MySQL Options

| Environment Variable              | YAML Path                 | Default   | Description                   |
| --------------------------------- | ------------------------- | --------- | ----------------------------- |
| `SHELLCN_DATABASE_MYSQL_ENABLED`  | `database.mysql.enabled`  | `false`   | Toggle MySQL configuration    |
| `SHELLCN_DATABASE_MYSQL_HOST`     | `database.mysql.host`     | _(empty)_ | MySQL host                    |
| `SHELLCN_DATABASE_MYSQL_PORT`     | `database.mysql.port`     | `0`       | MySQL port (typically `3306`) |
| `SHELLCN_DATABASE_MYSQL_DATABASE` | `database.mysql.database` | _(empty)_ | MySQL database name           |
| `SHELLCN_DATABASE_MYSQL_USERNAME` | `database.mysql.username` | _(empty)_ | MySQL username                |
| `SHELLCN_DATABASE_MYSQL_PASSWORD` | `database.mysql.password` | _(empty)_ | MySQL password                |

**YAML Example:**

```yaml
database:
  driver: postgres
  postgres:
    enabled: true
    host: postgres
    port: 5432
    database: shellcn
    username: shellcn
    password: super-secret-password
```

---

## Cache Configuration (Redis)

| Environment Variable           | YAML Path              | Default          | Description                      |
| ------------------------------ | ---------------------- | ---------------- | -------------------------------- |
| `SHELLCN_CACHE_REDIS_ENABLED`  | `cache.redis.enabled`  | `false`          | Enable Redis cache integration   |
| `SHELLCN_CACHE_REDIS_ADDRESS`  | `cache.redis.address`  | `127.0.0.1:6379` | Redis host and port              |
| `SHELLCN_CACHE_REDIS_USERNAME` | `cache.redis.username` | _(empty)_        | Redis username (Redis 6+)        |
| `SHELLCN_CACHE_REDIS_PASSWORD` | `cache.redis.password` | _(empty)_        | Redis password                   |
| `SHELLCN_CACHE_REDIS_DB`       | `cache.redis.db`       | `0`              | Redis database number (0-15)     |
| `SHELLCN_CACHE_REDIS_TLS`      | `cache.redis.tls`      | `false`          | Enable TLS for Redis connections |
| `SHELLCN_CACHE_REDIS_TIMEOUT`  | `cache.redis.timeout`  | `5s`             | Redis dial timeout               |

**YAML Example:**

```yaml
cache:
  redis:
    enabled: true
    address: redis.example.com:6379
    username: shellcn
    password: redis-secret
    db: 0
    tls: true
    timeout: 5s
```

---

## Vault Configuration

| Environment Variable           | YAML Path              | Default       | Description                                                                 |
| ------------------------------ | ---------------------- | ------------- | --------------------------------------------------------------------------- |
| `SHELLCN_VAULT_ENCRYPTION_KEY` | `vault.encryption_key` | _(empty)_     | 32-byte key for encrypting stored credentials. **Required for production.** |
| `SHELLCN_VAULT_ALGORITHM`      | `vault.algorithm`      | `aes-256-gcm` | Encryption algorithm                                                        |

**YAML Example:**

```yaml
vault:
  encryption_key: replace-with-32-byte-key
  algorithm: aes-256-gcm
```

---

## Monitoring Configuration

| Environment Variable                      | YAML Path                         | Default    | Description                   |
| ----------------------------------------- | --------------------------------- | ---------- | ----------------------------- |
| `SHELLCN_MONITORING_PROMETHEUS_ENABLED`   | `monitoring.prometheus.enabled`   | `true`     | Expose Prometheus metrics     |
| `SHELLCN_MONITORING_PROMETHEUS_ENDPOINT`  | `monitoring.prometheus.endpoint`  | `/metrics` | Metrics endpoint path         |
| `SHELLCN_MONITORING_HEALTH_CHECK_ENABLED` | `monitoring.health_check.enabled` | `true`     | Expose health check endpoints |

**YAML Example:**

```yaml
monitoring:
  prometheus:
    enabled: true
    endpoint: /metrics
  health_check:
    enabled: true
```

---

## Features Configuration

| Environment Variable                                | YAML Path                                   | Default | Description                                           |
| --------------------------------------------------- | ------------------------------------------- | ------- | ----------------------------------------------------- |
| `SHELLCN_FEATURES_SESSION_SHARING_ENABLED`          | `features.session_sharing.enabled`          | `true`  | Enable collaborative session sharing                  |
| `SHELLCN_FEATURES_SESSION_SHARING_MAX_SHARED_USERS` | `features.session_sharing.max_shared_users` | `5`     | Maximum simultaneous participants in a shared session |
| `SHELLCN_FEATURES_NOTIFICATIONS_ENABLED`            | `features.notifications.enabled`            | `true`  | Enable notification system                            |

**YAML Example:**

```yaml
features:
  session_sharing:
    enabled: true
    max_shared_users: 5
  notifications:
    enabled: true
```

---

## Protocol Configuration

Enable or disable individual protocol drivers:

| Environment Variable                       | YAML Path                          | Default | Description                     |
| ------------------------------------------ | ---------------------------------- | ------- | ------------------------------- |
| `SHELLCN_PROTOCOLS_SSH_ENABLED`            | `protocols.ssh.enabled`            | `true`  | Enable SSH protocol             |
| `SHELLCN_PROTOCOLS_TELNET_ENABLED`         | `protocols.telnet.enabled`         | `true`  | Enable Telnet protocol          |
| `SHELLCN_PROTOCOLS_SFTP_ENABLED`           | `protocols.sftp.enabled`           | `true`  | Enable SFTP protocol            |
| `SHELLCN_PROTOCOLS_RDP_ENABLED`            | `protocols.rdp.enabled`            | `true`  | Enable RDP protocol             |
| `SHELLCN_PROTOCOLS_VNC_ENABLED`            | `protocols.vnc.enabled`            | `true`  | Enable VNC protocol             |
| `SHELLCN_PROTOCOLS_DOCKER_ENABLED`         | `protocols.docker.enabled`         | `true`  | Enable Docker protocol          |
| `SHELLCN_PROTOCOLS_KUBERNETES_ENABLED`     | `protocols.kubernetes.enabled`     | `false` | Enable Kubernetes protocol      |
| `SHELLCN_PROTOCOLS_PROXMOX_ENABLED`        | `protocols.proxmox.enabled`        | `false` | Enable Proxmox protocol         |
| `SHELLCN_PROTOCOLS_OBJECT_STORAGE_ENABLED` | `protocols.object_storage.enabled` | `false` | Enable object storage protocols |

### Database Protocol Options

| Environment Variable                  | YAML Path                     | Default | Description               |
| ------------------------------------- | ----------------------------- | ------- | ------------------------- |
| `SHELLCN_PROTOCOLS_DATABASE_ENABLED`  | `protocols.database.enabled`  | `true`  | Enable database protocols |
| `SHELLCN_PROTOCOLS_DATABASE_MYSQL`    | `protocols.database.mysql`    | `true`  | Enable MySQL client       |
| `SHELLCN_PROTOCOLS_DATABASE_POSTGRES` | `protocols.database.postgres` | `true`  | Enable PostgreSQL client  |
| `SHELLCN_PROTOCOLS_DATABASE_REDIS`    | `protocols.database.redis`    | `true`  | Enable Redis client       |
| `SHELLCN_PROTOCOLS_DATABASE_MONGODB`  | `protocols.database.mongodb`  | `true`  | Enable MongoDB client     |

**YAML Example:**

```yaml
protocols:
  ssh:
    enabled: true
  telnet:
    enabled: true
  sftp:
    enabled: true
  rdp:
    enabled: true
  vnc:
    enabled: true
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
  object_storage:
    enabled: false
```

---

## Authentication Configuration

| Environment Variable                        | YAML Path                           | Default   | Description                                        |
| ------------------------------------------- | ----------------------------------- | --------- | -------------------------------------------------- |
| `SHELLCN_AUTH_JWT_SECRET`                   | `auth.jwt.secret`                   | _(empty)_ | **Required.** Secret for signing JWT access tokens |
| `SHELLCN_AUTH_JWT_ISSUER`                   | `auth.jwt.issuer`                   | _(empty)_ | JWT issuer string                                  |
| `SHELLCN_AUTH_JWT_ACCESS_TOKEN_TTL`         | `auth.jwt.access_token_ttl`         | `15m`     | Access token lifetime                              |
| `SHELLCN_AUTH_SESSION_REFRESH_TOKEN_TTL`    | `auth.session.refresh_token_ttl`    | `720h`    | Refresh token lifetime (30 days)                   |
| `SHELLCN_AUTH_SESSION_REFRESH_TOKEN_LENGTH` | `auth.session.refresh_token_length` | `48`      | Refresh token length in characters                 |
| `SHELLCN_AUTH_LOCAL_LOCKOUT_THRESHOLD`      | `auth.local.lockout_threshold`      | `5`       | Failed login attempts before lockout               |
| `SHELLCN_AUTH_LOCAL_LOCKOUT_DURATION`       | `auth.local.lockout_duration`       | `15m`     | Lockout duration                                   |

**YAML Example:**

```yaml
auth:
  jwt:
    secret: replace-with-strong-secret
    issuer: shellcn.local
    access_token_ttl: 15m
  session:
    refresh_token_ttl: 720h
    refresh_token_length: 48
  local:
    lockout_threshold: 5
    lockout_duration: 15m
```

---

## Email Configuration (SMTP)

| Environment Variable          | YAML Path             | Default   | Description           |
| ----------------------------- | --------------------- | --------- | --------------------- |
| `SHELLCN_EMAIL_SMTP_ENABLED`  | `email.smtp.enabled`  | `false`   | Enable outbound email |
| `SHELLCN_EMAIL_SMTP_HOST`     | `email.smtp.host`     | _(empty)_ | SMTP host             |
| `SHELLCN_EMAIL_SMTP_PORT`     | `email.smtp.port`     | `587`     | SMTP port             |
| `SHELLCN_EMAIL_SMTP_USERNAME` | `email.smtp.username` | _(empty)_ | SMTP username         |
| `SHELLCN_EMAIL_SMTP_PASSWORD` | `email.smtp.password` | _(empty)_ | SMTP password         |
| `SHELLCN_EMAIL_SMTP_FROM`     | `email.smtp.from`     | _(empty)_ | Sender address        |
| `SHELLCN_EMAIL_SMTP_USE_TLS`  | `email.smtp.use_tls`  | `true`    | Enable TLS for SMTP   |
| `SHELLCN_EMAIL_SMTP_TIMEOUT`  | `email.smtp.timeout`  | `10s`     | SMTP dial timeout     |

**YAML Example:**

```yaml
email:
  smtp:
    enabled: true
    host: smtp.example.com
    port: 587
    username: smtp-user
    password: smtp-pass
    from: no-reply@example.com
    use_tls: true
    timeout: 10s
```

---

## Complete YAML Configuration Example

```yaml
server:
  port: 8000
  log_level: info
  csrf:
    enabled: false

database:
  driver: postgres
  postgres:
    enabled: true
    host: postgres
    port: 5432
    database: shellcn
    username: shellcn
    password: super-secret-password

cache:
  redis:
    enabled: true
    address: redis:6379
    username: ""
    password: ""
    db: 0
    tls: false
    timeout: 5s

vault:
  encryption_key: replace-with-32-byte-key
  algorithm: aes-256-gcm

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
  notifications:
    enabled: true

protocols:
  ssh:
    enabled: true
  telnet:
    enabled: true
  sftp:
    enabled: true
  rdp:
    enabled: true
  vnc:
    enabled: true
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
  object_storage:
    enabled: false

auth:
  jwt:
    secret: replace-with-strong-secret
    issuer: shellcn.local
    access_token_ttl: 15m
  session:
    refresh_token_ttl: 720h
    refresh_token_length: 48
  local:
    lockout_threshold: 5
    lockout_duration: 15m

email:
  smtp:
    enabled: false
    host: smtp.example.com
    port: 587
    username: smtp-user
    password: smtp-pass
    from: no-reply@example.com
    use_tls: true
    timeout: 10s
```

---

## Docker Compose Example

```yaml
version: "3.9"

services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports:
      - "8000:8000"
    environment:
      # Required secrets
      SHELLCN_AUTH_JWT_SECRET: "replace-with-strong-secret"
      SHELLCN_VAULT_ENCRYPTION_KEY: "replace-with-32-byte-key"

      # Database
      SHELLCN_DATABASE_DRIVER: postgres
      SHELLCN_DATABASE_POSTGRES_ENABLED: "true"
      SHELLCN_DATABASE_POSTGRES_HOST: "postgres"
      SHELLCN_DATABASE_POSTGRES_PORT: "5432"
      SHELLCN_DATABASE_POSTGRES_DATABASE: "shellcn"
      SHELLCN_DATABASE_POSTGRES_USERNAME: "shellcn"
      SHELLCN_DATABASE_POSTGRES_PASSWORD: "super-secret-password"

      # Optional: Cache
      SHELLCN_CACHE_REDIS_ENABLED: "true"
      SHELLCN_CACHE_REDIS_ADDRESS: "redis:6379"

      # Optional: Disable some protocols
      SHELLCN_PROTOCOLS_KUBERNETES_ENABLED: "false"
      SHELLCN_PROTOCOLS_PROXMOX_ENABLED: "false"
    volumes:
      - shellcn-data:/var/lib/shellcn
    depends_on:
      - postgres
      - redis

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: shellcn
      POSTGRES_USER: shellcn
      POSTGRES_PASSWORD: super-secret-password
    volumes:
      - postgres-data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data

volumes:
  shellcn-data:
  postgres-data:
  redis-data:
```

---

## Environment Variable Priority

Configuration is loaded in the following order (later sources override earlier ones):

1. **Built-in defaults** (defined in `config.go`)
2. **YAML configuration file** (`config/config.yaml`)
3. **Environment variables** (`SHELLCN_*`)

This allows you to:

- Use YAML for base configuration
- Override specific values with environment variables
- Run without a config file (using only defaults + env vars)

---

## Configuration Tips

### Generating Secrets

```bash
# Generate JWT secret (32 bytes)
openssl rand -hex 32

# Generate vault encryption key (32 bytes)
openssl rand -hex 32
```

### Minimal Production Configuration

At minimum, set these environment variables for production:

```bash
SHELLCN_AUTH_JWT_SECRET=your-jwt-secret-here
SHELLCN_VAULT_ENCRYPTION_KEY=your-32-byte-encryption-key-here
SHELLCN_DATABASE_DRIVER=postgres
SHELLCN_DATABASE_POSTGRES_ENABLED=true
SHELLCN_DATABASE_POSTGRES_HOST=your-postgres-host
SHELLCN_DATABASE_POSTGRES_PORT=5432
SHELLCN_DATABASE_POSTGRES_DATABASE=shellcn
SHELLCN_DATABASE_POSTGRES_USERNAME=shellcn
SHELLCN_DATABASE_POSTGRES_PASSWORD=secure-password
```

### Protocol Optimization

Disable unused protocols to reduce memory footprint:

```bash
SHELLCN_PROTOCOLS_KUBERNETES_ENABLED=false
SHELLCN_PROTOCOLS_PROXMOX_ENABLED=false
SHELLCN_PROTOCOLS_OBJECT_STORAGE_ENABLED=false
SHELLCN_PROTOCOLS_DATABASE_MONGODB=false
```

---

## Health Checks

The platform exposes the following endpoints for container orchestration:

- `GET /health` - Simple up/down check
- `GET /health/ready` - Ready to serve traffic
- `GET /health/live` - Liveness probe
- `GET /metrics` - Prometheus metrics (if enabled)

**Docker Compose healthcheck example:**

```yaml
services:
  shellcn:
    healthcheck:
      test:
        [
          "CMD",
          "wget",
          "--quiet",
          "--tries=1",
          "--spider",
          "http://localhost:8000/health",
        ]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```
