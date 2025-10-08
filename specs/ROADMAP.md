# Project Roadmap

## 1. Core Module (Auth, Users, Permissions) — In Progress

### Phase 1: Foundation (Week 1)

- [x] **Project Setup**
  - [x] Initialize Go module
  - [x] Setup directory structure
  - [x] Configure Makefile
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

- [ ] **JWT Service**
  - [ ] Implement token generation
  - [ ] Implement token validation
  - [ ] Write JWT tests
- [ ] **Local Auth Provider**
  - [ ] Implement login
  - [ ] Implement password hashing
  - [ ] Implement account lockout
  - [ ] Write auth provider tests
- [ ] **Session Management**
  - [ ] Implement session service
  - [ ] Implement refresh token flow
  - [ ] Implement session revocation
  - [ ] Write session tests
- [ ] **MFA (Optional)**
  - [ ] Implement TOTP service
  - [ ] Implement QR code generation
  - [ ] Implement backup codes
  - [ ] Write MFA tests

### Phase 3: Authorization (Week 3)

- [ ] **Permission System**
  - [ ] Implement permission registry
  - [ ] Register core permissions
  - [ ] Implement permission checker
  - [ ] Implement dependency resolver
  - [ ] Write permission tests
- [ ] **Permission Service**
  - [ ] Implement role management
  - [ ] Implement permission assignment
  - [ ] Write permission service tests

### Phase 4: Core Services (Week 4)

- [ ] **User Service**
  - [ ] Implement CRUD operations
  - [ ] Implement activation/deactivation
  - [ ] Implement password management
  - [ ] Write user service tests
- [ ] **Organization Service**
  - [ ] Implement CRUD operations
  - [ ] Write organization service tests
- [ ] **Team Service**
  - [ ] Implement team management
  - [ ] Implement member management
  - [ ] Write team service tests
- [ ] **Audit Service**
  - [ ] Implement audit logging
  - [ ] Implement log filtering
  - [ ] Implement log export
  - [ ] Write audit service tests
- [ ] **Auth Provider Service**
  - [ ] Implement provider CRUD
  - [ ] Implement OIDC configuration
  - [ ] Implement OAuth2 configuration
  - [ ] Implement SAML configuration
  - [ ] Implement LDAP configuration
  - [ ] Implement local/invite settings
  - [ ] Implement connection testing
  - [ ] Write auth provider service tests

### Phase 5: API Layer (Week 5)

- [ ] **Middleware**
  - [ ] Implement auth middleware
  - [ ] Implement permission middleware
  - [ ] Implement CORS middleware
  - [ ] Implement logger middleware
  - [ ] Implement rate limiting
  - [ ] Write middleware tests
- [ ] **Handlers**
  - [ ] Implement auth handlers
  - [ ] Implement setup handler
  - [ ] Implement user handlers
  - [ ] Implement organization handlers
  - [ ] Implement team handlers
  - [ ] Implement permission handlers
  - [ ] Implement session handlers
  - [ ] Implement audit handlers
  - [ ] Implement auth provider handlers
  - [ ] Write handler integration tests
- [ ] **Router**
  - [ ] Configure all routes
  - [ ] Setup route groups
  - [ ] Apply middleware
  - [ ] Write router tests

### Phase 6: Security & Monitoring (Week 6)

- [ ] **Security**
  - [ ] Implement security headers
  - [ ] Implement CSRF protection
  - [ ] Implement input validation
  - [ ] Security audit
- [ ] **Monitoring**
  - [ ] Implement Prometheus metrics
  - [ ] Implement health check
  - [ ] Setup structured logging
  - [ ] Configure log levels
- [ ] **Background Jobs**
  - [ ] Implement session cleanup
  - [ ] Implement audit log retention
  - [ ] Implement token cleanup

### Phase 7: Testing & Documentation (Week 7)

- [ ] **Testing**
  - [ ] Achieve 80%+ test coverage
  - [ ] Run integration tests
  - [ ] Run contract tests
  - [ ] Performance testing
  - [ ] Security testing
- [ ] **Documentation**
  - [ ] API documentation (Swagger)
  - [ ] README updates
  - [ ] Deployment guide
  - [ ] Configuration guide
  - [ ] Troubleshooting guide

### Phase 8: External Auth Providers (Optional – Week 8)

- [ ] **OIDC Provider**
  - [ ] Implement OIDC authentication
  - [ ] Write OIDC tests
- [ ] **SAML Provider**
  - [ ] Implement SAML authentication
  - [ ] Write SAML tests
- [ ] **LDAP Provider**
  - [ ] Implement LDAP authentication
  - [ ] Write LDAP tests

---

## 2. Vault Module (Credentials, Encryption) — Not Started

---

## Monitoring Module (Metrics, Health) — Not Started

---

## SSH Module — Not Started

---

## Telnet Module — Not Started

---

## SFTP Module — Not Started

---

## RDP Module — Not Started

---

## VNC Module — Not Started

---

## Docker Module — Not Started

---

## Kubernetes Module — Not Started

---

## Database Module — Not Started

    ### MySQL — Driver support implemented (Phase 1)
    ### PostgreSQL — Driver support implemented (Phase 1)
    ### Redis — Not Started
    ### MongoDB — Not Started

---

## Proxmox Module — Not Started

---

---

## File Share Module — Not Started
