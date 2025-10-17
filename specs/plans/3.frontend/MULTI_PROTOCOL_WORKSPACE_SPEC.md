# Multi-Protocol Connection Launch & Workspace Revamp

This specification covers the end-to-end changes required to deliver a consistent, protocol-agnostic launch flow and workspace experience that takes full advantage of connection templates.

---

## 1. Current Gaps

| Area                   | Observations                                                                                                                                                                                                    | Source files                                                                                                                                                                                               |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Launch UI              | “Launch” buttons simply navigate to `/connections/:id`, a non-existent route. No session is ever created.                                                                                                       | `web/src/components/connections/ConnectionCard.tsx`, `web/src/components/layout/Sidebar.tsx`, `web/src/pages/connections/Connections.tsx`                                                                  |
| Launch backend         | There is no API that creates an active session on demand (`POST /api/active-sessions` is missing). Launch still assumes direct websocket tunnel.                                                                | `internal/handlers/ssh_session.go`, `internal/handlers/realtime.go`                                                                                                                                        |
| Workspace architecture | Only SSH is supported. `SshWorkspace.tsx`, `SftpWorkspace.tsx`, `useSshWorkspaceTabsStore.ts`, `useSshWorkspaceStore.ts` are protocol-specific.                                                                 | `web/src/pages/sessions/SshWorkspace.tsx`, `web/src/components/workspace/SftpWorkspace.tsx`, `web/src/store/ssh-session-tabs-store.ts`, `web/src/store/ssh-workspace-store.ts`                             |
| Active sessions UX     | `useActiveSshSession.ts` and UI consumers hard-code `protocol_id: 'ssh'`; sidebar and settings lists cannot resume non-SSH sessions.                                                                            | `web/src/pages/sessions/ssh-workspace/useActiveSshSession.ts`, `web/src/hooks/useActiveConnections.ts`, `web/src/components/layout/Sidebar.tsx`, `web/src/pages/settings/Sessions.tsx`                     |
| Template usage         | Frontend still reads legacy fields (e.g., `settings.host`, `settings.recording_enabled`). `connection.metadata.connection_template.fields` and targets are ignored. SFTP tabs load even when template disabled. | `web/src/components/connections/ConnectionCard.tsx`, `web/src/pages/sessions/SshWorkspace.tsx`, `web/src/components/workspace/SftpWorkspace.tsx`, `web/src/components/connections/ConnectionFormModal.tsx` |

---

## 2. Goals

1. **Launch Flow**: Provide a template-aware modal that launches or resumes sessions through a new `POST /api/active-sessions` endpoint.
2. **Workspace Framework**: Replace SSH-only code with a registry-driven system capable of mounting protocol-specific panes.
3. **Template Fidelity**: Display template-derived fields (targets, flags) throughout the UI and honour feature toggles (e.g., SFTP).
4. **Active Session Cohesion**: Ensure sidebar and settings can resume any protocol session and show appropriate metadata.
5. **Extensibility**: Make the addition of future protocols (Telnet, RDP, Docker, Kubernetes, databases) primarily a configuration exercise.

Non-goals: Implementing every future protocol workspace; rewriting websocket infrastructure; refactoring backend drivers beyond launch contract.

---

## 3. Launch Flow Redesign

### 3.1 Frontend (“Launch Assistant”)

Create a dedicated hook and modal:

- **New files**: `web/src/hooks/useLaunchConnection.ts`, `web/src/components/connections/LaunchConnectionModal.tsx`.
- **Responsibilities**:
  1. Fetch latest connection detail + template snapshot (`useConnectionTemplate.ts` already exists; reuse).
  2. Show summary (name, tags, template fields, identity status, last used).
  3. If there are existing active sessions (from `useActiveConnections`), list them with “Resume” actions.
  4. Provide protocol-specific launch options supplied by workspace registry (see §4.1).
  5. Submit to new API (`POST /api/active-sessions`): on success navigate to workspace route (descriptor-provided).
  6. Handle validation errors (missing identity, concurrency limits, template mismatch).

Update call sites:

- `ConnectionCard.tsx` primary CTA uses `useLaunchConnection`.
- `Sidebar.tsx` active session cards call registry resume path instead of `/connections/:id`.
- `Connections.tsx` bulk create flow sets selected protocol and opens launch modal if desired.

### 3.2 Backend Launch Endpoint

Implement `POST /api/active-sessions`:

| Concern        | Details                                                                                                                                                                                                                                                                                                                                                        | Files                                                                       |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- |
| Routing        | Add to `internal/api/routes_connection_sessions.go` (or new file `routes_active_sessions.go`).                                                                                                                                                                                                                                                                 | API router                                                                  |
| Handler        | New handler `internal/handlers/active_session_launch.go`.                                                                                                                                                                                                                                                                                                      | Handler                                                                     |
| Request schema | `{ "connection_id": string, "protocol_id"?: string, "fields_override"?: map }`.                                                                                                                                                                                                                                                                                | Handler DTO                                                                 |
| Validation     | - Ensure user has `connection.launch`. <br> - Load connection via `ConnectionService`. <br> - Verify template version (compare against stored `connection.metadata.connection_template.version`). Warn or reject if mismatch. <br> - Validate overrides using `ConnectionTemplateService.Materialise`. <br> - Check concurrency via `SessionLifecycleService`. | `ConnectionService`, `ConnectionTemplateService`, `SessionLifecycleService` |
| Launch         | - Resolve driver via registry. <br> - Acquire identity secret via `VaultService`. <br> - Call driver `Launch`. <br> - Register active session via `SessionLifecycleService.StartSession`.                                                                                                                                                                      | `internal/handlers/ssh_session.go` (reference for logic)                    |
| Response       | `{ "session": ActiveConnectionSessionDTO, "tunnel": { "url", "token", "expires_at" }, "descriptor": WorkspaceDescriptorDTO }`. <br> Use existing JWT service (`internal/handlers/realtime.go`) to mint tunnel token.                                                                                                                                           | Handler                                                                     |

### 3.3 Documentation

- Update `specs/plans/MODULES_API.md` with endpoint description, request/response, permissions.
- Update `specs/project/PROTOCOL_DRIVER_STANDARDS.md` to reference launch contract.

---

## 4. Workspace Framework

### 4.1 Protocol workspace registry

- **New module**: `web/src/workspaces/protocolWorkspaceRegistry.ts`.
- **Structure**:

  ```ts
  export interface WorkspaceDescriptor {
    id: string;
    icon: LucideIcon;
    displayName: string;
    mount: React.ComponentType<WorkspaceMountProps>;
    defaultRoute: (sessionId: string) => string;
    tabs: TabFactory[];
    panes: PaneDefinition[];
    actions?: ActionDefinition[];
    features: {
      supportsSftp?: boolean;
      supportsRecording?: boolean;
      supportsSharing?: boolean;
      supportsSnippets?: boolean;
      [key: string]: boolean | undefined;
    };
    launchOptions?: LaunchOptionDefinition[]; // injected into launch modal
  }
  ```

- `WorkspaceMountProps` includes session data, template snapshot, descriptor, `ProtocolWorkspaceStore`, tunnel info.
- Provide a fallback descriptor for unknown protocols (“Workspace coming soon”).

### 4.2 Shared layout & components

Create reusable shells under `web/src/workspaces/components`:

- `ProtocolWorkspaceLayout.tsx`: standard layout (toolbar, tabstrip, pane container, status bar, bottom panels).
- `ProtocolWorkspaceToolbar.tsx`, `ProtocolWorkspaceStatus.tsx`, `ProtocolWorkspaceParticipants.tsx`.
- Tabs/panes should be receiving definitions from descriptor (e.g., Terminal, Files, Dashboard).

### 4.3 Store refactor

Rename SSH stores and generalise:

- `ssh-session-tabs-store.ts` → `protocol-workspace-tabs-store.ts`.
- `ssh-workspace-store.ts` → `protocol-workspace-store.ts`.

Key changes:

- State keyed by `{ protocolId, sessionId }` rather than just session id.
- Keep tab definitions generic with `type` string (e.g., `"terminal"`, `"files"`, `"logs"`, `"custom"`).
- Provide helpers for protocol-specific defaults via registry (call `descriptor.tabs` when workspace created).
- Update dependent hooks/components (`useSessionTabsLifecycle.ts`, `useWorkspaceSnippets.ts`, `SftpWorkspace.tsx`, `SshWorkspace.tsx`, `TransferSidebar.tsx`, etc.).

### 4.4 Data contract

Update backend metadata:

- In `SessionLifecycleService.StartSession`, include:
  - `metadata["template"]` (fields, version, driver).
  - `metadata["capabilities"]` (e.g., `{ "panes": ["terminal", "files"], "features": { "supportsSftp": true, ... } }`).
- `SSHSessionHandler` should set `metadata["capabilities"]` using descriptor features and `metadata["template"]["fields"]`.

Update routing:

- `web/src/App.tsx` must import the workspace registry and delegate to descriptor-provided `defaultRoute`/`mount` components so new protocols can plug in without manual route additions. For now, keep `/active-sessions/:sessionId` pointing at a thin orchestration component that looks up the descriptor and renders it.

Update types:

- `web/src/types/connections.ts` → extend `ActiveConnectionSession` with `descriptor_id?: string`, `capabilities?: {...}`, `template?: ConnectionTemplateMetadata`.
- Ensure API responses (handlers + DTOs) serialise new fields.

---

## 5. Template Integration

1. **Connection summaries** (cards, lists, dashboard):

   - Derive endpoint from first `ConnectionTarget` (host/port) before falling back to `settings`.
   - Display important template fields (configurable list per protocol via registry).
   - Indicate template version and mismatch warnings.

2. **Launch modal**:

   - Show template version + driver version; if mismatch, warn user (“Update connection before launching”).
   - Highlight disabled features (e.g., SFTP toggle off) and propagate to workspace via metadata (`metadata.sftp_enabled`).

3. **Workspace behaviour**:

   - `SshWorkspace.tsx` / `SftpWorkspace.tsx` should consume `capabilities.supportsSftp` before rendering Files tab.
   - `recording_enabled` should use metadata (`session.metadata.recording`, as already done but ensure consistent).
   - Parameterise snippet availability via descriptor (`supportsSnippets`).

4. **Connection edit**:
   - `ConnectionFormModal.tsx` already persists template fields; ensure metadata `connection_template` adds `fields` map (`ConnectionTemplateService.Materialise` already returns `Fields`: verify `connection_service.go` merges into metadata).

---

## 6. Active Sessions UX

1. **Hook**: refactor `useActiveConnections` (web/src/hooks/useActiveConnections.ts) to accept `{ protocolIds?: string[], teamId?, scope?, enabled?, refetchInterval? }` and return descriptor info.
2. **Sidebar**: update `Active Sessions` section to render protocol icon/name via registry; resume using `descriptor.defaultRoute(session.id)`.
3. **Settings > Sessions** (`web/src/pages/settings/Sessions.tsx`):
   - Add protocol filter.
   - Display template fields (e.g., host, port) and features (recording, SFTP).
4. **Workspace resume hooks**: `useActiveSshSession.ts` becomes `useActiveSession.ts` (protocol-agnostic). Filter by session id only.

---

## 7. Implementation Plan

### 7.1 Backend

1. **Launch endpoint**:
   - Add handler + route (`internal/handlers/active_session_launch.go`, `internal/api/routes_connection_sessions.go`).
   - Extend services (`SessionLifecycleService`, `ConnectionTemplateService`) to support metadata described above.
2. **Session metadata**:
   - Update `internal/handlers/ssh_session.go` to set `metadata["capabilities"]`, `metadata["template"]`, `metadata["sftp_enabled"]`.
   - Ensure `ActiveConnectionService` returns new fields (DTO mapping in `internal/services/connection_service.go` and `internal/services/connection_service_test.go`).
3. **Realtime ticket**:
   - Reuse `RealtimeHandler.Stream` token generation; expose a helper to create tunnel tokens without HTTP request.

### 7.2 Frontend

1. Launch assistant hook + modal; update CTA call sites.
2. Implement workspace registry, shared layout components, and descriptor contract.
3. Migrate SSH workspace to registry system (Terminal, Files panes).
4. Generalise stores (`protocol-workspace-tabs-store`, `protocol-workspace-store`) and their consumers.
5. Update hooks (`useActiveConnections`, `useActiveSession` (new name)).
6. Surface template info in UI (cards, dashboards, settings).
7. Adjust tests:
   - Component tests for new launch modal.
   - Hooks tests for `useActiveConnections`.
   - Workspace tests (update snapshots due to new layout).

### 7.3 Documentation & Specs

- Update `specs/plans/MODULES_API.md` with launch endpoint description.
- Update `specs/project/PROTOCOL_DRIVER_STANDARDS.md` with new workspace descriptor expectations.
- Update onboarding docs (README) referencing new launch flow.

---

## 8. Risks & Mitigations

| Risk                                  | Mitigation                                                                                                                                |
| ------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Launch token generation correctness   | Reuse existing realtime JWT service; add unit tests for new helper.                                                                       |
| SSH regression (terminal/SFTP)        | Keep incremental migration with feature flag; run existing Vitest suites (`SshWorkspace`, `SftpWorkspace`) and add new integration tests. |
| Template version mismatch edge cases  | Provide opt-in override on launch modal (“Launch anyway”), log telemetry for out-of-date templates.                                       |
| Future protocols require custom panes | Allow descriptors to lazily load components to avoid bundling all protocols up front.                                                     |

---

## 9. Success Criteria

- Launch buttons initiate sessions via modal and redirect to functional workspace.
- Sidebar/Settings lists allow resuming sessions for any protocol descriptor registered.
- Template flags (e.g., `enable_sftp` false) result in appropriate UI (no Files tab, metadata indicator).
- Adding a new protocol requires: (1) driver declares template + descriptor metadata; (2) front-end appends descriptor to registry with minimal bespoke code.
- Existing SSH functionality remains working (terminal, SFTP transfers, recording, sharing).

---

## 10. File Inventory & Ownership

| Category               | Files (existing)                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Files (new/renamed)                                                                                                                                                                                                                                                                                                      |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **Backend**            | `internal/handlers/ssh_session.go`, `internal/handlers/realtime.go`, `internal/services/session_lifecycle_service.go`, `internal/services/connection_service.go`, `internal/services/connection_template_service.go`                                                                                                                                                                                                                                                                                                          | `internal/handlers/active_session_launch.go` (new), potential DTO updates in `internal/handlers/types/active_session.go` (new), router updates                                                                                                                                                                           |
| **API Specs**          | `specs/plans/MODULES_API.md`, `specs/project/PROTOCOL_DRIVER_STANDARDS.md`                                                                                                                                                                                                                                                                                                                                                                                                                                                    | N/A (update only)                                                                                                                                                                                                                                                                                                        |
| **Launch UI**          | `web/src/components/connections/ConnectionCard.tsx`, `web/src/components/layout/Sidebar.tsx`, `web/src/pages/connections/Connections.tsx`, `web/src/components/connections/ConnectionFormModal.tsx`, `web/src/hooks/useConnectionTemplate.ts`                                                                                                                                                                                                                                                                                 | `web/src/hooks/useLaunchConnection.ts` (new), `web/src/components/connections/LaunchConnectionModal.tsx` (new)                                                                                                                                                                                                           |
| **Workspace**          | `web/src/pages/sessions/SshWorkspace.tsx`, `web/src/components/workspace/SftpWorkspace.tsx`, `web/src/components/workspace/SshWorkspaceToolbar.tsx`, `web/src/store/ssh-session-tabs-store.ts`, `web/src/store/ssh-workspace-store.ts`, `web/src/pages/sessions/ssh-workspace/useActiveSshSession.ts`, `web/src/pages/sessions/ssh-workspace/useSessionTabsLifecycle.ts`, `web/src/pages/sessions/ssh-workspace/useWorkspaceSnippets.ts`, `web/src/pages/sessions/ssh-workspace/useCommandPaletteState.ts`, `web/src/App.tsx` | `web/src/workspaces/protocolWorkspaceRegistry.ts` (new), `web/src/workspaces/components/ProtocolWorkspaceLayout.tsx` (new), renamed stores (`protocol-workspace-tabs-store.ts`, `protocol-workspace-store.ts`), `web/src/hooks/useActiveSession.ts` (new), update `web/src/App.tsx` to register descriptor-driven routes |
| **Active Sessions UX** | `web/src/hooks/useActiveConnections.ts`, `web/src/components/layout/Sidebar.tsx`, `web/src/pages/settings/Sessions.tsx`                                                                                                                                                                                                                                                                                                                                                                                                       | Update existing                                                                                                                                                                                                                                                                                                          |
| **Templates**          | `web/src/components/connections/ConnectionCard.tsx`, `web/src/pages/dashboard/Dashboard.tsx`, `web/src/components/connections/ConnectionTemplateForm.tsx`, `web/src/components/connections/connectionTemplateHelpers.ts`                                                                                                                                                                                                                                                                                                      | Update existing                                                                                                                                                                                                                                                                                                          |

---

## 11. Follow-up (Post MVP)

- Telemetry: track launch modal usage, warnings (template mismatch).
- Session persistence: allow reopening last workspace layout per connection.
- Notifications: toast upon session creation for long-running driver launches.
- Protocol-specific quick actions: autopopulate command palette entries from descriptor.

---
