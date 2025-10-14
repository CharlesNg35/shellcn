# SSH / SFTP Module – Implementation Plan

## 1. Goals & Scope

- Deliver a production-grade SSH launcher with optional SFTP file management, aligned with existing driver architecture and frontend guidelines.
- Introduce active-session sharing, per-session chat, and write-access delegation in a protocol-agnostic way so other drivers can adopt the same lifecycle APIs.
- Ship session recording infrastructure (storage + playback metadata) starting with SSH terminal streams, with hooks for future RDP/VNC integrations.
- Respect enterprise guardrails: permission checks, concurrency controls, auditing, session metrics, and admin-configurable defaults.

## 2. Core Decisions

- **Terminal engine**: stay with `xterm.js` (already endorsed in frontend guidelines) with WebGL addon + fit addon; defer to dynamic import boundary for lazy loading.
- **Transport**: multiplex bidirectional SSH data, control messages, and heartbeat over a dedicated WebSocket stream (`ws://.../ws/ssh/{sessionID}`) using JSON frames.
- **Recording format**: Asciinema v2 JSON (gzipped) for terminal playback; abstract writer so additional codecs (e.g., ttyrec) can plug in.
- **Storage abstraction**: new `RecorderStore` interface with filesystem (default `./data/records`) and S3 backend (leveraging existing config storage options).
- **Concurrency enforcement**: persisted per-connection limit with `unlimited` sentinel. Enforced through active session service to avoid extra DB hits.

## 3. Backend Plan

### 3.1 Domain & Persistence Changes

- Add migrations:
  - `connection_settings`: extend JSON schema with `ssh.concurrent_limit`, `ssh.allow_sftp`, `ssh.default_sharing_mode`.
  - New table `connection_sessions` capturing lifecycle (id, connection_id, protocol_id, owner_user_id, team_id, started_at, closed_at, last_heartbeat_at, status, metadata JSON).
  - New table `connection_session_participants` with composite key (session_id, user_id), roles (`owner|participant`), `access_mode` (`read`/`write`), `granted_by`, timestamps.
  - New table `connection_session_messages` for chat (session_id, author_id, message, created_at); enforce cascade delete on session close.
  - New table `connection_session_records` linking recordings (session_id FK, storage_kind `fs|s3`, path/key, bytes, duration, checksum, created_by, retention policy flags).
  - Indexes on `session_id`, `connection_id`, `team_id`, `created_at` for efficient queries and purge jobs.
- Update GORM models and repositories with validation + JSON schema upgrades (`internal/models`).

### 3.2 Services & Orchestration

- Expand `ActiveSessionService`:
  - Support multi-user participation map keyed by session ID.
  - Track `write_holder` (user id) and enforce single-writer invariant.
  - Emit new realtime events (`session.participant_joined`, `session.write_granted`, `session.chat_posted`).
  - Accept `ConcurrentLimit` attribute per connection ID to reject inbound launches when limit reached; include reason codes for UI.
  - Maintain ephemeral chat history buffer until persisted.
- Introduce `SessionLifecycleService` to bridge persistent tables with in-memory service:
  - On launch: create `connection_sessions` row, register in active service, attach owner as writer (if allowed), seed metadata (host, port, identity id, sftp enabled).
  - On close/error: update `closed_at`, set status, flush remaining chat messages, trigger recorder finalization.
  - Provide methods for sharing updates (grant/revoke, permission checks).
- Create `SessionChatService` to persist chat entries asynchronously and publish via realtime hub; enforce message length limit and sanitize HTML.

### 3.3 SSH Driver Implementation

- Package layout `internal/drivers/ssh`:
  - `driver.go`: implements metadata, capabilities, default port (22), description, health check (ensure `golang.org/x/crypto/ssh` config OK).
  - `launcher.go`: handles `drivers.Launcher` interface. Steps:
    1. Validate connection settings (host, port, keepalive, preferred auth).
    2. Resolve identity via Vault (private key, password, keyboard-interactive fallback).
    3. Dial SSH, request PTY (respect size defaults), start command shell.
    4. Register active session, spawn goroutines for stdio <-> websocket bridging.
    5. Expose control channel for resizing, snippet execution, snippet macros.
  - `permissions.go`: register protocol scopes (`connect`, `sftp`, `share`, `record`, `manage_snippets`, `grant_write`).
  - `sftp.go`: instantiate SFTP client on demand, cached per active session, closed when final tab closes.
  - `snippets.go`: placeholder to call Snippet service (existing or to implement).
- Provide driver-specific validator for concurrency limit and sftp toggle on connection create/update.

### 3.4 SFTP Backend Interfaces

- Reuse a pooled `*sftp.Client` derived from the primary SSH connection; guard access with mutex + context cancellation so transfers stop when session closes.
- REST endpoints (all gated by `protocol:ssh.sftp` and connection access checks):
  - `GET /api/active-sessions/:id/sftp/list?path=` → returns directory entries with metadata (type, size, perms, owner, mtime).
  - `POST /api/active-sessions/:id/sftp/mkdir` → create directory; payload validates parent path.
  - `POST /api/active-sessions/:id/sftp/rename` → atomic rename/move with collision handling.
  - `DELETE /api/active-sessions/:id/sftp/file` and `/directory` variants → recursive delete with max depth guard.
  - `GET /api/active-sessions/:id/sftp/download` → streams files with `Content-Length`, supports HTTP range for resumable downloads.
  - `POST /api/active-sessions/:id/sftp/upload` → multipart or tus-like chunked uploads (initial MVP: multipart with 64 MiB cap, stream directly to remote to avoid buffering).
  - `GET /api/active-sessions/:id/sftp/file` → fetch file contents for editor (size guard, e.g., max 5 MiB inline).
  - `PUT /api/active-sessions/:id/sftp/file` → overwrite file contents (optionally via temp + rename for atomicity).
- Emit transfer status over realtime hub (`sftp.transfer.queued|started|progress|completed|failed`) so frontend queue stays in sync.
- Enforce path normalization (no `..` escaping), restrict to session-specific root (default user home, optional chroot-like base from connection settings).
- Add throttling + timeout defaults (e.g., 30s idle) and map SFTP errors to user-friendly codes (permission denied, disk full, etc.).
- Log every mutating action with user + session context for auditing.

### 3.5 WebSocket Contract

- Endpoint: `GET /api/connections/:id/sessions/:sessionID/ws` (auth via current middleware).
- Subprotocol: `shellcn.ssh.v1` communicating JSON envelopes: `{type, payload}` with types `data`, `control`, `resize`, `chat`, `share`, `recording`, `error`.
- Use binary frames for raw terminal data to minimize overhead; wrap metadata in JSON `control` messages for state transitions.
- Heartbeat: 20s ping/pong with `LastSeenAt` updates in active service; session auto-closes after 60s of missed heartbeats.

### 3.6 Shared Session Logic

- API endpoints (under `/api/active-sessions/:id`):
  - `POST /shares` – owner invites participants (team, user, or "team:all"). Validate permissions (`protocol:ssh.share`).
  - `PATCH /participants/:userID` – owner toggles write access, participants can `DELETE /write` to relinquish.
  - `DELETE /participants/:userID` – remove participant, close their socket.
  - `GET /participants` – real-time list for UI sidebar (leveraging caching).
  - `POST /chat` – add chat message; also accessible via websocket `chat` event.
- Ensure participant must have base connection access (reuse resource_permissions service) before joining; otherwise return 403.
- Write access invariant: only owner or designated participant; enforce server-side for keystroke injection and SFTP mutations.

### 3.7 Session Recording

- Feature flag in config: `Protocols.SSH.EnableRecording` and tenant-level override via admin settings.
- Recorder pipeline:
  - On session start, if enabled, create recording context (temporary file) using asynchronous writer capturing `{t, stdout, stdin? optional }`.
  - Support pausing when session is read-only to avoid capturing viewer noise.
- On finalization:
  - Close writer, compute checksum, gzip, upload to store, persist metadata row.
  - Emit audit log event (`session.record.finished`).
- Provide REST endpoints:
  - `GET /api/active-sessions/:id/recording/status`
  - `POST /api/active-sessions/:id/recording/stop`
  - `GET /api/session-records/:recordID/download`
- Add retention jobs (reuse `internal/tasks` cron scheduler if exists) to purge after admin-defined TTL.

### 3.8 Auditing & Metrics

- Emit audit entries for:
  - Session launched/closed.
  - Write access granted/revoked.
  - Session shared/unshared.
  - Recording started/stopped/deleted.
- Extend Prometheus collectors: count active SSH sessions, total shared sessions, recording durations.

### 3.9 Testing Strategy (Backend)

- Unit tests:
  - ActiveSessionService enhancements (write lock, concurrency limit, share events).
  - SessionLifecycleService for happy/error paths.
  - SSH launcher with mocked `ssh.Client` (use `golang.org/x/crypto/ssh/test` utilities).
  - Recorder store implementations (filesystem + mocked S3).
- SFTP-specific unit tests covering path sanitization, error mapping, and upload pipeline (mocking `sftp.Client`).
- Integration tests:
  - Handler flows for launching, sharing, chat posting (use httptest + in-memory hub).
  - Recording: spin up fake SSH echo server capturing streams; verify stored file contents.
  - SFTP operations against dockerized OpenSSH server to validate browse/upload/download flows and transfer events.
- Load/resilience tests: script hooking go test + ssh server to ensure concurrency limit enforcement and cleanup.

## 4. Frontend Plan

### 4.1 State & Routing

- Introduce `/active-sessions/:sessionId` route that loads shared `SSHWorkspace` shell.
- Create `useSSHWorkspaceStore` (Zustand) to keep component instance per protocol, track open tabs, handshake metadata, chat log, recording state, and persisted layout options.
- Ensure store retains sockets when user navigates away (component unmount) and rehydrate on return; rely on React 19 `use` pattern for suspense boundaries.

### 4.2 Active Sessions Sidebar

- Extend existing sidebar `ActiveSessionsPanel`:
  - Group by protocol, show badges for shared/public, recording indicator, write access status.
  - Clicking session highlights associated tab (if already open) or pushes new tab into workspace store.
  - Display concurrency-limit error toast when launch rejected.

### 4.3 SSH Workspace Layout

- Tab header:
  - Primary tab per active session showing connection name + host; when multiple SSH sessions are open they appear as sibling tabs with badge for shared/recording state.
  - Secondary tab for SFTP (lazy loaded) scoped to the currently focused active session.
  - Additional tabs for other shared sessions within same workspace. Clicking an already-open session tab focuses it instead of re-mounting content.
  - Close icon available only when more than one tab; closing requests backend to leave session but not terminate owner session.
  - Maintain component instance even when switching to other protocol routes so returning users get previous terminal buffers, SFTP navigation, and splitter layout without reload.
  - When user opens a new SSH/SFTP active session from the sidebar, the workspace either:
    - Activates the existing tab if already present.
    - Creates a new tab (before SFTP tab) with initial terminal view active.
  - Tabs display small pill showing access mode (`Read`/`Write`) and session owner if user is guest.
  - Tabs and sidebar stay in sync: selecting a tab highlights the matching entry in Active Sessions list, and clicking the sidebar item focuses the tab without creating duplicates.
  - Command palette shortcut (`Cmd/Ctrl+K`) opens a switcher listing active sessions/snippets for quick navigation.
- Screen splitter:
  - Dropdown (lucide `layout-grid`) offering 1–5 columns.
  - Implement via CSS grid with dynamic columns; each split hosts either terminal or file manager component instance.
  - Persist layout per active session (stored in workspace store keyed by session ID) with global default fallback.
- Toolbar:
  - `Snippets` dropdown (loads snippet list via API; permission guard) with `Manage snippets` entry pinned to top for modal launch, followed by global snippets and connection-scoped snippets grouped by label.
  - `File Manager` toggles SFTP tab; reuses existing instance if already open.
  - `Full Screen` toggles layout by adding `is-fullscreen` class to root container; hide sidebar/global header.
- Bottom bar:
  - Zoom controls (use xterm API `setOption('fontSize')`).
  - Search toggles overlay for incremental find within terminal buffer.
  - Display host/IP, connection status, latest latency reading, recording indicator, and queued transfer count.
- Content panes follow active tab:
  - When the focused tab is a terminal session, the main pane shows xterm instance (with optional splits showing linked terminals or SFTP panes from same session).
  - When focused tab is SFTP, panes show file manager layout while keeping terminal session alive in background (no disconnect).
  - Split panes can host multiple active session terminals simultaneously (e.g., owner compares two servers); pane header displays session name for clarity.
  - Tab context menu includes “Open in new window” to spawn a dedicated workspace for power users; new window preserves session state via URL token.

### 4.4 SFTP File Manager

- Lazy-load `FileManager` component bundle when SFTP tab requested to keep initial SSH load light.
- Layout structure mirrors provided reference:
  - **Header 1 (shared)**: same tab strip as terminal view with connection tabs on the left and `layout-grid` splitter trigger on the right.
  - **Header 2 (SFTP toolbar)**:
    - Left-aligned icon buttons: `Up one level`, `Refresh`, `Home`, plus optional `New folder`, `Upload`.
    - Center: editable path input showing current directory; supports copy/paste, history dropdown, and validation feedback.
    - Right-aligned quick filters (e.g., show hidden files toggle) and transfer queue toggle.
  - **Main body split view**:
    - Left pane: directory table (TanStack Table + virtualization) with columns `Name`, `Size`, `Modified`, `Perm.`, `Actions`; rows include folder/file icons matching reference styling while keeping DOM footprint minimal.
    - Right pane: transfer queue manager with list of uploads/downloads, progress bars, pause/resume/clear buttons, and dropzone helper text matching example.
    - Split sash draggable to resize panes; persisted per session.
  - **Dropzone overlay**: global drag state shows border + icon (“Drop files to upload”) across entire workspace, using Tailwind v4 utilities.
  - **Bottom bar**: reuse shared footer component but omit zoom/search; display host/IP, status, and transfer summary (e.g., “2 uploads in progress”).
- Interactions & tooling:
  - Context menu + action bar share command handlers (download, edit, rename, delete, chmod where permitted); optimistic updates roll back on failure.
  - Monaco editor modal for inline text edits; fetch via `GET /file`, save via `PUT /file`, with unsaved change guard.
  - Large file safeguard: backend flags oversize files; UI shows download-only banner instead of opening editor.
  - Hidden file toggle (dotfiles) and sorting persist in store.
- Uploads/downloads:
  - Drag & drop multi-file support using Dropzone; progress updates from websocket `sftp.transfer.*` events.
  - Resume-friendly uploads (chunk metadata stored client-side) once backend supports tus/multipart continuation.
  - Prefetch top-level directory metadata after SSH handshake so first SFTP tab render is instant.
- Defaults & navigation:
  - Initial directory comes from backend metadata (user home, e.g., `/home/ubuntu`); path normalization enforced client-side.
  - Breadcrumb clickable segments allow quick navigation; keyboard shortcuts mirror toolbar buttons (`Alt+↑`, `Ctrl+R`, `Ctrl+H`).
- State management:
  - `useSftpStore` (Zustand) caches listings per session/path, tracks selection, toolbar preferences, and queue state.
  - React Query handles data fetching with cache keys `[sessionId, 'sftp', path]`; invalidated on transfer completion events.
  - Error boundary surfaces permission/path errors inline with retry CTA and logs structured errors.
- Support SFTP-only connections: when connection type is `sftp`, workspace opens directly to this layout (terminal header hidden) while reusing SSH identity for auth.

### 4.5 Shared Session UX

- Participant panel (right drawer) listing users with avatars, role, access mode.
- Chat box:
  - Message list virtualized for performance.
  - Input area with send + keyboard (enter to send, shift+enter newline).
  - Clear on session close (store resets on `session.closed` event).
- Access change notifications (toast + inline banner).
- Visual indicator when user lacks write access (overlay on terminal, disable keyboard input).

### 4.6 Recording Controls & Playback

- Recording toggle button (owner/admin only) where permitted.
- Show live badge when recording.
- Add Session Recordings page under `/settings/protocols` to list captured sessions (admin view).
- Build playback modal leveraging Asciinema player (embed via react component) with download option.

### 4.7 Settings & Preferences

- **Admin Protocol Settings** (`/settings/protocols`, new page component):

  - Navigation: left sidebar lists enabled protocols (SSH, RDP, etc.) using `ProtocolCatalog` data; selecting SSH opens detail panel.
  - Detail layout:
    - Header with protocol icon, description, enable/disable toggle (mirrors backend config flag).
    - Form sections:
      1. **Session Defaults**
         - Default concurrency limit (numeric input, `0` = unlimited).
         - Default idle timeout (minutes) applied to new connections.
         - Checkbox to auto-enable SFTP channel on new connections.
      2. **Terminal Appearance**
         - Theme mode select (`Auto`, `Force Dark`, `Force Light`).
         - Font family dropdown (pre-populated, supports custom entry).
         - Font size slider (8–96 px) with live preview sample.
         - Scrollback limit numeric input.
         - Toggle for enabling WebGL renderer by default.
      3. **Recording Policy**
         - Global toggle (`Disabled`, `Optional`, `Forced`) with explanatory copy.
         - Retention period (days) and storage target (`filesystem`, `s3`) selectors.
         - Checkbox to require participant consent banner.
      4. **Security & Permissions**
         - Toggle to allow shared sessions by default.
         - Option to restrict write delegation to admins.
    - Save/Reset buttons pinned to footer; dirty state warning on navigation away.
  - Data flow: uses React Hook Form + Zod; on submit calls `/api/settings/protocols/ssh` (new REST handler) which persists into `system_settings` table under keys `protocol.ssh.*`.
  - Audit log entry emitted when admin changes settings.

- **Connection Form Integration**

  - Connection create/edit forms read admin defaults and pre-fill SSH-specific fields; overrides stored in connection settings but can fall back to global when omitted.
  - When admin toggles defaults, background job recalculates caches or signals clients via realtime config update event.

- **User Preferences** (`/settings/preferences`)
  - Terminal appearance overrides (font, colors, cursor style, key bindings) saved per user; applied to sessions unless connection forces admin policy.
  - SFTP preferences: default view mode (list/grid, future), whether to auto-open queue, remember hidden-files toggle.
  - Personal snippet collections accessible via same preferences panel (ties into snippets permissions).

### 4.8 Testing Strategy (Frontend)

- Component unit tests using Vitest + Testing Library:
  - Workspace store behavior, tab persistence, reducer logic.
  - Toolbar actions (snippets, file manager toggle, full screen).
  - Chat input: ensures blank/oversized message handling.
  - SFTP table sorting + actions (mock API).
- Integration tests (Playwright):
  - Launch SSH session, open SFTP, split view operations.
  - Shared session: owner grants write, participant receives indicator.
  - Recording banner toggle.
- Performance budgets: confirm lazy bundles (terminal, monaco) load separately (check Vite build report).

## 5. Cross-Cutting Concerns

- **Security**: sanitize all chat/file paths; enforce path whitelisting on SFTP (no root escape). All websocket messages validated server-side.
- **Permissions**: update constants for new scopes; ensure backend enforces on each endpoint (share, record, file operations).
- **Internationalization**: wrap UI strings in translation helper where available.
- **Accessibility**: respect ARIA roles for toolbar buttons, chat input; maintain keyboard navigation.
- **Logging**: structured logs for session events with session_id, connection_id, user_id; mask sensitive data.
- **Performance**: enforce lazy-loaded bundles, memoized selectors, virtualized lists, and requestAnimationFrame batching for terminal writes; monitor with Web Vitals in dev builds.

## 6. Phased Delivery

1. **Phase 0 – Foundations**
   - Migrations, models, ActiveSessionService upgrades, protocol standards update.
   - Baseline SSH driver (connect/disconnect, terminal streaming) without sharing or SFTP.
2. **Phase 1 – SFTP & UI Shell**
   - File manager API + UI, toolbar, splitter, preferences.
3. **Phase 2 – Shared Sessions**
   - Participant management, chat, write delegation, UI synchronization.
4. **Phase 3 – Recording**
   - Recording pipeline, admin toggles, playback surfaces.
5. **Phase 4 – Hardening**
   - Load tests, audit integration, documentation, final QA.

## 7. Risks & Mitigations

- **Terminal performance**: mitigate with xterm WebGL addon, throttle output (batch writes), and optional compression.
- **Recording storage growth**: enforce retention policy + configurable quota alerts.
- **Concurrency race conditions**: rely on atomic operations in ActiveSessionService and DB transactions for participant updates.
- **Complex UI state**: centralize workspace store and thoroughly test rehydration; provide fallback to reload session if stale.
- **Security of shared sessions**: ensure permission checks on every event, emit audit log for write grants, allow owner to eject participants quickly.

## 8. Open Questions

- Should chat history persist beyond session lifetime for auditing? (Currently cleared on close per requirements; confirm with stakeholders).
- Do we need per-team default concurrency overrides? (Connection-level planned, but team policy may be requested).
- Preferred lifecycle for orphaned recording files if DB record missing? (Consider background reconciliation job).

### 4.9 Optional Enhancements & Optimizations

- Per-user SSH session caps: allow admins to define maximum concurrent SSH sessions per user or team; ActiveSessionService enforces limit and surfaces alert toast + audit event when hit.
- Adaptive session recording: monitor bandwidth/latency; auto-throttle recording frame rate or pause capture when terminal latency exceeds threshold to favor interactivity.
- Terminal diff batching: coalesce outbound terminal writes (e.g., 50ms frames) to reduce DOM churn, improve battery use, and shrink recording size.
- Split-pane skeletons: render lightweight placeholders while xterm/SFTP bundles lazy-load so multi-column layouts remain visually stable.
- SFTP inline previews: provide quick preview pane for small text/images without launching editor/download; supports tabbed preview panel inside file manager.
- Collaboration cues: color-code read/write pills and terminal border; optional chat banner reminding owners they are sole write delegates.
- Chat quick actions: embed contextual buttons (e.g., “Grant write”, “Open SFTP”) inside chat messages for owners/admins to act faster.
- Connection escape hatch: from shared session UI provide “View Connection Details” link opening connection drawer/page for share or settings adjustments.

### 4.10 Performance Guardrails

- Budget terminal frame throughput to ≤120 fps by batching write events and deferring non-critical UI work with `requestIdleCallback`.
- Use TanStack Query `staleTime`/`gcTime` to avoid refetch storms; prefetch next probable SFTP directories but cap memory via LRU cache.
- Memoize expensive React components (terminal panes, transfer lists) and leverage `React.useTransition` for low-priority updates (chat, badges).
- Run bundle analyzer each release; target <300 KB initial SSH workspace chunk by pushing Monaco/xterm addons/file previews behind dynamic imports.
- Ship WebSocket message compression (permessage-deflate) where available to reduce network usage on chat/transfer updates.
- Monitor runtime via Web Vitals + custom metrics (render time, memory) in non-prod environments; fail build if thresholds regress.
