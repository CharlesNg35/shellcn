# Docker Deployment Reference

ShellCN can be configured entirely through environment variables when running under Docker or Docker Compose. Every setting in `internal/app/config.go` maps to a `SHELLCN_*` variable, and the server falls back to sensible defaults when a value is not provided. Environment variables always take precedence over settings loaded from `config/config.yaml`.

## Required Secrets

- `SHELLCN_AUTH_JWT_SECRET`: Secret used to sign and verify API tokens. Set this to a strong, unpredictable value before exposing ShellCN to other users.

## Server

- `SHELLCN_SERVER_PORT` (default `8000`): HTTP listen port.
- `SHELLCN_SERVER_LOG_LEVEL` (default `info`): Logging level (`debug`, `info`, `warn`, `error`).
- `SHELLCN_SERVER_CSRF_ENABLED` (default `false`): Enable CSRF protection middleware.

## Database

- `SHELLCN_DATABASE_DRIVER` (default `sqlite`): Storage backend (`sqlite`, `postgres`, `mysql`).
- `SHELLCN_DATABASE_PATH` (default `./data/shellcn.sqlite`, container default `/var/lib/shellcn/data.sqlite`): SQLite file path when using the SQLite driver.
- `SHELLCN_DATABASE_DSN` (default empty): Override connection string for Postgres or MySQL drivers.
- `SHELLCN_DATABASE_POSTGRES_ENABLED` (default `false`): Toggle Postgres configuration.
- `SHELLCN_DATABASE_POSTGRES_HOST` (default empty): Postgres host.
- `SHELLCN_DATABASE_POSTGRES_PORT` (default `0`): Postgres port.
- `SHELLCN_DATABASE_POSTGRES_DATABASE` (default empty): Postgres database name.
- `SHELLCN_DATABASE_POSTGRES_USERNAME` (default empty): Postgres username.
- `SHELLCN_DATABASE_POSTGRES_PASSWORD` (default empty): Postgres password.
- `SHELLCN_DATABASE_MYSQL_ENABLED` (default `false`): Toggle MySQL configuration.
- `SHELLCN_DATABASE_MYSQL_HOST` (default empty): MySQL host.
- `SHELLCN_DATABASE_MYSQL_PORT` (default `0`): MySQL port.
- `SHELLCN_DATABASE_MYSQL_DATABASE` (default empty): MySQL database name.
- `SHELLCN_DATABASE_MYSQL_USERNAME` (default empty): MySQL username.
- `SHELLCN_DATABASE_MYSQL_PASSWORD` (default empty): MySQL password.

## Cache

- `SHELLCN_CACHE_REDIS_ENABLED` (default `false`): Enable Redis cache integration.
- `SHELLCN_CACHE_REDIS_ADDRESS` (default `127.0.0.1:6379`): Redis host and port.
- `SHELLCN_CACHE_REDIS_USERNAME` (default empty): Redis username.
- `SHELLCN_CACHE_REDIS_PASSWORD` (default empty): Redis password.
- `SHELLCN_CACHE_REDIS_DB` (default `0`): Redis database number.
- `SHELLCN_CACHE_REDIS_TLS` (default `false`): Enable TLS for Redis connections.
- `SHELLCN_CACHE_REDIS_TIMEOUT` (default `5s`): Redis dial timeout.

## Vault

- `SHELLCN_VAULT_ENCRYPTION_KEY` (default empty): 32-byte key for encrypting stored credentials. Required for production.
- `SHELLCN_VAULT_ALGORITHM` (default `aes-256-gcm`): Encryption algorithm.
- `SHELLCN_VAULT_KEY_ROTATION_DAYS` (default `90`): Key rotation interval.

## Monitoring

- `SHELLCN_MONITORING_PROMETHEUS_ENABLED` (default `true`): Expose Prometheus metrics.
- `SHELLCN_MONITORING_PROMETHEUS_ENDPOINT` (default `/metrics`): Metrics endpoint path.
- `SHELLCN_MONITORING_HEALTH_CHECK_ENABLED` (default `true`): Expose health check endpoint.

## Features

- `SHELLCN_FEATURES_SESSION_SHARING_ENABLED` (default `true`): Enable collaborative session sharing.
- `SHELLCN_FEATURES_SESSION_SHARING_MAX_SHARED_USERS` (default `5`): Maximum simultaneous participants in a shared session.
- `SHELLCN_FEATURES_CLIPBOARD_SYNC_ENABLED` (default `true`): Enable clipboard synchronisation.
- `SHELLCN_FEATURES_CLIPBOARD_SYNC_MAX_SIZE_KB` (default `1024`): Maximum clipboard payload size in KB.
- `SHELLCN_FEATURES_NOTIFICATIONS_ENABLED` (default `true`): Enable notification system.

## Modules

- `SHELLCN_MODULES_SSH_ENABLED` (default `true`)
- `SHELLCN_MODULES_SSH_DEFAULT_PORT` (default `22`)
- `SHELLCN_MODULES_SSH_SSH_V1_ENABLED` (default `false`)
- `SHELLCN_MODULES_SSH_SSH_V2_ENABLED` (default `true`)
- `SHELLCN_MODULES_SSH_AUTO_RECONNECT` (default `true`)
- `SHELLCN_MODULES_SSH_MAX_RECONNECT_ATTEMPTS` (default `3`)
- `SHELLCN_MODULES_SSH_KEEPALIVE_INTERVAL` (default `60`)
- `SHELLCN_MODULES_TELNET_ENABLED` (default `true`)
- `SHELLCN_MODULES_TELNET_DEFAULT_PORT` (default `23`)
- `SHELLCN_MODULES_TELNET_AUTO_RECONNECT` (default `true`)
- `SHELLCN_MODULES_TELNET_MAX_RECONNECT_ATTEMPTS` (default `3`)
- `SHELLCN_MODULES_SFTP_ENABLED` (default `true`)
- `SHELLCN_MODULES_SFTP_DEFAULT_PORT` (default `22`)
- `SHELLCN_MODULES_RDP_ENABLED` (default `true`)
- `SHELLCN_MODULES_RDP_DEFAULT_PORT` (default `3389`)
- `SHELLCN_MODULES_VNC_ENABLED` (default `true`)
- `SHELLCN_MODULES_VNC_DEFAULT_PORT` (default `5900`)
- `SHELLCN_MODULES_DOCKER_ENABLED` (default `true`)
- `SHELLCN_MODULES_KUBERNETES_ENABLED` (default `false`)
- `SHELLCN_MODULES_DATABASE_ENABLED` (default `true`)
- `SHELLCN_MODULES_DATABASE_MYSQL` (default `true`)
- `SHELLCN_MODULES_DATABASE_POSTGRES` (default `true`)
- `SHELLCN_MODULES_DATABASE_REDIS` (default `true`)
- `SHELLCN_MODULES_DATABASE_MONGODB` (default `true`)
- `SHELLCN_MODULES_PROXMOX_ENABLED` (default `false`)
- `SHELLCN_MODULES_OBJECT_STORAGE_ENABLED` (default `false`)

## Authentication

- `SHELLCN_AUTH_JWT_SECRET` (default empty): Secret for signing JWT access tokens. Required.
- `SHELLCN_AUTH_JWT_ISSUER` (default empty): JWT issuer string.
- `SHELLCN_AUTH_JWT_ACCESS_TOKEN_TTL` (default `15m`): Access token lifetime.
- `SHELLCN_AUTH_SESSION_REFRESH_TOKEN_TTL` (default `720h`): Refresh token lifetime.
- `SHELLCN_AUTH_SESSION_REFRESH_TOKEN_LENGTH` (default `48`): Refresh token length in characters.
- `SHELLCN_AUTH_LOCAL_LOCKOUT_THRESHOLD` (default `5`): Failed login attempts before lockout.
- `SHELLCN_AUTH_LOCAL_LOCKOUT_DURATION` (default `15m`): Lockout duration.

## Email

- `SHELLCN_EMAIL_SMTP_ENABLED` (default `false`): Enable outbound email.
- `SHELLCN_EMAIL_SMTP_HOST` (default empty): SMTP host.
- `SHELLCN_EMAIL_SMTP_PORT` (default `587`): SMTP port.
- `SHELLCN_EMAIL_SMTP_USERNAME` (default empty): SMTP username.
- `SHELLCN_EMAIL_SMTP_PASSWORD` (default empty): SMTP password.
- `SHELLCN_EMAIL_SMTP_FROM` (default empty): Sender address.
- `SHELLCN_EMAIL_SMTP_USE_TLS` (default `true`): Enable TLS for SMTP.
- `SHELLCN_EMAIL_SMTP_TIMEOUT` (default `10s`): SMTP dial timeout.

## Example `docker-compose.yml`

```yaml
version: "3.9"

services:
  shellcn:
    image: ghcr.io/charlesng35/shellcn:latest
    ports:
      - "8000:8000"
    environment:
      SHELLCN_AUTH_JWT_SECRET: "replace-with-strong-secret"
      SHELLCN_VAULT_ENCRYPTION_KEY: "replace-with-32-byte-key"
      SHELLCN_DATABASE_DRIVER: postgres
      SHELLCN_DATABASE_POSTGRES_ENABLED: "true"
      SHELLCN_DATABASE_POSTGRES_HOST: "postgres"
      SHELLCN_DATABASE_POSTGRES_PORT: "5432"
      SHELLCN_DATABASE_POSTGRES_DATABASE: "shellcn"
      SHELLCN_DATABASE_POSTGRES_USERNAME: "shellcn"
      SHELLCN_DATABASE_POSTGRES_PASSWORD: "super-secret-password"
    volumes:
      - shellcn-data:/var/lib/shellcn
      - ./config/config.yaml:/var/lib/shellcn/config/config.yaml:ro
    depends_on:
      - postgres

  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: shellcn
      POSTGRES_USER: shellcn
      POSTGRES_PASSWORD: super-secret-password
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  shellcn-data:
  postgres-data:
```

Mounting `./config/config.yaml` into `/var/lib/shellcn/config/config.yaml` lets the container discover the file via `LoadConfig`, while still allowing individual values to be overridden with `SHELLCN_*` variables.

## Example `config/config.yaml`

```yaml
server:
  port: 8000
  log_level: info

database:
  driver: postgres
  postgres:
    enabled: true
    host: postgres
    port: 5432
    database: shellcn
    username: shellcn
    password: super-secret-password

vault:
  encryption_key: replace-with-32-byte-key
  algorithm: aes-256-gcm
  key_rotation_days: 90

auth:
  jwt:
    secret: replace-with-strong-secret
    issuer: shellcn.local
    access_token_ttl: 15m
```

Place this file in `config/config.yaml` next to your compose file and mount it read-only. Any environment variables provided at runtime will override the YAML values.
