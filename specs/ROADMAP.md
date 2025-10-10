# Project Roadmap

## 1. Core Module (Auth, Users, Permissions) — Feature Complete (QA & Docs Pending)

## Implementation Checklist

### Phase 1: Project Setup & Foundation (Week 1)

- [x] Initialize Vite 7 project with React 19 and TypeScript 5.7+
- [x] Configure Tailwind CSS v4 with custom theme
- [x] Set up ESLint, Prettier, and TypeScript strict mode
- [x] Configure path aliases (@/ for src/)
- [x] Install and configure core dependencies (React Router 7, TanStack Query, Zustand)
- [x] Set up project structure (pages/, components/, hooks/, lib/, store/, types/)
- [x] Create base UI components (Button, Input, Card, Modal, etc.) using Radix UI
- [x] Implement class-variance-authority (CVA) for component variants
- [x] Configure Vitest for unit testing

### Phase 2: Authentication & Setup Flow (Week 2)

- [x] Implement auth store (Zustand) with token management
- [x] Create API client (Axios) with interceptors for auth
- [x] Build Login page with form validation (react-hook-form + Zod)
- [x] Implement Setup wizard for first-time initialization
- [x] Create AuthLayout component
- [x] Build SSO provider buttons (OIDC, SAML, LDAP)
- [x] Implement MFA verification flow
- [x] Create Password reset flow
- [x] Build useAuth hook for authentication state
- [x] Implement token refresh logic
- [x] Add logout functionality
- [x] Create ProtectedRoute component
- [x] Write tests for authentication flows

### Phase 3: Connections & API Integration (Week 2)

- [x] Create API endpoints for connections, returning available or enabled connections.
- [x] A connection is considered enabled if its driver has been implemented.
- [x] The UI should display only enabled connections.
- [x] The UI should display only connections available to the user, based on their permissions.
- [x] Permissions should be fetched based on the user's role; if the user is an admin, all connections should be fetched.

### Phase 3: Dashboard & Layout (Week 3)

- [x] Create DashboardLayout with Sidebar and Header
- [x] Implement responsive navigation
- [x] Build Sidebar with permission-based menu items
- [x] Create Header with user profile dropdown
- [x] Implement Dashboard page with overview widgets
- [x] Build useCurrentUser hook
- [x] Create usePermissions hook
- [x] Implement PermissionGuard component
- [x] Add breadcrumb navigation
- [x] Create notification center UI
- [x] Implement WebSocket connection for real-time notifications
- [x] Write tests for layout components

### Phase 4: User Management (Week 4)

- [x] Create Users list page with pagination
- [x] Build UserTable component with TanStack Table
- [x] Implement UserFilters component
- [x] Create UserForm for create/edit
- [x] Build UserDetailModal
- [x] Implement user activation/deactivation
- [x] Create password management UI
- [x] Build useUsers hook with TanStack Query
- [x] Add user search functionality
- [x] Implement bulk operations
- [x] Write tests for user management

### Phase 5: Team Management (Week 5)

- [x] Implement Teams list page
- [x] Create TeamForm component
- [x] Build team member management UI
- [x] Implement member assignment/removal
- [x] Build useTeams hook
- [x] Write tests for team management

### Phase 6: Permission Management (Week 6)

- [ ] Create Permissions page
- [ ] Build PermissionMatrix component
- [ ] Implement RoleManager component
- [ ] Create role creation/editing forms
- [ ] Build permission dependency visualization
- [ ] Implement permission assignment UI
- [ ] Create usePermissions hook for registry
- [ ] Add role-based filtering
- [ ] Build permission search
- [ ] Write tests for permission management

### Phase 7: Auth Provider Administration (Week 7)

- [ ] Create AuthProviders page
- [ ] Build ProviderCard component
- [ ] Implement OIDCConfigForm
- [ ] Create SAMLConfigForm
- [ ] Build LDAPConfigForm
- [ ] Implement LocalSettingsForm
- [ ] Create InviteSettingsForm
- [ ] Build provider enable/disable toggle
- [ ] Implement provider test connection
- [ ] Create useAuthProviders hook
- [ ] Add provider configuration validation
- [ ] Write tests for provider management

### Phase 8: Session Management (Week 8)

- [ ] Create Sessions page
- [ ] Build SessionTable component
- [ ] Implement SessionCard for mobile view
- [ ] Add session revocation functionality
- [ ] Create "Revoke All" feature
- [ ] Build device/browser detection display
- [ ] Implement session filtering
- [ ] Create useSessions hook
- [ ] Add session activity timeline
- [ ] Write tests for session management

### Phase 9: Audit Log Viewer (Week 9)

- [ ] Create AuditLogs page
- [ ] Build AuditLogTable component
- [ ] Implement AuditFilters component
- [ ] Create AuditExport functionality (CSV)
- [ ] Build audit log detail modal
- [ ] Implement date range picker
- [ ] Create useAuditLogs hook
- [ ] Add audit log search
- [ ] Build security audit view
- [ ] Write tests for audit viewer

### Phase 10: Settings & Preferences (Week 10)

- [ ] Create Settings page with tabs
- [ ] Build user profile settings
- [ ] Implement password change form
- [ ] Create MFA setup flow with QR code
- [ ] Build appearance settings (theme, language)
- [ ] Implement notification preferences
- [ ] Create session preferences
- [ ] Build settings store (Zustand)
- [ ] Add settings persistence
- [ ] Write tests for settings

### Phase 11: Testing & Quality Assurance (Week 11)

- [ ] Achieve ≥80% unit test coverage
- [ ] Write integration tests for critical flows
- [ ] Set up Cypress for E2E testing
- [ ] Create E2E tests for authentication flow
- [ ] Test user management workflows
- [ ] Test permission assignment flows
- [ ] Verify accessibility (WCAG 2.1 AA)
- [ ] Test keyboard navigation
- [ ] Verify responsive design (mobile, tablet, desktop)
- [ ] Performance testing (Lighthouse score ≥90)

### Phase 12: Documentation & Polish (Week 12)

- [ ] Write README with setup instructions
- [ ] Document API integration patterns
- [ ] Create component usage examples
- [ ] Add inline code documentation
- [ ] Build developer onboarding guide
- [ ] Create user guide for admin features
- [ ] Optimize bundle size
- [ ] Implement code splitting
- [ ] Final UI/UX polish

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
