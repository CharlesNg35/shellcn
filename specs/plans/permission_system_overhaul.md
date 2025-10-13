# Permission System Overhaul Proposal

**Status:** Draft  
**Owner:** Platform / Security  
**Last Updated:** 2025-01-19

---

## 1. Current State – Summary & Pain Points

The existing RBAC layer is centred around **roles** (stored in `user_roles` / `team_roles`) and a **global permission registry** (`internal/permissions/core.go`). A `Checker` loads user + team roles, resolves dependencies, and grants access if any role (direct or via team) includes the permission. This gives us coarse-grained, registry-backed permission checks with dependency expansion.

However, several day-to-day scenarios expose gaps:

1. **Team-scoped connections vs protocol capabilities**

   - Creating a connection in a team merely sets `connections.team_id`. Whether team members can _use_ it depends on the union of roles assigned to that team.
   - There is no visibility in the UI about which protocol capabilities the team actually holds. A user may create an SSH connection, but teammates might lack `ssh.*` permissions, leading to launch failures that are confusing to diagnose.

2. **Driver-specific capabilities**

   - The spec anticipates per-driver permissions (e.g. `ssh.port_forward`, `docker.container.exec`). At the moment, we only have generic `connection.view`, `connection.launch`, etc. Protocol-specific permissions aren’t surfaced anywhere, so teams can’t reason about them, and the share UX cannot present them as options.

3. **Role reuse constraints**

   - Roles are global objects that can be linked to multiple teams and users. If one team needs a variant of a role (e.g. “Database readers” with extra protocol permission), cloning or mutating the role risks side effects for other assignments.
   - We need a way to layer additional capabilities for a team without mutating the shared, immutable roles those teams rely on.

4. **Lack of context-specific scopes**

   - Connections can be shared with individuals (per the previous plan), but the permissions chosen must map to global IDs. There is no way to express resource-scoped privileges such as “Alice may launch _only this connection_ with port-forward capability” without overgranting.
   - We need a richer model for resource-level grants that plays well with team inheritance.

5. **Discoverability & UX**
   - The UI doesn’t expose the permissions a team holds. Connection creation doesn’t warn when members lack required protocol scopes, and protocol-specific capabilities can’t be toggled when the necessary permissions are missing.

---

## 2. Design Goals

To address the gaps, the next iteration of the permission system should:

1. Maintain **centralized registry & dependency resolution** (a strength of the current design).
2. Support **resource-scoped grants** for individual users without exploding the role graph.
3. Provide a **team capability matrix** so connection creation (and other workflows) can show what the active team can actually do.
4. Provide **team-level capability overrides** that complement immutable base roles.
5. Enable **protocol-specific permission groups** that map cleanly to driver features.
6. Keep the system portable across SQLite/Postgres/MySQL (no raw SQL; continue using GORM migrations).

---

## 3. Proposed Architecture

### 3.1 Permission Entities

Reframe permissions into three layers:

1. **Global Permission Registry**

   - Continue using `permissions.Register`, but expand modules to include driver namespaces (e.g. `ssh.connect`, `ssh.port_forward`, `docker.exec`, `vnc.desktop_control`).
   - Provide metadata (category, UI label, default scope) via the registry to aid UX.

2. **Team Capability Grants**

   - Introduce a lightweight table (`team_capability_grants`) that records additional permission IDs a team should inherit globally (e.g. `protocol:ssh.connect`).
   - Grants are additive: the permission checker unions role-derived permissions with these capability grants so teams can gain protocol access without cloning roles.
   - Each grant captures provenance (`granted_by`, `created_at`) and can be surfaced in the UI as part of the team capability matrix.

3. **Resource Grants**
   - Introduce a `resource_permissions` table:
     ```
     id          uuid PK
     resource_id uuid (e.g. connection ID)
     resource_type string ("connection")
     principal_type string ("user" | "team")
     principal_id uuid
     permission_id string  (e.g. "connection.launch" or "protocol:ssh.port_forward")
     expires_at timestamp nullable
     granted_by uuid
     metadata jsonb (scope details)
     ```
   - This table sits alongside `connection_visibilities` (which can reference it or be merged) to represent fine-grained permissions per resource.

### 3.2 Evaluation Algorithm

1. **Base permissions**: combine user roles + team roles (unchanged) => set `P_roles`.
2. **Team capability grants**: union global capability overrides for the team => set `P_team`.
3. **Resource grants**: when checking access to a specific resource (e.g. launching connection `conn-123`), load grants where `(resource_id=conn-123 AND principal matches user/team)`, apply expiry filter => set `P_resource`.
4. **Effective permissions**: `P_effective = closure(P_roles ∪ P_team ∪ P_resource)` (closure = union with dependencies).
5. For teams lacking a protocol permission, the capability grant can add it globally while resource grants remain the fine-grained override.

### 3.3 Team Capability Matrix

- New endpoint `GET /api/teams/:id/capabilities` returning the union of permissions from team roles, team capability grants, and resource grants.
- The connection creation modal can call this to determine whether the chosen protocol is launchable by the team; warn or auto-complete required permissions otherwise.
- Provide per-protocol sections so the UI shows “Team allows: SSH Launch, SSH Port Forward, denies: Database Admin”.

### 3.4 Protocol Permission Bundles

- Each driver module registers a permission group with defaults, e.g.:
  ```go
  permissions.Register(&Permission{
      ID: "ssh.port_forward",
      Module: "ssh",
      DependsOn: []string{"connection.launch"},
      Description: "Establish SSH port forwarding tunnels",
  })
  ```
- An API endpoint `/api/protocols/:id/permissions` lists the available driver permissions, enabling UI toggles during sharing or role assignment.

### 3.5 UI/UX Changes

1. **Connection Creation Workflow**

   - When selecting a team + protocol, query team capabilities.
   - If the team lacks a required capability, surface options:
     - Add resource-specific grant (restricted share).
     - Grant a team-wide capability override (e.g. protocol bundle).
   - Show driver-specific checkboxes (e.g. “Allow port forwarding”) gated by permission availability.

2. **Sharing Modal**

   - Allows selecting permission scopes derived from the driver permission registry.
   - For each share, display expiry, granter, and granted scopes.

3. **Team Management**
   - Enhance role assignment screen with insights into inherited permissions, team capability grants, and outstanding resource shares.

---

## 4. Implementation Roadmap

### Phase 0 – Prep

- Document driver permissions that need registration.
- Identify teams that currently require protocol overrides to inform capability grant UX.

### Phase 1 – Backend Foundations

1. Add `resource_permissions` table via GORM migration.
2. Update services:
   - `ConnectionService` to consult resource grants.
   - `TeamService` (or dedicated capability service) to read/write team capability grants.
3. Extend API models to include `is_shared`, `shared_from`, `permission_scopes`.
4. Provide capability endpoint for teams.

### Phase 2 – Driver Integration

1. Register protocol-specific permissions within each driver module (SSH, RDP, VNC, Docker, etc.).
2. Expand `/api/protocols/available` to include `permissions` metadata.

### Phase 3 – UI Updates

1. Connection creation/ share modals show capability warnings and scopes.
2. Team management UI (`web/src/pages/settings/Teams.tsx`, `TeamDetail`, `TeamFilterTabs`, capability widgets) highlights inherited permissions, team capability grants, and resource shares.
3. Permissions administration page (`web/src/pages/settings/Permissions.tsx`, role dialogs, assign components) surfaces protocol bundles, capability grants, and dependency insights.
4. Connection list badges highlighting shared scopes + expiration (cards, tables, `ConnectionDetail`), including “Shared by” indicators and filters for “Shared with me”.

### Phase 4 – Migration & Seeding

1. Seed team capability grants only where required (e.g. default protocol access for core teams).
2. Backfill existing connection shares into `resource_permissions` (if merging with `connection_visibilities`).

### Phase 5 – QA & Documentation

1. Unit/integration tests for permission evaluation.
2. Update developer docs on how to register driver permissions.
3. Run regression on team-based connection workflows.

---

## 5. Frontend Impact – Pages & Components

To scope the UI work, here is the current surface area that will participate in the overhaul:

| Area                       | Files / Components                                                                                                            | Notes                                                                                                 |
| -------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- |
| Connection list & cards    | `web/src/pages/connections/Connections.tsx`, `web/src/components/connections/ConnectionCard.tsx`, `ConnectionDetail` (future) | Badge and tooltip for “Shared by …”, filter for “Shared with me”, display permission scopes & expiry. |
| Connection creation & edit | `web/src/components/connections/ConnectionFormModal.tsx`, future `ShareConnectionModal.tsx`, `ConnectionSidebar` actions      | Capability warnings, protocol permission options, per-share scope selector, expiry picker.            |
| Team management            | `web/src/pages/settings/Teams.tsx`, `TeamDetail` route, `TeamRolesManager`, `TeamFilterTabs`                                  | Display team capability matrix, manage capability grants, review resource-level shares.               |
| Permissions administration | `web/src/pages/settings/Permissions.tsx`, role dialogs (`RoleForm`, `PermissionMatrix`)                                       | Display driver permission bundles, manage capability grants, show dependencies.                       |
| Sharing overview           | New “Shares” tab within connection detail (`ConnectionSharesPanel`, to be created), notification badges                       | List resource grants, revoke/update shares, summarise scope.                                          |
| Global navigation badges   | `web/src/components/layout/Sidebar.tsx`, notification indicators                                                              | Highlight shared resources, link to permission settings.                                              |

Supporting UI primitives (modal, table, badge, date-picker) live under `web/src/components/ui/*`; they will be reused for the share workflows.

---

## 6. Trade-offs & Considerations

- **Complexity**: Introducing resource-level grants and capability overrides increases moving parts but keeps the global registry simple.
- **Performance**: Permission checks now consult `resource_permissions`; indexing by `(resource_id, principal_type, principal_id)` is essential.
- **Migration**: Capability grants should be seeded conservatively; existing system and user roles remain unchanged.
- **Auditability**: Share creation now hits both `resource_permissions` and audit logs; ensure share changes produce log entries.
- **Storage**: JSON metadata on `resource_permissions` supports future driver-specific settings without schema churn.

---

## 7. Conclusion

The current RBAC system provides a solid base but lacks the granularity and discoverability modern workflows require. By layering immutable roles, team capability grants, resource permissions, and driver-specific registries, we can:

- Keep team-wide roles manageable while offering additive overrides.
- Allow precise, expiring, per-connection shares.
- Give UI the context it needs to guide users through protocol-specific capabilities.
- Maintain the cross-database promise of the existing schema by leveraging GORM migrations only.

This document should serve as the blueprint for the next iteration of the permission system. Action items are captured in the implementation roadmap above; once agreed upon, individual tickets can be carved out per phase.

---

## 8. Connection Sharing & Expiration Details

This overhaul subsumes the connection-sharing enhancements discussed earlier. The key deliverables are:

### 8.1 Backend APIs

- `POST /api/connections/:id/shares`: accepts `{ user_id, permission_scopes[], expires_at }`.
- `DELETE /api/connections/:id/shares/:shareId`: revokes a share.
- `GET /api/connections/:id/shares`: lists active shares (owner/admin only) including grantor, expiry, and scope metadata.
- All endpoints require `connection.share`; validation ensures the grantor cannot bestow scopes they do not possess.
- Shares are recorded in `resource_permissions` (or equivalent) with optional `expires_at` and `granted_by`.

### 8.2 Share Semantics

- Scopes map to registry entries: `connection.view`, `connection.launch`, `connection.manage`, plus protocol-scoped items (`protocol:ssh.port_forward`, `protocol:docker.exec`, etc.).
- Expired shares are ignored automatically by service lookups.
- Shares can target individual users even when the connection belongs to a team, enabling temporary access without altering team membership.

### 8.3 Frontend Experience

- Share modal (`ShareConnectionModal.tsx`, new) provides:
  - User search (using `useUsers`).
  - Scope selector derived from protocol registry.
  - Expiration selector (presets + custom datetime).
  - List of existing shares with revoke/edit actions.
- Connection cards and detail views render “Shared by” badges with scope/expiration tooltips.
- Notifications (optional follow-up) can alert recipients when a share is created or about to expire.

### 8.4 Validation & Audit

- Service layer ensures:
  - Cannot share with oneself or duplicate active shares (updates merge scopes).
  - Expiration cannot precede current time or exceed policy limit (configurable).
- Audit logs capture share grant/revoke events (`connection.share.add`, `connection.share.remove`) including principal and scope.
- Integration tests cover share creation, expiry handling, and visibility merging.
