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
- [x] Add connection folder management UI (create, edit, delete) with empty state and context menus.
- [x] Implement guided connection creation flow (resource selection modal + basic connection form).
- [x] Expose POST /connections backend endpoint to support basic connection creation.

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
- [x] Support team-level role assignment with inherited permissions
- [x] Write tests for team management

### Phase 6: Permission Management (Week 6)

- [x] Create Permissions page
- [x] Build PermissionMatrix component
- [x] Implement RoleManager component
- [x] Create role creation/editing forms
- [x] Build permission dependency visualization
- [x] Implement permission assignment UI
- [x] Create usePermissions hook for registry
- [x] Add role-based filtering
- [x] Build permission search
- [x] Write tests for permission management

### Phase 6.5: Resource-Scoped Permissions & Sharing (Week 6+)

- [x] Introduce `resource_permissions` table and permission checker integration
- [x] Ship connection share service and `/api/connections/:id/shares` CRUD
- [x] Surface team capability endpoint (`/api/teams/:id/capabilities`) and UI card
- [x] Auto-grant missing team permissions during connection creation

### Phase 7: Auth Provider Administration (Week 7)

- [x] Create AuthProviders page
- [x] Build ProviderCard component
- [x] Implement OIDCConfigForm
- [x] Create SAMLConfigForm
- [x] Build LDAPConfigForm
- [x] Implement LocalSettingsForm
- [x] Implement user invitation flow (API, UI, and acceptance page)
- [x] Build provider enable/disable toggle
- [x] Implement provider test connection
- [x] Create useAuthProviders hook
- [x] Add provider configuration validation
- [x] Write tests for provider management
- [x] Sidebar: surface active connection sessions once backend exposes activity feed (critical for final navigation polish)

### Phase 8: Session Management (Week 8)

- [x] Add Sessions tab to profile settings
- [x] Build simplified session list component
- [x] Add session revocation functionality
- [x] Create "Revoke Other Sessions" feature
- [x] Surface session metadata (IP, last activity)
- [x] Implement useProfileSessions hook
- [x] Write tests for session management

### Phase 9: Audit Log Viewer (Week 9)

- [x] Create AuditLogs page
- [x] Build AuditLogTable component
- [x] Implement AuditFilters component
- [x] Create AuditExport functionality (CSV)
- [x] Build audit log detail modal
- [x] Implement date range picker
- [x] Create useAuditLogs hook
- [x] Add audit log search
  - [x] Build security audit view
  - [x] Write tests for audit viewer

### Phase 10: Settings & Preferences (Week 10)

- [x] Create Settings page with tabs
- [x] Build user profile settings
- [x] Implement password change form
- [x] Create MFA setup flow with QR code
- [x] Update the login flow to support MFA verification
- [x] Make sure the backend is properly integrated for MFA (TOTP)
- [x] Build appearance settings (theme, language)
- [x] Implement notification preferences
- [x] Create session preferences
- [x] Build settings store (Zustand)
- [x] Add settings persistence
- [x] Write tests for settings

### Phase 11: Documentation & Polish (Week 12)

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

## 8. Session Recording Module — Planned

- Implement unified capture and storage for SSH, RDP, and VNC sessions with replay tooling for auditors.

---

## 9. Docker Module — Not Started

---

## 10. Kubernetes Module — Not Started

---

## 11. Database Module — Not Started

    ### MySQL — Driver support implemented (Phase 1)
    ### PostgreSQL — Driver support implemented (Phase 1)
    ### Redis — Not Started
    ### MongoDB — Not Started

---

## 12. Proxmox Module — Not Started

---

## 13. File Share Module — Not Started
