# ShellCN — Platform Manifest (document v2, plugin API v1)

> An open-source **infrastructure access gateway / operations cockpit**: a single
> Go binary (with an embedded Vue frontend) through which users reach their SSH
> servers, SFTP/FTP/SMB/NFS/WebDAV/cloud storage, Docker hosts, Kubernetes
> clusters, Proxmox, databases (SQL & NoSQL), and remote desktops (VNC/RDP) — all
> behind one unified, audited, policy-controlled interface.
>
> ShellCN is a **client/gateway**, not a service provider. It brokers secure,
> observable access to infrastructure; it does not host it.

This is the canonical architecture spec. It records the **design model and the
invariants** — the "why". It is **not** an API reference: concrete types,
signatures, and field lists live in the code, and this document points at those
files rather than duplicating them (duplicated type dumps rot). When a section
names a source file, read that file for the exact shape; read this document for
the contract the shape must honor. The manifest wire/API version accepted by the
core is `APIVersion: 1`.

**Plugin model in one line:** every protocol is a Go **plugin** — first-party and
compiled in, or third-party and loaded at runtime as an isolated subprocess (§5.7)
— that exposes one **versioned manifest** (declarative data) plus typed **route
handlers** (behavior). The core validates the manifest, owns security / sessions /
routing / audit / rendering, and serves the browser a _projection_ of the
manifest. The frontend renders entirely from that projection — **adding a plugin
requires zero frontend changes.**

---

## 1. Guiding principles

1. **Plugins declare; the core owns.** A plugin ships a typed **manifest**
   (identity, config schema, views, resources, actions, streams, route metadata)
   and route handlers. The core owns rendering, routing, sessions, authn/authz,
   policy, secrets, audit, and transport. Plugins never become mini-applications:
   no UI code, no HTTP plumbing, no auth logic, no storage.
2. **The manifest is the contract.** One **versioned** manifest per plugin,
   validated on registration; the core serves the browser a **rendering
   projection** of it, and the frontend renders from that projection only.
3. **Keep the declarative model small and typed.** The manifest is _data_
   (fields, columns, trees, IDs), never a scripting language. Logic goes in a
   route handler — never in the manifest.
4. **Data, not pixels.** Plugins describe _what_ to show and _what_ can be done.
   The frontend is a universal renderer of ~20 panel types. (Grafana / Terraform
   / kubectl model.)
5. **One connection, many capabilities, many channels.** A single authenticated
   session is multiplexed: SSH yields a terminal, SFTP, and snippets without
   re-authenticating.
6. **Secure and auditable from day one.** AuthN/AuthZ/policy/audit and
   encryption-at-rest are core requirements, designed as interfaces with simple
   first implementations (embedded RBAC, local encrypted vault).
7. **Single self-contained binary.** Pure-Go dependencies only, so the frontend
   and datastore embed cleanly. Even remote desktops stay in-process: VNC streams
   raw RFB and RDP is decoded by a pure-Go client, both bridged to the browser's
   noVNC engine — no external daemon.

---

## 2. Non-goals (v1)

- **No in-process native (`.so`) loading.** The gateway never loads Go `.so`
  plugins into its own address space. First-party plugins are compiled in;
  third-party plugins **are** loaded dynamically at runtime, but as
  **process-isolated gRPC subprocesses** (§5.7) — never in-process. Browser-side
  WebAssembly is allowed only through the generic `PanelWasm` contract: the core
  owns the sandboxed iframe, asset loading, route/stream bridge, auth, CSP,
  validation, and lifecycle.
- **No automatic HA failover / session rebalancing.** Live sessions and agent
  tunnels are memory-resident on the instance that owns them; ShellCN does not
  replicate that state or fail an in-flight session over to another instance.
  Multi-instance deployment **is** supported: a store-backed **live-state lease**
  pins each connection's live state to its owner and other instances transparently
  reverse-proxy to the lease holder (§8.5). Out of scope is shared/replicated
  session state and automatic failover — if the owner dies, its live sessions are
  re-established on a new owner on next use.
- **No plugin-shipped arbitrary UI / iframe escape hatch.** Plugins cannot ship
  raw HTML/JS/frontend code. They may declare a sandboxed `PanelWasm` when a use
  case genuinely needs an isolated WASM program; bridge access is explicit in the
  manifest and limited to declared routes, streams, and assets.
- **No SPICE.** No production-grade browser client exists. RDP is decoded
  in-process by the pure-Go `grdp` client and bridged to noVNC/RFB (§6.2); the
  core exposes a generic `remote_desktop` panel contract and does not let plugins
  select a browser renderer.

---

## 3. Domain model (glossary)

| Term           | Meaning                                                                                                                                                              |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Connection** | Stored config describing _how to reach_ one target. Owned by a user, optionally shared. May hold inline encrypted secrets or reference reusable credentials.        |
| **Credential** | A reusable encrypted secret bundle (SSH key/password, DB password, API token) with its own ownership/grants, referenced by many connections without exposing values. |
| **Protocol**   | The plugin id a connection uses (`ssh`, `docker`, `postgres`, …).                                                                                                   |
| **Plugin**     | A stateless singleton that _declares_ a manifest and _connects_. Compiled in, or an out-of-tree subprocess (§5.7).                                                  |
| **Manifest**   | The plugin's single versioned contract: identity, config schema, views, resources, actions, streams, route metadata.                                               |
| **Session**    | A live, authenticated runtime for one connection. Holds all per-connection state.                                                                                   |
| **Channel**    | One stream inside a session: terminal, log tail, VNC framebuffer, metrics feed, streaming query. Tracked by the core for lifecycle/audit.                           |
| **Capability** | Declarative tag of what a connection/resource supports (`terminal`, `filesystem`…). Feature detection / panel selection only — **never** behavior dispatch.         |
| **Resource**   | A managed object exposed by a connection (container, pod, VM, table), identified by a stable `ResourceIdentity`.                                                     |
| **Action**     | A named operation on a connection/resource. A UI affordance pointing at a **route**; risk/permission live on the route.                                             |
| **Stream**     | A long-lived channel a panel binds to (terminal, logs, desktop, metrics). Points at a WS **route**.                                                                 |
| **Route**      | A typed server endpoint with metadata (id, method, permission, risk, audit, input schema). The **only** behavior mechanism.                                         |
| **Panel**      | A core frontend component that renders a capability (Terminal, Table, Metrics…).                                                                                    |
| **Transport**  | How a session reaches its target: `direct` (ShellCN dials out) or `agent` (an agent inside the target dials back). Orthogonal to protocol (§8.2).                   |
| **Agent**      | `shellcn-agent`: a plugin-agnostic reverse-tunnel proxy run inside a private target, exposing a socket/port/API back to the gateway (§8.3).                         |

**Critical distinction** (this killed the "Docker terminal tab" bug):

- **Connection-level** capability → meaningful without selecting a resource (SSH
  terminal, Docker container _list_). Rendered as a connection tab/tree.
- **Resource-level** capability → only meaningful for a specific resource (exec
  into _this_ container, console of _this_ VM). Rendered in that resource's
  **DetailView**.

---

## 4. Architecture: who owns what

| Core platform                                                     | Plugin                                   |
| ----------------------------------------------------------------- | ---------------------------------------- |
| AuthN (OIDC-ready + local), platform sessions                     | Protocol handshake / upstream auth       |
| AuthZ (RBAC + ownership/grants + policy), action-risk enforcement | —                                        |
| Connection + reusable credential storage; secret encryption       | Config **schema** (field shapes only)    |
| Session registry, lifecycle, **channel tracking**                 | Per-connection state in its `Session`    |
| Route mounting, auth-wrapping, validation, audit                  | Route **handlers** (pure business logic) |
| Error normalization, pagination                                   | Returns typed data / errors              |
| Manifest validation + **browser projection**                      | One **versioned manifest**               |
| UI shell, panels, schema/tree/table renderers                     | UI **declarations** (manifest data)      |
| Egress policy / SSRF guard, observability                         | Declares target from validated config    |

A plugin handler **never** sees `http.ResponseWriter`, status codes, headers,
cookies, or auth. It receives a typed `RequestContext` and returns `(any, error)`.

**Plugin storage is core-owned.** A plugin may persist small plugin-owned objects
through `rc.Storage`, supplying only a logical collection + key/value. The core
resolves and persists the plugin ID, authenticated user ID, connection ID, and
timestamps. `Put` always writes the resolved connection-owned row; scope is only a
read/list/delete filter. Empty scope means current-connection; `UserStorage`
reads across that user's rows for the current plugin.

---

## 5. The Plugin contract

**Types:** `sdk/plugin/session.go` (`Plugin`, `Session`, `ConnectConfig`,
`NetTransport`). A `Plugin` is a **stateless singleton**: it `Manifest()`s
(declarative data), exposes typed `Routes()` (handlers), and `Connect()`s
(returns a `Session` holding _all_ per-connection state). It holds no
per-connection state on itself — one instance serves every connection.

Invariants:

- **Routes are the one behavior mechanism.** No `HandleAction`, no plugin-owned
  HTTP. Every effect is a route handler wrapped by the core.
- **`Connect` receives a `ConnectConfig`** built by the core: decrypted config,
  resolved credentials, and a `NetTransport` wired for the connection's mode. The
  plugin uses the layer its client needs and **never branches on direct-vs-agent**
  (§8.2).
- **`NetTransport` exposes two layers** because "use the dialer for everything" is
  wrong: `DialContext` (L4 — socket/TCP protocols: SSH, Docker, Postgres, …) and
  `HTTP()` (L7 — fat clients like client-go that need base URL + RoundTripper +
  auth; in agent mode the agent's L7 reverse-proxy injects credentials).

### 5.1 Manifest — server-side source of truth

**Type:** `sdk/plugin/manifest.go` (`Manifest`, `Icon`, `Category`, `Layout`,
`AgentProfile`). The manifest declares identity, config `Schema`, capabilities,
credential kinds, supported transports (+ `AgentProfile` iff agent is supported),
layout + tabs/tree, resources, actions, header actions, scope filters, and
streams.

- **Categories** are core-owned display metadata (grouping in pickers), not
  behavior dispatch. Keys are an open builtin vocabulary (`shell`, `files`,
  `containers`, `databases`, `orchestration`, …); the browser gets the resolved
  `{key,label,icon,order}`.
- **Route _metadata_ lives in `Routes()` (§5.3), never in the manifest.** Actions
  and Streams only **reference** a `RouteID` + UI affordances + optional params.
  Permission and risk are single-sourced on the route.
- **Icons are structured** (`Icon{Type,Value}`): Lucide glyph (any name, resolved
  at runtime, casing/separator-insensitive), URL, base64 data URI, emoji, or raw
  SVG. The same type is used everywhere. URL/base64 are sanitized + size-bounded;
  inline SVG is DOMPurify-sanitized (scripts/handlers stripped) before DOM
  injection; unknown/empty falls back to a placeholder glyph.

### 5.2 Browser projection (rendering contract)

**Code:** `sdk/plugin/projection.go`, served via `GET /api/plugins` and
`GET /api/plugins/{name}`. The browser must **render** but never **execute**. The
projection **includes** identity, category, config schema, layout, tabs/tree,
resource columns + actions (with `risk` + `requiresConfirm`), stream kinds, panel
configs, the shared panel-config schema, and route bindings (`RouteID` + params
only). It **excludes** handler funcs, raw mount paths, permission keys,
audit-event names, and any server-only route internals. The opaque `RouteID` is
the only handle the browser holds; the core resolves it to a URL (§7.1).

### 5.3 Routes — typed, with metadata

**Type:** `sdk/plugin/route.go` (`Route`, `RiskLevel`, `Method`, `Handler`,
`StreamHandler`). A route carries an id (the UI/audit/policy handle), method,
plugin-relative path template, permission key, risk, audit event, optional input
`Schema`, timeout, and a handler (`Handle` for HTTP, `Stream` for `WS`).

Risk tiers: `safe` (read), `write` (create/update), `destructive` (delete/
truncate/restore), `privileged` (shell/exec/raw socket).

**The core wraps every route:** authn → authz (permission + risk) → session
resolution → input validation → audit → handler → error normalization. Dispatch
lives in `internal/server/dispatch.go`; the same `resolveRoute` path serves both
HTTP dispatch and AI/tool invocation, so nothing bypasses authz/audit.

### 5.4 RequestContext — typed access

**Type:** `sdk/plugin/context.go` (`RequestContext`, `PageRequest`, `Page`).
Handlers bind the body into a typed struct with `rc.Bind(&dst)` (validated),
read path params with `rc.Param`, the query with `rc.Query`, and pagination with
`rc.Page`. This exists to kill a real bug: `params["replicas"].(int)` **panics**
because JSON numbers decode to `float64`. Typed binding, no assertions. The core
guarantees the concrete `Session` type per protocol, so
`rc.Session.(*fooSession)` is safe.

### 5.5 Actions & Streams — UI affordances over routes

**Types:** `sdk/plugin/ui.go` (`Action`, `ActionSuccess`, `ActionEffect`,
`Stream`, `StreamKind`, `ScopeFilter`). An `Action` references a `RouteID` (which
holds method/permission/risk/audit/input) plus UI affordances: label/icon,
optional params, confirm text, `OnSuccess`, an `Open` target (`view`/`dock`/
`dialog`/`url`), an optional hosted `Panel`+`Config`, `EnabledWhen`, `IconOnly`,
and `Group`. `risk`/`requiresConfirm`/`onSuccess` are projected for UI flow; the
**enforced** permission/risk stay on the route.

Design points the code must honor:

- **`OnSuccess` is a declarative follow-up:** switch tab, `navigate` (e.g. `list`
  so a deleted resource's detail doesn't linger), or typed `Effects` —
  `terminal_input` (write a route's returned text into a visible terminal instead
  of a hidden exec) and `open_panel` (open a dock/dialog panel whose source params
  interpolate the action's JSON result via `${response.x}`, expressing multi-step
  flows — "Debug" adds an ephemeral container then opens an exec into it — with no
  bespoke server glue).
- **`HeaderActions`** pin connection-wide affordances (not bound to a selected
  resource) to the workspace header center; they show once connected and reuse the
  same `Open` targets.
- **Scope filters (`Manifest.Scope`)** are global header selectors whose value is
  injected into **every** request for the connection (the Lens/Headlamp namespace
  picker, generalized). Every control encodes to a single string param under
  `p.<Param>`; multiselect joins with the framework `ScopeSeparator` constant and
  the handler splits with `rc.ParamList`. Changing a selector re-fetches lists and
  re-attaches watches without collapsing the tree or remounting an identified
  detail. Plugin-agnostic: the core treats the value as opaque.
- **`EnabledWhen`** reuses the structured `Condition` (§6.1) against the active
  **row fields**; when false the action renders **disabled, not hidden** (a
  stopped container still shows a greyed `Stop`). Absent → always enabled.
- **Grouping & overflow:** same-`Group` actions collapse into one labelled
  dropdown; standalone buttons past a small cap fold into `More ▾`. **Row (bulk)
  bars stay lean** — destructive removal only (delete/drop/truncate/kill);
  lifecycle/edit/single-item actions live on the detail header.
- **`StreamResource`** is a server-push watch of resource state (list deltas or
  object snapshots). Panels opt in with a `Watch *DataSource`; the renderer keeps
  them fresh without polling. Treated like a log stream for keepalive.

### 5.6 ResourceIdentity — stable identity vs display label

**Type:** `sdk/plugin/ui.go` (`ResourceIdentity{Kind,Scope,Namespace,Name,UID}`).
The UI keys/links by `UID` and shows `Name`. `Scope` is one level above
`Namespace` for deep hierarchies (a SQL table: `Scope`=database,
`Namespace`=schema, `Name`=table); it interpolates as `${resource.scope}`
wherever `${resource.namespace}` does.

- **Tab disambiguation is derived from the tree, not hand-stamped.** Two tabs can
  share a name (`users` from DB A vs B), so a tab carries a dim qualifier built
  from the **tree ancestor path** (or the root group's name for flat resources).
  Automatic for any declared tree — no per-plugin qualifier field. Items opened
  outside the tree fall back to `Scope`/`Namespace`.
- **Never stamp a _soft grouping_ onto `Scope`/`Namespace`.** Those are the
  resource's hierarchical **address** (used to fetch it), not a display hint. A
  Docker container's compose project is a label → put it in a column or nest it in
  the **tree**, so the ancestor-path mechanism shows it only when navigated
  through.

### 5.7 External plugins (out-of-tree, dynamically loaded)

The plugin contract is not limited to compiled-in code. Third-party protocols
ship as **out-of-tree, separately compiled Go binaries** built against the SDK
(`sdk/grpcplugin`) and dropped into the configured plugins directory
(`cfg.Plugins.Dir`, e.g. `/data/plugins.d`). They implement the **same**
`Manifest`/`Routes`/`Connect` contract — the projection is byte-identical
in-process vs out — so the core and frontend cannot tell them apart.

- **Loading & supervision** (`internal/extplugin`): each external plugin runs as a
  **process-isolated gRPC subprocess** via `hashicorp/go-plugin`. The `Manager`
  loads on discovery, **crash-respawns**, hot-swaps (`Update`: start new
  subprocess, validate manifest, atomically swap the registry entry, drain old
  sessions, then stop the old — no gateway restart), and `Uninstall`s. External
  and compiled-in plugins register into the one `internal/pluginregistry.Registry`,
  so router/projection/dispatch treat them identically.
- **The security boundary holds across the process split.** A subprocess gets
  **no ambient authority** — it cannot dial the network, touch the DB, or write
  audit on its own. Every privileged effect is a **gRPC callback into the core**
  (`hostServer`): `DialTarget` goes through the core egress/SSRF guard (§9.4) and
  the connection's transport; L7 is wired the same way; `Storage` is core-scoped
  exactly like `rc.Storage`; `Audit` flows into the core audit log.
- **Marketplace** (`internal/pluginmarket`): index sources
  (`cfg.Plugins.Market.Indexes`) are resolved for discovery/install, feeding the
  `Manager` lifecycle.
- **Still-enforced non-goals (§2):** never in-process (no `.so`); manifest + route
  handlers only — no arbitrary UI, HTTP, auth, or storage.

---

## 6. Schema & declarative UI

### 6.1 Fields, secrets, and structured conditions

**Types:** `sdk/plugin/schema.go` (`Schema`, `Group`, `Field`, `FieldType`,
`Condition`, `Rule`, `Operator`, `Validator`, `CredentialKind`,
`CredentialKindInfo`, `CredentialSelector`).

- **Structured conditions, not a string DSL.** The rejected `ShowWhen:
  "auth == private_key"` mini-language forced a parser/evaluator on the frontend (a
  security + complexity hazard). `Condition` composes `AllOf`/`AnyOf`/`All`/`Any`/
  `Not` boolean logic over `Rule{Field,Op,Value}`. `Field` resolves a **dotted
  path** (`can.delete` → `record.can.delete`, exact flat key wins first). **One
  shared evaluator** runs identically in Go (server-side `VisibleWhen`) and the
  browser. Convention: absent predicate **fails open** for `EnabledWhen`, so an
  action the user might be able to perform is never hidden.
- **Field types map to PrimeVue controls** by the renderer (the numeric widget —
  plain/stepper/slider — is the plugin's choice; `min`/`max` come from validators,
  enforced server-side too). Adding a field type is a renderer concern only.
- **`json` / `object` / `array` fields** replace hand-typed JSON blobs. `json`
  renders the CodeMirror editor and binds back as a parsed object/array (parse
  failure blocks submit). `object` is a nested sub-form; `array` is a repeatable,
  keyboard-accessible row list (`MinItems`/`MaxItems`, `+ Add`) whose element is
  another `Field` — so "Create table columns" is a form-builder, not a blob. The
  submitted body is the nested value, so `any`-binding handlers are unchanged.
- **Route-sourced choices:** a `select`/`radio`/`multiselect` may set
  `OptionsSource` (a route) instead of static `Options`; params interpolate from
  the form context (`${resource.*}`, `${record.*}`). Keeps "name an existing
  thing" fields pickers, not free text.
- **Reserved `$…` fields** read ambient form context supplied by the core
  (`$transport`, `$protocol`), so one schema can hide direct-only fields under an
  agent tunnel without storing transport in plugin config.
- **`Secret: true` is the single source of truth** for: encrypt-at-rest (§9.3),
  redact in logs/audit, never serialize back to the client (UI shows "set / not
  set"). `credential_ref` fields carry only a credential ID (never secret
  material); the service layer resolves and attaches decrypted values to
  `ConnectConfig.Credentials` immediately before `Connect`, read via
  `CredentialValueFor(field,key)`. Each selector declares exactly one kind;
  protocols with alternative auth types use separate fields.

**Roles & sharing** (`models.Role`; frontend mirrors `constants/roles.ts` — never
hardcode strings):

- **viewer** consumes shared resources, creates nothing (create routes gated by
  `canCreate` server-side); **operator** has full self-service over its own
  connections/credentials/recordings; **admin** manages **user accounts only**.
- **Admin is a user-management role, not a super-user** — no implicit access to
  other users' connections/credentials/recordings. Resource access is purely
  ownership + grants; the Casbin role/risk policy decides _what risk tier_ a role
  may perform on resources it can already reach.
- **Sharing:** only the **owner** may share; a `manage`-grantee can edit/delete but
  **cannot re-share** (`canShare` vs `canManage`). Admins enumerate users for the
  picker; operators share by exact email (`Users.GetByEmail`). Connection sharing
  does **not** imply credential sharing — a `use`-grantee connects through the
  bound credential without reading it.
- **Accounts are deactivated, never hard-deleted** (`Disabled`, audit trail kept);
  the protected root admin can never be deactivated/demoted.
- **Recordings are private to their creator** — admin included (`RecordingService.
  List` is always actor-scoped).
- Admin surfaces live under **Settings** (users list, per-user Overview/Connections
  metadata/Audit); every user has **My activity** (`GET /audit/me`). Client gates
  (`RoleGate`) are cosmetic; the admin APIs enforce server-side.

Credential-kind metadata comes from the core `GET /api/credential-kinds` catalog;
core owns broad shapes, plugins declare protocol-specific kinds via
`Manifest.CredentialKinds` and the registry derives `CompatibleProtocols` from the
selectors that use them (registration rejects unused or duplicate kinds).

### 6.2 Layout, tabs, tree, panels — bound to routes by ID

**Types:** `sdk/plugin/ui.go` (`Layout`, `Panel`, `PanelType`, `PanelConfig`,
`DataSource`, `Column`, `TableConfig`, `TreeGroup`, `TreeNode`, `TerminalConfig`,
`TerminalGridConfig`, `MetricsConfig`, `RemoteDesktopConfig`, and the specialized
`*Config` structs). `PanelConfig` is a **sealed interface**, so a panel's `Config`
accepts only a real config struct (assigned directly — no `.Map()` ceremony); its
JSON is the wire object the renderer reads. The shared panel-config schema is
projected from these SDK definitions (`sdk/plugin/panel_schema.go`) and consumed by
registration, plugin starter tests, marketplace ingestion, and the frontend
runtime guard, so Go and TypeScript cannot drift.

**Layouts:** `tabs` (flat top tab bar), `sidebar_tree` (hierarchical, keeps
multiple open views as a closable tab strip), `dashboard` (grid of all panels at
once), `single` (one full-bleed panel). A plugin declares a default; the user may
override per connection (stored in preferences).

**Panel types** (each is one core component; the renderer branches on none of the
plugin's identity): `terminal`, `terminal_grid`, `file_browser`, `table`,
`metrics`, `log_stream`, `code_editor`, `diff`, `document`, `query_editor`,
`remote_desktop`, `form`, `enroll`, `object_detail`, `timeline`, `task_progress`,
`split`, `canvas`, `wasm`, `graph`, `trace`, `kv`, `http_client`, `dashboard`.
Adding a panel type (or a file-preview viewer, §6.4) is a one-time core addition,
like a new `PanelType` — it scales across all plugins without touching them.

**Data binding & interpolation** (the load-bearing invariant):

- A panel binds via `DataSource{RouteID, Params}` — never a raw URL. Params
  interpolate `${resource.*}` (the navigable `ResourceIdentity`, present only when
  a row/tree/detail is a real resource) and `${record.*}` (the current data
  row/object; nested paths valid).
- **Single-token rule:** a param whose value is exactly one `${…}` token is
  **omitted** if it resolves to nothing (the handler applies its default); a token
  _embedded_ in a larger string must resolve or it errors loudly. The resolver
  special-cases no field name — only the token structure.
- **Renderer context invariant:** every panel host receives the same tuple
  (`connectionId`, `source`, `config`, optional `resource`, optional `record`,
  actions). Any container that mounts child panels (details, dashboards, splits,
  dock/dialog, tree workspaces) **must pass `resource`/`record` through unchanged**
  unless deliberately creating a new context. Dropping either silently omits
  single-token params.
- **URL sync:** the active location syncs to `/c/:id?v=…&vc=:connectionID` so
  Back/Forward walk visited resources and a pasted link restores the view;
  navigation is query-only so the workspace never remounts and live streams
  survive.

**Table grid** (`TableConfig`) is fully generic — no notion of databases, keys, or
links beyond a few reserved row fields a route may attach:

- `ref` — a **navigable** `ResourceIdentity` (rows with it open a DetailView);
  present only when there's a real destination.
- `_id` — a **stable opaque** row identity for keying/diff/refresh/selection;
  behavior-free (never implies navigable). Flat rows use it so they needn't fake a
  `ref`.
- `_key` — an opaque key map for inline update/delete (absent → read-only).
- `_links` — column key → `ResourceIdentity`, turning cells into links to related
  resources (the renderer doesn't know _why_ they relate).

`HiddenColumns` drops fields; the renderer hard-codes no field names beyond its
reserved keys. Editing is **opt-in and explicit**: `TableConfig.Editable` + a
per-column `Editor` (and matching Insert/Update/Delete routes); `StagedEdits`
buffers a batch and replays it through the same per-row routes; `Exportable` gives
client-side CSV/JSON of loaded rows. **Row-click is automatic** (a row whose `ref`
is a navigable kind opens it; otherwise a selectable table selects it); `RowClick`
only overrides. SQL plugins build mutations through `plugins/shared/sqldb`
(parameterized, identifier-validated, single-row-affected) — invisible to the
renderer.

**Tree** (`TreeGroup`/`TreeNode`): a node is **expandable** when it declares a
children `Source`; omit it for a **leaf** destination (`ResourceKind` → that kind's
list, `Ref` → a specific detail). A pure **container** (source that only yields
child nodes) just expands, never opening an empty tab. A leaf `TreeNode` carries
`Data` (its row fields) so a detail opened from the tree gets the **same record** a
table row would — header badge and `EnabledWhen` behave identically. Expand-vs-open
is decided purely from which field is set; no per-plugin renderer code.

**Route-reference validation at registration** (`sdk/plugin/validate.go`): every
route/action/source reference is checked. **Route IDs are plugin-owned, not global**
— a plugin named `docker` must prefix routes `docker.`, and a reference resolves
only against that plugin's route set (a plugin cannot call another's route by
spelling its ID). Read panels must source from `GET` routes; streaming panels
(`terminal`, `log_stream`, `metrics`, `query_editor`, `remote_desktop`, watch
sources, …) from `WS` routes; mutation/save sources from write methods. Dashboard/
split children validate recursively. `sdk/pluginux` additionally applies UX rules
(destructive/privileged actions must confirm; stream kind must match panel type;
tables declare column types/empty states; etc.) — errors block release.

**Specialized & sandboxed panels:** `code_editor`/`diff`/`query_editor`/`form`/
`graph`/`trace`/`kv`/`http_client` all stay route-bound with generic payloads (the
frontend never branches on plugin name); see their `*Config` in `sdk/plugin/ui.go`.
`canvas` uses a JSON wire protocol via typed SDK structs (`sdk/plugin/canvas`) with
`ScaleMode` (`resize`/`fit`/`scroll`). `PanelWasm` declares `WasmConfig` (not
`Source`): read-only route-backed `Assets`, an `Entry`, and a `Bridge` naming
allowed non-WS routes and WS streams; the browser runs it in a sandboxed iframe
with no same-origin privilege, all data access flowing through the bridge the
parent enforces. `remote_desktop` exposes an RFB/VNC byte stream (VNC streams raw
RFB; RDP is decoded by `grdp` into a synthetic RFB stream) and the browser
lazy-loads noVNC — no plugin-declared browser engine.

### 6.3 Resources, actions, detail views

**Types:** `sdk/plugin/ui.go` (`ResourceType`, `ResourceActions`, `DetailView`,
`HeaderSpec`). One model expresses K8s, Proxmox, Docker, and SQL browsers
identically — the data differs, the renderer does not.

- **One action block, three surfaces** (`ResourceActions`): `Toolbar` (list, no
  row), `Row` (bulk over selected rows — declaring any makes rows selectable, and
  it is destructive-removal only), `Detail` (the open resource's header). No
  overlap. Per-item lifecycle lives in `Detail`.
- `DefaultTab` chooses the initial detail tab (validated against `Tabs`); a
  single-tab detail renders with **no tab bar**.
- `ColumnsSource` covers runtime-only columns (a CRD's printer columns, a SQL
  view): leave `Columns` empty and point at a route returning column defs; the
  renderer fetches them (same nav params) or derives display-only columns from row
  data.
- **Badge color by value:** a `badge` column (and a `HeaderSpec.StatusField`)
  declares `Severities map[string]Severity` mapping cell value → severity; the core
  knows only severity→color, the plugin owns the domain mapping. Apply it
  consistently across list column, embedded tables, and detail header. `number`/
  `percent` columns set `Precision`.

### 6.4 File browser (generic, reused by every fs plugin)

**Type:** `sdk/plugin/ui.go` (`FileBrowserConfig`, `FileBrowserRoutes`,
`FileUploadConfig`, `FileEntry`). The `file_browser` panel is **one generic
component** reused unchanged by `sftp`, the SSH Files tab, `ftp`/`ftps`, `webdav`,
`smb`, `s3`, … — differences are **manifest-only**, zero per-plugin code.

- **Listing:** `Source` returns `Page[FileEntry]` for the current dir; navigating
  re-fetches with the `pathParam` updated (breadcrumb-driven, dirs before files).
  Client-side name filter, sort by name/size/modified, extension-aware icons,
  keyboard nav, `aria-current` — all in the panel.
- **Preview:** selecting a file picks a viewer by MIME/extension. Text/code is
  fetched inline (size-capped, via the CodeMirror editor); image/pdf/audio/video
  **stream bytes from the download route** (served inline) so large media never
  buffers; archives/unknown/over-cap degrade to a metadata card + download. The
  MIME→viewer map is **core, data-driven, extensible**.
- **Mutations** (upload/download/mkdir/rename/delete/move/copy/chmod/archive) are
  ordinary routes carrying risk/permission/audit; shown only when the manifest
  declares them and `Writable` is set. Path-bearing ops send the path under the
  `p.` prefix; JSON bodies are small + validated; upload is `multipart/form-data`
  (button or drag-drop). Writable UTF-8 files are editable in place with a
  dirty-gated Save.
- **Streaming & Range:** downloads/previews stream with constant memory via a
  `Download` whose handler supplies one byte source — a seekable handle (full
  Range/conditional/HEAD via `http.ServeContent`), an offset opener (single-range
  `206`), or a plain body. Backends opt into Range via optional capabilities; no
  per-plugin serving code. Auth is the session cookie, so a bare media `src`
  loads.
- **Safety:** reads are size-bounded; inline responses send
  `X-Content-Type-Options: nosniff` + `Content-Security-Policy: sandbox`; ranges
  are clamped; path traversal is validated server-side; every read/download is
  audited.

---

## 7. Data & transport protocol

### 7.1 URL scheme — RouteID, not raw paths

The browser holds opaque `RouteID`s + a params map. The core resolves them against
the route's registered path template:

```
HTTP : /api/connections/{connectionID}/x/{routeID}?p.<name>=<value>&…
WS   : wss://host/api/connections/{connectionID}/x/{routeID}?p.<name>=<value>&ticket=…
```

- **Route/path params** travel as query keys under a reserved **`p.`** prefix
  (`/vms/{vmid}/…` → `?p.vmid=101`; handler reads `rc.Param("vmid")`), keeping them
  clear of reserved list keys.
- **List controls** use reserved top-level keys: `cursor`, `limit`, `filter`,
  `sort` (§7.2).
- **Body** (POST/PUT/PATCH) is JSON by default, validated via `rc.Bind`;
  file-accepting routes declare a multipart contract (sent only when a payload
  carries `File` values).
- **Every `{name}` in a path template is mandatory identity** (`{id}`, `{kind}`,
  `{namespace}`) — a request missing it is rejected (400) before the handler.
  Optional/config values (terminal `cols`/`rows`, log `tail`/`follow`) must **not**
  be path params; they ride `p.*` with handler defaults. Baking config into a path
  makes routes brittle. Matched-but-rejected routes (missing param, authz denial)
  are logged server-side so a 4xx is never silent.

This makes plugin paths refactorable, gives audit a stable operation id, and keeps
permission checks keyed to the route — without the frontend ever building a path.

### 7.2 Pagination, filter, sort

**Types:** `sdk/plugin/context.go` (`PageRequest`, `Page[T]`). All list routes
paginate (a 10k-pod cluster must not choke). A plugin honors `Sort` itself — DB
plugins push it into `ORDER BY`; in-memory plugins use `plugin.FilterRows` /
`plugin.SortRows` (`sdk/plugin/filter.go`, `sort.go`; numeric cells compare
numerically) so every `Sortable` column actually sorts (a column may sort by an
underlying field, e.g. "age" by its `createdAt`).

### 7.3 Live updates (no blind polling)

**Type:** `sdk/plugin/ui.go` (`ResourceEvent`, `EventType`). A list may declare a
`Watch` WS route emitting `added`/`updated`/`deleted` events; the renderer patches
in place — good for **low-churn** lists. For **high-churn** tables (process/
connection lists where nearly every row changes each tick) a plugin sets
`TableConfig.RefreshIntervalMs`: the renderer re-fetches the current page (same
sort/filter/cursor) on that cadence, bounding work to one page per tick. Neither →
manual refresh.

### 7.4 WebSocket authentication

WS upgrades can't carry `Authorization`. Flow (tickets in `internal/auth`):

1. Authenticated client `POST /api/connections/{id}/tickets` with the `routeID` +
   its resolved params → a **short-lived (~30s), single-use, signed ticket** scoped
   to `(connectionID, routeID, params, user)`.
2. Client opens `wss://…/x/{routeID}?p.…&ticket=<t>`.
3. Core validates the ticket on upgrade (params must match), checks **WS origin**,
   binds the user, runs the `StreamHandler`. Same-site cookies also accepted.

Binding params into the ticket means a ticket minted for `exec into pod-A` can't be
replayed against `pod-B`.

### 7.5 Stream failures carry a meaningful reason

A failed WS **upgrade** exposes no readable body, so the core **accepts the socket
first, then opens the upstream**; a dial/auth/stream failure is delivered as the
**WebSocket close reason** (trimmed to the ~123-byte frame limit), never swallowed.
This is generic across every plugin, so plugins **must** return descriptive,
user-meaningful errors (`dial ssh target: connection refused`) wrapping a sentinel
(`sdk/plugin/errors.go`) for status mapping. The browser surfaces the reason in the
stream/health UI, so "disconnected" is never reasonless.

---

## 8. Session, Channel & connectivity model

### 8.1 Session & Channel

**Code:** `internal/session`, `sdk/plugin/session.go` (`Session`, `Channel`,
`ClientStream`). Invariants:

- **Registry keyed by `(connectionID, actorScope)`**, where `actorScope` is the
  acting user's id — a shared connection does **not** share a live upstream between
  users. The registry is **per-instance**; cross-instance affinity is the
  live-state lease (§8.5). No shared/replicated session store.
- **Every request carries the acting `User` and is independently authorized and
  audited.** Authorization is never inherited from whoever opened the session.
- **Lifecycle:** lazy `Connect` on first use → browser keepalive touches the
  session while connected → idle timeout only when no channels are open → max
  sessions/channels per user → explicit disconnect → periodic `HealthCheck` →
  graceful `Close` on shutdown. A WS close is channel state, not connection health.
- **Failure status:** failed connects/health-checks close the upstream but retain a
  short-lived `error` status (reason + last check) for the UI; explicit disconnect
  clears it.
- **The core pins the session for every accepted WS stream** even if the plugin
  opens no tracked upstream `Channel`, so watch/log/desktop streams aren't reclaimed
  as idle. `Handle.OpenChannel` wraps plugin channels to enforce limits and
  preserve optional capabilities (`ResizableChannel`, `ServerInitChannel`) that
  terminal/desktop handlers discover by type assertion.
- **Concurrency:** plugin structs are stateless singletons; all mutable
  per-connection state lives in the `Session`, and lazily-opened sub-clients (SFTP
  over an existing SSH client) are mutex-guarded.

### 8.2 Transport: direct vs agent (reverse connectivity)

**Type:** `sdk/plugin/session.go` (`Transport`). A target may be unreachable _from_
ShellCN (private/NAT/firewall), so connectivity is orthogonal to protocol: a
connection declares `direct` (core dials from config) or `agent` (an agent inside
the target dials back). The core wires the `NetTransport` at the layer the protocol
needs (§5); plugins share session logic across transports and **never branch on
mode**. Security-sensitive protocols may be agent-only so a shared gateway never
dials its own local Docker/Podman socket on an arbitrary user's behalf.

- **L4** (SSH, Docker, Postgres, …): `cfg.Net.DialContext`.
- **L7** (Kubernetes client-go, private REST APIs): `cfg.Net.HTTP()` — a bare
  dialer isn't enough for fat clients; in agent mode the agent runs an L7
  reverse-proxy that injects the target's credentials.

### 8.3 The agent

**Code:** `cmd/agent`, `internal/transport`. `shellcn-agent` is a single small
static Go binary (also `ghcr.io/charlesng35/shellcn-agent`). It is
**plugin-agnostic**: it proxies one declared target — an L4 forward (`tcp`/`unix`/
`udp`) or an L7 reverse-proxy that injects credentials (`http_proxy`) — over one
outbound, multiplexed (yamux), mutually-authenticated tunnel back to the gateway.
The gateway tells it the mode + endpoint at enrollment. TCP/unix streams are copied
as byte pipes; a `udp` stream is **datagram-framed** (a 2-byte length prefix per
datagram, `WriteDatagram`/`ReadDatagram`) so boundaries survive the stream tunnel,
letting SNMP/IPMI-style protocols reach targets behind NAT.

- **Per-stream forwarding (`ProxyTarget.Forward`):** normally the agent proxies
  every stream to its one `Address`; with `Forward` the gateway prefixes each L4
  stream with a `network`+`address` preamble and the agent dials that instead —
  needed to reach _more_ of the target's own network (a Docker container's web
  port). Negotiated/opt-in (advertised in the hello); stays plugin-agnostic ("dial
  what the gateway names"). Reach widens only within the same target the agent
  already fronts, so it adds no trust boundary (a Docker agent needs host
  networking).
- **Upgrades/hijacks use a loopback bridge:** client-go SPDY/WebSocket (k8s exec/
  attach/port-forward) and moby HTTP hijack (Docker/Podman exec) bypass a custom
  `DialContext` and need a real socket, so for agent transport the tunnel is fronted
  by a session-lived `127.0.0.1` bridge (`plugins/shared/loopback`); plain
  request/response rides the tunnel directly.

### 8.4 Enrollment flow

All enrollment is **connection-scoped, authenticated API** (same TLS as the rest).
The agent connect endpoint is global; the connection it binds to is determined
server-side by the enrollment token, never by the URL.

1. `POST /api/connections/{id}/agent/enrollments` → `{enrollmentId, expiresAt,
   artifacts}` — each an inline `command` (token as an **env var**) or a `url` to
   fetch (for `kubectl apply -f`).
2. Artifact fetch is guarded by a short-lived **single-use signed ticket** (§7.4);
   for `url`-delivery the real token is **minted into the body at fetch time** (the
   record stores a non-redeemable placeholder), so the credential reaches exactly
   one target and never appears in a path/query.
3. UI shows the command in a `PanelEnroll` panel with a **copy** button + live
   "waiting → online" status.
4. The agent dials the global `wss://…/api/agent/connect` and presents the token in
   the **handshake**, never the path/query. After first enrollment the
   installed-agent credential may reconnect until revoked/rotated.
5. The connection flips online; `Connect` is now served via the agent dialer.

**Security:** request paths are routinely logged, so tokens travel only as env var,
handshake message, or single-use `?ticket=`. Unused install tokens are short-lived;
enrolled-agent credentials are scoped to one connection + one `ProxyTarget` (not
repurposable); the tunnel is TLS + mutually authenticated; enroll/fetch/connect/
disconnect are audited; Docker-socket access is `privileged` risk and surfaced as
such.

### 8.5 Multi-instance: live-state leases & cross-instance proxy

**Code:** `internal/livelease` (backed by the `LiveStateLease` store model) +
`internal/server/lease_proxy.go`. Some gateway state is inherently in-memory and
non-shareable — a live session, an agent tunnel, a single-use ticket. To run more
than one instance without a shared session store, ShellCN pins that state to one
instance with a lease and proxies to the owner.

- **Leases** are store-backed, exclusive, renewable claims on a key:
  `SessionLeaseKey(connectionID, actorScope)`, `AgentLeaseKey(connectionID)`, and
  ticket-scoped keys. Modes: exclusive (default) and replace (take over a stale
  key); leases renew while live and release on teardown.
- **Instance identity:** each instance advertises an `InstanceRef` with the
  internal URLs others can reach it on, discovered from the environment (platform
  env host, ECS task metadata, private-IPv4 interfaces) — no static config in
  common cloud setups.
- **Cross-instance proxy:** before serving a connection request the core checks the
  lease holder; if another instance owns the state it transparently reverse-proxies
  there (resolve candidate URLs, probe `/healthz`, promote the reachable one via
  `PreferInternalURL`, forward). An `X-ShellCN-Lease-Proxy` header is a loop guard;
  an unreachable holder fails with `ErrUnavailable`.
- **Affinity, not failover:** any node serves any request, but the live upstream is
  opened exactly once, on the owner. If the owner dies its sessions die and are
  re-established on a new owner on next use (§2).

---

## 9. Security model

Every layer is a core module (`internal/auth`, `internal/policy`,
`internal/secrets`, `internal/audit`), not middleware glued onto plugins.

### 9.1 Authentication

OIDC/OAuth2-ready (Authentik-friendly) **and** local accounts. Platform session via
a secure `HttpOnly`, `SameSite` cookie + CSRF token for state-changing HTTP; WS uses
signed tickets (§7.4). Optional MFA/TOTP for local accounts. v1 ships local
accounts; the OIDC interface is present from day one.

### 9.2 Authorization & action risk

- **RBAC + per-connection ownership/sharing grants**, enforced on every route via
  its `Permission` + `Risk`. **v1 uses embedded Casbin**; OPA is a later additive
  option. Built-in role defaults are seeded in code; additive role/permission/risk
  grants persist in the store and load into Casbin on startup.
- **The risk model** (`safe`/`write`/`destructive`/`privileged`) lets policy
  express rules like: viewers may open terminals but not delete VMs; destructive DB
  statements require approval in production; Docker `exec` is forbidden on
  `critical` containers.

### 9.3 Secrets at rest

`Config` is not stored plaintext. `Secret: true` fields are encrypted with
AES-256-GCM (a data key wrapped by a master key from env/file/KMS), **write-only**
over the API (UI shows "set / not set"), and redacted from logs/audit. v1 ships a
local encrypted vault behind a `SecretStore` interface (OpenBao is a later
drop-in). Reusable credentials are first-class records (owner, grants, stable ID,
kind, non-secret metadata, encrypted material): a connection references one by ID,
`use`-grantees connect through it without reading the value, and rotation updates
once and affects every referencing connection. Audit records credential
ID/name/kind, never material.

### 9.4 Egress / SSRF

A plugin may only dial host(s) from its validated connection config; targets
(including any jump/proxy-URL fields) are checked against an egress allow/deny
policy. External plugins reach the network only via the core's `DialTarget`
callback (§5.7), so the guard applies to them too.

### 9.5 Audit & session recording

**Code:** `internal/audit`, `internal/recording`. Append-only audit keyed by route
`AuditEvent`: who, what, when, which connection, params (secrets redacted), result,
risk. Recording is **plugin-declared and off by default** — the core never records
merely because a panel is `terminal`/`remote_desktop`; the projection must declare
support and the connection policy must enable it. Two classes: **terminal/event**
(asciicast v2; output + resize captured, input disabled unless explicitly enabled)
and **desktop/graphical** (VNC/RFB + RDP; browser canvas capture, operationally
useful but not compliance-grade). Recordings are **private to their creator** (admin
included); read/delete are audited separately from the stream route.

### 9.6 Per-protocol safety (non-negotiable defaults)

- **SSH/SFTP:** password, private-key, stored-credential auth; pinned host-key
  verification by default.
- **Docker:** socket access is root-equivalent → `privileged` risk by default.
- **Databases:** read-only toggle, query timeout, row limit, dangerous-statement
  detection (`UPDATE/DELETE/TRUNCATE/DROP`), confirmation/approval hook, every query
  audited with redacted statement metadata, configurable result redaction by column
  pattern.

---

## 10. Observability

The gateway itself is observable, behind interfaces from day one: `log/slog`
structured logs (JSON for production, colorized console only for interactive
local); Prometheus metrics (sessions/channels open, action latency, WS
connections, failed authorizations, secret-access counts); optional OpenTelemetry
traces; `/healthz` for gateway/store readiness (per-connection health is
session-owned).

---

## 11. Technology stack (committed)

**Backend:** Go 1.26+, `chi` (router), `coder/websocket`, `gorm` (cross-DB store
behind repository interfaces, `AutoMigrate`), pure-Go SQL drivers (no CGO):
`glebarez/sqlite` (default, backed by modernc — **not** the cgo `gorm.io/driver/
sqlite`), `gorm.io/driver/postgres` (pgx, also serves the Postgres plugin's LISTEN/
NOTIFY/COPY), `gorm.io/driver/mysql`. `casbin` (RBAC/ABAC), `embed` (frontend),
`log/slog`.

**Frontend:** Vue 3 (Composition API) + Vite + TypeScript, Pinia + Vue Router +
VueUse, **PrimeVue in unstyled / pass-through mode** + Tailwind (DataTable virtual
scroll, Tree, Tabs, Splitter), xterm.js (terminal), noVNC (remote desktop),
CodeMirror (code/SQL/YAML).

**Build:** `vite build` → `web/dist` → embedded via `web/embed.go` → `go build`
→ one binary.

### 11.1 Persistence & data access (cross-database)

Scope: the **platform's own control-plane store** (users, roles, grants,
connections, reusable credentials + secrets, audit, snippets, session/agent
metadata, preferences) — **not** the database _plugins_, which manage remote DBs.

- **Cross-database, single-binary default.** SQLite is the zero-config default;
  Postgres and MySQL/MariaDB are opt-in for larger/shared deployments; all
  pure-Go. A shared external DB is also the prerequisite for **multi-instance**
  deployment — the live-state lease (§8.5) is persisted there. Full HA (replicated
  live sessions + automatic failover) additionally needs shared session state,
  which stays out of scope (§2).
- **`internal/models` IS the model layer.** Core entity structs carry `gorm:"…"`
  tags directly and are used as the GORM models — **no** parallel row/DTO + mapper
  layer. Tags don't import gorm, so the "**only `internal/store` imports gorm**"
  invariant holds. `serializer:json` for slice/map columns; secret columns
  (`User.PasswordHash`) are `json:"-"` and cleared on read.
- **Repository pattern:** the app depends on small interfaces (`UserStore`,
  `ConnectionStore`, `CredentialStore`, `AuditStore`, `GrantStore`, …) so the
  engine is swappable, the ORM never leaks, and tests use in-memory fakes.
- **Migrations are automatic via `AutoMigrate`** (additive only). Destructive
  changes are never automatic; the `internal/store` boundary preserves the option
  of versioned migrations later. Secrets are encrypted in the service layer (§9.3);
  the store only ever persists ciphertext.

**Caveats:** RDP is pure-Go via GPL `grdp` (decoded server-side, bridged to noVNC
as synthetic RFB) — adopting it makes ShellCN GPL-3.0. SPICE stays out of scope
until a maintained browser engine exists.

---

## 12. Frontend genericity (why adding a plugin needs zero frontend work)

The frontend is a fixed renderer driven entirely by the browser projection (§5.2):
it fetches `GET /api/plugins/{name}` → renders the workspace from `Layout` +
`Tabs`/`Tree` → each panel is one of the ~20 `PanelType` components loading/
streaming from its resolved `DataSource` → clicking a resource opens its
`DetailView`, actions render with `risk`/`requiresConfirm` styling. An agent-mode
connection with no tunnel yet renders `PanelEnroll` until it comes online (§8.4).

Invariants:

- **Long-lived runtime lives in a Pinia session/channel store, never in
  components.** Components attach/detach; switching tabs or rearranging panes never
  drops a stream.
- **Renderer state is connection-bounded.** URL locators, open views, scope values,
  tree expansion, selected rows, and stream instances are keyed by `connectionID`
  plus the view/resource identity, so two connections with overlapping resource
  kinds never restore each other's state.
- **Fixture-first for the declarative surface.** Because the renderer is the
  load-bearing bet, it's built against static fixture manifests + mock data
  (`web/fixtures`) before any real plugin. Fixtures prove the **declarative** panels
  (form/table/tree/detail/enroll); they do **not** prove **streaming** panels
  (xterm resize/backpressure, noVNC handshake), which are validated with their
  first real plugin.

### 12.1 Lazy loading & performance

Load work only when a user reaches it: **panels are code-split** (only shell +
lightweight declarative panels bundled up front; CodeMirror/noVNC/xterm/viz panels
dynamically imported on first use); **plugin projections fetched on demand** (the
connection list needs only id/title/icon); **data is lazy** (tree children on
expand, tables paginate, watches stream deltas); **sessions/channels connect
lazily** and idle-timeout out; platform modules use route-level code-splitting. Net:
first paint stays small and constant regardless of catalog size.

### 12.2 Platform management (control-plane CRUD)

Everything _inside_ a connection is manifest-rendered; the platform's own surfaces
(sign-in, connection/credential CRUD, sharing, administration) are **core UI**
backed by **control-plane CRUD APIs** (same authn → authz → audit guarantees), not
plugin-rendered. Two rules keep them consistent:

1. **The connection config form is manifest-driven too** — "Add connection" fetches
   the protocol's projection and renders its `config` `Schema` with the same generic
   `SchemaForm`; a new plugin gets create/edit for free.
2. **Secrets stay write-only (§9.3)** — edit forms show set/not-set + "replace",
   never the value.

Endpoints: connection `GET/POST/PUT/DELETE` (transport chosen here; agent shows
`PanelEnroll` until online), credential `POST/PUT(rotate)/DELETE` (write-only
material), and `POST/DELETE …/grants` for sharing (`use`/`manage`; owner-only
share). The SPA bootstraps from `GET /api/auth/me`, redirects to login on 401, and a
single client interceptor turns 401→login, 403→forbidden toast, and validation/CSRF/
agent-unavailable errors into consistent actionable feedback.

---

## 13. Worked plugin examples

The reference plugins _are_ the worked examples — read them alongside this spec:
`plugins/ssh` (flat `tabs`: terminal + files + snippets, one shared session; SFTP
reuses the TCP conn), `plugins/docker` & `plugins/kubernetes` (`sidebar_tree`,
resource details, agent transport L4/L7), `plugins/proxmox` (deep hierarchy +
remote desktop), and `plugins/postgresql` / `plugins/mysql` (query editor, schema
browser, editable grids via `plugins/shared/sqldb`). Family helpers live in
`plugins/shared`. Registration is one line in `plugins/registry.go`.

---

## 14. Repository layout & code structure (DX)

A contributor should add a protocol by writing one Go package, and read/test/run
any layer in isolation.

```
cmd/
  server/      entrypoint; wires dependencies, calls plugins.Register(reg)
  agent/       shellcn-agent: the plugin-agnostic reverse-tunnel proxy (§8.3)
internal/
  models/         core entity types = the GORM models (gorm tags on them; no DTO+mapper)
  store/          GORM repositories behind *Store interfaces — the ONLY importer of gorm (§11.1)
  service/        business logic: store + plugins + policy + secrets + audit
  server/         HTTP/WS adapters: chi router, route mounting, dispatch, projection, embed
  pluginregistry/ shared runtime registry every plugin registers into (builtin + external)
  extplugin/      out-of-tree plugin supervisor: gRPC-subprocess load/hot-update/uninstall (§5.7)
  pluginmarket/   plugin marketplace: index resolution, discovery, install
  session/        in-memory session + channel registry + lifecycle
  transport/      direct + agent dialers (NetTransport), tunnel registry, enrollment
  livelease/      store-backed live-state leases + instance-URL discovery (§8.5)
  auth/ policy/ secrets/ audit/ telemetry/ config/   security, observability, config
  ai/             AI assistant (engine confined to internal/ai/engine)
sdk/
  plugin/         Plugin/Manifest/Route/Schema/RequestContext/Session + projection + validate + plugintest
  grpcplugin/     out-of-tree plugin SDK (subprocess side of §5.7)
  pluginux/       manifest UX-lint rules
plugins/
  registry.go     the ONE place first-party plugins are wired: all() → plugins.Register(reg)
  ssh/ docker/ kubernetes/ postgresql/ …   (each: manifest + routes + session + tests)
  shared/         reusable protocol-family helpers only; no manifests, no frontend assumptions
web/
  src/            Vue app; vite build → web/dist (embedded by web/embed.go)
  fixtures/       static manifests + mock data/streams for fixture-first UI dev (§12)
```

**Conventions:** dependencies point inward (`models` ← `store`/`service` ←
`server`/transport; `models` imports no internal packages); explicit DI wired once
in `cmd/server/main.go` (no globals/service-locator/`init()` magic); small
consumer-side interfaces; `context.Context` threaded everywhere; errors wrapped
with `%w` + typed sentinels, normalized to API responses only at the server
boundary; secrets/PII never logged (redaction enforced centrally).

**The contributor promise:** a documented plugin skeleton + the `plugintest`
harness (fake `RequestContext`/`Session`/`NetTransport`, no real infrastructure);
the manifest validated at registration with actionable errors; **adding a protocol
= one plugin package + one line appended to `all()` in `plugins/registry.go` — zero
other core changes, zero frontend changes.** Tooling: `golangci-lint` + `gofumpt` +
`go vet`, no codegen; table-driven unit tests with in-memory fakes + a cross-DB
integration matrix + golden projection tests (the FE/BE contract can't drift); TS
`strict` + ESLint + Prettier; `make build · test · lint · dev`.

---

## 15. Delivery status

The platform is built out well beyond the original MVP: ~20 first-party protocol
plugins across shells, file transfer, containers/clusters, remote desktops,
databases, observability, and directory; the manifest-driven renderer; direct +
agent transport; external (out-of-tree) plugins + marketplace; live-state
multi-instance leases; session recording; and an AI assistant. The original build
order was deliberately **UI-first** — the fixture-driven renderer proven before the
real core or any protocol, streaming panels validated with their first real plugin
(terminal, logs, VNC, query) rather than mocks — because the "renders any plugin"
renderer was the load-bearing bet.

---

## 16. Open questions

- **Secrets backend:** local AES-GCM vault is v1; when do we add OpenBao?
- **Policy depth:** Casbin covers v1; do we need OPA policy-as-code, and when?
- **Connection import:** ingest `~/.ssh/config`, kubeconfig, Docker contexts?
- **Manifest/schema migration:** when a plugin's `Config` schema changes, how are
  stored connection configs migrated/validated? (Tie to `APIVersion`.)
- **Approval workflows:** how are "requires approval in production" actions queued
  and approved?
- **External plugin trust:** out-of-tree gRPC-subprocess plugins are implemented
  (§5.7); the open question is distribution trust — **signing/verification** of
  third-party plugin binaries and provenance of marketplace index sources.
- **Agent operations:** versioning/auto-update of `shellcn-agent`, agent↔core
  version skew, tunnel credential rotation, multiple agents per connection.
- **UDP over agent:** implemented via a datagram-framed `udp` agent mode (§8.3).
  Open follow-ups: connected sockets only (no broadcast/multicast, no unconnected
  multi-destination `PacketConn`), and an agent-side idle reaper for UDP flows.
- **Additional desktop renderers:** vSphere **WebMKS** and SPICE stay out of scope
  until they have maintained browser clients and a real need for a selector-backed
  renderer contract.
- **Specialized panels:** `graph`, `trace`, `kv`, `http_client` are core panels;
  their first plugins should validate protocol-specific route payloads and UX.
