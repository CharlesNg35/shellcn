# ShellCN — Platform Manifest (v2)

> An open-source **infrastructure access gateway / operations cockpit**: a single
> Go binary (with an embedded Vue frontend) through which users reach their SSH
> servers, SFTP/FTP/SMB/NFS/WebDAV/cloud storage, Docker hosts, Kubernetes
> clusters, Proxmox, databases (SQL & NoSQL), and remote desktops (VNC/RDP) — all
> behind one unified, audited, policy-controlled interface.
>
> ShellCN is a **client/gateway**, not a service provider. It does not host the
> infrastructure; it brokers secure, observable access to it.

This document is the canonical spec. It supersedes the earlier design transcript.
Where the transcript contradicted itself (the `Plugin` interface was redefined
~6 times, behavior was dispatched three different ways), this manifest fixes one
authoritative model.

**Plugin model in one line:** every protocol is a **first-party, compiled-in Go
plugin** that exposes one **versioned manifest** (declarative data) plus typed
**route handlers** (behavior). The core validates the manifest, owns security /
sessions / routing / audit / rendering, and serves the browser a _projection_ of
the manifest. The frontend renders entirely from that projection — **adding a
plugin requires zero frontend changes.** The plugin boundary is an internal
design seam for contributor ergonomics, not a dynamic-loading mechanism.

---

## 1. Guiding principles

1. **Plugins declare; the core owns.** A plugin ships a typed **manifest**
   (identity + config schema + views + resources + actions + streams + route
   metadata) and route handlers. The **core** owns rendering, routing, sessions,
   authn/authz, policy, secrets, audit, and transport. Plugins never become
   mini-applications: no UI code, no HTTP plumbing, no auth logic, no storage.
2. **The manifest is the contract.** Plugins expose one **versioned** manifest.
   The core validates it on registration and serves the browser a **rendering
   projection** of it. The frontend renders from that projection only.
3. **Keep the declarative model small and typed.** The manifest is _data_
   (fields, columns, trees, IDs), never a scripting language. If a plugin needs
   logic, it goes in a route handler — never in the manifest.
4. **Data, not pixels.** Plugins describe _what_ to show and _what_ can be done.
   The frontend is a universal renderer of ~10 panel types. (Grafana / Terraform
   / kubectl model.)
5. **One connection, many capabilities, many channels.** A single authenticated
   session is multiplexed: an SSH session yields a terminal, SFTP, and command
   snippets without re-authenticating.
6. **Secure and auditable from day one.** AuthN/AuthZ/policy/audit and
   encryption-at-rest are core requirements. Design the interfaces now; ship
   simple implementations first (embedded RBAC, local encrypted vault).
7. **Single self-contained binary.** Pure-Go dependencies only, so the frontend
   and datastore embed cleanly. Even remote desktops stay in-process: VNC streams
   raw RFB and RDP is decoded by a pure-Go client, both bridged to the browser's
   noVNC engine — no external daemon required.

---

## 2. Non-goals (v1)

- **No native dynamic loading.** First-party plugins are compiled in, and
  third-party plugins run as gRPC subprocesses. The gateway never loads Go
  `.so` plugins. Browser-side WebAssembly is allowed only through the generic
  `PanelWasm` contract: the core owns the sandboxed iframe, asset loading,
  route/stream bridge, auth, CSP, validation, and lifecycle.
- **No horizontal scaling / HA clustering.** Sessions live in memory; v1 runs as
  a **single instance** (§8 notes the future path: session affinity).
- **No plugin-shipped arbitrary UI / iframe escape hatch.** Plugins cannot
  provide raw HTML, JavaScript, or frontend code for the ShellCN app. They may
  declare a sandboxed `PanelWasm` when the use case genuinely needs an isolated
  WASM program; bridge access is explicit in the manifest and limited to declared
  routes, streams, and assets.
- **No SPICE.** SPICE has no production-grade browser client, so it stays out of
  scope. RDP is decoded in-process by the pure-Go `grdp` client and bridged to
  noVNC/RFB — see §6.2. The core exposes a generic `remote_desktop` panel
  contract; it does not let plugins select a browser renderer.

---

## 3. Domain model (glossary)

| Term           | Meaning                                                                                                                                                              |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Connection** | Stored config describing _how to reach_ one target. Owned by a user, optionally shared. It may contain inline encrypted secrets or reference reusable credentials.   |
| **Credential** | A reusable encrypted secret bundle (SSH key/password, DB password, API token) with its own ownership/grants, referenced by many connections without exposing values. |
| **Protocol**   | The plugin id a connection uses (`ssh`, `docker`, `postgres`, …).                                                                                                    |
| **Plugin**     | A stateless, compiled-in singleton that _declares_ a manifest and _connects_.                                                                                        |
| **Manifest**   | The plugin's single versioned contract: identity, config schema, views, resources, actions, streams, route metadata.                                                 |
| **Session**    | A live, authenticated runtime for one connection. Holds all per-connection state.                                                                                    |
| **Channel**    | One stream inside a session: a terminal, log tail, VNC framebuffer, metrics feed, streaming query. Tracked by the core for lifecycle/audit.                          |
| **Capability** | Declarative tag of what a connection/resource supports (`terminal`, `filesystem`…). Feature detection / panel selection only — **never** behavior dispatch.          |
| **Resource**   | A managed object exposed by a connection (a container, pod, VM, table), identified by a stable `ResourceRef`.                                                        |
| **Action**     | A named operation on a connection/resource (`start`, `scale`). A UI affordance pointing at a **route**; risk/permission live on the route.                           |
| **Stream**     | A long-lived channel a panel binds to (terminal, logs, desktop, metrics). Points at a WS **route**.                                                                  |
| **Route**      | A typed server endpoint with metadata (id, method, permission, risk, audit, input schema). The **only** behavior mechanism.                                          |
| **Panel**      | A core frontend component that renders a capability (Terminal, Table, Metrics…).                                                                                     |
| **Transport**  | How a session reaches its target: `direct` (ShellCN dials out) or `agent` (an agent inside the target dials back). Orthogonal to protocol (§8.2).                    |
| **Agent**      | `shellcn-agent`: a plugin-agnostic reverse-tunnel proxy run inside a private target, exposing a socket/port/API back to the gateway (§8.3).                          |

Critical distinction (this killed the "Docker terminal tab" bug in the transcript):

- **Connection-level** capability → makes sense without selecting a resource
  (SSH terminal, Docker container _list_). Rendered as a connection tab/tree.
- **Resource-level** capability → only meaningful for a specific resource
  (exec into _this_ container, console of _this_ VM). Rendered inside that
  resource's **DetailView**.

---

## 4. Architecture: who owns what

| Core platform                                                     | Plugin                                   |
| ----------------------------------------------------------------- | ---------------------------------------- |
| AuthN (OIDC-ready + local), platform sessions                     | Protocol handshake / upstream auth       |
| AuthZ (RBAC + ownership/grants + policy), action risk enforcement | —                                        |
| Connection + reusable credential storage; secret encryption       | Config **schema** (field shapes only)    |
| Session registry, lifecycle, **channel tracking**                 | Per-connection state in its `Session`    |
| Route mounting, auth-wrapping, validation, audit                  | Route **handlers** (pure business logic) |
| Error normalization, pagination                                   | Returns typed data / errors              |
| Manifest validation + **browser projection**                      | One **versioned manifest**               |
| UI shell, ~10 panels, schema/tree/table renderers                 | UI **declarations** (manifest data)      |
| Egress policy / SSRF guard, observability                         | Declares target from validated config    |

A plugin handler **never** sees `http.ResponseWriter`, status codes, headers,
cookies, or auth. It receives a typed `RequestContext` and returns `(any, error)`.

Plugin storage is also core-owned. A plugin may persist small plugin-owned
objects through `rc.Storage`, but it only supplies a logical collection plus the
record key/value. Core resolves and persists the plugin ID, authenticated user
ID, current connection ID, and write timestamps. `Put` always creates or updates
the resolved connection-owned row; scope is only a read/list/delete filter. Empty
storage scope level means current connection scope, while `UserStorage(collection)`
reads across that user's rows for the current plugin.

---

## 5. The Plugin contract (canonical — defined exactly once)

```go
// A Plugin is a STATELESS SINGLETON, compiled in and registered at startup.
// It DECLARES (Manifest), exposes typed ROUTES (handlers), and CONNECTS
// (returns a Session holding all per-connection state). It must hold no
// per-connection state on itself — one instance serves every connection.
type Plugin interface {
    // Pure declarative data. Validated by core; projected to the frontend.
    Manifest() Manifest

    // Typed server endpoints (carry handler funcs; never serialized to client).
    // This is the ONE behavior mechanism: no HandleAction, no plugin-owned HTTP.
    Routes() []Route

    // Open one authenticated runtime for a connection. The core supplies a
    // transport in cfg wired for the connection's mode (direct or agent, §8.2).
    // The plugin uses the layer its client needs — DialContext for socket/TCP
    // protocols, or HTTP() for fat clients like client-go — and never branches
    // on direct-vs-agent.
    Connect(ctx context.Context, cfg ConnectConfig) (Session, error)
}

// ConnectConfig is built by the core: decrypted config + a transport wired for
// the connection's mode. The plugin picks the layer its client needs.
type ConnectConfig struct {
    ConnectionID string      // stable connection id, for plugin-owned caches/log labels
    Transport    Transport   // "direct" | "agent"
    Config    map[string]any // decrypted connection config (typed via Schema)
    Net       NetTransport   // how to reach the target (same API for both modes)
}

func (c ConnectConfig) String(key string) string
func (c ConnectConfig) Int(key string) (int, bool)

// NetTransport exposes the upstream at the layer the protocol needs. A bare
// dialer is enough for socket/TCP protocols but NOT for fat HTTP clients (e.g.
// client-go wants base URL + RoundTripper + auth), which is why "use the dialer
// for everything" is wrong.
type NetTransport interface {
    // L4 — socket/TCP protocols (SSH, Docker, Podman, Postgres, MySQL, Redis,
    // Mongo, SFTP). Identical for direct and agent transport.
    DialContext(ctx context.Context, network, addr string) (net.Conn, error)
    // L7 — fat HTTP clients (Kubernetes client-go, private REST APIs). Returns a
    // base URL + RoundTripper wired to the target; in agent mode the agent's L7
    // reverse-proxy injects credentials (§8.3). ok=false unless an L7 agent mode
    // is in use.
    HTTP() (baseURL string, rt http.RoundTripper, ok bool)
}
```

### 5.1 Manifest — server-side source of truth

```go
type Manifest struct {
    APIVersion  int      // plugin contract version the core must support
    Name        string   // stable id, e.g. "ssh"; [a-z][a-z0-9_-]*
    Version     string   // plugin's own semver
    Title       string   // "SSH"
    Description string
    Icon        Icon     // structured icon (see below)
    Category    Category // builtin catalog key for visual grouping

    Config          Schema               // the connection form
    Capabilities    []Capability         // declarative tags (feature detection only)
    CredentialKinds []CredentialKindInfo // plugin-owned reusable credential kinds

    // Connectivity. Every plugin supports "direct". Targets that may sit in a
    // private network (Docker, K8s, private DBs) ALSO declare "agent" + an
    // AgentProfile describing what the agent proxies and how to install it (§8.4).
    SupportedTransports []Transport
    Agent               *AgentProfile // required iff TransportAgent is supported

    Layout        Layout         // how the connection workspace is arranged
    Tabs          []Tab          // connection-level tabs (LayoutTabs: one at a time; LayoutDashboard: all at once in a grid)
    Tree          []TreeGroup    // connection-level sidebar (Layout == LayoutSidebarTree)
    Resources     []ResourceType // managed object types: columns, actions, detail
    Actions       []Action       // declared actions (reference routes by ID)
    HeaderActions []string       // Action IDs pinned to the workspace header center (connection-wide)
    Scope         []ScopeFilter  // global header selectors (e.g. namespace) injected into every request
    Streams       []Stream       // declared streams (reference WS routes by ID)
}
```

Plugin categories are core-owned display metadata, not behavior dispatch. A
manifest declares one builtin `Category` so the platform can group protocol
pickers consistently without frontend-specific protocol lists. Current builtin
keys include: `shell`, `files`, `containers`, `virtualization`,
`remote_desktop`, `databases`, `orchestration`, `cloud`, `network`, `security`,
`devops`, `observability`, `search`, `messaging`, and `other`. The browser receives the
resolved `{ key, label, icon, order }` category object in both plugin summaries
and full projections.

Route _metadata_ (id, permission, risk, audit, input schema) is declared in
`Routes()` (§5.3). Actions and Streams only **reference** a `RouteID` plus UI
affordances and optional route params — permission and risk are never
re-declared (single source of truth).

**Icons** are structured, not bare strings, so a plugin can ship a Lucide
glyph, a remote image, an inline image, or an emoji. The same `Icon`
type is used by every icon field in the manifest (plugin, tabs, tree groups,
actions, resources):

```go
type IconType string
const (
    IconLucide IconType = "lucide" // a Lucide icon, named kebab-case e.g. "ellipsis-vertical"
    IconURL    IconType = "url"    // remote image (https://…/icon.svg)
    IconBase64 IconType = "base64" // inline data ("data:image/svg+xml;base64,…")
    IconEmoji  IconType = "emoji"  // "🐳"
    IconSVG    IconType = "svg"    // raw inline SVG markup
)
type Icon struct {
    Type  IconType
    Value string
}
```

The whole Lucide set is imported on the frontend and resolved by name at
runtime, so any Lucide icon is usable without registering it. The name is
normalized to Lucide's PascalCase export — any separator or casing works
(`ellipsis-vertical`, `Ellipsis Vertical`, `ellipsis_vertical` all resolve to
`EllipsisVertical`). The renderer falls back to a placeholder glyph if
`Type`/`Value` is empty, the Lucide name is unknown, or an
image fails to load. `url`/`base64` images are sanitized and size-bounded;
inline `svg` markup is **DOMPurify-sanitized** (svg profile — scripts/handlers
stripped) and size-bounded before it is ever injected into the DOM.

### 5.2 Browser projection (rendering contract)

The browser must **render** but never **execute**. The core derives a projection
from the manifest and serves it via `GET /api/plugins` and
`GET /api/plugins/{name}`. The projection **includes**: identity, category,
config schema, layout, tabs/tree, resource columns + actions (with `risk` +
`requiresConfirm`), stream types, panel configs, the shared panel-config schema,
and route bindings (`RouteID` + params only). It
**excludes**: handler funcs, raw mount paths, permission keys, audit-event names,
and any server-only route internals. The opaque `RouteID` is the only handle the
browser holds; the core resolves it to a real URL (§7.1).

### 5.3 Routes — typed, with metadata

```go
type Method string // GET, POST, PUT, PATCH, DELETE, WS
type RiskLevel string
const (
    RiskSafe        RiskLevel = "safe"        // read-only (list, describe)
    RiskWrite       RiskLevel = "write"       // create/update
    RiskDestructive RiskLevel = "destructive" // delete, truncate, restore
    RiskPrivileged  RiskLevel = "privileged"  // shell, exec, raw socket
)

type Route struct {
    ID         string        // "proxmox.vm.snapshots.list" — UI/audit/policy handle
    Method     Method
    Path       string        // plugin-relative mount path: "/vms/{vmid}/snapshots"
    Permission string        // required permission key (server-only)
    Risk       RiskLevel
    AuditEvent string        // e.g. "vm.snapshot.list"
    Input      *Schema       // core validates request body before the handler
    Timeout    time.Duration // 0 = core default
    Handle     Handler       // for HTTP methods
    Stream     StreamHandler // for Method == WS
}

type Handler       func(rc *RequestContext) (any, error)
type StreamHandler func(rc *RequestContext, client ClientStream) error
```

The core wraps every route: authn → authz (permission + risk) → session
resolution → input validation → audit → handler → error normalization.

### 5.4 RequestContext — typed access (fixes the panic-prone `map[string]any`)

The transcript used `params["replicas"].(int)`. JSON numbers decode to
`float64` in Go, so that assertion **panics on every call**. Handlers bind into
typed structs instead:

```go
type RequestContext struct {
    Ctx     context.Context
    User    User    // the acting user — for authz AND audit
    Session Session // the live connection session
}

func (rc *RequestContext) Param(name string) string   // path param
func (rc *RequestContext) Query() url.Values
func (rc *RequestContext) Bind(dst any) error          // body → struct + validate
func (rc *RequestContext) Page() (PageRequest, error)  // cursor/limit/filter/sort

// Example — no assertions, no panics:
type scaleReq struct {
    Replicas int `json:"replicas" validate:"min=0,max=1000"`
}
func (p *K8sPlugin) scale(rc *RequestContext) (any, error) {
    var req scaleReq
    if err := rc.Bind(&req); err != nil {
        return nil, err
    }
    s := rc.Session.(*k8sSession) // core guarantees the session type per protocol
    return s.scale(rc.Ctx, rc.Param("name"), req.Replicas)
}
```

### 5.5 Actions & Streams — UI affordances over routes

```go
type Action struct {
    ID          string // "proxmox.vm.start"
    Label       string
    Icon        Icon
    RouteID     string // route holds Method, Permission, Risk, AuditEvent, Input
    Params      map[string]string // optional route params, same interpolation rules as DataSource
    Confirm     bool
    ConfirmText string
    OnSuccess   *ActionSuccess // optional declarative UI follow-up
    Open        OpenTarget // "view" (default, run the route) | "dock" | "dialog" | "url"
    Panel       PanelType  // panel to host when Open=dock/dialog (sourced from RouteID)
    Config      PanelConfig // typed config for the hosted Panel (e.g. a code_editor's saveRouteId), so a dock/dialog action can open an *editable* panel
    EnabledWhen *Condition // optional: gate the button on the active resource's row
    IconOnly    bool // render as the icon alone (Label becomes the tooltip) — presentation stays manifest-driven
    Group       string // optional: cluster same-Group actions on a surface into one labelled dropdown menu
}

type ActionSuccess struct {
    SelectTab string         // switch to this declared tab after a successful action
    Navigate  NavigateTarget // move the workbench after success, e.g. "list" — return a deleted resource's detail to its list so it doesn't linger
}

type StreamKind string // terminal, logs, desktop, metrics, file, task
type Stream struct {
    ID      string // "docker.container.logs"
    Kind    StreamKind
    RouteID string // a WS route
}
```

`risk`, `requiresConfirm`, and `onSuccess` are projected to the browser for UI
styling and flow control; the **enforced** permission/risk live on the route and
are checked server-side. `onSuccess.selectTab` must reference a declared tab key
and is validated at registration. `onSuccess.navigate` is a generic post-action
move — `"list"` returns a resource detail to its list, so a deleted resource's
detail doesn't linger; the renderer applies it without knowing the resource type.

**Header actions.** `Manifest.HeaderActions` lists `Action` IDs the renderer pins
to the **center of the connection workspace header**, visually grouped and set
apart from the connection's own controls (disconnect/share/edit/delete). They are
connection-wide affordances **not bound to a selected resource** — e.g. a cluster
shell that docks a terminal, or an "apply manifest" dialog. They reuse the same
`Open` targets (`dock`/`dialog`/`url`/run-inline) as any action, so the feature is
fully plugin-agnostic: the core renders whatever IDs the manifest declares and
never special-cases a plugin. They show only once the session is connected.

**Scope filters (global header selectors).** `Manifest.Scope` declares header
selectors whose chosen value scopes **every** request for the connection — the
Lens/Headlamp-style namespace picker, generalized.

```go
type ScopeFilter struct {
    Param         string       // route param the value is injected as, e.g. "namespace"
    Label         string
    Icon          Icon
    Control       ScopeControl // input widget: select (default) | multiselect | search | toggle | …
    OptionsSource *DataSource    // route whose rows are the choices (ValueField/LabelField)
    Options       []FilterOption // or static choices
    ValueField    string
    LabelField    string
    AllLabel      string // label for the empty value that clears the scope
    DefaultValue  string // optional initial value; empty means the cleared/all state
}
```

`Control` is an **open vocabulary**, not a fixed enum: the renderer maps known
names (`select`, `multiselect`, `search`, `toggle`, …) to widgets and falls back
to a select for anything it doesn't recognize, so a new control needs no core
change. Every control encodes its value into the **single string** the param
carries — a select stores one value, a multiselect joins members with the fixed
`ScopeSeparator` wire convention, a search stores free text, a toggle stores the
first option's value when on — so the injection path stays identical regardless of
widget. This keeps the feature **global**, not shaped around any one plugin's needs.

**Consuming a scope.** The handler reads its scope param like any other:
`rc.Param("namespace")` for single-valued controls. A multiselect arrives as the
joined string, so the handler splits it with `rc.ParamList("namespace",
ScopeSeparator)` — the separator is a framework constant (like the `p.*` prefix),
not a per-plugin choice, so the renderer and handler never disagree. The core
validates at registration that a select/multiselect has choices, so a malformed
scope can't ship.

The renderer shows the selectors in the header toolbar (next to header actions)
and keeps the chosen values in a **per-connection scope state** only when the
manifest declares `Scope`. Plugins that do not declare scope have no selector and
receive no injected scope params. The data layer merges that state into every
read and stream request as `p.<Param>` — under any explicit params, which always
win — so lists, watches, detail docs, and streams observe one scope without each
panel remembering to wire it. Changing a selector re-fetches open list-style data
and re-attaches watches; it must not collapse expanded tree nodes or remount a
resource detail that is already identified by its own `ResourceRef`. It is
**plugin-agnostic**: the core treats the params as opaque, and route handlers read
them where relevant (a cluster-scoped kind simply ignores its scope param). This
is strictly better than a per-table filter: one selector, one source of truth,
every scoped request receives the same value.

`Open: "url"` runs the action's route and opens the returned `{"url": "…"}` in a
new browser tab. The route decides the URL — it may be any link (e.g. an external
console) or a relative one the gateway serves, such as a connection-proxy link
(§8.1). The renderer just opens it.

`Config` lets a `dock`/`dialog` action open an **editable** panel: the renderer
hosts `Panel` sourced from the action's route and passes `Config` through as the
panel's config. This is what makes a generic "Create / Edit" flow work — e.g. an
action opens a `code_editor` seeded from `CodeEditorConfig.InitialContent` and
saves via `SaveRouteID`. Plugin-agnostic: the core never interprets the config
keys; the hosting panel does.

**Scope inheritance.** An action rendered on a **list** (`ResourceType.ListActionIDs`)
inherits that list's own `DataSource.Params` as default route params; the
action's explicit `Params` still win. This lets a list-level action act within
the list's scope without restating it — e.g. a "create" action on a list whose
params carry a parent id (a database name, a kind, an index) receives that id
automatically. It is fully generic: the renderer merges the surrounding view's
params into params-less, non-resource actions and never inspects what they mean,
so any plugin gets it for free. The dock tab an action opens is keyed by the
resolved params (or the resource, when one is present), so the same action run
against different scopes opens distinct tabs rather than collapsing into one.

`EnabledWhen` reuses the same structured `Condition` as field visibility, but is
evaluated against the **active resource's row fields** (the data record, e.g.
`state == "running"`). When it's false the renderer shows the action **disabled,
not hidden** — so a stopped container still shows a greyed-out `Stop`, and a
running one a greyed-out `Start`. It is plugin-agnostic: the renderer knows
nothing about containers or states; a plugin declares the predicate over its own
row fields (Docker/Podman gate on `state`, Proxmox on `status`). Omit it for
actions that always apply. Both render sites (detail header, table row actions)
pass the row to the action bar; toolbar actions with no row record evaluate
against an empty record, so only declare `EnabledWhen` on row-scoped actions.

**Grouping & overflow.** By default each action renders as its own button. Actions
that share a `Group` on a surface collapse into a single labelled dropdown menu
(e.g. a container detail's lifecycle ops under `Lifecycle ▾`), and the renderer
folds any standalone buttons past a small cap into a trailing `More ▾` menu — so
a crowded surface stays tidy without per-plugin layout code. Selection (bulk) bars
stay deliberately lean: a resource's `Row` actions are limited to destructive
removal/termination (delete/drop/truncate/purge, or a single kill/cancel);
lifecycle, edit, and single-item actions live on the detail header (`Detail`).

### 5.6 ResourceRef — stable identity vs display label

```go
type ResourceRef struct {
    Kind      string // "vm", "container", "pod", "table"
    Scope     string // optional outer container (database, cluster) — one level above Namespace
    Namespace string // optional scope (k8s namespace, db schema)
    Name      string // display name
    UID       string // stable id; UI keys/links by UID, shows Name
}
```

`Scope` exists for hierarchies deeper than `Namespace/Name` (e.g. a SQL table
lives in `Scope`=database, `Namespace`=schema, `Name`=table). It interpolates as
`${resource.scope}` wherever `${resource.namespace}` does, so a connection can
browse every database/cluster without per-plugin frontend code.

**Tab disambiguation is derived from the resource tree, not hand-stamped.** Two
tabs can share a name — `users` from database A vs database B — so a tab carries
a dim qualifier. The renderer builds it from the **tree ancestor path**: the
intermediate parent labels you navigated through (`business_db / public` for a
SQL table), or, when the node sits directly under its root group, that **group's
name** (`Containers`, `Compose`) so flat resources still get a category hint.
This is automatic for any tree the plugin already declares — **no per-plugin
qualifier field, zero extra wiring**. For items opened _outside_ the tree (a
cross-link cell, a detail's sub-table) the renderer falls back to `Scope`/
`Namespace` — which exist anyway as the resource's intrinsic identity (interpolated
as `${resource.scope}`/`${resource.namespace}` for data), so this is not
qualifier-specific stamping.

**Never stamp a _soft grouping_ onto `Scope`/`Namespace`.** Those fields are the
resource's hierarchical **address** (used to fetch it), not a display hint. A
Docker container's compose project is a label, not an address — putting it in
`Namespace` makes the fallback leak it as the qualifier even when the container
was opened from the flat `Containers` node (it didn't go _through_ compose). Such
groupings belong in a column; if a plugin wants them in the tab qualifier, it
nests them in the **tree** (so the ancestor-path mechanism shows them only when
you actually navigate through them) — there is no per-plugin qualifier code.

---

## 6. Schema & declarative UI

### 6.1 Fields, secrets, and structured conditions

The transcript used a string mini-DSL (`ShowWhen: "auth == private_key"`), which
forces a parser/evaluator (and one on the _frontend_ = a security + complexity
hazard). Replaced with a **structured condition**:

```go
type Schema struct{ Groups []Group }
type Group struct {
    Name   string // form section/tab: "Basic", "Auth", "Advanced"
    Fields []Field
}

type FieldType string // text, email, url, tel, number, stepper, slider,
                      // password, select, radio, multiselect, file, toggle,
                      // textarea, json, duration, credential_ref,
                      // object (nested sub-form), array (repeatable rows),
                      // autocomplete (free text + Options/OptionsSource suggestions)

type Field struct {
    Key         string
    Label       string
    Type        FieldType
    Required    bool
    Secret      bool        // ENCRYPTED at rest; WRITE-ONLY over the API (§9.3)
    Default     any
    Placeholder string
    Help        string
    Options       []Option    // static choices for select/radio/multiselect
    OptionsSource *DataSource // route → live choices for select/radio/multiselect
    Credential    *CredentialSelector // only for Type == "credential_ref"
    VisibleWhen   *Condition  // structured — NOT a string expression
    Validators    []Validator // min/max/regex/oneOf, evaluated server-side too
    Step          any         // increment for number/slider (default 1)
    Fields        []Field     // sub-fields of an "object" field
    Item          *Field      // element descriptor of an "array" field (recurses)
    MinItems      int         // array bounds (seed/keep this many rows)
    MaxItems      int
    ItemLabel     string      // singular row label, e.g. "Column"
    AddLabel      string      // "+" button label (default "Add")
}

type CredentialKind string // ssh_private_key, ssh_password, kubeconfig, tls_client_cert, db_password, api_token, ...
type CredentialKindInfo struct {
    Kind                CredentialKind
    Label               string
    SecretLabel         string
    SecretMultiline     bool
    IdentityLabel       string   // optional non-secret principal label
    CompatibleProtocols []string // derived by core from registered plugin selectors
}
type CredentialSelector struct {
    Kinds     []CredentialKind // credential kinds this field accepts
    Protocols []string         // optional protocol filter; empty = any compatible kind
    Required  bool             // true when inline secret fallback is not allowed
}

type Condition struct {
    AllOf []Rule // AND
    AnyOf []Rule // OR
}
type Operator string // eq, neq, in, nin, empty, notEmpty
type Rule struct {
    Field string
    Op    Operator
    Value any
}
```

The generic form renderer maps each `FieldType` to a PrimeVue control — `number`
to a plain numeric input, `stepper` to `[−] value [+]` buttons, `slider` to a
track+readout, `radio` to an option group, `email`/`url`/`tel` to typed text
inputs, etc. The numeric widget is the plugin's choice (a stepper/slider is
opt-in, not forced on every number). Numeric `min`/`max` come from the `min`/`max`
validators (single source of truth, enforced server-side too) and `Step` sets the
increment. Adding a field type is a renderer concern only; plugins just declare
the type, options, default, and validators.

**Structured (`json`) fields.** A `json` field renders as the same CodeMirror
editor used by `PanelCodeEditor` (syntax highlight, JSON validation), not a plain
textarea — `textarea` stays a plain multiline control for free text. The renderer
pretty-prints an object `Default` for editing and **parses the text back into an
object on submit** (a parse failure blocks submit with a field error), so a `json`
field always binds server-side as a JSON object/array — never a string. This is
the in-form counterpart to `CodeEditorConfig.SaveBodyKey`: a single `json` blob
that fills one whole resource (a mapping, an index's settings) is better as an
action that opens `PanelCodeEditor` in a dialog; a `json` field is for structured
input that lives **alongside other fields** in a form (e.g. a _Create table_ form's
`columns` next to its `name`). Both paths are plugin-agnostic — the plugin only
declares the field/config; the renderer owns the editor and the round-trip.

**Composite (`object`/`array`) fields.** For structured input that would otherwise
be a hand-typed `json` blob, a field declares `object` (a nested sub-form whose
sub-fields are `Fields`) or `array` (a repeatable list whose element is `Item`,
which recurses — so an array of objects is the common "rows" case). The renderer
turns an `array` into a bordered, keyboard-accessible row list with a **"+ Add"**
control (labelled by `AddLabel`/`ItemLabel`) and per-row remove, honouring
`MinItems`/`MaxItems`; an `array` of `object` items gives the full form-builder
(e.g. _Create table_ columns: each row a `name` text + `type` select + `nullable`
toggle). The submitted body is the **nested value** (`columns: [{name,type,…}]`),
identical to what the JSON path produced, so handlers that bind `any` are
unchanged. Nested `VisibleWhen`/`Validators`/`Required` are evaluated per element
against that element's own values. Every field type — including `select`,
`multiselect`, and `optionsSource` route-sourced choices — is available inside a
composite, since the element is just another `Field`. This is the structured,
type-safe replacement for the JSON-textarea pattern; a bare `json` field remains
for genuinely freeform documents (a query, a raw policy) where a builder doesn't
help.

**Route-sourced choices.** A `select`/`radio`/`multiselect` field may set
`OptionsSource` instead of (or in addition to) static `Options`: the renderer
fetches it when the form opens and maps each row to `{value,label}`. Its params
interpolate `${resource.*}` from the form's resource context, so a field can
offer the live values of a related resource — e.g. a _Create index_ form whose
_Columns_ field is a multiselect of the table's real columns, instead of a
free-typed comma-separated string. Plugin-agnostic: the core only fetches rows
and reads `value`/`name`/`label`; the plugin's route decides what the choices
are. This keeps any "name an existing thing" field a picker, not free text.

Rules normally read submitted schema fields. Reserved `$...` field names read
ambient form context supplied by the core, currently `$transport` and
`$protocol`, so one schema can hide direct-only target fields when a connection
uses an agent tunnel without storing transport metadata inside plugin config.

`Secret: true` is the single source of truth for: encrypt-at-rest, redact in
logs/audit, never serialize back to the client. `credential_ref` fields never
carry secret material either; they carry only a credential ID selected from the
user's authorized reusable credentials. The service layer resolves the credential
and injects decrypted values into `ConnectConfig.Config` immediately before
`Connect`, so plugin code does not learn whether the value came from an inline
connection secret or a shared credential. It also injects the selected
credential kind alongside the resolved material, so plugins that explicitly
choose a multi-kind `credential_ref` can route each kind correctly. Manifests
should still prefer separate fields when the user experience or protocol
semantics differ, such as password authentication versus client-certificate
authentication.

Connection sharing does not imply credential sharing. A user with connection
`use` may open the shared connection even when they cannot list or use the
underlying credential directly; the backend resolves the already-bound
credential through the connection owner after authorizing connection access.
Connection edit/detail responses redact credential IDs the acting user cannot
use directly and return only per-field state. A manager may keep that hidden
credential via `preserveCredentials` or replace it with a credential they can
use. Credential grants remain managed from the credentials surface, not from a
connection form.

**Three roles (`models.Role` — never hardcode the strings; the frontend mirrors
them in `constants/roles.ts`).**

- **viewer** — consumes only resources shared to it; creates nothing. Every create
  route (`POST /connections|/credentials|/connection-folders`) is gated by
  `canCreate` (operator/admin) **server-side**; the UI also hides those affordances
  when `!auth.canCreate`, but the check is enforced regardless of the UI.
- **operator** — full self-service over its own connections/credentials/recordings.
- **admin** — manages user accounts only (see below).

**Sharing the picker.** Only admins may enumerate users
(`GET /admin/users/search`, the autocomplete). Operators, who can't enumerate
accounts, share by the recipient's **exact email**: a grant request carries either
`subjectId` (admin picked) or `email` (resolved server-side via
`Users.GetByEmail`). The ShareDialog shows an autocomplete for admins and a plain
email field otherwise.

**Admin is a user-management role, not a super-user.** Admin grants the right to
manage users (create/role/deactivate/invite, email status) — it confers **no
implicit access** to other users' connections, credentials, or recordings.
Resource access is purely ownership + grants: the role/risk policy (Casbin) decides
_what risk tier_ a role may perform on resources it can reach, while
ownership/grants decide _which_ resources. `canAccessConnection`,
`canManageConnection`, `canManageCredential`, and connection/credential sharing are
all owner/grant-based with no admin bypass.

**Accounts are deactivated, never hard-deleted.** Deactivating sets `Disabled`
(the account can't sign in) while keeping its audit trail and owned resources. No
admin may deactivate themselves; the **protected** root admin (created on first run)
can never be deactivated or demoted; only the root admin manages other admins.

**Sharing visibility & re-share rule.** Connection/credential lists return the
viewer's owned **plus** shared-with-me resources; each carries the owner's display
name (name only, no email) so the UI shows a "Shared" badge and "Shared by {name}".
Only the **owner** may share (create/list/revoke grants) — a connection
`manage`-grantee can edit/delete it but **cannot re-share**. The list DTO exposes
`canShare` (owner) distinct from `canManage` (owner||manage-grant); the frontend
gates the Share affordance on `canShare` and the backend enforces it.

**Recordings are private to their creator.** Every user — admin included — sees
only their own recordings (`RecordingService.List` is always scoped to the actor;
`canView` is owner-only). Admins never view another user's recordings or content.

**Admin management lives in Settings.** The Settings page is a hub of navigable,
breadcrumbed links (`AppBreadcrumb` wraps PrimeVue `Breadcrumb`, styled in the
preset). Admin-only links (Users) are gated by a generic `RoleGate` (client gate;
the admin APIs enforce server-side). `Settings → Users` (`/settings/users`) lists
users + invitations; opening one goes to `Settings → Users → {name}`
(`/settings/users/:id`) with three tabs: **Overview** (name, email, status, role +
deactivate), **Connections** (a metadata-only inventory — name, protocol, icon,
created date; never config, secrets, or access), and **Audit** (the user's
paginated audit trail, `GET /admin/users/:id/audit`). These are the only cross-user
views an admin gets, and none expose another user's resources.

**My activity.** Every user has `Settings → My activity` (`/settings/activity`),
their own paginated audit trail via `GET /audit/me` — the self-service counterpart
to the admin Audit tab, reusing the same `AuditTable`.

The schema renderer resolves choices for a `credential_ref` field through a core
API, not through plugin routes: `GET /api/credentials?kind=...&protocol=...`
returns only `CredentialSummary` records the acting user may use (`id`, `name`,
`kind`, optional `identity`, derived `protocols`, timestamps) and filters by the
field selector plus the selected connection protocol. The response never contains
secret material, encrypted blobs, storage keys, or values. Selecting one stores
the credential ID in the connection config.

The credential create/edit UI gets kind metadata from the core
`GET /api/credential-kinds` catalog. Core owns only broad reusable credential
shapes; protocol-specific kinds are declared by plugin manifests through
`Manifest.CredentialKinds`. Plugins declare protocol support by using kinds in
their `credential_ref` selectors, and the registry derives
`CompatibleProtocols` from the registered selectors instead of hardcoding
possible protocol names in the kind definition. Registration rejects duplicate
kind IDs and rejects plugin-declared kinds that are not used by that plugin's
schema. A plugin still owns the stricter selector on each `credential_ref` field,
so an SSH field only lists SSH-compatible credentials and never shows unrelated
kinds such as kubeconfig. Credential create/edit forms never let users manually
pick protocol compatibility; they display the registry-derived compatible
protocols as read-only badges, and the backend derives the stored protocol list
from the selected kind.

### 6.2 Layout, tabs, tree, panels — bound to routes by ID

```go
type Layout string
const (
    LayoutTabs        Layout = "tabs"         // flat: SSH, Redis (a top tab bar)
    LayoutSidebarTree Layout = "sidebar_tree" // hierarchical: Docker, K8s, Proxmox, SQL
    LayoutDashboard   Layout = "dashboard"    // grid: every Tab panel shown at once
    LayoutSingle      Layout = "single"       // one full-bleed panel, no tab bar: VNC, RDP, SFTP, Telnet
)

// Workbench primitives (all generic / cross-plugin — no per-plugin frontend):
//   - A resizable bottom DOCK hosts panels (terminal/logs/editor) as persistent,
//     closable tabs across navigation. An Action with Open=dock/dialog opens its
//     Panel there (or in a modal), sourced from the action's route.
//   - A TreeNode may carry ResourceKind (+ optional ListParams to scope it, e.g.
//     a namespace) to open that kind's LIST view (like a top-level group) instead
//     of a single-resource detail — for nested nav.
//   - A leaf TreeNode carries Data (its resource row fields), so a detail opened
//     from the tree gets the SAME record a table row would — the header status
//     badge and action gating (Action.EnabledWhen) behave identically whether the
//     resource was reached via the tree or a table. Without it a tree node only
//     knows its ref, and state-dependent UI can't evaluate.
//   - The sidebar_tree workspace keeps MULTIPLE open views as a closable tab strip
//     (details + lists), not one selection at a time — switch/close, state kept.
//   - The ACTIVE location syncs to the URL (`/c/:id?v=…&vc=:connectionID`): the
//     top tab (tabs layout) or active workbench view (sidebar_tree), encoded
//     self-sufficiently so browser Back/Forward walk the visited resources and a
//     pasted/refreshed link restores the view. `vc` owns the locator; a workspace
//     ignores `v` when `vc` is for another connection, even if both plugins have a
//     resource kind with the same name. Navigation is query-only (same `/c/:id`),
//     so the workspace never remounts — live terminals/streams survive.
//     single/dashboard carry no `v`.
//   - MetricsConfig drives the metrics panel (stat cards, gauges, time-series)
//     entirely from declared field keys; the renderer hardcodes none.
//   - TerminalConfig opts a terminal panel into zoom and/or scrollback search.
//     TerminalGridConfig adds a renderer-owned split workspace for protocols that
//     can safely open one independent terminal channel per pane. Split workspaces
//     use the same StreamTerminal route; mandatory recording keeps using the
//     single terminal panel so audit capture is unambiguous.

type PanelType string
const (
    PanelTerminal      PanelType = "terminal"       // single xterm.js terminal
    PanelTerminalGrid  PanelType = "terminal_grid"  // user-managed split terminal workspace
    PanelFileBrowser   PanelType = "file_browser"
    PanelTable         PanelType = "table"
    PanelMetrics       PanelType = "metrics"
    PanelLogStream     PanelType = "log_stream"
    PanelCodeEditor    PanelType = "code_editor"    // CodeMirror (YAML/JSON/SQL)
    PanelDiff          PanelType = "diff"           // read-only CodeMirror diff/merge view
    PanelDocument      PanelType = "document"       // JSON/BSON tree editor
    PanelQueryEditor   PanelType = "query_editor"
    PanelRemoteDesktop PanelType = "remote_desktop"
    PanelForm          PanelType = "form"           // schema-rendered
    PanelEnroll        PanelType = "enroll"         // agent install command + live status (§8.4)
    PanelObjectDetail  PanelType = "object_detail"  // structured property sheet with copy/redaction/raw JSON
    PanelTimeline      PanelType = "timeline"       // events/tasks/audit trail over a list route
    PanelTaskProgress  PanelType = "task_progress"  // long-running task stream with progress/cancel/retry
    PanelSplit         PanelType = "split"          // resizable horizontal/vertical child panel composition
    PanelCanvas        PanelType = "canvas"         // plugin-driven draw/input protocol over a WS stream
    PanelWasm          PanelType = "wasm"           // sandboxed browser-side WASM app with declared assets/bridge

    PanelGraph      PanelType = "graph"       // node/edge viz — Neo4j, topology
    PanelTrace      PanelType = "trace"       // span waterfall — Jaeger, Tempo
    PanelKV         PanelType = "kv"          // typed key-value editor; value types are config-driven (KVConfig.ValueTypes)
    PanelHTTPClient PanelType = "http_client" // request builder + response viewer — http-api, graphql, grpc
    PanelDashboard  PanelType = "dashboard"   // grid of child panels (DashboardConfig.Cells), as a tab/view
)

// A panel binds to data via a route ID, not a raw URL. Params interpolate from
// the active resource ("${resource.uid}") or static values. The core resolves
// the RouteID + params to a concrete URL (§7.1).
type DataSource struct {
    RouteID string
    Params  map[string]string // {"vmid": "${resource.uid}", "node": "${resource.namespace}"}
}
```

**Param interpolation rule (plugin-agnostic).** A param whose value is a _single_
`${…}` token is sourced entirely from that one value: if it resolves to nothing
(an unset optional ref field such as `namespace`/`scope` on a cluster-scoped
resource), the param is **omitted** and the route handler applies its own
default/validation — never a blank request. A token _embedded_ in a larger
string must resolve, since a blank would corrupt the value, so that errors
loudly. The resolver special-cases no field name — only the token structure.

```go
// Panel is one renderable panel — a detail/connection tab OR a dashboard cell
// (they are the same shape, so there is one type). Config holds a typed config
// struct (TableConfig, MetricsConfig, …) — see PanelConfig below.
type Panel struct {
    Key    string
    Label  string
    Icon   Icon
    Type   PanelType   // wire key "panel"
    Source *DataSource // RouteID-based; never a raw path
    Config PanelConfig // a typed config struct, set directly (no .Map())
    Span   int         // dashboard layout hint (>=2 fills the row)
}

// PanelConfig is a sealed interface every config struct implements, so Config
// accepts only a real config — never arbitrary data. Plugins assign the struct
// directly (Config: TableConfig{…}); JSON marshalling produces the same wire
// object the renderer reads, so there is no hand-written .Map() ceremony.
type PanelConfig interface{ /* sealed: TableConfig, MetricsConfig, … */ }

type TableConfig struct {
    Columns      []Column
    Watch        *DataSource
    RefreshIntervalMs int      // live view: re-fetch the current page on this cadence (alternative to Watch)
    DefaultSort       *SortKey // column to sort by on first load
    ActionIDs    []string // toolbar actions; references Manifest.Actions
    RowActionIDs []string // selected-row actions; references Manifest.Actions
    Selectable   bool     // row checkboxes WITHOUT a row-action bar (actions in detail); implied by RowActionIDs
    // Declaring RowActionIDs makes the table's rows selectable (checkbox column,
    // multi-select). The row-action bar then operates on the selection: a route
    // action runs once per selected row (bulk), gated by Action.EnabledWhen
    // across every selected row. A row's ResourceRef supplies each target's
    // params. Set Selectable instead to keep checkboxes but no row bar (a browse
    // table whose actions live in the detail). Inline-editable grids keep their
    // own row controls instead.

    // Editable data grid — plugin-agnostic; the renderer assumes nothing about
    // what the data represents.
    Editable      bool        // master switch for inline cell edit / add-row / delete-row
    RowKey        []string    // columns identifying a row (when not carried per-row)
    Insert        *DataSource // POST {"values":{col:val}}
    Update        *DataSource // PATCH {"key":{col:val},"values":{col:val}}
    Delete        *DataSource // DELETE {"key":{col:val}}
    EmptyText     string
    StagedEdits   bool        // opt-in: buffer edits/inserts/deletes; commit or discard as a batch
    HiddenColumns []string    // field keys to omit from auto-derived columns
    Exportable    bool        // opt-in: show the generic CSV/JSON export of loaded rows
    RowClick      RowClickAction // override; empty → auto (navigate navigable rows, else select)
}

type RowMutation struct {
    Key    map[string]any `json:"key,omitempty"`
    Values map[string]any `json:"values,omitempty"`
}
```

The grid is fully generic: it has no notion of databases, keys, or links beyond
a few reserved row fields a route may attach, and renders nothing plugin-specific
on its own:

- `ref` — a **navigable** resource identity: rows that carry it can open that
  resource's DetailView. Present only when there is a real destination.
- `_id` — a **stable, opaque** row identity used for keying, diffing, live
  refresh, and selection. Behavior-free: it never implies a row is navigable.
  Rows that aren't resources (a process, a metric sample) use `_id`; navigable
  rows may reuse `ref.uid` as their identity. This separation keeps identity
  distinct from navigation, so a flat table needn't fake a `ref` just to be keyed.
- `_key` — an **opaque** key map identifying the row for inline update/delete;
  when absent the row is read-only. Mutation bodies are uniform across plugins
  (`{values}` / `{key,values}` / `{key}`), so the renderer ships zero per-plugin
  code. A plugin may also declare static `RowKey` columns instead.
- `_links` — map of column key → `ResourceRef`; the grid turns those cells into
  links that open the related resource. The renderer doesn't know or care _why_
  they're related (the SQL plugins derive them from foreign keys; others could
  use ownership, parentage, etc.).

Columns the grid should not display are declared with `HiddenColumns` — the
renderer never hard-codes field names beyond its own reserved keys.

`Column.Type = "icon"` renders a compact icon-only cell. The row value may be a
Lucide icon name string or a full `Icon` object, and the cell is rendered through
the same sanitized icon pipeline as every other manifest icon. This is for visual
kind/status hints that are still data-driven; the renderer must not infer icons
from plugin names, resource kinds, or column keys.

**Add-row inputs are typed by column.** The add-row form derives each input
widget from its column's declared type — a numeric column gets a number input, a
boolean a toggle, JSON a code area, the rest a text box — rather than a
one-size-fits-all text field. When columns come from a `ColumnsSource`, the
renderer maps the column's data-type string (e.g. `integer`, `boolean`,
`timestamptz`, `jsonb`) onto these generic widgets, so the typing works for any
plugin whose column route reports a type, with no per-plugin code.

The SQL plugins are one consumer: they build mutations through the driver-neutral
`plugins/shared/sqldb` `Dialect` (parameterized, identifier-validated),
re-validate the client key against the real primary key (`sqldb.ValidateRowKey`),
and require exactly one affected row — but none of that is visible to the
renderer.

**Editing is opt-in and can be staged.** With `StagedEdits` the grid buffers cell
edits, added rows, and deletions locally — highlighting pending cells/rows and
showing a commit/discard bar — instead of sending each change immediately. On
commit it replays the buffer through the same per-row Insert/Update/Delete routes
(no batch endpoint, no extra contract); discard reverts to the loaded values. Off
by default, so a plugin opts into the review-then-apply workflow deliberately.

**Export is opt-in.** `Exportable` (table grids) and `exportable` (query-editor
config) are off by default; a plugin must declare them, so data never leaves a
panel unless the manifest allows it. Export is client-side (the loaded rows) and
fully generic — every panel that sets the flag gets CSV + JSON for free.
Graph image export is the exception: `GraphConfig.Exportable` is a pointer, so
omitted/`null` keeps export enabled, while `false` disables the client-side
PNG/JPEG/SVG export menu for sensitive graph panels.

**Row-click is automatic; selection is the checkbox.** The renderer learns which
resource **kinds are navigable** from the projection (those with a detail view)
and decides per row, with no per-table declaration: a row whose `ref` is a
navigable kind **opens it**; otherwise, on a selectable table, the body click
**selects** the row (selection is also always available via the checkbox column).
So a database's _tables_ list navigates on row-click while its _columns_ list
selects — automatically, because `table` is a resource kind and `column` is not.
`RowClick` exists only to **override** this: `detail` (open a dialog of every
field — for field-rich flat tables like processes — and show a per-row details
icon), or an explicit `navigate` / `select` / `none`. Editable grids always
reserve the body for cell editing; interactive cells (link cells, action buttons,
the details icon) work regardless.

```go

type RemoteDesktopConfig struct {
    Resize     bool
    Clipboard  bool
    Audio      bool
    RepeaterID string
}

// TerminalConfig opts a terminal panel into extra controls; both off by default
// so a plugin enables only what its terminal needs. Generic and plugin-agnostic:
// the renderer ships the zoom (font-size +/- and Ctrl/⌘ +/-/0) and scrollback
// search (find with match navigation) UI for any plugin that declares them.
type TerminalConfig struct {
    Zoom   bool // font-size +/- controls and Ctrl/⌘ +/-/0
    Search bool // scrollback find with match navigation
}

type TreeGroup struct {
    Key          string
    Label        string
    Icon         Icon
    Source       DataSource   // expandable: returns Page[TreeNode], loaded lazily
    ResourceKind string       // leaf: click opens this resource's list
    Ref          *ResourceRef // leaf: click opens this resource's detail
    Badge        *Badge       // optional count/status, route-backed
}

type TreeNode struct {
    Key            string
    Label          string
    Icon           Icon
    Ref            *ResourceRef       // opens this resource's detail
    Leaf           bool               // true means no expandable children
    ChildrenSource *DataSource        // expandable: returns child TreeNode rows
    Badge          *Badge
    ResourceKind   string             // opens this resource kind's list
    ListParams     map[string]string  // merged into that list's DataSource params
    Data           map[string]any     // row fields for detail header/actions
}
```

A group is **expandable** when it declares a `Source` (children load on expand).
Omit `Source` to make it a **leaf** — a direct destination with no expandable
children: set `ResourceKind` to open that kind's list, or `Ref` to open a
specific resource's detail (e.g. a single dashboard/landing view). This keeps
single-destination roots (an overview, a flat top-level list) from rendering a
spurious expand arrow and a duplicate child. Plugin-agnostic: the renderer
decides expand-vs-open purely from which field is set.

A group **opens a view only if it resolves to a resource** — via `ResourceKind`,
`Ref`, or a `Source` whose route matches a resource's list. A pure **container**
(a `Source` that only yields child nodes, e.g. a category like Workloads) has no
view of its own: clicking it just expands, never opening an empty tab. A group
may set both `Source` and a destination to expand _and_ open a view.

A returned `TreeNode` follows the same rule. A node with `ChildrenSource` is
expandable unless `Leaf` is true. A node with `ResourceKind` opens that kind's
list, with `ListParams` merged under the resource list's own `DataSource.Params`;
this is for drill-down navigation where the node itself is a category or parent
object but the next screen is still a generic table. A node with `Ref` opens a
detail. A node can be a non-expandable list destination by setting
`ResourceKind`, `ListParams`, and `Leaf: true`; no plugin-specific renderer code
is allowed for this.

The core validates every route/action/source reference during plugin
registration. Route IDs are plugin-owned, not global: every route declared by a
plugin named `docker` must use the `docker.` prefix, and every manifest
reference resolves only against that same plugin's route set. A plugin cannot
call another plugin's route by spelling its ID; the gateway first resolves the
connection protocol, then looks up the route inside that plugin only.
`DataSource.Method`, when declared, must match the referenced route. Read panels
(`table`, `form`, `document`, `code_editor`, `diff`,
`file_browser`, `object_detail`, `timeline`, etc.) must source from `GET` routes; streaming
panels (`terminal`, `terminal_grid`, `log_stream`, `metrics`, `query_editor`,
`remote_desktop`, `task_progress`, `canvas`, and table/resource watch sources) must source
from `WS` routes.
Canvas streams use a JSON wire protocol, but plugin code should use the SDK's
typed canvas structs (`CanvasFrame`, `CanvasCommand`, `CanvasRegion`,
`CanvasPointerEvent`, etc.) rather than hand-built maps.
Canvas panels declare their sizing policy in `CanvasConfig.ScaleMode`:
`resize` reports the current viewport as the logical drawing surface, `fit`
scales a declared `Width`/`Height` logical surface into the available panel while
mapping input back to logical coordinates, and `scroll` keeps a declared
`Width`/`Height` surface at 1:1 CSS pixels with panel scrolling for naturally
oversized surfaces such as maps, whiteboards, timelines, dependency graphs, and
linked-node diagrams. Ready and resize events include logical size, viewport
size, scale, DPR, and theme.
`PanelWasm` does not bind through `Panel.Source`; it declares `WasmConfig`
instead. `Assets` are read-only route-backed files, `Entry` names the primary
WASM artifact, boot scripts must also be listed in assets, `Bridge.Routes` must
name non-WS routes with matching methods, and `Bridge.Streams` must name WS
routes. The browser runs the WASM app in a sandboxed iframe with no same-origin
privilege; all data access goes through the declared bridge, which the parent
renderer enforces. The host exposes the entry path as `window.shellcn.entry`,
the current ShellCN theme through `window.shellcn.theme`, and live theme changes
through `window.shellcn.onTheme(fn)`. Generic WASM without boot scripts is
instantiated directly from `Entry`, which is useful for simple C/C++/Rust modules
that export `_start` or `main`. Generic WASM with boot scripts lets the loader
own startup, which is required by framework builds such as Leptos, Yew,
wasm-bindgen, or Emscripten; those loaders should fetch bytes with
`window.shellcn.asset(window.shellcn.entry)`. `Width` and `Height` are optional;
omit them for a full-panel app, use `scroll` for naturally taller sandbox
content, and declare both dimensions only for a fixed logical viewport that
should be fitted or scrolled as a surface.
Table mutation sources (`insert`, `update`, `delete`) and editor/form save
methods must resolve to write methods (`POST`, `PUT`, `PATCH`, or `DELETE`).
Dashboard and split child panels are validated recursively with the same rules
as top-level tabs. The core projects a shared panel-config schema derived from
SDK panel/config definitions; registration, plugin starter tests, marketplace
ingestion, and the frontend runtime guard all consume that schema so Go and
TypeScript do not drift. The frontend renders a panel error instead of mounting
a panel with malformed config.

`sdk/pluginux` applies renderer UX rules to manifests: destructive and
privileged actions must confirm; `OpenDock` is reserved for long-lived
interactive panels; stream route kind must match the panel type; closed-value
fields use select/radio, suggested custom values use autocomplete; tables
declare meaningful column types, empty states, and sort/watch/refresh behavior;
actions declare icons, labels, and useful success behavior. Errors block
release; warnings are review prompts.

`remote_desktop` routes expose an RFB/VNC byte stream and the browser lazy-loads
noVNC. VNC plugins stream raw RFB after the gateway authenticates upstream; RDP
plugins decode the session with the pure-Go `grdp` client and bridge it to a
synthetic RFB stream. Both therefore render through the same panel and noVNC
client. There is no plugin-declared browser engine selector in the v1 contract.

For `PanelForm`, `Source` returns a `Schema`. Read-only forms declare only
`Source`; editable forms add `Config.submitRouteId` plus optional
`submitMethod`, `submitLabel`, and `params`. The renderer validates visible
schema fields client-side, interpolates submit params from the active resource
using the same `${resource.*}` rules as `DataSource`, and submits either JSON or
`multipart/form-data`. Multipart is selected automatically when a visible
`file` field contributes browser `File` values; non-file structured values are
JSON-encoded parts.

```go
type FormPanelConfig struct {
    SubmitRouteID string
    SubmitMethod  Method
    SubmitLabel   string
    Params        map[string]string
}
```

`PanelCodeEditor` is read-only unless its config declares a save route:

```go
type CodeEditorConfig struct {
    Language       string            // syntax mode, e.g. "yaml", "json", "sql"
    InitialContent string            // optional template; skips the read route when set
    SaveRouteID    string            // write route; absent means read-only
    SaveMethod     Method            // POST/PUT/PATCH/DELETE; defaults to PUT in renderer
    SaveParams     map[string]string // interpolated route params
    SaveBodyKey    string            // optional: JSON-parse editor content into this request body key
    SaveExtra      map[string]any    // optional static fields merged into the save body
}
```

By default saves send `{"content":"<editor text>"}`. When `SaveBodyKey` is set,
the renderer parses the editor text as JSON and sends it under that key, merging
`SaveExtra` first. This keeps document-store create/upsert flows generic: a
manifest action can open `PanelCodeEditor` in a dialog with `InitialContent`, then
save `{"document": {...}}`, `{"item": {...}}`, etc. without frontend
plugin-specific code. Writable code editors show a renderer-owned **Diff** button
only after the loaded buffer changes; it opens a read-only before/after diff of
the loaded content and the edited buffer. Plugins do not declare custom UI for
this common review flow.

`PanelDiff` is the route-backed version for preview workflows where a plugin can
compute both sides, such as Kubernetes dry-run apply, MongoDB document replace,
Swarm service spec update, or generated DDL preview:

```go
type DiffMode string
const (
    DiffSideBySide DiffMode = "side_by_side"
    DiffUnified    DiffMode = "unified"
)

type DiffConfig struct {
    Language          string   // syntax mode for both sides
    OriginalField     string   // route response field for the left/original side; default "original"
    ModifiedField     string   // route response field for the right/modified side; default "modified"
    OriginalLabel     string
    ModifiedLabel     string
    Mode              DiffMode // side_by_side default, or unified
    CollapseUnchanged bool
}
```

The source route returns an object with the configured fields. Values may be
strings or structured JSON; structured values are pretty-printed before display.
Do not use `PanelDiff` for current-state inspection; use `PanelObjectDetail` for
structured objects and `PanelDocument` for rendered documents.

`PanelQueryEditor` sends statements over its declared stream route. It may also
declare a best-effort cancel route. It is an executable editor/results panel,
not a plain editor; plugins that need editing without an execute affordance use
`PanelCodeEditor` instead. Labels and language are plugin-declared so the panel
stays protocol-neutral.

```go
type QueryEditorConfig struct {
    Language          string
    Label             string
    ExecuteLabel      string
    CancelLabel       string
    RunningLabel      string
    EmptyText         string
    InitialQuery      string
    CancelRouteID     string
    CancelParams      map[string]string
    CompletionRouteID string
    CompletionParams  map[string]string
    Exportable        bool // opt-in CSV/JSON export of the result set
}
```

Specialized panels are also core renderer components. They stay route-bound and
plugin-neutral:

```go
type GraphConfig struct {
    Layout        GraphLayout // legacy hint; the panel auto-lays out with dagre
    FitView       bool
    ExpandRouteID string // optional; nodes become expandable — the panel fetches a
                         // node's neighbourhood from this read route and merges it
    ExpandParam   string // node-id param name for ExpandRouteID (default "node")
    Exportable    *bool  // nil/null/default true; false hides PNG/JPEG/SVG export
}

// The graph payload is plugin-emitted JSON the renderer treats generically:
//   nodes: [{ id, label, group?, summary?, properties?, fields?[] }]
//   edges: [{ id?, source, target, label?, sourceField?, targetField? }]
// A node with `fields` ([{name,type?,key?}]) renders as a record/ERD table box
// (relational schemas); a node without renders as a plain node (graph DBs). The
// panel auto-lays out (dagre), colours edges by label, and filters by edge type.

type TraceConfig struct {
    ServiceField string // optional span field used as service label
}

type KVConfig struct {
    CreateRouteID string // optional; enables New/create affordance
    ReadRouteID   string
    WriteRouteID  string // optional; enables saving an existing selected key
    DeleteRouteID string
    KeyParam      string // default "key"
    Writable      bool
}

type HTTPClientConfig struct {
    ExecuteRouteID string
    Methods        []string
    DefaultMethod  string
    DefaultURL     string
    DefaultHeaders []HeaderDefault
    DefaultBody    string
}
```

Their route payloads are generic: graph nodes/edges, trace spans, key summaries
and key details, or request execution responses. The frontend does not branch on
plugin name.

A plugin declares a default layout; the user may override it per connection, and
the choice is stored in user preferences. The default is a suggestion, not a lock.

### 6.3 Resources, actions, detail views

```go
type ResourceType struct {
    Kind          string          // matches ResourceRef.Kind
    Title         string
    List          DataSource      // route → Page[Row]
    Watch         *DataSource     // optional WS route → stream of ResourceEvent (§7.3)
    Columns       []Column        // static columns
    ColumnsSource *DataSource     // optional: route → column defs {name,label} when only known at runtime
    Actions       ResourceActions // this resource's actions, grouped by render surface
    Detail        DetailView      // opened when a row is clicked
}

// ResourceActions groups a resource's action IDs by the one surface each renders
// on — the single, non-overlapping action contract for a resource.
type ResourceActions struct {
    Toolbar    []string // list toolbar (no row context): create, prune
    Row        []string // bulk over selected rows (delete); declaring any makes rows selectable
    Detail     []string // the one open resource, in its detail header
    Selectable bool     // row checkboxes without a row bar; Row implies it
}

type DetailView struct {
    Header     HeaderSpec // title + status badge (actions come from ResourceActions.Detail)
    DefaultTab string     // optional initial tab key; must reference Tabs
    Tabs       []Panel    // resource-level panels (Overview/Console/Logs/Config…)
}
```

This single model expresses K8s, Proxmox, Docker, and SQL schema browsers
identically — the data differs, the renderer does not.

**One action block, three surfaces.** A resource declares its actions once, in
`Actions`, grouped by where the renderer shows them — no overlap, no duplication.
`Toolbar` → the list toolbar (no row); `Row` → the selected-row **bulk** bar, and
declaring any is what makes rows selectable; `Detail` → the open resource's detail
header; `Selectable` → row checkboxes without a row bar (a browse table whose
actions live in the detail). `Row` implies `Selectable`. Per-item lifecycle
(start/stop/restart/open) lives in `Detail`; a bulk delete goes in `Row`. The
container/engine browse tables (docker/podman/swarm) are selectable with a
delete-only row bar and everything else in the detail.

`DefaultTab` lets a plugin choose which tab a resource detail opens first while
keeping the tab list fully declarative. It is a view preference only: users can
still switch tabs normally, and the manifest validator rejects a key that is not
present in `Tabs`. This is useful when the most common workflow is an editable
view (for example a document resource whose first tab is read-only JSON and whose
second tab is the `code_editor`).

A detail with a **single tab** renders just that panel, with **no tab bar** —
so a one-view detail (e.g. a dashboard landing page) doesn't show a redundant
lone tab. The renderer decides this from `len(Tabs)`; plugins declare tabs the
same way regardless of count.

`ColumnsSource` covers lists whose columns are only known at runtime — e.g. a
Kubernetes CRD's own printer columns, or a SQL view's projected columns. Leave
`Columns` empty and point `ColumnsSource` at a route returning `{name,label}`
rows; the renderer fetches them (scoped by the same nav params as the list) and
falls back to deriving columns from the row data if neither is set. Generic: the
core never knows the column names; the plugin's route supplies them.

Flat `PanelTable` tabs use the same declarative action model as resources via
`TableConfig.ActionIDs` and `TableConfig.RowActionIDs`. Utility tables such as
SSH snippets stay plugin-owned and manifest-driven: the plugin declares
create/run/delete actions once, the validator ensures the IDs exist, and the
generic table renderer places toolbar or selected-row affordances without
plugin-specific frontend code.

**Number formatting.** A `Column` of type `number` or `percent` may set
`Precision` to fix its fraction digits, so volatile metrics (CPU %, load) render
as stable values rather than long floats; `percent` also appends `%`.

**Badge color by value.** A `Column` of type `badge` may declare
`Severities map[string]Severity`, mapping a lower-cased cell value to a severity
(success/warn/danger/info/secondary) so the renderer colors it (e.g. a pod's
`running` reads green, `failed` red). A `HeaderSpec` carries the same
`Severities` map for its `StatusField`, so a detail view's header badge is
colored by the same contract as the list column it mirrors. Plugin-agnostic: the
core only knows severity→color, never the domain values — the plugin owns the
mapping. Unmapped values render neutral. Apply it wherever a status/state/health
value appears — the list column, the embedded tables (e.g. a workload's Pods
tab), and the detail header — so a value is colored consistently everywhere it
shows.

### 6.4 File browser & file-type preview (generic, reused by every fs plugin)

The `file_browser` panel is **one generic component** reused unchanged by every
filesystem/storage plugin (`sftp`, the SSH **Files** tab, `ftp`, `ftps`,
`webdav`, `smb`, `nfs`, `s3`, `minio`, `rclone`, …). Differences between those
plugins are **manifest-only**; the panel ships zero per-plugin code (§12).

It binds to routes via panel `Config`:

```go
type FileBrowserConfig struct {
    PathParam       string // route param carrying the directory/file path
    ReadRouteID     string // GET inline preview content
    DownloadRouteID string // GET original bytes
    WriteRouteID    string // PUT text content for the selected path
    UploadRouteID   string // POST multipart/form-data into current dir
    MkdirRouteID    string // POST JSON {name} in current dir
    RenameRouteID   string // PATCH JSON {name} for selected path
    DeleteRouteID   string // DELETE selected path
    Writable        bool   // gates mutation affordances
    MultipleUpload  bool
    MaxUploadBytes  int64
    UploadFieldName string // optional; defaults to "files"
}
```

- **Listing.** The `Source` returns `Page[FileEntry]` for the current directory.
  Navigating into a directory re-fetches the same route with the `pathParam`
  updated (breadcrumb-driven); directories always group before files. A toolbar
  filter narrows the current listing by name (client-side, case-insensitive)
  with a distinct "no match" empty state and resets on navigation; a sort
  control orders by name / size / modified in either direction. List and grid
  rows show an extension-aware icon plus size and modified time, and the list is
  keyboard-navigable (arrow keys, `aria-current` on the selection). Breadcrumbs
  are a `<nav>` with the current folder marked `aria-current="page"`. All of this
  lives in the panel, not the manifest — identical for every fs plugin.

  ```go
  type FileEntry struct {
      Name     string // base name
      Path     string // full path (stable id)
      IsDir    bool
      Size     int64
      MIME     string    // optional; the panel also infers from extension
      ModTime  time.Time
      Mode     string    // optional, e.g. "rwxr-xr-x"
      Symlink  string    // optional link target
  }
  ```

- **Preview, popular types by default.** Selecting a file renders it with a
  viewer chosen by MIME/extension. Text/code is fetched inline via `readRouteId`
  (size-capped utf8); binary viewers (image/pdf/audio/video) **stream the bytes
  from `downloadRouteId`** (served inline) rather than an inline base64 payload —
  so arbitrarily large media works and never buffers in memory. The default,
  built-in viewer set (no plugin code):
  - **text / code / config** (`.txt .log .md .json .yaml .toml .sh .py .go .ts
.js .sql .conf …`) → code viewer (reuses the CodeMirror `code_editor`; plain
    `<pre>` fallback). Markdown may render or show source.
  - **images** (`.png .jpg .jpeg .gif .webp .svg .bmp .ico`) → image viewer.
  - **pdf** → embedded document viewer.
  - **audio / video** (`.mp3 .wav .ogg .flac .mp4 .webm .mov`) → native player.
  - **archives / binaries / unknown** → metadata card + **download** (no inline
    preview); large files past the cap also degrade to download.

  The MIME→viewer mapping is **core, data-driven, and extensible** (a new viewer
  is a one-time core addition, like a new `PanelType`), so it scales across all
  storage plugins without touching any of them.

- **Mutations** (upload / download / mkdir / rename / delete) are ordinary
  routes carrying `risk`/`permission`/`audit` like any action (§5.3); the panel
  shows them only when the manifest declares them and `writable` is set.
  Path-bearing operations send the selected/current path through the configured
  `pathParam` under the standard `p.` query prefix (§7.1). JSON mutations carry
  small validated request bodies (`{name}` for mkdir/rename, delete body optional
  because the path param is authoritative). Upload uses `multipart/form-data`,
  appending selected browser `File` objects under `uploadFieldName` and leaving
  the browser to set the content boundary. Files can be added via the upload
  button **or dropped onto the panel** (a drop overlay appears while dragging
  when uploads are enabled); rename/mkdir submit is disabled until the value is
  non-empty and actually changed, so a no-op can't be triggered.

- **Selected-file pane.** Selecting a file opens a pane with a header (name,
  size, modified time, permissions, symlink target, download) above the
  viewer/editor; writable UTF-8 files are editable in place via the CodeMirror
  editor with a dirty-gated Save.

- **Streaming & Range.** Downloads/previews stream over HTTP with constant memory.
  The core serves a `Download` whose handler supplies one byte source — a seekable
  handle (full `Range`/conditional/HEAD via `http.ServeContent`), an offset opener
  (single-range `206` for object-store/WebDAV/FTP-style backends), or a plain body
  (full `200`). `HEAD` is allowed on `GET` routes for range probing. Backends opt
  into Range by implementing the optional `Seekable`/`RangeOpener` capabilities;
  this is generic — no per-plugin serving code. Auth is the session cookie, so a
  bare media-element `src` loads a protected stream.

- **Safety.** Read is size-bounded; inline responses send `X-Content-Type-Options:
nosniff` and `Content-Security-Policy: sandbox` (neutralizing inline SVG/HTML);
  ranges are clamped to the file size; path traversal is validated server-side;
  every read/download is audited.

---

## 7. Data & transport protocol

### 7.1 URL scheme — RouteID, not raw paths

The browser holds opaque `RouteID`s + a params map (`DataSource.Params` and
`Action.Params`, §6.2/§5.5).
The core resolves them against the route's registered path template:

```
HTTP : /api/connections/{connectionID}/x/{routeID}?p.<name>=<value>&…
WS   : wss://host/api/connections/{connectionID}/x/{routeID}?p.<name>=<value>&ticket=…
```

**Parameter transport (explicit):**

- **Route/path params** travel as query keys under a reserved **`p.`** prefix —
  e.g. a route registered as `/vms/{vmid}/snapshots` is reached with
  `?p.vmid=101`. The core fills the template from `p.*`; the handler reads
  `rc.Param("vmid")`. The `p.` prefix keeps them from colliding with reserved
  list keys.
- **List controls** use reserved top-level keys: `cursor`, `limit`, `filter`,
  `sort` (§7.2).
- **Request body** (POST/PUT/PATCH) is JSON by default, validated against
  `Route.Input` via `rc.Bind` (§5.4). Routes that accept browser files declare a
  multipart input contract; the renderer sends `multipart/form-data` only when a
  schema/action payload contains `File` values.

This makes plugin paths refactorable, gives audit a stable operation id, and
keeps permission checks keyed to the route — without the frontend ever building a
path.

**Path-template params are required identity only.** Every `{name}` in a route's
path template is **mandatory**: a request missing it is rejected (400) before the
handler runs. So a path template carries only **resource identity** (`{id}`,
`{kind}`, `{namespace}`). Optional/config values (terminal `cols`/`rows`/
`command`/`tty`, log `tail`/`follow`/`timestamps`, …) must **not** be path
params — they ride the same `p.*` query and the handler reads them with a default
when absent. Baking config into the path makes a route brittle: dropping that
param from a manifest source silently breaks the route. A matched-but-rejected
route (missing param, authz denial) is logged server-side so a 4xx is never
silent.

### 7.2 Pagination, filter, sort (lists must scale)

A flat list that returns everything will choke on a 10k-pod cluster. All list
routes are paginated:

```go
type PageRequest struct {
    Cursor string            // opaque; empty = first page
    Limit  int               // core clamps to a max
    Filter map[string]string // server-side filtering
    Sort   []SortKey
}
type Page[T any] struct {
    Items      []T
    NextCursor string // "" when exhausted
    Total      *int   // optional/approximate
}
```

A plugin honors `Sort` itself: DB plugins push it into `ORDER BY`; plugins that
fetch a whole list and paginate in memory use the shared, generic helpers
`plugin.FilterRows` / `plugin.SortRows` (numeric cells compare numerically, others
case-insensitively) so every `Sortable` column actually sorts. A column whose
displayed value isn't directly comparable can sort by an underlying field (e.g. a
relative "age" sorts by its `createdAt` timestamp).

### 7.3 Live updates (no blind polling)

A list resource MAY declare a `Watch` WS route emitting events; the renderer
patches the table/tree in place:

```go
type EventType string // added, updated, deleted
type ResourceEvent struct {
    Type     EventType
    Ref      ResourceRef
    Resource json.RawMessage
}
```

`Watch` suits low-churn lists (containers, pods) where state changes
occasionally and per-row diffs are cheap. For **high-churn** tables — process or
connection lists where nearly every row changes every tick — a plugin instead
sets `TableConfig.RefreshIntervalMs`: the renderer re-fetches the current page
(same sort/filter/cursor) on that cadence and replaces it in place. This bounds
work to one page per tick and reuses pagination/sort/filter, where streaming a
per-row diff of the whole table would flood the client. Plugins with neither
rely on manual refresh.

### 7.4 WebSocket authentication (browsers can't set Authorization)

WS upgrades cannot carry an `Authorization` header. The flow:

1. Authenticated client calls `POST /api/connections/{id}/tickets` with the
   `routeID` **and its resolved params** → receives a **short-lived (~30s),
   single-use, signed ticket** scoped to `(connectionID, routeID, params, user)`.
2. Client opens `wss://…/x/{routeID}?p.<name>=…&ticket=<t>`.
3. Core validates the ticket on upgrade (and that its bound params match the
   request), checks **WebSocket origin**, binds the user, then runs the
   `StreamHandler`. Same-site cookies are also accepted.

Binding the params into the ticket means a ticket minted for `exec into pod-A`
can't be replayed against `pod-B`.

### 7.5 Stream failures carry a meaningful reason (contract)

A failed WS **upgrade** exposes no body the browser can read, so the core
**accepts the socket first, then opens the upstream session**; a dial/auth/stream
failure is delivered to the client as the **WebSocket close reason** (close code

- a human message trimmed to the 123-byte close-frame limit), never swallowed.
  This is **generic across every plugin** — the core conveys whatever error the
  plugin's `Connect`/`StreamHandler` returns. Therefore plugins **MUST** return
  descriptive, user-meaningful errors (e.g. `dial ssh target: connection refused`,
  `ssh handshake failed: unable to authenticate`), wrapping a sentinel (§ errors)
  for status mapping. The browser surfaces this reason in the UI (the stream status
  bar / connection health), so "disconnected" is never reasonless. Non-stream
  (HTTP) routes already return the error body; the client treats a 5xx/network
  failure as a connection-level fault and shows its message.

---

## 8. Session, Channel & connectivity model

### 8.1 Session & Channel

```go
type Session interface {
    HealthCheck(ctx context.Context) error
    // Open a tracked upstream stream (terminal, sftp, logs, desktop, metrics).
    // The core records it for idle-timeout, counters, per-channel audit/cleanup.
    OpenChannel(ctx context.Context, req ChannelRequest) (Channel, error)
    Close() error
}

// Optional: a Session may also reverse-proxy browser HTTP to an upstream it
// reaches, powering generated "open in browser" links (with Open:"url"). The
// core mounts GET|POST|… /api/connections/{id}/proxy/* (cookie-authenticated,
// authorized, audited, privileged), strips the prefix, and delegates; the plugin
// maps the remaining path to its upstream and streams (incl. WebSocket upgrades).
//
// The proxy subtree is CSRF-exempt (a proxied third-party app can't carry our
// token; SameSite=Lax already blocks cross-site cookie use on non-GET). Generic
// HTML/CSS/redirect/cookie rewriting + the in-scope service worker live in the
// shared `plugins/shared/webproxy`; a plugin only resolves how to reach the
// upstream: Docker/Swarm/Podman dial the container/service port (over the agent
// when applicable); Kubernetes proxies via the pod **port-forward** subresource —
// an L4 tunnel, so the app's own Authorization/cookies survive (the API server's
// HTTP proxy strips them). Web ports are picked best-effort (no hardcoded lists);
// HTTPS is inferred from a "443" suffix or a port name.
type HTTPProxy interface {
    ServeHTTPProxy(w http.ResponseWriter, r *http.Request)
}

type ChannelRequest struct {
    Kind   StreamKind
    Params map[string]string
}
type Channel interface {
    io.ReadWriteCloser
    Kind() StreamKind
}

// Optional Channel capabilities are preserved by the session tracker but never
// invented by it. Handlers may type-assert only when their stream kind needs it.
type ResizableChannel interface {
    Resize(cols, rows int) error
}
type ServerInitChannel interface {
    ServerInit() []byte // initial desktop/RFB server bytes after gateway auth
}

// What a WS StreamHandler receives — the browser side of the pipe.
type ClientStream interface {
    io.ReadWriteCloser
    Context() context.Context // closed when the client disconnects
}
```

- **Registry:** sessions live in an in-memory registry keyed by
  `(connectionID, ownerScope)`. In v1 `ownerScope` is the acting user's id, so a
  shared connection does **not** share a live upstream session between users. v1
  is single-instance (§2); future scale path is session-affinity routing or a
  session broker — explicitly deferred.
- **Shared vs per-user:** live sessions are effectively keyed by
  `(connectionID, userID)`.
  Shared connections let another user connect through the saved connection
  without reading its credentials, but **every request carries the acting `User`
  and is independently authorized and audited**. Authorization is never inherited
  from whoever opened the session.
- **Lifecycle:** lazy `Connect` on first use or explicit browser connect →
  browser keepalive touches the one `(connectionID, userID)` session while
  the workspace is connected → idle timeout only when no channels are open →
  max sessions / channels per user → explicit disconnect → periodic
  `HealthCheck` → graceful `Close` on shutdown. A WebSocket close is channel
  state, not connection health; the pooled session status drives connection
  badges.
- **Failure status:** failed connects and failed health checks close the live
  upstream but retain a short-lived in-memory `error` status with the failure
  reason and last health-check time. Explicit disconnect clears that status.
- **Session health:** `Session.HealthCheck(ctx)` is the per-user live connection
  probe used for badges and idle lifecycle. Plugins do not expose a separate
  global health contract; stateless plugin singletons are considered healthy if
  the gateway process loaded them and their manifest validated.
- **Active browser streams:** the core pins the session for every accepted
  WebSocket stream even if the plugin does not open a tracked upstream
  `Channel`. This keeps watch/log/desktop/terminal-style streams from being
  reclaimed as idle while the browser socket is still open.
- **Tracked channel wrappers:** `Handle.OpenChannel` wraps plugin channels to
  enforce channel limits and decrement the count exactly once on `Close`. The
  wrapper must preserve optional capabilities such as `ResizableChannel` and
  `ServerInitChannel`, because terminal and remote-desktop handlers discover
  those capabilities by type assertion. It must not expose those methods when the
  original channel did not implement them.
- **Transport keepalive:** the core may run WebSocket ping/pong keepalive on
  gateway/browser and gateway/agent hops. Keepalive frames are transport control
  frames, never plugin payload bytes, so they cannot change terminal input, query
  text, or log output. If a plugin's upstream protocol has its own idle timeout,
  the plugin session owns that protocol-specific keepalive.
- **Concurrency (fixes the data race in the transcript):** plugin structs are
  stateless singletons; all mutable per-connection state lives in the `Session`,
  and lazily-opened sub-clients (e.g. SFTP over an existing SSH client) are
  mutex-guarded:

```go
type sshSession struct {
    client *ssh.Client
    mu     sync.Mutex
    sftp   *sftp.Client // opened lazily, guarded
}
func (s *sshSession) filesystem() (*sftp.Client, error) {
    s.mu.Lock(); defer s.mu.Unlock()
    if s.sftp == nil {
        c, err := sftp.NewClient(s.client) // reuses the one TCP connection
        if err != nil { return nil, err }
        s.sftp = c
    }
    return s.sftp, nil
}
```

### 8.2 Transport: direct vs agent (reverse connectivity)

A target may not be reachable _from_ ShellCN (private network, NAT, firewall).
So connectivity is orthogonal to protocol — a connection declares a transport:

```go
type Transport string
const (
    TransportDirect Transport = "direct" // core dials the target from config (host:port)
    TransportAgent  Transport = "agent"  // an agent inside the target dials BACK to ShellCN
)
```

The core wires the transport (§5 `NetTransport`) at the layer the protocol needs,
and the plugin uses that layer without branching on direct-vs-agent:

- **L4 — socket/TCP protocols** (SSH, Docker, Podman, containerd, Postgres,
  MySQL, Redis, Mongo, SFTP): use `cfg.Net.DialContext`. This is the genuinely
  clean case — session code is identical for direct and agent.

```go
// Docker — identical for direct and agent transport.
cli, _ := dockerclient.NewClientWithOpts(
    dockerclient.WithHost("tcp://docker"),              // logical target
    dockerclient.WithDialContext(cfg.Net.DialContext),  // core decides how to reach it
)
```

- **L7 — fat HTTP clients that can't take a dialer** (Kubernetes `client-go`,
  private REST APIs): a bare `DialContext` is not enough — `client-go` wants a
  `rest.Config` (base URL + RoundTripper + auth). Use `cfg.Net.HTTP()`; in agent
  mode the agent runs an L7 reverse-proxy that injects the target's credentials
  (§8.3). This is precisely why "just use the dialer everywhere" is false.

### 8.3 The agent

`shellcn-agent` is a single small, static Go binary (also shipped as the
`ghcr.io/charlesng35/shellcn-agent` container image). It is **plugin-agnostic**: it proxies one
declared target — an L4 forward (`tcp`/`unix`) or, for API servers, an L7
reverse-proxy that injects credentials. At enrollment the gateway tells it which
mode + endpoint to expose, then it opens one outbound, multiplexed (yamux/HTTP2),
mutually-authenticated tunnel back to the gateway. One binary, multiple modes —
not a dumb byte pipe, and not per-plugin logic:

```go
type AgentMode string
const (
    AgentTCP  AgentMode = "tcp"   // forward a TCP address  — v1
    AgentUnix AgentMode = "unix"  // forward a unix socket  — v1
    AgentHTTP AgentMode = "http_proxy" // generic L7 proxy for private HTTP APIs
)

type ProxyTarget struct {
    Mode    AgentMode
    Address string    // tcp: "127.0.0.1:5432"; unix: "/var/run/docker.sock"; http_proxy: upstream base URL
    Risk    RiskLevel // e.g. docker.sock ⇒ privileged
    // http_proxy credential injection (generic; no protocol vocabulary). The
    // agent injects a bearer token read from TokenFile (re-read for rotation)
    // and verifies the upstream's TLS with CAFile. Empty = none / system roots.
    TokenFile string // target-side path to a bearer token file
    CAFile    string // target-side path to a PEM CA bundle
    Forward   bool   // let the gateway name each stream's dial target (see below)
}

type AgentProfile struct {
    Proxy   ProxyTarget
    Install []InstallArtifact // how the user launches the agent (§8.4)
}
```

**Per-stream forwarding (`ProxyTarget.Forward`).** Normally the agent proxies every
stream to its one declared `Address`. Some plugins need to reach _more_ of the
target's own network than a single endpoint — e.g. opening a Docker container's web
port (the §8.1 browser proxy): the container's IP is reachable from the daemon host
where the agent runs, but it isn't the daemon socket. With `Forward`, the gateway
prefixes each L4 stream with a tiny target preamble (`network` + `address`) and the
agent dials _that_ instead of `Address`. It stays plugin-agnostic — the agent gains
no protocol knowledge, just "dial what the gateway names." It is **negotiated and
opt-in**: the agent advertises support in its hello and the gateway enables it only
when the plugin set `Forward`, so older agents keep the single-endpoint behavior.
Reach widens only within the same target the agent already fronts (a docker.sock
agent can already run any container), so it adds no new trust boundary. A Docker
agent must run with **host networking** to route to container IPs across networks.

**Upgrades/hijacks over the agent use a loopback bridge.** Connection _upgrades_ —
client-go's SPDY/WebSocket executor (k8s exec, attach, port-forward) and moby's
HTTP **hijack** (Docker/Podman exec) — bypass a custom `DialContext` and require a
real socket. So for agent transport the tunnel is fronted by a tiny **loopback
bridge** (`127.0.0.1`, session-lived): the client dials the bridge, the bridge
pipes each connection to a fresh tunnel stream. Plain request/response (logs,
listing) rides the tunnel directly; only the upgrade/hijack needs the bridge. This
is why exec over the agent works the same as a direct connection — and the bridge
is shared (`plugins/shared/loopback`) by both the k8s and Docker engines.

```go

type ArtifactDelivery string // "" (inline) | "url" | "file"

type InstallArtifact struct {
    Label    string // "Docker", "Kubernetes manifest", "Shell"
    Kind     string // "docker-run" | "docker-compose" | "k8s-manifest" | "shell"
    Delivery ArtifactDelivery // inline (default): Template → a copyable command with the token injected. url: Template is the fetch command ({{.ArtifactURL}}) and Content is served from a single-use signed URL with the token minted into the body. file: Content (token injected) renders directly in the panel as a copyable/downloadable file (e.g. a Compose YAML); Filename names it (§8.4).
    Template string // rendered with generic context + funcs: .ConnectURL, .GatewayConnectURL, .ArtifactURL, .Token, .Slug, .Image, .Insecure, .LocalhostHost, .LocalhostHostRequired, shellquote
    Content  string // url/file delivery: the artifact body, rendered with the same context
    Filename string // file delivery: suggested save name
    ConnectURL ArtifactConnectURL
}
type ArtifactConnectURL struct {
    LocalhostHost string // optional host replacement for localhost in containerized artifacts
}
```

### 8.4 Enrollment flow

All enrollment is **connection-scoped, authenticated API** — same shape and TLS as
the rest of the platform. The agent connect endpoint is global; the connection it
binds to is determined server-side by the enrollment token, never by the URL.

1. **Create enrollment** (authenticated):
   `POST /api/connections/{connectionID}/agent/enrollments` → returns
   `{ enrollmentId, expiresAt, artifacts: [...] }`. Each artifact is either an
   inline `command` (token injected as an **env var**, not a path) or a `url` to
   fetch (for `kubectl apply -f`), e.g.:
   ```jsonc
   { "kind": "docker-run",
    "command": "docker run --rm --name shellcn-agent --network host --group-add \"$(stat -c '%g' /var/run/docker.sock)\" -e SHELLCN_CONNECT_URL=wss://host/api/agent/connect -e SHELLCN_ENROLL_TOKEN=… -v /var/run/docker.sock:/var/run/docker.sock ghcr.io/charlesng35/shellcn-agent:latest" }
   { "kind": "k8s-manifest",
     "url": "https://host/api/connections/{connectionID}/agent/enrollments/{enrollmentId}/artifacts/k8s-manifest?ticket=…" }
   ```
2. **Fetch artifact** (for `kubectl`/`curl`, which have no browser session):
   `GET /api/connections/{connectionID}/agent/enrollments/{enrollmentId}/artifacts/{kind}`
   guarded by a short-lived, single-use **signed ticket** (same mechanism as §7.4).
   For `url`-delivery artifacts the real enrollment token is **minted into the
   body at fetch time** (the record is created with a non-redeemable placeholder
   hash), so the credential reaches exactly one target and never appears in a
   path/query. This is a generic `InstallArtifact` capability — any plugin may
   serve large or sensitive artifact bodies this way.
3. The UI shows the command/manifest in a `PanelEnroll` panel with a **copy**
   button and a live **"waiting for agent → online"** status.
4. The user runs it on the target. The agent dials the global
   `wss://host/api/agent/connect` and presents the enrollment token **in the
   handshake/first message** (never in the path or query). An unused token must
   be redeemed before its short install window expires; after first successful
   enrollment, the same installed-agent credential may reconnect until it is
   revoked or rotated. The gateway registers the active tunnel in memory.
5. The connection flips to online; `Connect` is now served via the agent dialer.

**Why not token-in-path:** request paths are routinely logged (access logs,
proxies, history), so the enrollment token is passed only as an env var, a
handshake message, or a single-use signed `?ticket=` on the artifact fetch —
never as a path segment. Env vars can still be visible through process/container
inspection, so unused install tokens stay short-lived, and enrolled-agent
credentials stay scoped to one connection + one `ProxyTarget`. Everything is
`https`/`wss`.

**Security:** unused install tokens are short-lived. After enrollment, the
installed-agent credential can reconnect without user action until it is revoked
or rotated, and is scoped to exactly one connection and one `ProxyTarget` (it
cannot be repurposed); the tunnel is TLS + mutually authenticated; enrollment
create, artifact fetch, and agent connect/disconnect/re-enroll are audited; the
agent runs least-privilege (Docker-socket access is `privileged` risk and
surfaced as such in the UI).

---

## 9. Security model

Design the interfaces now; ship simple implementations first. Every layer is a
core module (`internal/auth`, `internal/policy`, `internal/secrets`,
`internal/audit`), not middleware glued onto plugins.

### 9.1 Authentication

OIDC/OAuth2-ready (Authentik-friendly) **and** local accounts. Platform session
via a secure, `HttpOnly`, `SameSite` cookie + CSRF token for state-changing HTTP.
WS uses signed tickets (§7.4). Optional MFA/TOTP for local accounts. **v1 ships
local accounts; the OIDC interface is present from day one.**

### 9.2 Authorization & action risk

- **RBAC + per-connection ownership/sharing grants**, enforced on every route via
  its `Permission` + `Risk`. **v1 uses embedded Casbin**; OPA (policy-as-code) is
  a later, additive option.
- Built-in role defaults are seeded in code; additive role/permission/risk grants
  are persisted in the control-plane store and loaded into Casbin on startup.
- **Action risk model** (`safe` / `write` / `destructive` / `privileged`) lets
  policy express rules like: viewers may open terminals but not delete VMs;
  destructive DB statements require approval in production; Docker `exec` is
  forbidden on containers tagged `critical`.

### 9.3 Secrets at rest

`Config` is **not** stored plaintext. Fields with `Secret: true` are encrypted
with AES-256-GCM using a data key wrapped by a master key from env/file/KMS.
Secrets are **write-only** over the API (UI shows "set / not set", never the
value) and redacted from logs and audit. **v1 ships a local encrypted vault**
behind a `SecretStore` interface; OpenBao is a later drop-in.

Reusable credentials are first-class records, not copied blobs. A credential has
an owner, grants, a stable ID, a kind (`ssh_private_key`, `ssh_password`,
`kubeconfig`, `tls_client_cert`, `db_password`, `api_token`, ...), non-secret
metadata (display name, optional identity/principal metadata, registry-derived
compatible protocols), and encrypted secret material in the vault. A
connection may reference a credential by ID instead of storing its own secret;
users with `use` access can connect through it but can never read the secret
value. Rotation updates the credential once and affects every connection that
references it. Audit logs record credential ID/name/kind usage, never secret
material.

### 9.4 Egress / SSRF

A plugin may only dial host(s) from its validated connection config. Targets
(including any "jump"/"proxy URL" fields) are checked against an egress
allow/deny policy.

### 9.5 Audit & session recording

Append-only audit log keyed by route `AuditEvent`: who, what, when, which
connection, params (secrets redacted), result, risk. Terminal and (future) RDP
sessions support optional recording (asciinema-style cast for terminals).

Recording is **plugin-declared and off by default**. The core never starts
recording merely because a panel is `terminal` or `remote_desktop`; the plugin
projection must declare recording support and the connection policy must enable
it. During connection create/edit, the UI shows "auto record" options only for
plugins that support recording.

Two recording classes are first-class:

- **Terminal/event recording:** terminal-like streams (SSH, Docker exec,
  Kubernetes exec, telnet, serial) use asciicast v2 where possible. Output and
  resize events are captured by the core wrapper; input events are sensitive and
  disabled unless explicitly enabled.
- **Desktop/graphical recording:** VNC/RFB and RDP share a platform recording
  contract. M1.6 supports browser canvas capture
  (`webm_canvas`) only; it is useful operationally but not compliance-grade.
  Plugins declare capability only and do not implement their own recording
  providers.

Recordings are private to their creator: every user — admin included — may
list/view/delete only recordings they created. Sharing grants and connection
ownership never expose another person's recordings, and admin (a user-management
role) gets no exception. Recording read/delete operations are audited separately
from the original stream route.

### 9.6 Per-protocol safety (non-negotiable defaults)

- **SSH/SFTP:** password, private-key, and stored-credential authentication.
  The first implementation keeps additional trust-management fields out of the
  connection form to avoid a half-built approval workflow.
- **Docker:** socket access is root-equivalent — Docker connections are
  `privileged` risk by default.
- **Databases:** read-only mode toggle, query timeout, row limit, dangerous-
  statement detection (`UPDATE/DELETE/TRUNCATE/DROP`), confirmation/approval
  hook, every query audited with redacted statement metadata, and configurable
  result redaction by column pattern.

---

## 10. Observability

The gateway itself must be observable. Behind interfaces from day one:

- `log/slog` structured logs: JSON for production/file sinks; colorized console
  formatting only for interactive local terminal output.
- Prometheus metrics: sessions/channels open, action latency, WS connections,
  failed authorizations, secret-access counts.
- OpenTelemetry traces (optional, additive).
- `/healthz` for gateway/store readiness; per-connection health is session-owned.

---

## 11. Technology stack (committed)

**Backend**

- Go 1.26+
- `chi` — router (net/http compatible; clean per-group middleware at the plugin
  mount point). ConnectRPC for typed core control APIs is an _optional_ later
  consideration, not a v1 commitment.
- `coder/websocket` (or `gorilla/websocket`) — WebSocket
- `gorm` (gorm.io) — the cross-database ORM for the platform store (§11.1),
  wrapped behind repository interfaces; schema kept in sync via `AutoMigrate`
- pure-Go SQL drivers (no CGO ⇒ single-binary holds): `glebarez/sqlite` (GORM's
  pure-Go SQLite driver, backed by `modernc.org/sqlite`; default, embedded —
  `.gitignore` already targets `*.db`), `gorm.io/driver/postgres` (backed by
  `jackc/pgx`), `gorm.io/driver/mysql` (backed by `go-sql-driver/mysql`).
  **Not** the default `gorm.io/driver/sqlite`, which is cgo-based. `pgx` also
  serves the PostgreSQL plugin (LISTEN/NOTIFY, COPY).
- `casbin/casbin` — embedded RBAC/ABAC (v1 authorization)
- `embed` — embeds the built frontend; `log/slog` — logging

**Frontend** (decision committed — the transcript left this open)

- Vue 3 (Composition API) + Vite + TypeScript
- Pinia (state) + Vue Router + VueUse (`useWebSocket` with reconnect, etc.)
- PrimeVue in **unstyled / pass-through** mode + Tailwind (DataTable with virtual
  scroll, Tree, Tabs, Splitter — exactly the data-heavy panels needed)
- xterm.js (terminal), noVNC (remote desktop), CodeMirror (code/SQL/YAML)

**Build**: `vite build` → `web/dist` → embedded via `web/embed.go` → `go build`
produces one binary. (The embed path is `web/dist`, matching the existing
`web/embed.go`.)

### 11.1 Persistence & data access (cross-database)

Scope: this is the **platform's own control-plane store** (users, roles, grants,
connections, reusable credentials + encrypted secrets, audit log, snippets,
session/agent metadata, preferences). It is **not** the database _plugins_
(`postgresql`, `mysql`, …) manage _remote_ databases and are unrelated to how ShellCN persists its own state).

- **Cross-database, single-binary default.** SQLite is the zero-config default
  (embedded, covers most self-host deployments). Postgres and MySQL/MariaDB are
  opt-in for larger or shared deployments. All three use **pure-Go drivers**, so
  enabling cross-DB never breaks the single binary. (An external DB is also the
  prerequisite for future multi-instance HA — but HA additionally requires
  solving the in-memory session blocker, which is a v1 non-goal, §2.)
- **Data access: `gorm` (GORM).** A developer-friendly, full-featured ORM with
  **no code-generation step** — the schema is kept in sync at runtime via
  `AutoMigrate`. One model set emits dialect-correct SQL for SQLite/Postgres/MySQL,
  so there is no per-engine query duplication.
  - **One struct set: `internal/models` IS the model layer.** The core entity
    structs in `internal/models` carry the `gorm:"…"` tags directly and are used
    as the GORM models — there is **no** parallel row/DTO struct set + `toX/fromX`
    mapper layer (rejected as needless complexity). gorm struct _tags_ don't import
    gorm, so the "only `internal/store` imports the gorm package" invariant holds.
    Use `serializer:json` for slice/map columns; keep secret-ish columns
    (`User.PasswordHash`) `json:"-"` and have the store clear/omit them on read.
  - **Pure-Go drivers only (no CGO).** Use `glebarez/sqlite` (a pure-Go SQLite
    driver backed by `modernc.org/sqlite`) — **not** the default
    `gorm.io/driver/sqlite`, which is cgo-based and would break the single-binary
    promise. Postgres uses `gorm.io/driver/postgres` (pgx) and MySQL uses
    `gorm.io/driver/mysql` (go-sql-driver); both are pure Go.
  - _Escape hatch:_ for hot paths (e.g. audit inserts) or dynamic queries, drop
    to raw SQL via GORM's `db.Raw`/`db.Exec` — kept inside `internal/store`.
- **Repository pattern (the DX/structure win).** Nothing outside `internal/store`
  imports `gorm`. The app depends on small interfaces — `UserStore`,
  `ConnectionStore`, `CredentialStore`, `AuditStore`, `GrantStore`, … — so
  the engine is swappable, the ORM never leaks, and tests use in-memory fakes (no DB needed for unit tests).
- **V1 migrations are automatic via `AutoMigrate`.** On startup the server opens
  the configured store and auto-creates missing tables/indexes plus additive
  schema changes. Destructive changes (drop table/column, rewrite data,
  delete/truncate audit rows) are never automatic; those require an explicit
  reviewed migration. This keeps early development fast while the `internal/store`
  boundary preserves the option to adopt versioned migrations later.
- **Secrets stay encrypted above the store.** Encryption happens in the service
  layer (§9.3); the store only ever persists opaque ciphertext and credential references,
  independent of the engine.

### Caveats

- **RDP is pure-Go** via the GPL-licensed `grdp` client: the gateway authenticates
  and decodes the session server-side and bridges it to noVNC as a synthetic RFB
  stream. (Adopting `grdp` makes ShellCN GPL-3.0.) The `remote_desktop` panel stays
  generic; plugins declare stream/config capabilities, not browser engines.
- **SPICE** has no production-grade browser client comparable to noVNC. It is
  out of scope until a maintained browser engine exists.

---

## 12. Frontend genericity (why adding a plugin needs zero frontend work)

The frontend is a fixed renderer driven entirely by the browser projection (§5.2):

1. On open, it fetches `GET /api/plugins/{name}` → layout, tabs/tree, columns,
   actions, streams, panel configs, schema field metadata, `credential_ref`
   selectors, and `DataSource` bindings.
2. It renders the connection workspace from `Layout` + `Tabs`/`Tree`.
3. Each panel is one of the ~10 `PanelType` components; it loads/streams from the
   resolved `DataSource` (RouteID → URL).
4. Clicking a resource opens its `DetailView` tabs; actions render from
   `ActionIDs` with `risk`/`requiresConfirm` styling.

5. For an **agent-mode** connection that has no live tunnel yet, the workspace
   renders a `PanelEnroll` panel (install command + live "waiting → online"
   status) instead of the normal tabs/tree, then swaps to them once online (§8.4).

**Long-lived runtime (terminals, VNC, log streams) lives in a Pinia session/
channel store, never inside components.** Components attach/detach; switching
tabs or rearranging panes never drops a stream.

**Renderer state is connection-bounded.** URL locators, open workbench views,
scope values, tree expansion, selected rows, and stream/channel instances are
owned by a connection id. A renderer may reuse the same generic component for
every plugin, but it must key any state that can survive navigation by
`connectionID` plus the view/resource identity. This prevents two connections
with overlapping resource kinds or ids from restoring each other's detail views,
expanded nodes, dock tabs, or live streams.

**Build the frontend first, fixture-driven — for the declarative surface.**
Because the renderer is the load-bearing bet ("handles any plugin"), build it
against **static fixture manifests + mock data** before any real plugin or core
exists. Fixtures genuinely prove the **declarative** panels — form, table, tree,
detail/tabs, action rendering, enroll — against SSH/Docker/Proxmox/Postgres
fixture shapes. They do **not** prove the **streaming** panels (terminal, logs,
remote desktop, query results): a mock WebSocket can't exercise xterm
resize/backpressure, the noVNC RFB handshake, or stream latency, so a green
fixture demo gives false confidence there. Stub those in M0 and validate each
with its first real plugin (terminal@M2, logs@M3, VNC@M4, query@M5). Build panels
in fixture-demand order, not speculatively.

### 12.1 Lazy loading & performance

The platform will accumulate a lot of weight (many plugins, heavy panel libs,
large resource lists), so **lazy-load aggressively** — load work only when a user
actually reaches it:

- **Panels are code-split.** Only a small core is bundled up front (shell + the
  lightweight declarative panels: form, table, tree, detail, enroll). Heavy panels
  — `code_editor` (CodeMirror), `remote_desktop` (noVNC), `metrics`/`graph`/`trace`
  (visualization libs),
  `terminal` (xterm), and request/editor-style specialized panels — are
  **dynamically imported on first use**.
- **Plugin projections are fetched on demand**, not all at startup; the
  connection list needs only id/title/icon.
- **Data is lazy by default:** tree children load on expand, tables paginate
  (cursor), watches stream deltas — never bulk-load (§7.2, §7.3).
- **Sessions/channels connect lazily** on first use and idle-timeout out (§8.1).
- **The platform's own modules** follow the same rule: route-level code-splitting,
  lazy-mounted admin/settings areas. Only the essentials are built-in initially;
  everything else is loaded when first needed.

Net: first paint stays small and constant no matter how many plugins exist; cost
scales with what the user opens, not with the size of the catalog.

### 12.2 Platform management (control-plane CRUD + administration)

The manifest-driven renderer (§12) covers everything _inside_ a connection. The
platform's own management surfaces — sign-in, creating/editing connections and
reusable credentials, sharing, and administration — are **core UI** backed by
**control-plane CRUD APIs**, not plugin-rendered. Two rules keep them consistent
with the architecture:

1. **The connection config form is manifest-driven too.** "Add connection" =
   choose a protocol → fetch its projection → render its `config` `Schema` with
   the **same generic `SchemaForm`** the action/forms use → submit. No per-plugin
   management code; a new plugin gets a create/edit form for free.
2. **Secrets stay write-only (§9.3).** Edit forms show `set` / `not set` for
   `Secret` fields and offer "replace", never the value. The same holds for a
   credential's secret material.

**Control-plane APIs (platform, not plugin routes).** These sit beside the
existing read endpoints and carry the same authn → authz → audit guarantees:

- **Connections:** `GET /api/connections/{id}` for edit/detail, plus
  `POST /api/connections`, `PUT /api/connections/{id}`, and
  `DELETE /api/connections/{id}`. The body's non-secret fields and inline
  `Secret` values are validated against the plugin's `config` schema; secrets are
  encrypted in the service layer before the store. Transport (`direct`/`agent`)
  is chosen here; an `agent` connection shows `PanelEnroll` until its tunnel is
  online (§8.4).
- **Credentials:** `POST /api/credentials`, `PUT /api/credentials/{id}` (rotate),
  `DELETE /api/credentials/{id}`. Secret material is write-only; rotation updates
  once and every referencing connection picks up the new value (§9.3). Deleting a
  credential that is still referenced is blocked in M1.5.
- **Sharing:** `POST/DELETE /api/connections/{id}/grants` and the credential
  equivalent — an owner grants `use`/`manage` to another user (§9.2). Connection
  `use` permits opening/using; connection `manage` permits edit/share/delete;
  credential `use` permits direct selection/replacement and direct connect-time
  resolution only. A shared connection can still connect with its already-bound
  credential without giving the grantee direct credential visibility. Subject lookup is a
  minimal `GET /api/users?query=` endpoint for grant assignment; full user
  management remains M-Admin.

**Auth UI.** A login gate (local accounts; OIDC behind the same interface)
establishes the session cookie + CSRF token (§9.1); the SPA bootstraps from
`GET /api/auth/me`, redirects to login on `401`, attaches the CSRF token to
state-changing requests, and offers logout. A single client interceptor turns
`401`→login, `403`→forbidden toast, and validation / CSRF / agent-unavailable
errors into consistent, actionable feedback.

**Administration (M-Admin — later, additive).** Once the usable core lands, admin
surfaces follow the same data-driven approach and need their own control-plane
endpoints: user/role management + role assignment; the additive stored policy
rules (role + permission + risk, §9.2); broader audit-log views with filters
(user / connection / route / risk / result); and a **light** status page
(gateway health, live session/channel counts) — deep metrics stay in
Prometheus/Grafana, not reinvented here.

**Milestone split.** Tier 1 (**M1.5**) is the "make it usable" core: auth gate +
global error UX, connection CRUD, credential CRUD, sharing — plus the backend
CRUD endpoints they require. Tier 2 (**M-Admin**) is the "operate it" surfaces
above. Several Phase-2 backend behaviors need **no** UI (wrapper schema
validation, denied-route audit, stored-policy loading, secret-access metrics) —
they are verified by tests, not screens.

---

## 13. Worked plugin examples (this model)

**SSH** — flat; all capabilities at connection level; one shared session:

```
Layout = LayoutTabs
Tabs   = [Terminal, Files, Snippets]               // all connection-level
Session = sshSession{ client, mu, sftp }           // SFTP reuses the TCP conn
Routes  = ssh.shell(WS,privileged), ssh.sftp.*(safe/write), ssh.snippet.*(safe/write/privileged)
Actions = ssh.snippet.create, ssh.snippet.run, ssh.snippet.delete
ssh.snippet.run.OnSuccess = { SelectTab: "terminal" }
```

> **SSH vs. standalone `sftp` — a manifest difference, not a frontend one.**
> An `ssh` connection exposes SFTP as its **Files** tab over the _same_
> `ssh.Client` (no second connection, no re-auth). A standalone `sftp` connection
> is a **file-only** plugin for users who don't want a shell: its manifest
> declares just the `file_browser` tab + `sftp.*` routes. Both render the same
> `file_browser` panel. The frontend **special-cases neither** — it renders
> whatever tabs the manifest declares (§12). The two plugins simply ship different
> manifests over (largely) the same SFTP route handlers.

**Docker** — hierarchical; terminal/logs are **resource-level** (no nonsensical
connection-level Terminal tab):

```
Layout    = LayoutSidebarTree
Tree      = [Containers, Images, Volumes, Networks]   // lazy
Resources = container{ List:docker.container.list, Watch:docker.container.watch,
              ActionIDs:[docker.container.start/stop/restart/remove],
              Detail.Tabs:[Overview(metrics), Terminal(stream), Logs(stream),
                           Inspect(code_editor), Env(table)] }
Transport = [direct, agent]   // agent proxies /var/run/docker.sock (unix, privileged)
Note: docker.container.exec is privileged risk by default; session uses cfg.Net.DialContext
      so direct and agent transport share one code path (§8.2).
```

**Proxmox** — deep hierarchy; consoles/snapshots live in the VM DetailView:

```
Tree      = [Nodes, Storage, Network, Datacenter]
vm.Detail = [Overview(metrics), Console(remote_desktop), Hardware(form),
             Snapshots(table), Backups(table)]
```

**Kubernetes** — resource catalog rendered by generic panels; raw manifests are
available, but operational overviews are structured property sheets:

```
Layout    = LayoutSidebarTree
Tree      = [Overview, Workloads, Config, Network, Storage, Access Control, Custom Resources]
Resources = pod{ List:kubernetes.resource.list, Watch:kubernetes.resource.watch,
              Detail.Tabs:[Overview(object_detail), YAML(code_editor),
                           Metrics(metrics), Logs(log_stream), Shell(terminal),
                           Events(timeline)] }
Workloads = deployment/statefulset/daemonset/replicaset{
              Detail.Tabs:[Overview(object_detail), YAML(code_editor),
                           Pods(table), Events(timeline)] }
Metrics  = cluster/node/pod metrics stream from metrics.k8s.io when available;
           frames degrade gracefully and still show declared requests/limits
           where those values come from the resource spec.
```

**PostgreSQL** — schema browser as tree, query editor + results as panels:

```
Tree         = [database → Tables/Views/Functions]   (lazy)
table.Detail = [Data(query_editor), Schema(table), Indexes(table)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard.
Transport   = [direct]   // database TCP from the gateway; no agent mode in M5
```

**MySQL/MariaDB** — database browser with reusable SQL plugin primitives:

```
Tree         = [Databases, Tables, Views, Routines, Users]   (lazy)
table.Detail = [Data(table), Columns(table), Indexes(table), Constraints(table), SQL(query_editor)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard,
        confirmation for write/DDL/privileged statements, column redaction.
Transport   = [direct]   // database TCP from the gateway; no agent mode
```

**Microsoft SQL Server** — T-SQL database browser with SQL Server catalog surfaces:

```
Tree         = [Databases, Schemas, Tables, Views, Procedures, Users, Jobs]   (lazy)
table.Detail = [Data(table), Columns(table), Indexes(table), Constraints(table), SQL(query_editor)]
view.Detail  = [Data(table), Definition(document), SQL(query_editor)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard,
        confirmation for write/DDL/EXEC/privileged statements, column redaction.
Transport   = [direct]   // SQL Server TCP from the gateway; no agent mode
```

**Oracle Database** — Oracle SQL/PLSQL browser with Oracle catalog surfaces:

```
Tree         = [Schemas, Tables, Views, Procedures, Packages, Sequences,
                Users, Tablespaces, Sessions]   (lazy)
table.Detail = [Data(table), Columns(table), Indexes(table), Constraints(table), SQL(query_editor)]
package.Detail = [Spec(document), Body(document)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard,
        confirmation for write/DDL/PLSQL/privileged statements, column redaction.
Catalog: uses DBA_* views when available and falls back to ALL_/USER_* views
         where normal application users lack catalog privileges.
Transport   = [direct]   // Oracle Net from the gateway via pure-Go go-ora; no agent mode
```

**CockroachDB** — distributed SQL browser with CockroachDB cluster surfaces:

```
Tree         = [Databases, Nodes, Ranges, Jobs, Sessions, Queries, Schemas,
                Tables, Views, Functions]   (lazy)
table.Detail = [Data(table), Columns(table), Indexes(table), Constraints(table), SQL(query_editor)]
view.Detail  = [Data(table), Definition(document), SQL(query_editor)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard,
        confirmation for write/DDL/IMPORT/BACKUP/RESTORE/privileged statements,
        column redaction.
Catalog: uses CockroachDB-supported SHOW statements and information_schema for
         user-visible catalog data; restricted internal cluster tables are not
         required for the default UX.
Transport   = [direct]   // CockroachDB SQL over the PostgreSQL wire protocol; no agent mode
```

**ClickHouse** — analytics database browser with ClickHouse system surfaces:

```
Tree         = [Databases, Tables, Views, Dictionaries, Mutations, Merges,
                Processes, Users]   (lazy)
table.Detail = [Data(table), Columns(table), Indexes(table), Constraints(table),
                Mutations(table), Definition(document), SQL(query_editor)]
view.Detail  = [Data(table), Definition(document), SQL(query_editor)]
Safety: read-only toggle, query timeout, row limit, destructive-stmt guard,
        confirmation for INSERT/ALTER/DELETE/DDL/TRUNCATE/OPTIMIZE/SYSTEM/KILL
        and privileged statements, column redaction.
Catalog: uses ClickHouse native protocol through `clickhouse-go/v2` and
         `system.*` tables for databases, tables, dictionaries, mutations,
         merges, processes, users, definitions, and completion metadata.
Transport   = [direct]   // ClickHouse native TCP from the gateway; no agent mode
```

**Redis** — flat data-store cockpit backed by generic panels:

```
Tabs      = [Overview(document), Keys(kv), Console(terminal), Clients(table),
             Channels(table), Info(document)]
KV        = SCAN-backed key browser; typed read/write for string/hash/list/set/zset;
            stream read preview; create/delete through the generic kv contract.
Console   = Redis REPL over the terminal panel with read-only and
            write-confirmation safety gates.
Transport = [direct]   // Redis TCP from the gateway; no agent mode
```

**MongoDB** — document database browser backed by generic panels:

```
Tree              = [Databases, Collections]   (lazy)
collection.Detail = [Documents(table), Indexes(table), Stats(document),
                     Console(query_editor)]
document.Detail   = [Document(document), Editor(code_editor)]
Safety: read-only toggle, document limit, Extended JSON parsing, write-command
        confirmation, and command audit metadata.
Transport         = [direct]   // MongoDB TCP from the gateway; no agent mode
```

SQL plugins share only driver-neutral primitives in `plugins/shared/sqldb`:
query editor request/result envelopes, identifier and qualified-name helpers,
basic DDL column validation/building, statement splitting/classification,
duration/bool config parsing, TLS client/CA config assembly, query audit
metadata, and result redaction helpers. Driver, catalog, dialect, and manifest
details stay inside each concrete SQL plugin.

---

## 14. Repository layout & code structure (DX)

DX is a first-class goal: a contributor should add a protocol by writing one Go
package, and should be able to read, test, and run any layer in isolation.

```
cmd/
  server/      entrypoint; wires dependencies, calls plugins.Register(reg)
  agent/       shellcn-agent: the plugin-agnostic reverse-tunnel proxy (§8.3)
internal/
  models/      core entity types (no deps): User, Connection, Credential, Grant, AuditEntry… These structs ARE the GORM models (gorm tags live on them); no separate row/DTO + mapper layer.
  store/       GORM-backed repositories behind UserStore/ConnectionStore/CredentialStore/AuditStore… — the ONLY package that imports the gorm package (§11.1)
  service/     business logic: orchestrates store + plugins + policy + secrets + audit
  server/      HTTP/WS adapters: chi router, route mounting, projection, embed serving
  plugin/      Plugin/Manifest/Route/Schema/RequestContext/Session types + registry + projection + plugintest harness
  session/     in-memory session + channel registry + lifecycle
  transport/   direct + agent dialers (NetTransport), tunnel registry, enrollment
  auth/        OIDC interface + local accounts, WS tickets
  policy/      Casbin RBAC/ABAC, action-risk enforcement
  secrets/     SecretStore interface + local AES-GCM vault
  audit/       append-only audit + session recording
  telemetry/   slog, metrics, health
  config/      typed config from env/file/flags
plugins/
  registry.go  the ONE place first-party plugins are wired: plugins.Register(reg) iterates all()
  ssh/ sftp/ docker/ proxmox/ postgresql/ kubernetes/ …   (each: plugin.go, routes.go, session.go, manifest.go, *_test.go)
  shared/      reusable protocol-family helpers only; no plugin manifests or frontend assumptions
web/
  src/         Vue app; vite build → web/dist (embedded by web/embed.go)
  fixtures/    static manifests + mock data/streams for fixture-first UI dev (§12)
```

### 14.1 Conventions

- **Layered, dependencies point inward:** `models` ← `store`/`service` ←
  `server`/transport. `models` imports no internal packages (only stdlib + gorm
  tags); the HTTP layer is a thin adapter.
- **Explicit dependency injection:** constructors (`New(...)`) wired once in
  `cmd/server/main.go`. No globals, no service locator, no `init()` magic. The
  first-party plugin set is wired in one place — `plugins/registry.go` — which
  `main.go` invokes via `plugins.Register(reg)`.
- **Interfaces at the consumer, kept small.** `context.Context` threaded through
  every call; cancellation honored by sessions/streams.
- **Errors:** wrap with `%w`, typed domain/sentinel errors, normalized to API
  responses **only** at the server boundary (handlers return `(any, error)`).
- **Secrets/PII never logged;** redaction enforced centrally (§9.3, §9.5).

### 14.2 Plugin-author DX (the contributor promise)

- A documented **plugin skeleton** + the `plugintest` harness: fake
  `RequestContext`, `Session`, and `NetTransport` so a plugin is unit-testable
  with **no real infrastructure**.
- The manifest is **validated at registration** with actionable errors (unknown
  RouteID, duplicate IDs, missing AgentProfile when `agent` is declared, §5).
- Adding a protocol = one plugin package + one line appended to `all()` in
  `plugins/registry.go`. **Zero** other core changes, **zero** frontend changes.

### 14.3 Tooling

- `golangci-lint` + `gofumpt` + `go vet`. No code generation — GORM uses plain
  structs + `AutoMigrate`.
- **Tests:** table-driven units with in-memory store fakes; a **cross-DB
  integration matrix** (SQLite + Postgres + MySQL via testcontainers) in CI;
  golden tests for the manifest projection so the FE/BE contract can't drift.
- `Makefile`: `build · test · lint · dev` (Go live-reload via
  `air`/`wgo`; Vite HMR for the frontend; the mock server for fixture-only FE work).
- Frontend: TS `strict`, ESLint + Prettier; the shared projection types (§5.2)
  are the single FE/BE contract.

---

## 15. MVP scope & build order

**UI-first (for the declarative surface).** The renderer is the load-bearing bet,
so its declarative core is built and proven _before_ the real core or any protocol
code (§12); streaming panels are validated with their first real plugin, not
mocks. Each milestone ships something runnable.

- **M0 — Declarative UI renderer (fixture-driven):** the Vue shell + manifest
  projection renderer + the panels static data fully exercises — **form, table,
  tree, detail/tabs, action rendering, enroll**. Driven by **static fixture
  manifests + mock data**, no real backend. Proves "renders any plugin" for the
  declarative surface against SSH/Docker/Proxmox/Postgres fixture shapes. Streaming
  panels (terminal, logs, remote desktop, query results) are **stubbed** here and
  validated later with their first real plugin — a mock WebSocket does not prove
  xterm/noVNC/backpressure. **This is the priority.**
- **M1 — Core runtime:** plugin registry + manifest validation + real browser
  projection; control-plane models/repositories for connections, grants,
  reusable credentials, encrypted secrets, audit, sessions, and policies;
  session/channel manager; transport (direct dialer + tunnel registry); chi route
  wrapper + WS tickets; GORM store (SQLite default, §11.1) + repository
  interfaces; local auth + Casbin; audit. Swap the mock server for the real one —
  the UI is unchanged.
- **M1.5 — Platform management:** auth gate + CSRF-aware SPA client, global
  error UX, connection create/edit/delete from manifest config schemas,
  reusable credential create/rotate/delete, and connection/credential sharing
  grants. This is core UI + control-plane CRUD, not plugin UI.
- **M1.6 — Session recording foundation:** plugin-declared recording capability,
  off-by-default connection recording policy, recording metadata/blob storage,
  core stream recording wrapper, terminal asciicast playback, desktop graphical
  recording framework, and role-aware recording list/playback APIs.
- **M2 — SSH/SFTP (reference plugin, direct transport):** validates the **real
  terminal + file-browser** panels (first real streaming), command snippets, plus
  auth, reusable credentials, inline secrets, channel lifecycle, and audit end to end.
- **M3 — Docker (validates agent transport, L4):** containers/images/volumes/
  networks tree; validates **real logs + exec + watch streams**, inspect,
  start/stop/restart; hardens the generic `shellcn-agent` (`tcp`/`unix` modes),
  enrollment, and tunnel registry against a real Docker socket. Proves
  sidebar-tree + resource detail + reverse connectivity + privileged-risk
  handling.
- **M4 — Proxmox:** nodes/VMs/LXC, start/stop/reboot, snapshots, storage;
  validates the **real noVNC remote-desktop** panel and deep hierarchy.
- **M5 — PostgreSQL:** schema browser tree; validates the **real query editor +
  results** panel, read-only mode, query audit, snippets.
- **M6 — Kubernetes (last):** workloads, pods, logs, exec, YAML editor, events;
  introduces the **L7 agent mode** (`http_proxy`, §8.3). Deliberately
  last — largest surface; benefits from a proven core + agent.

---

## 16. Open questions

- **Secrets backend:** local AES-GCM vault is v1; when do we add OpenBao?
- **Policy depth:** Casbin covers v1; do we need OPA policy-as-code, and when?
- **Connection import:** ingest `~/.ssh/config`, kubeconfig, Docker contexts?
- **Manifest/schema migration:** when a plugin's `Config` schema changes, how are
  stored connection configs migrated/validated? (Tie to `APIVersion`.)
- **Approval workflows:** how are "requires approval in production" actions queued
  and approved?
- **Out-of-tree plugins:** if ever needed, confirm gRPC subprocess (not `.so`).
- **Agent operations:** versioning/auto-update of `shellcn-agent`, agent↔core
  version skew, agent tunnel credential rotation, and multiple agents for one connection (HA).
- **UDP over agent:** `NetTransport.DialContext` dials UDP directly (SNMP, raw
  IPMI), but the agent tunnel is stream-oriented — do we add a UDP-over-agent mode
  for those protocols behind NAT, or require they be reached directly / via SSH?
- **Additional desktop renderers:** `remote_desktop` currently normalizes browser
  rendering to noVNC/RFB. vSphere **WebMKS** and SPICE remain out of scope until
  they have maintained browser clients and a real need to add a selector-backed
  renderer contract.
- **Specialized panels:** `graph` (Neo4j/topology), `trace` (Jaeger/Tempo),
  `kv` (Redis), and `http_client` (http-api/graphql/grpc) are core panels. Their
  first plugins should validate protocol-specific route payloads and UX details.
