# Project Roadmap

## 1. Core Module (Auth, Users, Permissions) — Feature Complete (QA & Docs Pending)

### Phase 1: Foundation (Week 1)

- [x] **Project Setup**
  - [x] Initialize Go module
  - [x] Setup directory structure
  - [x] Configure Makefile
  - [x] Implement server entrypoint (configuration, database, router wiring)
  - [ ] Setup CI/CD pipeline
- [x] **Shared Packages**
  - [x] Implement `pkg/logger`
  - [x] Implement `pkg/errors`
  - [x] Implement `pkg/response`
  - [x] Implement `pkg/crypto`
  - [x] Implement `pkg/validator`
  - [x] Write tests for shared packages
- [x] **Database Layer**
  - [x] Define all GORM models
  - [x] Implement database initialization
  - [x] Create migration system
  - [x] Setup SQLite driver
  - [x] Setup PostgreSQL driver
  - [x] Setup MySQL driver
  - [x] Write model tests

### Phase 2: Authentication (Week 2)

- [x] **JWT Service**
  - [x] Implement token generation
  - [x] Implement token validation
  - [x] Write JWT tests
- [x] **Local Auth Provider**
  - [x] Implement login
  - [x] Implement password hashing
  - [x] Implement account lockout
  - [x] Write auth provider tests
- [x] **Session Management**
  - [x] Implement session service
  - [x] Implement refresh token flow
  - [x] Implement session revocation
  - [x] Add cache abstraction with Redis primary and SQL fallback for session tokens
  - [x] Write session tests
- [x] **MFA (Optional)**
  - [x] Implement TOTP service
  - [x] Implement QR code generation
  - [x] Implement backup codes
  - [x] Write MFA tests

### Phase 3: Authorization (Week 3)

- [x] **Permission System**
  - [x] Implement permission registry
  - [x] Register core permissions
  - [x] Implement permission checker
  - [x] Implement dependency resolver
  - [x] Write permission tests
- [x] **Permission Service**
  - [x] Implement role management
  - [x] Implement permission assignment
  - [x] Write permission service tests

### Phase 4: Core Services (Week 4)

- [x] **User Service**
  - [x] Implement CRUD operations
  - [x] Implement activation/deactivation
  - [x] Implement password management
  - [x] Write user service tests
- [x] **Organization Service**
  - [x] Implement CRUD operations
  - [x] Write organization service tests
- [x] **Team Service**
  - [x] Implement team management
  - [x] Implement member management
  - [x] Write team service tests
- [x] **Audit Service**
  - [x] Implement audit logging
  - [x] Implement log filtering
  - [x] Implement log export
  - [x] Write audit service tests
- [x] **Auth Provider Service**
  - [x] Implement provider CRUD
  - [x] Implement OIDC configuration
  - [x] Implement SAML configuration
  - [x] Implement LDAP configuration
  - [x] Implement local/invite settings
  - [x] Implement connection testing
  - [x] Write auth provider service tests

### Phase 5: API Layer (Week 5)

- [x] **Middleware**
  - [x] Implement auth middleware
  - [x] Implement permission middleware
  - [x] Implement CORS middleware
  - [x] Implement logger middleware
  - [x] Implement rate limiting with Redis/SQL cache fallback
  - [x] Write middleware tests
- [x] **Handlers**
  - [x] Implement auth handlers
  - [x] Implement setup handler
  - [x] Implement user handlers
  - [x] Implement organization handlers
  - [x] Implement team handlers
  - [x] Implement permission handlers
  - [x] Implement session handlers
  - [x] Implement audit handlers
  - [x] Implement auth provider handlers
  - [x] Write handler integration tests
- [x] **Router**
  - [x] Configure all routes
  - [x] Setup route groups
  - [x] Apply middleware
  - [x] Write router tests
- [ ] **Router Maintenance**
  - [ ] Refactor route registration into modular helpers to keep `router.go` concise
- Added comprehensive middleware and handler integration tests to validate Phase 5 endpoints end-to-end.

### Phase 6: Security & Monitoring (Week 6)

- [x] **Security**
  - [x] Implement security headers
  - [x] Implement CSRF protection
  - [x] Implement input validation
  - [x] Security audit
- [x] **Monitoring**
  - [x] Implement Prometheus metrics
  - [x] Implement health check
  - [x] Setup structured logging
  - [x] Configure log levels
- [x] **Background Jobs**
  - [x] Implement session cleanup
  - [x] Implement audit log retention
  - [x] Implement token cleanup

### Phase 7: Testing & Documentation (Week 7)

- [ ] **Testing**
  - [ ] Achieve 80%+ test coverage — enforce ≥80% overall (≥70%/pkg), expand unit tests for services, permissions, routing, and first-user/session/audit edge cases. (Progress: dial/auth SMTP shims, provider registry + OIDC/SAML/LDAP factory tests, runtime defaults, logger helpers; total backend coverage at ~55%.)
  - [ ] Run integration tests — exercise auth, org/team, permission, audit, and setup flows against in-memory stack with seeded fixtures.
  - [ ] Run contract tests — lock JSON response envelopes, JWT claims, and permission dependency rules with golden tests.
  - [ ] Performance testing — benchmark hot endpoints with `hey`/`vegeta`, capture pprof traces, document tuning levers for DB/cache/rate limits.
  - [ ] Security testing — run `golangci-lint`, `gosec`, `staticcheck`, `govulncheck`, and manual privilege/rate-limit/MFA abuse checks.
- [ ] **Documentation**
  - [ ] API documentation — publish OpenAPI 3.1 spec + markdown in `specs/plans/CORE_MODULE_API.md` with schemas, errors, permissions.
  - [ ] Deployment CI/CD — extend GH Actions to build/test/sign multi-arch images and push to GHCR on tag/manual trigger with rollback notes.
  - [ ] Configuration guide — document all config/env toggles, single-node vs production examples, security handling for secrets.
  - [ ] Troubleshooting guide — catalog common failures, log snippets, diagnostic commands, and escalation checklist.

### Phase 8: External Auth Providers (Optional – Week 8)

- [x] **Shared SSO Foundation**
  - [x] Provider registry + unified callback flow
  - [x] User mapping & provisioning rules
  - [x] Secure secret storage + audit logging
- [x] **OIDC Provider**
  - [x] Authorization code + PKCE flow
  - [x] Claim mapping & unknown user handling
  - [x] Handler + service test coverage
- [x] **SAML Provider**
  - [x] SP metadata + ACS implementation
  - [x] Attribute mapping & assertion validation
  - [x] Sample assertion + handler tests
- [x] **LDAP Provider**
  - [x] Bind/search strategies with TLS options
  - [x] Attribute mapping & optional sync job
  - [x] Connection test API + mock LDAP tests

---

## 2. Vault Module (Credentials, Encryption) — Not Started

---

## 3. Monitoring Module (Metrics, Health) — Not Started

---

## 4. SSH Module — Not Started

### SFTP Module — Not Started

---

## 5. Telnet Module — Not Started

---

## 6. RDP Module — Not Started

---

## 7. VNC Module — Not Started

---

## 8. Docker Module — Not Started

---

## 9. Kubernetes Module — Not Started

---

## 10. Database Module — Not Started

    ### MySQL — Driver support implemented (Phase 1)
    ### PostgreSQL — Driver support implemented (Phase 1)
    ### Redis — Not Started
    ### MongoDB — Not Started

---

## 11. Proxmox Module — Not Started

---

## 12. File Share Module — Not Started
