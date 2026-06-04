# External (out-of-tree) plugins — implementation plan

**Status:** Proposed · **Owner:** core · **Depends on:** M1 core runtime (plugin
registry, projection, route wrapper, session/transport)
**Spec refs:** [project.md §2](project.md) (lines 57–62, out-of-tree → gRPC
subprocess), [project.md §5](project.md) (plugin contract), [project.md §8](project.md)
(sessions + transport)

---

## 1. The need

The 40 first-party protocol plugins stay **compiled in** and maintained in-tree.
This plan adds a **second, out-of-process backend** so a third-party developer can
write a _new_ protocol plugin, build it **separately** as its own binary, and load
it into a self-hosted ShellCN **without recompiling the core**.

Goals:

- Third-party plugins are **discovered and loaded at runtime** from a directory —
  no edit to `plugins/registry.go`, no core rebuild.
- A third-party plugin reuses the **same declarative contract** (Manifest, Routes,
  Session) so the universal renderer, audit, and policy work unchanged.
- **Full capability parity:** an external plugin can do **everything** a built-in
  plugin can — unary routes, server-streaming, interactive bidirectional
  terminals/exec, tracked upstream channels, both transports (direct **and**
  agent, L4 **and** L7), the "open in browser" HTTP proxy, and session recording.
  No feature is first-party-only.
- The **gateway stays the gatekeeper**: authn/authz/audit and the network egress
  (direct/agent) remain in the core, not in untrusted plugin code.

Non-goals (explicit): commercial/licensing gates, reducing core binary size,
moving any first-party plugin out-of-process. This is purely runtime
extensibility for code the operator chooses to install.

The spec already pre-committed the mechanism (§2): _"If out-of-tree third-party
plugins are ever needed, the path is gRPC subprocesses — never Go `.so`."_ This
plan is the realization of that line.

---

## 2. What we already have (and why this is mostly an adapter, not a rewrite)

The core never programs against a concrete plugin — only against the `Plugin`
**interface** plus **declarative data**. That is what makes an out-of-process
backend a drop-in.

| Asset                                                                                                  | Location                                       | Relevance                                                                                 |
| ------------------------------------------------------------------------------------------------------ | ---------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `Plugin` interface (`Manifest`/`Routes`/`Connect`)                                                     | `internal/plugin/session.go`                   | The seam an external plugin plugs into.                                                   |
| `Manifest` is pure data, already JSON-serialized                                                       | `internal/plugin/manifest.go`, `projection.go` | Crosses the process boundary for free (incl. `AgentProfile`, `Recording`).                |
| `Route` carries `Permission`/`Risk`/`AuditEvent`/`Input` as data; `Handle`/`Stream` are the only funcs | `internal/plugin/route.go`                     | Core enforces security from the data; only the func bodies move out-of-process.           |
| `Registry.Register(p Plugin)` validates + indexes any `Plugin`                                         | `internal/plugin/registry.go`                  | An external plugin registers through the **same** path; validation is identical.          |
| `BuildProjection` derives the browser view from Manifest + routes                                      | `internal/plugin/projection.go`                | Unchanged — it never sees handler funcs; already projects agent + recording.              |
| `NetTransport` (DialContext / HTTP RoundTripper) wired by core                                         | `internal/plugin/session.go`                   | Brokered back to the subprocess so egress (direct **or** agent) stays in the core.        |
| `Session` (`HealthCheck`/`OpenChannel`/`Close`), `Channel`, `ClientStream`, `HTTPProxy`                | `internal/plugin/session.go`                   | Session state lives in the subprocess; streams/channels/proxy bridge over brokered conns. |
| The route HTTP/WS wrapper (authn→authz→validate→audit→handler)                                         | core server adapter                            | Stays in the core; an external route is wrapped exactly like a built-in one.              |

**Consequence:** the runtime core change is a single host-side adapter type
(`grpcPlugin` implementing `plugin.Plugin`) plus a discovery/loader and the
brokered-conn plumbing. The registry, projection, route wrapper, policy, audit,
recording, and the entire frontend keep their **logic** unchanged.

**One up-front packaging refactor (Step 0):** the contract types a plugin author
needs currently live in `internal/plugin`, which Go forbids any external module
from importing. They must move to an importable **nested SDK module** (§3.8). The
40 built-ins and the core then import the contract from there. This is a one-time,
mostly-mechanical change (import rewrite across ~329 files) plus decoupling
`RequestContext` from `internal/models` — no behavior change.

---

## 3. Design

### 3.1 Two backends behind one interface

```
                    plugin.Registry  (unchanged)
                          ▲
         ┌────────────────┴─────────────────┐
   in-process Plugin                 grpcPlugin (NEW)
   (the 40, compiled in)             implements plugin.Plugin by
                                     proxying to a subprocess over gRPC
                                          │  hashicorp/go-plugin
                                          ▼
                                  external plugin binary
                                  (imports the ShellCN plugin SDK)
```

`grpcPlugin.Manifest()` returns the manifest the subprocess sent at load time.
`grpcPlugin.Routes()` returns real `plugin.Route` values whose `Handle`/`Stream`
are **shims** that call into the subprocess. `grpcPlugin.Connect()` calls the
subprocess and returns a `grpcSession` handle. The `Registry` validates this
exactly like a built-in plugin — a malformed external manifest is rejected at
load, before it can serve a request.

### 3.2 Transport: `hashicorp/go-plugin` (gRPC only) — "raw conns for bytes, gRPC for control"

Verified current capabilities: `AllowedProtocols: [ProtocolGRPC]`, `AutoMTLS`
(encrypted + mutually authenticated localhost channel), HTTP/2 multiplexing for
bidirectional streams, `GRPCBroker` for brokering additional `net.Conn`s,
magic-cookie handshake + `ProtocolVersion`, reattach. gRPC only (never net/rpc).

**Core design principle that makes full parity performant:** every _byte-stream_
capability (interactive terminal, exec, desktop, `OpenChannel`, file transfer,
the HTTP proxy, and the L4 target dial) is carried over a **raw brokered
`net.Conn`** — a local Unix-domain socket on Unix — **not** as per-frame protobuf
messages. gRPC unary/stream RPCs are reserved for request/response routes and
control (manifest, connect, route invocation, resize/close signals, audit). This
keeps a terminal or framebuffer at one local-socket memcpy of overhead instead of
marshaling every keystroke/frame.

### 3.3 Full capability parity

External plugins support the **entire** in-process contract. Nothing is
first-party-only.

| Capability                                        | Built-in | External | How it crosses the boundary                                                                                     |
| ------------------------------------------------- | -------- | -------- | --------------------------------------------------------------------------------------------------------------- |
| Manifest + projection                             | ✅       | ✅       | Manifest JSON at load (reuses projection schema)                                                                |
| Unary HTTP routes (`list`/`describe`/CRUD/query)  | ✅       | ✅       | `Invoke` RPC                                                                                                    |
| Server-streaming routes (logs, query results)     | ✅       | ✅       | `InvokeServerStream` RPC                                                                                        |
| Bidirectional WS routes (terminal, exec)          | ✅       | ✅       | raw brokered conn + control RPC (resize/close)                                                                  |
| Tracked upstream `OpenChannel`                    | ✅       | ✅       | raw brokered conn                                                                                               |
| `NetTransport` L4 (DialContext)                   | ✅       | ✅       | core-served `DialTarget` → brokered conn                                                                        |
| `NetTransport` L7 (HTTP RoundTripper)             | ✅       | ✅       | core-run per-session forward proxy, addr handed to plugin                                                       |
| Agent transport — L4 (tcp/unix)                   | ✅       | ✅       | `DialTarget` routes via the agent tunnel; `AgentProfile` is declarative                                         |
| Agent transport — L7 (http_proxy)                 | ✅       | ✅       | forward proxy routes via the agent's L7 reverse-proxy                                                           |
| `HTTPProxy` ("open in browser", incl. WS upgrade) | ✅       | ✅       | core hijacks the browser conn, bridges to a brokered conn the plugin serves                                     |
| Session recording                                 | ✅       | ✅       | core stays the byte-pump on every stream → records identically; authoritative recording rides a declared stream |

### 3.4 The wire contract (`.proto` sketch)

**Two** services. The core serves `Host` (egress + audit — keeps the gateway the
gatekeeper); the plugin serves `Plugin` (its actual behavior). The Manifest
crosses as the **same JSON** the projection already emits, so no schema is
duplicated.

```proto
syntax = "proto3";
package shellcn.plugin.v1;

// Served by the PLUGIN; called by the core.
service Plugin {
  rpc GetManifest(Empty) returns (ManifestJSON);              // JSON-encoded plugin.Manifest
  rpc Connect(ConnectRequest) returns (SessionHandle);        // opaque session id
  rpc HealthCheck(SessionHandle) returns (Empty);
  rpc Close(SessionHandle) returns (Empty);

  rpc Invoke(InvokeRequest) returns (InvokeResponse);         // unary HTTP route
  rpc InvokeServerStream(InvokeRequest) returns (stream Frame); // logs/results
  rpc OpenStream(StreamStart) returns (stream Control);       // bidi WS route: control plane (resize/close);
                                                              //   the data plane is a raw brokered conn (broker_id)
  rpc OpenChannel(ChannelRequest) returns (BrokerRef);        // tracked upstream Channel → raw brokered conn
  rpc ServeHTTPProxy(ProxyStart) returns (BrokerRef);         // plugin serves HTTP/WS over a raw brokered conn
}

// Served by the CORE; called by the plugin SDK. This is where egress + audit live.
service Host {
  rpc DialTarget(DialRequest) returns (BrokerRef);            // L4 egress through core: direct OR agent tunnel
  rpc HTTPProxyEndpoint(SessionHandle) returns (ProxyAddr);   // L7 egress: addr of core forward proxy (direct OR agent http_proxy)
  rpc Audit(AuditRecord) returns (Empty);                     // stream-internal audit hook
}

message InvokeRequest {
  string session_id = 1;
  string route_id   = 2;     // stable handle; core resolves perms/audit, NOT the plugin
  map<string,string> params = 3;
  map<string,string> query  = 4;
  bytes  body = 5;           // already validated against Route.Input by the core
  ActingUser user = 6;       // context only; authz already done in core
}

message BrokerRef { uint32 broker_id = 1; }  // GRPCBroker stream id for a raw net.Conn
```

**Key invariant:** `route_id` is a handle. Before any `Invoke`/`OpenStream`/etc.,
the core looks up the route's `Permission`/`Risk`/`AuditEvent`/`Input` from the
manifest it loaded and runs the **same wrapper** as a built-in route. The plugin
cannot widen its own permissions — it ships data and handler bodies only.

### 3.5 How each capability maps (decisions)

**Session lifecycle.** `Connect` returns an opaque `session_id`; the plugin SDK
holds a session registry keyed by it; every call carries the id. The core's
existing session registry stores the `grpcSession` handle and drives lifecycle
(idle timeout, health, channel pinning) exactly as today.

**Egress stays in the core (direct AND agent).** The subprocess never dials a
target itself. For L4 it calls `Host.DialTarget`; the core dials via the
connection's `NetTransport` — which is wired for **direct or agent** by the core —
and brokers back a `net.Conn`. For L7 it calls `Host.HTTPProxyEndpoint` and points
a stock `http.Client` at a **core-run per-session forward proxy** that applies the
RoundTripper (auth injection, TLS, or the agent's http_proxy reverse-proxy). So
**agent transport is automatic**: the plugin declares `AgentProfile` (data) and
dials through the broker; whether the core's transport is direct or an agent
tunnel is invisible to the plugin. Enrollment/tunnel/agent binary are all
core-owned and unchanged; the projected `AgentProfile` drives the existing
enrollment panel with no frontend change.

**Bidirectional streams (terminal/exec).** `OpenStream` returns a control stream
(resize, close, exit-status); the **data plane is a raw brokered `net.Conn`**. The
SDK reconstructs a `plugin.ClientStream` over that conn so the plugin's existing
`StreamHandler` runs nearly unchanged. The core remains the bridge between the
browser WS and the brokered conn.

**Tracked channels.** `OpenChannel` brokers a raw conn the SDK wraps as a
`plugin.Channel`; the core pins the session while it is open, identical to today.

**HTTPProxy / open-in-browser.** The core authenticates + authorizes, strips the
route prefix, **hijacks the browser connection**, and bridges it to a raw brokered
conn that the plugin's `ServeHTTPProxy` serves. Because it is raw bytes, redirects,
assets, and **WebSocket upgrades** pass through unchanged.

**Recording.** The core is the byte-pump on every stream (browser WS ⇄ brokered
conn), so it records any declared stream class exactly as for built-ins. For
`Authoritative` recording (e.g. asciinema frames the plugin emits), the canonical
frames ride a declared server-stream the core records.

### 3.6 Trust model

An external plugin is operator-installed code that receives decrypted credentials
for the connections it serves. Out-of-process gives **isolation** (separate
address space; can't read other plugins' memory; killable; sandboxable) but not
**safety** — like a Terraform provider or VS Code extension, the operator is
trusting it. Mitigations: `AutoMTLS` on the channel, optional binary signature
verification at load, the plugin's declared permission/risk surface shown to the
admin before enable, and an explicit **per-plugin acknowledgement before agent
transport is allowed** (it opens a tunnel into the operator's network). These are
trust _controls_, not capability cuts — every feature remains available.

### 3.7 Build & distribution (cross-compilation)

A plugin is a **normal compiled Go executable**, so it is **OS- and arch-specific**
— exactly like the ShellCN core binary itself. There is no single universal plugin
file (only WebAssembly offers that, ruled out because a Wasm sandbox cannot open
the raw sockets an infra plugin needs).

| Must match the host's…                    | Go `.so` (rejected) | gRPC subprocess (this plan) |
| ----------------------------------------- | ------------------- | --------------------------- |
| OS + CPU arch                             | yes                 | yes                         |
| Exact Go compiler version                 | yes (brittle)       | no                          |
| Exact build flags / CGO                   | yes (brittle)       | no                          |
| Loads into core memory (crash kills core) | yes                 | no (isolated)               |
| Wire/protocol version                     | n/a                 | yes (stable, versioned)     |

A subprocess only needs the same **OS/arch** and **protocol version** — not the
same Go toolchain — and cannot corrupt core memory. Guidance for authors:

- Ship a **build matrix** (`linux/amd64`, `linux/arm64`, `darwin/arm64`,
  `windows/amd64`) via a CI release (GoReleaser-style); Go cross-compilation makes
  each target a one-liner.
- **Prefer pure-Go drivers** (aligns with ShellCN's stack). Cross-compiling is
  trivial only without CGO.
- The operator downloads the build matching their server and drops it in
  `plugins.d/`. A wrong-arch binary **fails the handshake cleanly at load** with a
  clear error — it never half-works.

### 3.8 Module layout (nested SDK module)

Go forbids external modules from importing anything under `internal/`, so the
shared contract cannot stay in `internal/plugin`. It moves to a **nested Go module**
in this same repo, `github.com/charlesng35/shellcn/sdk`, with its own `go.mod` and a
**minimal dependency surface** (grpc, go-plugin, the contract types only). A
third-party plugin imports just `…/sdk` — not the core's heavy dependency tree —
and the SDK is versioned independently (`sdk/vX.Y.Z`) so the wire/ABI version
travels with it.

The **entire** `internal/plugin` package moved to `sdk/plugin` (it had a single
`internal/*` coupling — `RequestContext` → `internal/models` — now decoupled).
`internal/plugin` no longer exists, so plugins import **only** `sdk/plugin`. Moving
the whole package (rather than splitting contract vs machinery) avoids a
`package plugin` self-import collision and is purely mechanical.

```
sdk/                         # module github.com/charlesng35/shellcn/sdk — owns the
                             #   whole wire contract (source + config + generated code)
  go.mod
  buf.yaml  buf.gen.yaml     # codegen config (run via `cd sdk && buf generate`)
  proto/pluginv1/plugin.proto   # the .proto source (flat; travels with go get)
  gen/pluginv1/                 # generated stubs (package pluginv1), checked in
  plugin/                    # package plugin — the whole contract + machinery, zero internal/* deps:
                             #   manifest, schema, ui, route, session interfaces, category,
                             #   recording, credentials, errors, sort/filter, RequestContext,
                             #   lean User/AuditResult/Snippet, AND registry/validate/projection
  grpcplugin/                # go-plugin glue: handshake, codec, conn/pipe, transport, server, proxy
  serve.go                   # sdk.Serve(p): the plugin main entry

internal/server/plugin_bridge.go   # boundary mappers: models.User→plugin.User,
                                    #   plugin.Snippet↔models.Snippet, plugin.AuditResult→models.AuditResult
```

**The one real decoupling:** `RequestContext.User` was a `models.User` (a GORM
model). The contract now exposes a **lean `plugin.User`** (id, username, displayName,
roles) — authz is already enforced in the core, so a handler needs identity only.
Likewise `plugin.AuditResult` and `plugin.Snippet` replace their `models` twins. The
**server** maps at the boundary (`internal/server/plugin_bridge.go`): it keeps
`models.User` internally and converts to `plugin.User` when building the request
context, bridges the snippet store, and maps the stream audit hook back to
`models.AuditResult`. Everything else in the contract moved verbatim.

Wiring: root `go.mod` `require` + `replace ./sdk`, a `go.work` (`use . ./sdk`), and
Makefile `PKG`/`GO_SOURCE_DIRS` include `sdk` — so `go build`/`test`/`lint` cover
both modules, with or without the workspace. The SDK will be tagged `sdk/vX.Y.Z`
once the wire ABI (Step 1) lands.

---

## 4. Implementation steps

Each step is independently testable and ends green (`make fmt && make lint &&
make test`).

### Step 0 — Extract the public contract into the nested `sdk/` module — §3.8

**Goal:** Make the plugin contract importable by external modules, with a minimal
dependency surface, without changing any behavior. **Prerequisite for Steps 1–8.**
**Checklist:**

- [ ] Create `sdk/` nested module (`go.mod` = `github.com/charlesng35/shellcn/sdk`).
- [ ] Move the contract files (`manifest`, `schema`, `ui`, `route`, `session`,
      `category`, `recording`, `credentials`, `errors`, `sort`, `filter`,
      `response`) into `sdk/plugin`; keep `registry`/`validate`/`projection`/
      credential-resolution in `internal/plugin`.
- [ ] Define a lean `contract.User` (id, username, roles); decouple
      `RequestContext` from `internal/models`; core maps `models.User → contract.User`.
- [ ] Rewrite imports `…/internal/plugin` → `…/sdk/plugin` across core + 40 plugins
      (~329 files; mechanical `gofmt -r`/sed pass).
- [ ] `sdk` module depends on nothing under `internal/`; core requires the `sdk`
      module (single module graph; tagged `sdk/vX.Y.Z`).
      **DoD:** `make fmt && make lint && make test` green with **zero behavior change**;
      the 40 built-ins compile against `sdk/plugin`; `sdk/plugin` has no `internal/*` import.

### Step 1 — Wire contract (`.proto` for `Plugin` + `Host`) + stubs

**Goal:** A versioned gRPC contract + a shared Go package the host and SDK import.
**Checklist:**

- [ ] `proto/plugin/v1/plugin.proto` with both services in §3.4
- [ ] Buf (or protoc) generation wired into the build; checked-in generated stubs
- [ ] Handshake config (magic cookie + `ProtocolVersion`) in a shared package
- [ ] Manifest crosses as the existing projection JSON (reuse, do not redefine)
      **DoD:** Stubs generate reproducibly; `make build` includes generation; no schema
      duplicated between proto and `internal/plugin`.

### Step 2 — Host-side adapter (`grpcPlugin` implements `plugin.Plugin`)

**Goal:** A loaded subprocess looks like any other `plugin.Plugin` to the registry.
**Checklist:**

- [ ] `grpcPlugin.Manifest()` returns the manifest fetched at load (incl. agent/recording)
- [ ] `grpcPlugin.Routes()` returns `plugin.Route`s with gRPC-shim `Handle`/`Stream`
- [ ] `grpcPlugin.Connect()` → `grpcSession{ id }` implementing `plugin.Session`
      (`HealthCheck`/`OpenChannel`/`Close`) **and** `plugin.HTTPProxy`
- [ ] Errors from the subprocess normalize to the core's `plugin.Err*` sentinels
- [ ] Crash/exit of the subprocess surfaces as session error, not a core panic
      **DoD:** A trivial in-repo test plugin registers through `Registry.Register` and
      its projection is byte-identical to an equivalent in-process plugin.

### Step 3 — Discovery + lifecycle manager

**Goal:** Load external plugins from a directory at startup; manage processes.
**Checklist:**

- [ ] Scan a configured `plugins.d/` dir; one subprocess per plugin binary
- [ ] `plugin.NewClient` with `AllowedProtocols=[gRPC]`, `AutoMTLS=true`, handshake
- [ ] Register each into the **same** `Registry` (validation gates bad manifests)
- [ ] Restart-on-crash with backoff; bounded; surfaced in admin/health
- [ ] Clean shutdown kills all subprocesses; no zombies
      **DoD:** Dropping a built plugin binary into `plugins.d/` and restarting makes the
      new protocol appear in the connection catalog with **zero** code change.

### Step 4 — Brokered egress through the core (L4 + L7, direct + agent)

**Goal:** The plugin reaches targets only through the core, for **every** transport.
**Checklist:**

- [ ] `Host.DialTarget` dials via the connection's `NetTransport` and brokers a
      `net.Conn`; works for **direct and agent (L4 tcp/unix)** unchanged
- [ ] `Host.HTTPProxyEndpoint` runs a per-session forward proxy applying the core
      RoundTripper; covers **L7 direct and agent (http_proxy)**
- [ ] SDK `NetTransport`: `DialContext` → `DialTarget`; `HTTP()` → proxy endpoint
- [ ] `Host.Audit` hook records stream-internal operations (parity with `AuditHook`)
- [ ] Egress + connection audit happen in the core, identical to in-process
      **DoD:** An external plugin reaches a DB over **direct** and the **same** plugin
      reaches it through an **enrolled agent** with no plugin code change; with brokering
      disabled it cannot reach the target (proving egress is core-owned).

### Step 5 — Streaming parity (server-stream, bidi terminal/exec, channels) + recording

**Goal:** Every stream kind a built-in supports, recorded identically.
**Checklist:**

- [ ] `InvokeServerStream` for logs/results → generic `log_stream`/results panels
- [ ] `OpenStream` control plane + **raw brokered conn** data plane → interactive
      terminal/exec (`stdin`/`stdout`, resize, exit-status)
- [ ] `OpenChannel` → raw brokered conn wrapped as `plugin.Channel`; session pinned
- [ ] Backpressure + cancellation propagate (client disconnect tears down cleanly)
- [ ] Core records external streams via its byte-bridge; authoritative recording
      via a declared server-stream
      **DoD:** An external plugin serves a live exec terminal (with working resize) and a
      followed log stream; both produce a session recording identical in shape to a
      built-in plugin's.

### Step 6 — HTTPProxy parity ("open in browser", incl. WebSocket)

**Goal:** The reverse-proxy capability works for external plugins.
**Checklist:**

- [ ] Core authn/authz, strips prefix, hijacks the browser conn, bridges to a
      brokered conn the plugin's `ServeHTTPProxy` serves
- [ ] Redirects, assets, and **WebSocket upgrades** pass through (raw bytes)
- [ ] CSRF-exempt proxy subtree handled as for built-ins
      **DoD:** A generated "open in browser" link to an external plugin's upstream loads
      a full web UI including a working WebSocket, through the brokered proxy.

### Step 7 — Plugin SDK + reference external plugin

**Goal:** A third-party dev writes a full-capability plugin with near-identical DX.
**Checklist:**

- [ ] `sdk` module: implement `Manifest()/Routes()/Connect()`, call `sdk.Serve`
- [ ] Reference plugin in `examples/` exercising **unary + terminal + agent
      transport + open-in-browser + recording** (proves parity end-to-end)
- [ ] `docs/external-plugins.md`: build, install, trust model, version policy,
      cross-compile matrix, agent-transport acknowledgement
- [ ] Golden test: SDK round-trips a manifest → projection identical to in-process
      **DoD:** Following the doc, a clean checkout builds the example plugin and loads it
      into a dev server, demonstrating every capability, without touching core code.

### Step 8 — Admin surface + trust controls

**Goal:** Operators see and gate what they load — without reducing capability.
**Checklist:**

- [ ] Admin lists loaded external plugins: name, version, declared permissions,
      transports, health
- [ ] Enable/disable per plugin; disabled plugins are not spawned
- [ ] Explicit per-plugin **agent-transport acknowledgement** before it may enroll
- [ ] Optional binary signature/checksum verification at load (config-gated)
- [ ] Audit events for plugin load/enable/disable/crash
      **DoD:** An admin can review a plugin's full capability/permission surface, must
      acknowledge agent transport before it tunnels, and can disable a misbehaving plugin
      without a restart.

---

## 5. Compatibility & versioning

- The `.proto` + `ProtocolVersion` + manifest `APIVersion` form a **stable wire
  contract**; once published it follows the project's compat rules. Breaking
  changes require a new `ProtocolVersion` and a documented migration.
- The host refuses plugins whose handshake/`ProtocolVersion` it does not support,
  with a clear operator-facing error (never a silent skip).
- Marketplace manifests do not pin the SDK module version. The install/runtime
  contract is the plugin's own version plus `APIVersion`, `ProtocolVersion`, the
  platform asset, and its checksum.

## 6. Risks & open questions

- **Maintenance tax:** publishing the SDK means owning a stable, full-surface
  plugin ABI indefinitely. This is larger than a request/response-only ABI —
  accept it deliberately.
- **High-bandwidth stream overhead:** raw brokered conns keep terminals and most
  streams at ~one local-socket memcpy, but a heavy framebuffer (desktop) still
  pays a copy+hop vs. in-process. Measure; the raw-conn design (not per-frame
  gRPC) is the mitigation. Acceptable for parity.
- **Resource limits:** per-subprocess CPU/mem/FD limits and concurrency caps —
  design before exposing to untrusted code (Step 8 follow-up).
- **Single-binary identity:** installing external plugins means core + a plugins
  dir of subprocesses. A conscious product trade for plugin users; the first-party
  single-binary experience is unchanged.

## 7. Testing

Per the project testing standard: unit tests for the adapter/SDK round-trip
(manifest→projection parity, error normalization); integration tests that load a
real example plugin binary and exercise **a unary route, a server-stream, an
interactive exec terminal with resize, an `OpenChannel`, the open-in-browser proxy
with a WebSocket, direct egress, and agent egress** — asserting the brokered-dial
path and that a recording is produced; a crash-recovery test asserting a killed
subprocess degrades to session error and restarts under backoff.

## 8. Cheaper alternative on record

If real-world demand for runtime loading proves small, the **bring-your-own-build**
model (a third-party adds an import to `all()` and runs `go build`) gives full
in-process power and a single binary at zero ongoing core cost — no proto, no SDK,
no ABI to maintain. This plan is justified only when load-without-recompile is a
firm requirement for non-developer operators.
