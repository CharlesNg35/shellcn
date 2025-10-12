# ShellCN Core – Troubleshooting Guide

This guide catalogues common operational issues, symptoms, diagnostic commands, and remediation steps for the Core backend.

---

## 1. Quick Reference Table

| Symptom                                                        | Likely Cause                                            | Resolution                                                                                                                              |
| -------------------------------------------------------------- | ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| `/api/setup/status` never flips to `completed`                 | Seed user creation failed or database read-only.        | Check server logs for `setup` module errors, verify DB permissions, re-run `POST /api/setup/initialize`.                                |
| `auth.invalid_credentials` despite correct password            | Account locked by brute-force protection.               | Inspect audit log (`action=auth.login`, `result=failed`), wait for `auth.local.lockout_duration` or reset via admin.                    |
| Redis fallback warnings (`redis unavailable; falling back...`) | Redis not reachable or TLS mismatch.                    | Validate `SHELLCN_CACHE_REDIS_*` settings, attempt `redis-cli -u rediss://... ping`, confirm certificates.                              |
| `permission.denied` for admin users                            | Missing role assignment or permissions cache stale.     | Call `/api/auth/me` to inspect roles; ensure `admin` role includes required permissions; use `POST /api/auth/logout` to refresh tokens. |
| OIDC test fails (`discovery failed`)                           | Incorrect issuer URL or firewall.                       | Ensure issuer resolves inside cluster, check HTTPS certificates, verify `.well-known/openid-configuration` access.                      |
| SAML callback errors (`parse response`)                        | Clock skew, certificate mismatch, or incorrect ACS URL. | Sync IdP/SP clocks (NTP), re-upload certificate, confirm ACS matches metadata.                                                          |
| LDAP "user not found"                                          | Search base/filter misconfigured.                       | Use `ldapsearch` with same bind account and filter; adjust `user_filter` placeholders.                                                  |
| Database deadlock / migrations missing                         | Concurrent startup or DB permissions missing.           | Run `go run ./cmd/server --config ... --migrate-only` (planned), ensure DB user has `CREATE TABLE`.                                     |
| High latency on `/api/auth/login`                              | Password hashing cost or external provider slowdown.    | Tune bcrypt cost in code (future flag), validate provider response times, enable Redis for session cache.                               |

---

## 2. First-Time Setup Issues

- **Symptom:** `POST /api/setup/initialize` returns 500.  
  **Diagnostics:**
  - Inspect server logs (`module=setup`).
  - Check database path permissions: `ls -ld data/`.
  - If using Postgres/MySQL ensure schema owner has create privilege.
    **Fix:** Correct filesystem permissions, ensure DB connectivity, then retry setup; the handler is idempotent and will only succeed when no users exist.

---

## 3. Database Connectivity

1. **SQLite corruption / locking**

   - Error: `database is locked`.
   - Remediation: stop all processes accessing the file, enable WAL mode by setting `database.dsn: file:data/shellcn.sqlite?_busy_timeout=5000&_journal_mode=WAL`. Consider migrating to Postgres for concurrent workloads.

2. **Postgres authentication failure**

   - Error chain: `pq: password authentication failed for user`.
   - Steps: test DSN using `psql "host=..."`, validate secrets, ensure `SHELLCN_DATABASE_POSTGRES_ENABLED=true`.

3. **MySQL TLS requirement**
   - Set `database.mysql.tls` DSN parameter (e.g., `dsn: "shellcn:pass@tcp(mysql:3306)/shellcn?tls=true"`). Register custom certs using `mysql.RegisterTLSConfig`.

---

## 4. Cache & Rate Limiting

- If Redis is optional in your deployment, warnings about fallback are informational. If Redis is required for scale, monitor the log `redis connected`.
- To verify rate limiting, send >100 requests/min from single IP—`429 Too Many Requests` should be returned. Tune using environment variable `SHELLCN_RATE_LIMIT_REQUESTS` once the knob is exposed (roadmapped).

---

## 5. Authentication Providers

### 5.1 OIDC

- **Common errors**
  - `oidc provider: discovery failed` – issuer unreachable or incorrect path.
  - `oidc provider: pkce challenge is required` – frontend must generate PKCE pair; confirm API docs.
- **Diagnostics**
  - `curl -k https://issuer/.well-known/openid-configuration`.
  - Check server logs for `module=sso`.
- **Fixes**
  - Adjust provider configuration via `POST /api/auth/providers/oidc/configure`.
  - Ensure redirect URL matches UI (e.g., `https://app.example.com/callback/oidc`).

### 5.2 SAML

- Test metadata via `/api/auth/providers/saml/metadata`.
- Use `samltool.com` to validate assertions.
- Ensure `vault.encryption_key` remains unchanged; otherwise stored private keys become unreadable.

### 5.3 LDAP

- Use `ldapsearch -H ldap://host:389 -D "bindDN" -W -b "baseDN" "(&(objectClass=person)(uid=username))"` to confirm filters.
- When `UseTLS=true`, ensure either trusted CA or set `SkipVerify=true` for testing (not recommended for production).

---

## 6. Email Delivery

- Error `smtp: dial` indicates firewall or wrong host.
- Error `smtp: auth` means credentials rejected—verify service account and enabling authentication.
- Use `openssl s_client -starttls smtp -connect smtp.example.com:587` to inspect TLS handshake.
- For development, disable SMTP by leaving `email.smtp.enabled=false`.

---

## 7. Operational Diagnostics

- **Logs** – default log level `info`. Increase temporarily with `SHELLCN_SERVER_LOG_LEVEL=debug`. Logs are structured JSON via Zap; integrate with ELK or Loki.
- **Health Checks** – `/health` returns `{success:true}` (HTTP 200). If database connection fails, status becomes 500 with error message.
- **Metrics** – `/metrics` includes `shellcn_api_latency_seconds_bucket` for p95 tracking. Scrape with Prometheus and alert if latencies exceed SLO.
- **Profiling** – expose pprof endpoints via `go tool pprof` by building debug binary (roadmap item). Until then, wrap the binary with `GODEBUG=http2debug=2` for HTTP traces.

---

## 8. Maintenance Tasks

- **Expired sessions/tokens not clearing** – ensure maintenance cleaner is running; logs with `module=maintenance`. When shutting down, watch for `maintenance shutdown cleanup failed`.
- **Audit log bloat** – configure `maintenance.WithAuditRetentionDays`.Export logs periodically via `/api/audit/export`.

---

## 9. Support Checklist

When escalating to the core team, capture:

1. ShellCN version (`docker inspect ghcr.io/... | jq '.[0].Config.Labels["org.opencontainers.image.version"]'`).
2. Configuration excerpts (redact secrets).
3. Relevant logs with timestamps and correlation IDs.
4. Output from `/health` and `/metrics` for the same window.
5. Steps to reproduce, including CURL commands where applicable.

---

## 10. Additional Resources

- [Configuration Guide](CONFIGURATION_GUIDE.md) – full list of settings.
- [API Documentation](../plans/CORE_MODULE_API.md) – request payloads and endpoints.
- [Roadmap](../ROADMAP.md) – track upcoming fixes and features.
