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
  - [x] Implement OAuth2 configuration
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
  - [x] Implement rate limiting
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
  - [ ] Achieve 80%+ test coverage
  - [ ] Run integration tests
  - [ ] Run contract tests
  - [ ] Performance testing
  - [ ] Security testing
- [ ] **Documentation**
  - [ ] API documentation (Swagger)
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
