# Connection Sharing & Permission Granularity Plan

**Status:** Draft  
**Owner:** Platform Team  
**Last Updated:** 2025-01-19

---

## 1. Problem Statement

Connections today can only be grouped by team or personal folders, and visibility is effectively all-or-nothing once a user can see a folder. The backend already models connection visibilities, but the UI has no affordance for sharing a connection with an individual user, limiting collaboration workflows (e.g. onboarding contractors or cross-team SREs). Additionally, permissions are coarse: we cannot express “read-only” vs “launch” on a single connection, and protocol-specific privileges (e.g. `ssh.port_forward`) are not surfaced.

We need a holistic approach that:

1. Allows admins or delegates to share specific connections with users outside of folder scope, optionally with expiry.
2. Exposes fine-grained permission levels per share.
3. Surfaces sharing state to recipients (e.g. who shared it, how long they retain access).
4. Integrates driver-specific permission sets so UI and automation understand what capabilities exist (SSH vs Docker vs VNC).
5. Seeds sensible defaults for “User” role permissions.

All changes must remain portable across SQLite/PostgreSQL/MySQL (no raw SQL migrations).

---

## 2. Goals & Non‑Goals

### Goals

- Add share-with-user support with optional expiration.
- Represent per-share permission levels (view, launch, manage, protocol extras).
- Display shared metadata in connection lists and detail panes.
- Provide API to fetch protocol-specific permissions (via the registry).
- Improve default permission seeding for the built-in `user` role.
- Document the end-to-end approach (this plan).

### Non-Goals

- Implement protocol-specific business logic (e.g. enforcing SSH port-forwarding) beyond permission scaffolding in this phase.
- Replace existing folder/team sharing workflows.
- Modify the core permission registry beyond registering driver modules (that will happen per driver implementation).

---

## 3. Proposed Architecture

This plan now aligns with the broader permission-system overhaul (see `permission_system_overhaul.md`).

### 3.1 Backend Model Changes

1. **`resource_permissions` table**

   - Created via GORM migration; captures fine-grained permissions per resource. Fields:
     - `resource_id`, `resource_type` (`"connection"` for this effort).
     - `principal_type` (`"user"`/`"team"`), `principal_id`.
     - `permission_id` (e.g. `connection.launch`, `protocol:ssh.port_forward`).
     - `expires_at`, `granted_by`, `metadata` (JSON for protocol extras).
   - Supersedes the ad-hoc columns we previously considered for `connection_visibilities` (that table may reference or be merged into this structure).

2. **Share service / visibility updates**

   - CRUD helpers manage entries in `resource_permissions` with validation (expiry, scope inheritance, audit logging).
   - Expired entries ignored during evaluation; optional background cleanup.

3. **Connection listing (`ConnectionService.ListVisible`)**

   - Aggregate role permissions + resource grants to determine effective access.
   - Include in API payload: `is_shared`, `shared_from`, `shared_scopes[]`, `share_expires_at` when the caller relies on a resource grant.

4. **Protocol permissions endpoint**

   - `/api/protocols/available` returns `permissions` array per driver (id, label, dependencies) for UI automation.
   - `permissions.GetByModule(protocolID)` provides the data; drivers register entries like `ssh.connect`, `ssh.port_forward`.

5. **Share management APIs**
   - `POST /api/connections/:id/shares` accepts `{ user_id, permission_scopes[], expires_at }`.
   - `DELETE /api/connections/:id/shares/:shareId` revokes a grant.
   - `GET /api/connections/:id/shares` lists active grants (owner/admin). Responses include grantor, scopes, expiry, and protocol metadata.

### 3.2 Frontend Workflow

1. **Connection creation wizard**

   - When a team is selected, call `/api/teams/:id/capabilities` to pull effective permissions.
   - If the chosen protocol requires permissions the team lacks, surface suggestions:
     - “Grant team `<permission>` (opens role modal).”
     - “Share this connection with selected users and give them `<permission>`.”
   - Auto-select default icon/color and permission toggles based on protocol registry metadata.

2. **“Share Connection” modal**

   - Entry point in `ConnectionCard` overflow menu (guarded by `connection.share`).
   - Components: user autocomplete (`useUsers`), permission scope checklist derived from `/api/protocols/:id/permissions`, expiration presets, summary of inheritances.
   - Shows existing resource grants with revoke/edit options.

3. **Connection list badges**

   - Display “Shared by …” chip when `is_shared` is true.
   - Tooltips show granted scopes, expiration, and grantor; filters allow “Shared with me” view.

4. **Permissions administration surfaces**

   - `settings/Permissions.tsx` and `settings/Teams.tsx` expose driver permission bundles and highlight team coverage gaps.
   - When an admin adds a template/role, UI previews which connections/protocols will become available.

5. **Validation & automation**
   - Prevent granting scopes the sharer lacks (client + server validation).
   - Warn if expiry is missing or beyond policy.
   - Optional reminder to send notification or set auto-expire.

### 3.3 Permissions Strategy

1. **Permission registry usage**

   - Drivers (SSH, VNC, Docker, Proxmox, etc.) register their permissions in init() of their module (e.g. `ssh.connect`, `ssh.port_forward`, `docker.container.exec`).
   - Registry ensures dependencies; `ValidateDependencies()` already runs at seed time.

2. **Scopes mapping**

   - Standard scopes map to existing core permissions:
     - `view` → `connection.view`
     - `launch` → `connection.launch`
     - `manage` → `connection.manage`
   - Protocol extras follow naming `protocol:<module.permission>` (e.g. `protocol:ssh.port_forward`).

3. **Default role seeding**
   - Update user role to include:
     - `notification.view`
     - `team.view`
     - `connection.view`
     - `connection.launch`
     - `connection.folder.view`
     - `vault.use_shared`
   - Admin remains full-access via `ensureRoleHasAllPermissions`.

---

## 4. API Surface Summary

| Endpoint                               | Method | Description                                           |
| -------------------------------------- | ------ | ----------------------------------------------------- |
| `/api/connections/:id/shares`          | POST   | Create/replace share for user.                        |
| `/api/connections/:id/shares/:shareId` | DELETE | Revoke share.                                         |
| `/api/connections/:id/shares`          | GET    | List current shares (owner/admin only).               |
| `/api/protocols/available`             | GET    | Already exists; add `permissions` array per protocol. |

All requests require CSRF + auth as per existing middleware. Response payloads must include the new fields to support UI rendering.

---

## 5. UI/UX Summary

1. **Connections list**

   - Add share badge in cards + tooltip with sharer, scope, expiry.
   - Filter chips to view “Shared with me”.

2. **Connection detail**

   - “Shares” tab listing recipients, permission scope, expiry, action buttons (extend/revoke).

3. **Creation/edit modals**

   - Protocol permission hints to align with registry.

4. **Validations**
   - Prevent expiry earlier than now, highlight near-expiry shares.
   - Confirm destructive actions (revoke).

---

## 6. Rollout Plan

1. **Phase 1 – Backend foundation**
   - Extend schema via GORM auto-migrate.
   - Implement share CRUD service + APIs (with tests).
   - Return share metadata in connection list/detail responses.
2. **Phase 2 – Frontend share modal**
   - Build share management UI.
   - Add list badges + “Shared” filter.
3. **Phase 3 – Protocol permission exposure**
   - Update protocol API + frontend consumption.
   - Provide “Permissions required” section in driver-specific UI.
4. **Phase 4 – QA & docs**
   - Regression tests (units + integration).
   - Update user onboarding guide to explain sharing and expiration.

---

## 7. Risk & Mitigation

| Risk                                            | Mitigation                                                                    |
| ----------------------------------------------- | ----------------------------------------------------------------------------- |
| Expired shares linger                           | Filter by `expires_at` in SQL queries and background cleanup cron (optional). |
| Permission mismatch between registry and driver | Enforce registration in driver init; CI fails if dependencies unresolved.     |
| UI confusion around scopes                      | Provide concise descriptions and default to most common (`launch`).           |
| Database portability                            | Stick to GORM schema changes; avoid raw SQL.                                  |

---

## 8. Open Questions

1. Should expiry defaults be configurable per tenant? (Likely yes — future config flag).
2. How to audit share activity? (Add audit entries on grant/revoke similar to user operations).
3. Should shared connections auto-create notifications? (Recommended—`notification.view` already seeded; follow-up task).

---

## 9. Next Steps

1. Implement schema + service changes (Phase 1).
2. Update API contracts and unit tests.
3. Build frontend share management modal.
4. Extend protocol registry usage once driver modules register their permissions.
5. Update documentation and onboarding flows.
