# Protocol Driver Standards

This document defines the contract that every protocol driver (formerly "module") must follow across backend, frontend, and specification layers. It complements `specs/plans/1. core/CONNECTION_PROTOCOLS_PLAN.md` and supersedes legacy module-language inside historical docs.

## 1. Driver Taxonomy & Naming

- **Driver ID**: lower-case, hyphen-less identifier (e.g. `ssh`, `docker`, `kubernetes`). IDs become permission prefixes, connection protocol ids, and filesystem folder names under `specs/project/drivers/<driver-id>.md`.
- **Title**: human readable string displayed in UI tab labels ("Kubernetes", "Docker Engine").
- **Category**: standard categories `terminal`, `desktop`, `container`, `database`, `object_storage`, `vm`, `network`. Custom categories must be documented and added to UI icon mapping.
- **Module Field**: persisted value mirroring configuration namespace (current config keys under `Config.Modules`). Prefer the driver ID unless multiple sub-protocols share the same driver (database family).

## 2. Specification Layout

Each driver receives its own spec file under `specs/project/drivers/<driver-id>.md` with the following sections:

1. **Overview** – summary, target infrastructure, backend driver type (native, FFI, proxy).
2. **Connection Schema** – base settings persisted in `connections.settings` (host, port, namespace, context, path, etc.). Include `required?`, `default`, `validation`, and the capability flag(s) each property unlocks.
3. **Identity Requirements** – identities or vault credentials needed (e.g. SSH key, kubeconfig, Docker TLS cert). Specify secret schema keys so Credential Vault integration can be automated.
4. **Permission Profile** – list of permission ids (base + optional). Align with section 4 below.
5. **Frontend Contract** – form panels, quick actions, optional wizards, capability-specific UI toggles. For drivers that implement dynamic connection fields, document the connection template described in §11.
6. **Testing Guidance** – driver-specific fixtures, integration tests, and mocks.
7. **Future Enhancements** – optional roadmap for driver-specific features.

## 3. Driver Registration Pipeline

1. Driver package implements the `Driver` interface in `internal/drivers/driver.go`:

   ```go
   type Driver interface {
       // Metadata methods (required)
       ID() string
       Name() string
       Module() string
       Category() string
       Icon() string
       Description() string
       DefaultPort() int
       SortOrder() int

       // Capabilities (required)
       Capabilities(ctx context.Context) (Capabilities, error)
   }
   ```

   **Implementation Tip**: Use `drivers.BaseDriver` to automatically implement metadata methods:

   ```go
   type SSHDriver struct {
       drivers.BaseDriver  // Embed for automatic metadata
   }

   func NewSSHDriver() *SSHDriver {
       return &SSHDriver{
           BaseDriver: drivers.NewBaseDriver(drivers.Descriptor{
               ID:        "ssh",
               Module:    "ssh",
               Title:     "SSH",
               Category:  "terminal",
               Icon:      "terminal",
               SortOrder: 1,
           }),
       }
   }
   ```

2. Driver registers with `drivers.Registry` during bootstrap (`drivers.MustRegister`).

3. `ProtocolCatalogService.Sync(ctx, driverRegistry, config)` reads metadata directly from drivers and persists to database with config enablement state.

4. Driver packages declare launch support by implementing `drivers.Launcher`. Launchers must cooperate with the shared session lifecycle hooks described in §11 (register session on success, propagate heartbeats, unregister on close/error).

5. Driver `init()` functions must register protocol-scoped permissions using `permissions.RegisterProtocolPermission` (see section 4) before `permissions.Sync` is invoked.

6. Frontend fetches protocol catalog from `/api/protocols` (served from database cache).

## 4. Permission Model

| Layer               | Responsibility                                                            |
| ------------------- | ------------------------------------------------------------------------- |
| `connection.view`   | Grants access to protocol/connection catalog routes.                      |
| `connection.launch` | Required to start or preview sessions.                                    |
| `connection.manage` | Required for CRUD operations on connections and driver advanced settings. |
| `connection.share`  | Required for editing visibility ACLs.                                     |

### 4.1 Permission Prefix & Naming

- All protocol permissions must use the canonical id format `protocol:<driver-id>.<action>`.
- Modules default to `protocols.<driver-id>` unless overridden.
- Categories default to `protocol:<driver-id>` to simplify registry filters.
- Metadata **must** include the driver id (automatically enforced by the helper) and may add capability hints (e.g. `capability: "exec"`).

### 4.2 Registration Helper

Drivers register their permissions in `init()` using the dedicated helper:

```go
func init() {
    permissions.Must(permissions.RegisterProtocolPermission("ssh", "connect", &permissions.Permission{
        DisplayName:  "SSH Connect",
        Description:  "Initiate SSH sessions",
        DefaultScope: "resource",
        DependsOn:    []string{"connection.launch"},
    }))
}
```

Notes:

- Use `RegisterProtocolPermission(driverID, action, definition)` instead of `Register` to guarantee consistent prefixes and metadata.
- `RegisterProtocolPermission` does **not** panic; drivers should wrap it in a small helper (for example, `func must(err error) { if err != nil { panic(err) } }`) or bubble the error during bootstrap.
- Declare additional actions (e.g. `port_forward`, `exec`, `desktop_control`) with the same helper. Dependencies can reference previously registered protocol permissions (`protocol:<driver>.connect`) or global ones (`connection.manage`).
- Avoid registering protocol permissions from `internal/permissions/core.go`; ownership lives inside each driver package so optional drivers can gate their scopes cleanly.
- Dependency guidelines:
  - Base connect scopes must depend on `connection.launch`.
  - Mutating actions (write, manage, admin) must depend on `connection.manage`.
  - Read-only feature scopes may depend on either the base connect scope or `connection.launch`, whichever best reflects runtime enforcement.

## 5. Connection Schema Requirements

- Store driver settings as JSON on `Connection.Settings`. Drivers supply a JSON schema via `drivers.SchemaProvider` describing field names, types, validation, and whether a field is identity-backed.
- `Connection.Metadata` holds UI-only preferences (favorite tags, color). Do not duplicate driver settings in metadata.
- Provide a helper `DriverConfig.Normalize(settings map[string]any) (map[string]any, error)` to coerce defaults, merge ports, and handle capability toggles.
- Drivers that require multiple targets (e.g., Kubernetes API + kubeconfig) should use `ConnectionTargets` to persist per-cluster endpoints.

## 6. Identity & Credential Vault Integration

- Drivers declare required secret slots (e.g., `ssh.key`, `ssh.password`, `kubeconfig`, `docker.cert`).
- Secrets always reference vault identities; drivers must never accept raw credential payloads from connection settings.
- `Identity` feature (future) must map to driver requirements using the same key names to allow auto-binding.
- Drivers must expose a `CredentialTemplate()` descriptor describing expected fields, validation rules, compatible protocol IDs, and version metadata (`TemplateVersion`, optional `DeprecatedAfter`). Templates are synced via `ProtocolCatalogService.Sync()` during startup and on-demand refresh. Each field should specify `type` (string, secret, file, enum, boolean, number), `required`, optional validation hints, and supported `input_modes` (e.g., `['text','file']` for kubeconfigs).
- `ProtocolService` and UI should surface missing credential requirements so users can attach identities before launching.

**Credential Field Schema Example**

```go
type CredentialField struct {
    Name        string   // e.g. "kubeconfig"
    Type        string   // string, secret, file, enum, boolean, number
    Required    bool
    Description string
    InputModes  []string // e.g. []string{"text", "file"}
    Options     []string // for enums
}
```

A Kubernetes driver can expose a field like `kubeconfig` with `Type: "secret"` and `InputModes: []string{"text", "file"}` so the UI offers either paste or upload flows, while an SSH driver may provide both `private_key` (file or text) and `password` (text) as optional secrets.

For protocol families with multiple engines (e.g. MySQL, PostgreSQL, Redis), each driver/feature should register its own `CredentialTemplate` keyed by driver ID (for example `mysql`, `postgres`, `redis`). Shared code can leverage helper structs, but the registry must surface distinct templates so the frontend knows which fields to show for each connection type.

- Connection credentials are always sourced from the vault:
  - Connections store an `identity_id` referencing a vault identity (global, team, or connection-scoped).
  - One-off credentials MUST be wrapped in a connection-scoped identity created by the backend/UI helpers; drivers never read secrets from `Connection.Settings` directly.
  - Driver specs must clearly state whether identities are mandatory (SSH, Kubernetes, database) or optional (Telnet, health probes) so UI flows can prompt users accordingly.
- When template versions change, drivers must publish migration guidance (matching `TemplateVersion`, `DeprecatedAfter`) and a handler for rehydrating existing identities into the new schema.

## 7. Frontend Contract

- `/api/protocols` returns `ProtocolInfo` with `capabilities` and `features` arrays. UI uses this to display capability chips and to decide which tabs (terminal, desktop, metrics) to show.
- `/api/protocols/:id` (future) will include configuration schema for driver forms.
- Frontend state hooks (`useUserProtocols`, `useConnections`) cache responses and filter by permission-derived availability.
- React components should rely on `capabilities` when toggling UI actions (e.g., show "File Transfer" if `file_transfer` in features).

## 8. Testing Expectations

- **Unit Tests**: driver-specific packages should test descriptor registration, capability responses, permission profile registration, and config validation.
- **Integration Tests**: cover `ProtocolService`, handler endpoints, and driver health sync (mocking registries where needed).
- **Frontend Tests**: ensure the Connections page renders capability chips, disables launch buttons when permissions are missing, and respects category filters.

## 9. Example Permission Profiles

| Driver     | Base Connect                  | Manage Permission            | Feature Scopes                                                  | Admin Scopes                        |
| ---------- | ----------------------------- | ---------------------------- | --------------------------------------------------------------- | ----------------------------------- |
| SSH        | `protocol:ssh.connect`        | `protocol:ssh.manage`\*      | `protocol:ssh.port_forward`, `protocol:ssh.sftp`\*              | `protocol:ssh.global_admin`\*       |
| Kubernetes | `protocol:kubernetes.connect` | `protocol:kubernetes.manage` | `protocol:kubernetes.exec`, `protocol:kubernetes.port_forward`  | `protocol:kubernetes.cluster_admin` |
| Docker     | `protocol:docker.connect`     | `protocol:docker.manage`\*   | `protocol:docker.exec`, `protocol:docker.logs`\*                | `protocol:docker.stack.deploy`\*    |
| Database   | `protocol:database.connect`   | `protocol:database.manage`\* | `protocol:database.query.read`, `protocol:database.query.write` | `protocol:database.cluster.manage`  |

(\*) Illustrative examples; drivers should define only the scopes they truly support.

## 10. Architecture & Migration Notes

### 10.1 Simplified Architecture (October 2025)

The protocol registry layer has been **removed** to simplify the architecture:

**Old Flow (Deprecated)**:

```
Driver → Driver Registry → Protocol Registry → Database → API
```

**New Flow (Current)**:

```
Driver → Driver Registry → Database → API
         ↓
    (metadata methods)
```

**Key Changes**:

- ❌ **Removed**: `internal/protocols/Registry` and `internal/protocols/Protocol`
- ✅ **Driver interface now includes metadata methods** (ID, Name, Category, Icon, etc.)
- ✅ **`ProtocolCatalogService.Sync()`** reads directly from drivers
- ✅ **Single source of truth**: Driver implementations define all metadata
- ✅ **Database cache** still used for fast API responses

**For New Drivers**:

- Implement the full `Driver` interface OR use `drivers.BaseDriver` helper
- No need to interact with protocol registry
- All metadata comes from driver methods

### 10.2 Legacy Migration Notes

- Historical references to the "Core Module" now map to the "Core Protocol Driver Set". Where documentation still mentions modules, annotate them with the new term on sight to maintain clarity.
- New drivers must include their spec document _before_ code merges.
- Any config change that toggles driver availability must update the relevant spec sections (config schema + permission updates).
- **`Descriptor()` method**: Still supported for backward compatibility but deprecated. Use direct metadata methods instead.

### 10.3 Dynamic Connection Templates (2025-02 Proposal)

> See `specs/plans/1.core/DYNAMIC_CONNECTION_FORM_SPEC.md` for the full implementation plan.

Drivers can optionally publish a connection configuration schema so the platform renders protocol-specific fields dynamically.

- **Interface**

  - Implement `ConnectionTemplater` in addition to `Driver`, `Launcher`, etc.:

    ```go
    type ConnectionTemplater interface {
        ConnectionTemplate() (*drivers.ConnectionTemplate, error)
    }
    ```

  - Templates describe sections/fields, validation rules, and bindings (e.g., settings map vs. `connection_targets`).
  - Shared fields (name, description, folder, team, icon, identity selector) remain part of the common form shell; drivers should not duplicate them.

- **Driver registration pattern**

  - Construct the template during driver initialisation and return it from `ConnectionTemplate()`.
  - Version templates (e.g., `"2025-01-01"`) so the catalog can detect schema changes and trigger migrations.
  - Reference the template schema inside the driver spec file (`specs/project/drivers/<driver-id>.md`) alongside credential requirements.

- **Platform responsibilities**
  - `ProtocolCatalogService.Sync` persists templates from drivers implementing the interface.
  - Backend connection services validate create/update payloads against the template and normalise settings/targets.
  - Frontend form builder fetches the template through `/api/protocols/:id/connection-template` (planned) and renders driver-specific fields automatically.

Until the registry ships, implementing `ConnectionTemplater` is recommended for new drivers so the UI can adopt the dynamic form without additional code changes.

## 11. Session Lifecycle & Active Connection Tracking

Active connection visibility is powered by `services.ActiveSessionService`. Every launcher-enabled driver participates in the following flow:

1. **Launch Gatekeeping**
   - API layer checks `ActiveSessionService.HasActiveSession(userID, connectionID)` to enforce the one-session-per-(user, connection) rule.
   - Launch is rejected with `ErrActiveSessionExists` when violated.
2. **Successful Launch Registration**
   - After a driver establishes the transport and returns a `drivers.SessionHandle`, the orchestrator calls  
     `ActiveSessionService.RegisterSession(&ActiveSessionRecord{ ... })`.
   - Drivers must supply enough metadata for the record:
     - `ID`: unique session identifier (driver-specific UUID).
     - `ConnectionID`, `UserID`, `ProtocolID` (required).
     - `ConnectionName`, `UserName`, `TeamID` when available.
     - Protocol metadata such as `Host`, `Port`, or additional `Metadata` map (e.g. namespace, pod, database).
3. **Heartbeat**
   - Long-running drivers should periodically call `ActiveSessionService.Heartbeat(sessionID)` or delegate to a scheduler so that stale sessions (timeout default: 5 minutes) are not garbage collected.
   - Drivers without natural heartbeats must emit one when user activity is detected (command executed, data streamed, etc.).
4. **Close / Error Handling**
   - `SessionHandle.Close` must invoke `ActiveSessionService.UnregisterSession(sessionID)` even on error paths.
   - If a driver detects a disconnect asynchronously (e.g., remote host dropped), it must unregister and surface the failure back to callers so UI and auditing remain consistent.
5. **Broadcasts & Consumers**
   - Registering/unregistering sessions triggers `realtime.StreamConnectionSessions` events (`session.opened`, `session.closed`), consumed by React hooks such as `useActiveConnections` to keep the sidebar and badges in sync.
   - Admin users receive enriched payloads (`user_name`, `team_id`) while regular users only receive their own sessions via `/api/connections/active`.
   - Terminal-style drivers (SSH, Telnet, Kubernetes exec, etc.) MUST stream their websocket traffic through the shared terminal bridge (`internal/handlers/terminal`). This helper handles stdin/stdout tunnelling, control frames (`resize`, `heartbeat`), binary-to-base64 encoding, and publishes structured events on `realtime.StreamSSHTerminal` (eavesdropping UIs can subscribe regardless of protocol).
   - Desktop/video protocols (RDP, VNC) SHOULD follow the same pattern, but may swap in a protocol-specific bridge (e.g. frame transport) if the terminal helper is insufficient.
6. **Driver-Specific Data**
   - Drivers should populate `ActiveSessionRecord.Metadata` with protocol context (e.g., Kubernetes namespace/workload, database name) using flat JSON-friendly primitives.
   - Avoid storing sensitive secrets in metadata; leverage Vault identities instead.
7. **Testing**
   - Unit tests must assert that session registration occurs on launch success and that `Close` tears down sessions.
   - Integration tests should verify that `/api/connections/active` reflects driver launches and that duplicate launches are rejected.

---

**Status**: Draft ready for implementation guidance.
**Maintainer**: Core platform architecture team.

## 12. Shared Session Collaboration Standard

Shared sessions allow multiple users to attach to the same live connection. All drivers that enable shared access must follow these rules:

1. **Eligibility & Permissions**
   - Drivers expose capability flag `shareable` when they support multi-user viewing.
   - Sharing requires the base `connection.launch` permission plus the protocol-scoped permission `protocol:<driver-id>.share`.
   - Participants must already have launch access to the underlying connection (enforced via resource permissions).
2. **Active Session Ownership**
   - The launcher creates a **session owner** (first user) who implicitly has `write` access.
   - Owners may invite additional participants individually or scoped to a team. Invitations are scoped per active session, not globally.
3. **Access Modes**
   - Access modes are `read` (default) and `write`.
   - Only one participant may hold `write` access at a time. Owners can transfer or revoke write access; participants may voluntarily drop it.
   - Drivers must treat non-writers as view-only: terminal keystrokes, mouse events, uploads, and clipboard actions are ignored server-side.
4. **Lifecycle API**
   - REST endpoints under `/api/active-sessions/:id` handle share management (invite, list participants, update access, revoke).
   - WebSocket control frames (`share.*` events) inform clients about participant joins, leaves, and access changes.
   - When a participant disconnects, the backend cleans up their association automatically.
5. **Chat Channel**
   - Each shared session exposes a per-session chat stream. Messages are ephemeral unless the protocol driver opts into persistent logging.
   - Chat history must be cleared when the active session closes and should honour organization profanity/security filters.
6. **Audit & Metrics**
   - Every grant/revoke action emits an audit log entry and updates Prometheus counters (`protocol_<id>_shared_sessions_total`).
   - UI should surface when a session is shared (badge) and who currently has write control.
7. **Extensibility**
   - These rules apply to all terminal/desktop drivers that plan to support collaboration (SSH, Telnet, Kubernetes exec, RDP shadowing, etc.). Drivers that cannot technically enforce read-only mode should not expose the feature flag.

## 14. Multi-Protocol Workspace State Persistence

When users work across multiple protocol types (SSH, K8s, Docker, etc.), the frontend must preserve component state when switching between protocols.

**Requirements**:

1. **Component Lifecycle**

   - Keep up to 3 protocol workspace components mounted simultaneously
   - Use CSS `display: none` for inactive protocols (not unmount)
   - Evict least-recently-used protocol when mounting 4th type

2. **State Preservation**

   - Terminal buffers (xterm instances) remain in memory when switching away
   - SFTP navigation history and open tabs persist
   - WebSocket connections stay alive in background
   - Split pane layouts saved per active session

3. **Workspace Store Contract**

   ```typescript
   interface ProtocolWorkspace {
     protocolType: string;
     sessions: Map<string, SessionState>; // sessionId → state
     layout: SplitLayout;
     lastActiveAt: number;
   }

   // Global store
   workspaces: Map<ProtocolType, ProtocolWorkspace>;
   ```

4. **Routing Strategy**

   - Route pattern: `/active-sessions/:sessionId`
   - Protocol inferred from session metadata
   - Clicking sidebar entry focuses existing tab or creates new one
   - URL sync: navigating to session URL rehydrates workspace state

5. **Memory Management**

   - Enforce max scrollback (1000 lines) per terminal
   - Clear buffers on explicit session close
   - Warn user if >3 protocols mounted (memory usage banner)

6. **Testing Expectations**
   - Verify state persists across protocol switches (integration test)
   - Confirm WebSocket reconnection on tab focus
   - Assert LRU eviction purges oldest workspace

## 15. Session Recording Standard

Recording provides audit playback for supported protocols (SSH terminal, RDP, VNC, etc.). Drivers opting into recording must implement:

1. **Capability Flag & Permission**
   - Advertise `SessionRecording: true` in driver capabilities.
   - Require `protocol:<driver-id>.record` permission in addition to `connection.launch`.
   - Obey global or per-connection configuration toggles that enable/disable recording.
2. **Recorder Interface**
   - Drivers stream raw session data into the shared `RecorderService` which handles buffering, compression, and persistence.
   - Supported codecs start with Asciinema v2 for terminal streams; binary protocols (desktop) may adopt video or image-diff codecs later.
3. **Storage Abstraction**
   - Recordings are stored through `RecorderStore` implementations. Default is filesystem (`./data/records/<protocol>/<year>/<month>/<session>.cast.gz`); S3-compatible storage must use configurable bucket/prefix.
   - Metadata row (`connection_session_records`) tracks storage kind, key/path, size, duration, checksum, and retention policy.
4. **Privacy & Controls**
   - Admins can enforce always-on recording, opt-in, or disallow per protocol.
   - Owners can see recording status and stop/resume when policy permits. Participants cannot disable recordings started by admins.
   - UI must inform participants when recording is active.
5. **Playback & Access**
   - REST endpoints provide metadata listing, secure download, and playback token generation. Access requires either session ownership or elevated auditing permissions.
   - Playback consumers should stream data rather than load entire file when possible.
6. **Retention & Cleanup**
   - Each recording honours configurable retention (days) with background jobs purging expired files and metadata.
   - Deletions are audited and require `protocol:<driver-id>.record` plus `connection.manage` or a dedicated `session_record.manage` permission.
7. **Testing**
   - Integration tests must cover start/stop flows, storage failures, and playback retrieval.
   - Simulate large sessions to ensure chunking and resource cleanup works as expected.

---

**Status**: Draft ready for implementation guidance.
**Maintainer**: Core platform architecture team.
