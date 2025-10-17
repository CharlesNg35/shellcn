# Multi-Protocol Launch & Workspace Implementation Plan

This document turns the revamp spec into implementation steps with clear file ownership and checklists. See `specs/plans/3.frontend/MULTI_PROTOCOL_WORKSPACE_SPEC.md` for the high-level rationale.

---

## 1. Backend Tasks

### 1.1 Session Launch API

- [ ] Add `POST /api/active-sessions` route (`internal/api/routes_connection_sessions.go` or new file).
- [ ] Implement handler `internal/handlers/active_session_launch.go`:
  - Load connection & permissions via `ConnectionService`.
  - Validate template version vs. stored metadata.
  - Use `ConnectionTemplateService` to materialise overrides.
  - Resolve identity via `VaultService`.
  - Launch driver, start lifecycle (`SessionLifecycleService`).
  - Generate tunnel token (reuse `RealtimeHandler` helpers).
  - Return session DTO + tunnel info + workspace descriptor id.
- [ ] Create handler DTOs/tests (e.g., `internal/handlers/launch_test.go`).

### 1.2 Session Metadata Enhancements

- [ ] Extend `SessionLifecycleService.StartSession` to accept `capabilities` & `template` metadata.
- [ ] Update `SSHSessionHandler` to populate:
  - `metadata["template"]` (driver id, version, fields snapshot).
  - `metadata["capabilities"]` (panes array, feature flags).
  - `metadata["sftp_enabled"]`.
- [ ] Ensure `ActiveConnectionSession` DTO (and tests) serialise new fields.

### 1.3 Documentation

- [ ] Update `specs/plans/MODULES_API.md` with the new launch endpoint.
- [ ] Update `specs/project/PROTOCOL_DRIVER_STANDARDS.md` with workspace descriptor expectations.

---

## 2. Frontend Tasks

### 2.1 Launch Assistant

- [ ] Implement `useLaunchConnection` hook (`web/src/hooks/useLaunchConnection.ts`).
- [ ] Build `LaunchConnectionModal.tsx` (summary, identity warning, template version check, overrides).
- [ ] Replace CTA in `ConnectionCard.tsx`, `Sidebar.tsx`, `Connections.tsx` to use the hook.
- [ ] Handle resume scenarios when sessions already exist (list sessions in modal).

### 2.2 Workspace Registry & Layout

- [ ] Create `protocolWorkspaceRegistry.ts` with descriptor contract.
- [ ] Add shared layout components (`ProtocolWorkspaceLayout.tsx`, toolbar, status).
- [ ] Introduce protocol-agnostic stores (`protocol-workspace-store.ts`, `protocol-workspace-tabs-store.ts`) and update imports.
- [ ] Update `App.tsx`: orchestrator route that looks up descriptor and renders workspace.
- [ ] Migrate SSH workspace to registry (descriptor providing terminal/files panes).
- [ ] Respect `session.metadata.sftp_enabled` before mounting Files tab.

### 2.3 Hooks & Active Sessions

- [ ] Rename `useActiveSshSession.ts` → `useActiveSession.ts`, remove protocol filter.
- [ ] Generalise `useActiveConnections` to accept optional `protocolIds`.
- [ ] Update sidebar (`Sidebar.tsx`) to use descriptor resume path.
- [ ] Update Settings Sessions page to show protocol-aware info.

### 2.4 Template Usage Enhancements

- [ ] Display template fields in `ConnectionCard`, dashboard, etc.
- [ ] Show template version mismatch warning.
- [ ] Launch modal should display capability toggles (e.g., SFTP disabled).

### 2.5 Tests & QA

- [ ] Add unit/integration tests for launch modal and hooks.
- [ ] Update Vitest suites (`SshWorkspace`, `SftpWorkspace`, `SessionFileManager`).
- [ ] Manual QA of SSH flows (launch, SFTP, recording, sharing).

### 2.6 Documentation/Telemetry (optional follow-up)

- [ ] Update README/onboarding docs.
- [ ] Add telemetry for launch modal usage (future).

---

## 3. UX Checklist

- Launch modal includes: name, tags, host, identity status, version mismatch warning.
- Resume button(s) present when active sessions exist.
- Workspace displays terminal/files tabs only when appropriate.
- Sidebar shows protocol icon/name and resumes sessions via registry route.
- Settings Sessions page lists sessions with protocol badges and template metadata.

---

## 4. Sequencing Guidance

1. Backend API + metadata (1.x).
2. Launch assistant (2.1).
3. Workspace registry & SSH migration (2.2).
4. Hook refactors & sidebar/settings updates (2.3).
5. Template info UX (2.4).
6. Tests/docs (2.5/2.6).

CI can be updated incrementally; feature-flag the new launch flow if necessary (optional).

---
