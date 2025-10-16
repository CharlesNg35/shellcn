# SSH / SFTP Module – Implementation Plan

## 1. Goals & Scope

- Deliver a production-grade SSH launcher with optional SFTP file management, aligned with existing driver architecture and frontend guidelines.
- Introduce active-session sharing, per-session chat, and write-access delegation in a protocol-agnostic way so other drivers can adopt the same lifecycle APIs.
- Ship session recording infrastructure (storage + playback metadata) starting with SSH terminal streams, with hooks for future RDP/VNC integrations.
- Respect enterprise guardrails: permission checks, concurrency controls, auditing, session metrics, and admin-configurable defaults.

## 2. Core Decisions

- **Terminal engine**: `xterm.js` (v5+) with WebGL addon + fit addon; lazy-load via dynamic import.
- **Transport**: multiplex bidirectional SSH data, control messages, and heartbeat over WebSocket (`ws://.../ws?tunnel=ssh&connection_id={connectionID}`). Binary frames are proxied for terminal data; control messages remain JSON.
  - Frontend connects via the shared websocket utility (`web/src/lib/utils/websocket.ts`) using `buildWebSocketUrl('/ws', { tunnel: 'ssh', connection_id, token })`. The `token` parameter carries the bearer token so the server can authenticate the tunnel outside of the legacy `/ws/:stream` path.
  - UI consumers must subscribe to `ssh.terminal` events on the existing realtime stream helper (`web/src/lib/realtime/useRealtimeStream.ts` / equivalent). These events bubble up from the backend terminal bridge and include `session_id`, `connection_id`, payload (`stdout`, `stderr`, etc.), and resize metadata. Handlers should decode base64 payloads before writing to xterm.
  - For launch lifecycle UI (ready/opened/closed/error), listen to the same stream and update workspace state accordingly (e.g. mark session as ready, attach participants, surface errors). The tunnel connection itself continues to deliver binary frames for the active buffer; the broadcast stream is purely for passive observers/state updates.
- **Recording format**: Asciinema v2 JSON (gzipped) for terminal; extensible codec registry for future RDP/VNC.
- **Storage abstraction**: `RecorderStore` interface with filesystem (default `./data/records/<protocol>/<year>/<month>/`) and S3 backend.
- **Concurrency enforcement**: per-connection limit (0 = unlimited) enforced in `ActiveSessionService` before launch.

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

- Package layout `internal/drivers/ssh` (new driver package registered via `drivers.MustRegister` during init):
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
- Register both protocol descriptors through the same driver:
  - `ssh`: terminal-first experience (capabilities: terminal, file_transfer, session_recording, shareable) with optional SFTP tab.
  - `sftp`: file-manager-only shortcut (capability: file_transfer) that reuses the SSH launcher with terminal suppressed; defaults to same port/identity.
  - Ensure `ProtocolCatalogService` persists both entries so connection creation shows “SSH” and “SFTP (File Manager)” while sharing code paths.

### 3.4 SFTP Backend Interfaces

- **Connection Pooling**: Reuse `*sftp.Client` from primary SSH session; mutex-guarded, closed when last tab exits.
- **REST Endpoints** (gated by `protocol:ssh.sftp` + connection access):
  ```
  GET    /api/active-sessions/:id/sftp/list?path=       → list directory
  POST   /api/active-sessions/:id/sftp/mkdir            → create folder
  POST   /api/active-sessions/:id/sftp/rename           → move/rename
  DELETE /api/active-sessions/:id/sftp/file             → delete file
  DELETE /api/active-sessions/:id/sftp/directory        → recursive delete (max 10 depth)
  GET    /api/active-sessions/:id/sftp/download         → stream file (HTTP range support)
  POST   /api/active-sessions/:id/sftp/upload           → multipart upload (64 MiB chunks)
  GET    /api/active-sessions/:id/sftp/file             → fetch for editor (5 MiB max)
  PUT    /api/active-sessions/:id/sftp/file             → save edited file
  ```
- **Transfer Events**: Emit via realtime hub (`sftp.transfer.{queued|started|progress|completed|failed}`) with `{session_id, file, bytes, total}`.
- **Security**: Path normalization (reject `..`), enforce root chroot (default: user home), validate UTF-8 filenames.
- **Performance**: Stream uploads directly (no disk buffering), 30s idle timeout per transfer.
- **Error Mapping**: Translate SFTP codes to `PERMISSION_DENIED`, `QUOTA_EXCEEDED`, `NOT_FOUND`, etc.
- **Auditing**: Log all mutations with `{user, session, operation, path, timestamp}`.

### 3.5 WebSocket Contract

- **Endpoint**: `GET /api/connections/:id/sessions/:sessionID/ws` (auth via current middleware).
- **Subprotocol**: `shellcn.ssh.v1`
- **Frame Types**:
  - Binary frames: raw terminal I/O (stdin/stdout)
  - JSON frames: control messages with envelope `{type, payload}`
- **Message Types**:

  ```typescript
  // Terminal data (binary frame)
  type: "data" → payload: raw bytes

  // Control messages (JSON frame)
  type: "resize"     → payload: {cols, rows}
  type: "chat"       → payload: {user_id, message, timestamp}
  type: "share"      → payload: {action: "joined|left|write_granted", user_id}
  type: "recording"  → payload: {status: "started|paused|stopped"}
  type: "error"      → payload: {code, message}
  type: "heartbeat"  → payload: {timestamp}
  ```

- **Heartbeat**: Client sends ping every 20s; server updates `LastSeenAt`. Auto-close after 60s silence.
- **Reconnection**: Client retries with exponential backoff (1s → 30s max). Server rejects stale sessions (>5min idle).
- **Error Codes**: `PERMISSION_DENIED`, `CONCURRENT_LIMIT`, `SESSION_CLOSED`, `WRITE_CONFLICT`

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
- Recording permissions are derived from the session protocol (`protocol:<driver>.record`). Backends MUST NOT hardcode SSH so future drivers (RDP, VNC, etc.) automatically inherit the same recording lifecycle.

### 4.7 Settings & Preferences

**Admin Protocol Settings** (`/settings/protocols`):

- **New page** under Settings nav (admin permission required)
- **Single source of truth** for SSH defaults: concurrency, idle timeout, SFTP availability, recording policy, and appearance are configured here (no static config toggles beyond master enable/disable).
- **Layout**:
  - Protocol selector sidebar (mirrors existing Security tab pattern)
  - Detail panel for selected protocol (SSH)
- **Form Sections** (React Hook Form + Zod):
  1. **Session Defaults**
     - Concurrent limit (number input, `0` = unlimited)
     - Idle timeout minutes (number input)
     - Enable SFTP by default (checkbox)
  2. **Terminal Appearance**
     - Theme mode (Select: Auto/Force Dark/Force Light)
     - Font family (Combobox with presets)
     - Font size (Slider 8–96px + preview)
     - Scrollback limit (number input)
     - WebGL renderer (checkbox)
  3. **Recording**
     - Mode (Select: Disabled/Optional/Forced)
     - Storage (Select: filesystem/s3)
     - Retention days (number input)
     - Require consent (checkbox)
  4. **Collaboration**
     - Allow sharing by default (checkbox)
     - Restrict write to admins (checkbox)
- **Data Flow**:
  - `useProtocolSettings('ssh')` hook fetches from `/api/settings/protocols/ssh`
  - Submit calls `PUT /api/settings/protocols/ssh` with bulk updates
  - Backend persists to `system_settings` table via `UpsertSystemSetting`
  - Emits audit event: `protocol_settings.updated`
- **UI Pattern**: Reuse SecuritySettingsPanel layout pattern

**Connection Form Integration**:

- Forms fetch admin defaults via `GET /api/settings/protocols/ssh`
- Pre-fill fields with defaults
- User can override any setting (stored in `connections.settings` JSON)
- Show badge "Using admin default" when not overridden

**User Preferences** (`/settings/account` or `/settings/preferences`):

- Extend existing preferences panel with SSH section
- Terminal preferences: font, cursor style, copy behavior
- SFTP preferences: hidden files toggle, auto-open queue
- Stored in `users.preferences` JSON column

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

## 8. Recording Consent & Compliance

**Privacy Requirements**:

- **Banner Display**: When recording is active (forced or optional), show persistent banner: _"This session is being recorded for security and compliance purposes."_
- **Consent Mode**:
  - `forced`: No opt-out, banner shows info only
  - `optional`: Owner can toggle recording on/off
  - `disabled`: No recording UI shown
- **Participant Notification**: When joining shared session, modal warns: _"Owner is recording this session. By continuing, you consent to recording."_
- **Compliance**:
  - Add config flag `protocols.ssh.recording.require_consent` (default: `true`)
  - Store consent acceptance in `session_participants.consented_to_recording` (boolean)
  - Audit log: `session.recording.consent_given` event

**Banner Example**:

```tsx
{
  isRecording && (
    <div className="bg-red-500/10 border-b border-red-500/20 px-4 py-2">
      <div className="flex items-center gap-2 text-sm text-red-600">
        <RecordIcon className="animate-pulse" />
        <span>Recording active • {formatDuration(recordingDuration)}</span>
        {canStopRecording && (
          <Button size="sm" onClick={stopRecording}>
            Stop
          </Button>
        )}
      </div>
    </div>
  );
}
```

## 9. Proposed Optimizations

### 9.1 Adaptive Terminal Rendering

- **Dynamic FPS**: Reduce frame rate to 30fps when tab inactive, restore to 120fps on focus
- **Viewport Culling**: Only render visible terminal rows (xterm handles this natively with WebGL)
- **Throttled Resize**: Debounce window resize events (300ms) before triggering terminal fit

### 9.2 Smart SFTP Caching

```typescript
// Prefetch strategy
interface SftpCache {
  // Cache directory listings with TTL
  listings: Map<string, { data: FileEntry[]; timestamp: number }>;

  // Prefetch breadcrumb parent on directory open
  prefetch: (path: string) => void;

  // Invalidate on mutations
  invalidate: (path: string) => void;
}
```

### 9.3 Optimistic UI Updates

- File operations (rename, delete, mkdir): update UI immediately, rollback on error
- Chat messages: append locally before server confirmation
- Write access transfer: show visual feedback before backend ACK

### 9.4 Connection Pooling Strategy

```go
// SSH connection reuse across sessions
type SSHConnectionPool struct {
  conns map[string]*ssh.Client  // key: "connection_id:user_id"
  mu    sync.RWMutex
  ttl   time.Duration  // 5 minutes idle timeout
}
```

- **Benefit**: Reduce SSH handshake overhead for rapid session switches
- **Tradeoff**: Memory usage increases, add max pool size limit

### 9.5 Progressive Recording Uploads

- Stream recording chunks to S3 every 5 minutes (don't wait for session end)
- Reduces memory footprint, enables faster playback start
- Final chunk uploaded on session close with metadata update

### 4.9 Protocol Settings Configuration

**Admin Settings Storage** (`system_settings` table):
Protocol defaults managed via Admin UI, not config files. Settings stored as key-value pairs:

```go
// Backend: Protocol settings keys
const (
    "sessions.concurrent_limit_default"     → "0"  // 0 = unlimited
    "sessions.idle_timeout_minutes"         → "30"
    "protocol.ssh.enable_sftp_default"          → "true"
    "recording.mode"               → "optional"  // disabled|optional|forced
    "recording.storage"            → "filesystem"  // filesystem|s3
    "recording.retention_days"     → "90"
    "recording.require_consent"    → "true"
    "protocol.ssh.terminal.theme_mode"          → "auto"  // auto|force_dark|force_light
    "protocol.ssh.terminal.font_family"         → "monospace"
    "protocol.ssh.terminal.font_size"           → "14"
    "protocol.ssh.terminal.scrollback_limit"    → "1000"
    "protocol.ssh.terminal.enable_webgl"        → "true"
    "session_sharing.allow_default" → "true"
    "session_sharing.restrict_write_to_admins" → "false"
)
```

**API Endpoints**:

```
GET    /api/settings/protocols/ssh      → fetch current settings
PUT    /api/settings/protocols/ssh      → bulk update (audit logged)
PATCH  /api/settings/protocols/ssh/:key → update single setting
```

**Connection Settings** (JSON in `connections.settings`):
Per-connection overrides inherit from admin defaults when omitted:

```json
{
  "host": "192.168.1.100",
  "port": 22,
  "concurrent_limit": 5, // overrides admin default
  "enable_sftp": true, // overrides admin default
  "terminal_config_override": {
    // optional overrides
    "font_size": 16
  }
}
```

**User Preferences** (`users.preferences` JSON):

```json
{
  "ssh": {
    "terminal": {
      "font_family": "Fira Code",
      "cursor_style": "block",
      "copy_on_select": true
    },
    "sftp": {
      "show_hidden_files": false,
      "auto_open_queue": true
    }
  }
}
```

**Resolution Hierarchy**:

1. User preference (if exists)
2. Connection override (if exists)
3. Admin default (from `system_settings`)
4. Hardcoded fallback

### 4.10 Snippet Management System

**Database Schema**:

```sql
CREATE TABLE snippets (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  command TEXT NOT NULL,
  scope VARCHAR(20) NOT NULL,  -- 'global' | 'connection' | 'user'
  owner_id UUID REFERENCES users(id),
  connection_id UUID REFERENCES connections(id),
  created_at TIMESTAMP,
  updated_at TIMESTAMP,
  CONSTRAINT check_scope CHECK (
    (scope = 'global' AND owner_id IS NULL AND connection_id IS NULL) OR
    (scope = 'connection' AND connection_id IS NOT NULL) OR
    (scope = 'user' AND owner_id IS NOT NULL)
  )
);
```

**API Endpoints**:

```
GET    /api/snippets?scope=global|connection|user&connection_id=
POST   /api/snippets                     (requires: protocol:ssh.manage_snippets)
PUT    /api/snippets/:id
DELETE /api/snippets/:id
POST   /api/active-sessions/:id/snippet  (execute snippet)
```

**Security**:

- Global snippets: requires `admin.manage_snippets`
- Connection snippets: requires `connection.manage` + `protocol:ssh.manage_snippets`
- User snippets: owner only
- Execution: sanitize input, log to audit trail, no shell interpolation

### 4.11 Multi-Protocol Workspace State Management

**Workspace Store** (Zustand):

```typescript
interface WorkspaceStore {
  // Map protocol type → component state
  protocolSessions: Map<
    ProtocolType,
    {
      tabs: SessionTab[];
      focusedTabId: string | null;
      layout: SplitLayout;
      mounted: boolean; // keep component mounted when inactive
    }
  >;

  // Current focused protocol
  activeProtocol: ProtocolType | null;

  // Switch protocol (hide current, show target)
  switchProtocol: (protocol: ProtocolType) => void;

  // Session tab management
  openSession: (session: ActiveSession) => void;
  closeSession: (sessionId: string) => void;
  focusSession: (sessionId: string) => void;
}
```

**Strategy**:

- Keep all protocol components mounted with CSS `display: none` when inactive
- WebSocket connections remain open in background
- Terminal buffers (xterm instances) persist in memory
- Switching protocols toggles CSS visibility, preserves state
- Max 3 protocols mounted simultaneously (LRU eviction)

### 4.12 Optional Enhancements & Optimizations

- Per-user SSH session caps: allow admins to define maximum concurrent SSH sessions per user or team; ActiveSessionService enforces limit and surfaces alert toast + audit event when hit.
- Adaptive session recording: monitor bandwidth/latency; auto-throttle recording frame rate or pause capture when terminal latency exceeds threshold to favor interactivity.
- Terminal diff batching: coalesce outbound terminal writes (e.g., 50ms frames) to reduce DOM churn, improve battery use, and shrink recording size.
- Split-pane skeletons: render lightweight placeholders while xterm/SFTP bundles lazy-load so multi-column layouts remain visually stable.
- SFTP inline previews: provide quick preview pane for small text/images without launching editor/download; supports tabbed preview panel inside file manager.
- Collaboration cues: color-code read/write pills and terminal border; optional chat banner reminding owners they are sole write delegates.
- Chat quick actions: embed contextual buttons (e.g., “Grant write”, “Open SFTP”) inside chat messages for owners/admins to act faster.
- Connection escape hatch: from shared session UI provide “View Connection Details” link opening connection drawer/page for share or settings adjustments.

### 4.13 Performance Guardrails

**Rendering**:

- Terminal writes: batch to ≤120 fps, defer non-critical DOM with `requestIdleCallback`
- Memoize terminal panes, chat, transfer lists with `React.memo`
- Use `React.useTransition` for badge updates and low-priority UI

**Network**:

- Enable WebSocket compression (`permessage-deflate`)
- Prefetch SFTP directories (top-level only), LRU cache (max 100 entries)
- TanStack Query: `staleTime: 30s`, `gcTime: 5min`

**Bundle Size**:

- Initial SSH chunk: <300 KB (lazy-load Monaco, xterm addons, SFTP)
- Dynamic imports for: terminal WebGL addon, file editor, image preview
- Run Vite bundle analyzer; fail CI if baseline regresses >10%

**Memory**:

- Limit terminal scrollback: 1000 lines default (configurable)
- Max 3 mounted protocol workspaces (LRU eviction)
- Clear xterm buffers on session close

**Monitoring**:

- Track Web Vitals (LCP, FID, CLS) in dev builds
- Custom metrics: `terminal_render_ms`, `sftp_list_latency`, `websocket_reconnects`
- Alert on: >500ms SFTP response, >5 reconnects/session

## 10. Open Questions

- **Chat Persistence**: Should chat history persist beyond session lifetime for auditing? (Currently cleared on close; consider opt-in archival for compliance teams)
- **Team Concurrency Overrides**: Do we need per-team default limits in addition to per-connection? (Could simplify admin workflows)
- **Orphaned Recordings**: If DB record missing but file exists, reconcile or purge? (Suggest background job: match by filename pattern, log + delete after 7 days)
- **SFTP Resume Protocol**: Use TUS protocol for chunked uploads or custom implementation? (TUS adds complexity but industry standard)
