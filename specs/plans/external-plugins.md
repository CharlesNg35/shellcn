# External (out-of-tree) plugins — implementation plan

**Status:** Proposed · **Owner:** core · **Depends on:** M1 core runtime (plugin
registry, projection, route wrapper, session/transport)
**Spec refs:** [project.md §2](../project.md) (lines 57–62, out-of-tree → gRPC
subprocess), [project.md §5](../project.md) (plugin contract), [project.md §8](../project.md)
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

| Asset                                                                                                  | Location                                       | Relevance                                                                        |
| ------------------------------------------------------------------------------------------------------ | ---------------------------------------------- | -------------------------------------------------------------------------------- |
| `Plugin` interface (`Manifest`/`Routes`/`Connect`)                                                     | `internal/plugin/session.go`                   | The seam an external plugin plugs into.                                          |
| `Manifest` is pure data, already JSON-serialized                                                       | `internal/plugin/manifest.go`, `projection.go` | Crosses the process boundary for free.                                           |
| `Route` carries `Permission`/`Risk`/`AuditEvent`/`Input` as data; `Handle`/`Stream` are the only funcs | `internal/plugin/route.go`                     | Core enforces security from the data; only the func bodies move out-of-process.  |
| `Registry.Register(p Plugin)` validates + indexes any `Plugin`                                         | `internal/plugin/registry.go`                  | An external plugin registers through the **same** path; validation is identical. |
| `BuildProjection` derives the browser view from Manifest + routes                                      | `internal/plugin/projection.go`                | Unchanged — it never sees handler funcs.                                         |
| `NetTransport` (DialContext / HTTP RoundTripper) wired by core                                         | `internal/plugin/session.go`                   | The thing we must **broker** back to the subprocess to keep egress in the core.  |
| `Session` (`HealthCheck`/`OpenChannel`/`Close`) + `ClientStream`                                       | `internal/plugin/session.go`                   | Session state lives in the subprocess; lifecycle is proxied by handle.           |
| The route HTTP/WS wrapper (authn→authz→validate→audit→handler)                                         | core server adapter                            | Stays in the core; an external route is wrapped exactly like a built-in one.     |

**Consequence:** the core change is a single host-side adapter type
(`grpcPlugin` implementing `plugin.Plugin`) plus a discovery/loader. The
registry, projection, route wrapper, policy, audit, and the entire frontend do
**not** change.

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
are **shims** that make a gRPC call into the subprocess. `grpcPlugin.Connect()`
calls the subprocess and returns a `grpcSession` handle. The `Registry` validates
this exactly like a built-in plugin — a malformed external manifest is rejected at
load, before it can serve a request.

### 3.2 Transport: `hashicorp/go-plugin` (gRPC only)

Verified current capabilities used here: `AllowedProtocols: [ProtocolGRPC]`,
`AutoMTLS` (encrypted + mutually authenticated localhost channel), HTTP/2
multiplexing for bidirectional streams, `GRPCBroker` for brokering additional
`net.Conn`s, magic-cookie handshake + `ProtocolVersion`, and reattach. We use
gRPC exclusively (never net/rpc).

### 3.3 Scoped contract — smaller than the in-process one

External plugins get a **deliberately reduced** surface (v1). The brutal
streaming/transport features stay first-party only.

| Capability                                       | In-process (first-party) | External v1       |
| ------------------------------------------------ | ------------------------ | ----------------- |
| Manifest + projection                            | ✅                       | ✅                |
| Unary HTTP routes (`list`/`describe`/CRUD/query) | ✅                       | ✅                |
| Server-streaming routes (logs, query results)    | ✅                       | ✅                |
| Bidirectional WS routes (interactive terminal)   | ✅                       | ⚠️ deferred to v2 |
| `NetTransport` (direct) brokered through core    | ✅                       | ✅ (brokered)     |
| Agent transport / reverse tunnel                 | ✅                       | ❌ (direct only)  |
| `HTTPProxy` ("open in browser")                  | ✅                       | ❌                |
| Session recording                                | ✅                       | ❌                |

This covers the realistic long tail of third-party protocols (a new database, a
REST/SaaS API, a search engine, a queue) without paying for the hardest cases.

### 3.4 The wire contract (`.proto` sketch)

One service. The request `Context` mirrors the fields the core already builds for
`RequestContext` (params, query, body, acting user, session handle). The Manifest
crosses as the **same JSON** the projection already emits, so no schema is
duplicated.

```proto
syntax = "proto3";
package shellcn.plugin.v1;

service Plugin {
  rpc GetManifest(Empty) returns (ManifestJSON);              // JSON-encoded plugin.Manifest
  rpc Connect(ConnectRequest) returns (SessionHandle);        // returns opaque session id
  rpc HealthCheck(SessionHandle) returns (Empty);
  rpc Close(SessionHandle) returns (Empty);

  rpc Invoke(InvokeRequest) returns (InvokeResponse);         // unary HTTP route
  rpc InvokeStream(InvokeRequest) returns (stream Frame);     // server-stream route
}

message InvokeRequest {
  string session_id = 1;
  string route_id   = 2;          // stable handle; core resolves perms/audit, NOT the plugin
  map<string,string> params = 3;
  map<string,string> query  = 4;
  bytes  body = 5;                // already validated against Route.Input by the core
  ActingUser user = 6;            // for plugin-side context only; authz already done in core
  uint32 broker_dial_id = 7;      // GRPCBroker stream id for the brokered NetTransport
}
```

**Key invariant:** `route_id` is a handle. The core looks up the route's
`Permission`/`Risk`/`AuditEvent`/`Input` from the manifest it loaded and runs the
**same wrapper** before `Invoke` is ever called. The plugin cannot widen its own
permissions — it only ships data and handler bodies.

### 3.5 The two hard problems — decisions

**A. Session lifecycle across the boundary.** Core keeps sessions keyed by
`(connectionID, userID)`; the real state lives in the subprocess. Decision:
`Connect` returns an opaque `session_id`; the plugin SDK holds a session registry
keyed by it; every `Invoke`/`InvokeStream`/`HealthCheck`/`Close` carries the id.
The core's existing session registry stores the `grpcSession` handle and drives
lifecycle exactly as today.

**B. Who dials the target — and is it audited?** An access gateway must remain the
single, audited egress. Decision: the subprocess does **not** dial targets
directly. The core brokers a `net.Conn` to it via `GRPCBroker` (the `broker_dial_id`
on `InvokeRequest`/`Connect`), so the external plugin's driver dials _through_ the
core's `NetTransport`. The core stays the egress + policy + audit point. Direct
transport only in v1; brokering the dial is mandatory, not optional — a plugin
that freelances its own sockets would void the gateway's core guarantee.

### 3.6 Trust model

An external plugin is operator-installed code that receives decrypted credentials
for the connections it serves. Out-of-process gives **isolation** (separate
address space; can't read other plugins' memory; killable; sandboxable) but not
**safety** — like a Terraform provider or VS Code extension, the operator is
trusting it. Mitigations: `AutoMTLS` on the channel, optional binary signature
verification at load, a manifest of the plugin's declared permissions surfaced to
the admin before enable, and clear documentation that third-party plugins run with
the trust the operator grants them.

---

## 4. Implementation steps

Each step is independently testable and ends green (`make fmt && make lint &&
make test`).

### Step 1 — Define the plugin wire contract (`.proto` + generated stubs)

**Goal:** A versioned gRPC contract + a shared Go package the host and the SDK
both import.
**Checklist:**

- [ ] `proto/plugin/v1/plugin.proto` with the service in §3.4
- [ ] Buf (or protoc) generation wired into the build; checked-in generated stubs
- [ ] Handshake config (magic cookie + `ProtocolVersion`) in a shared package
- [ ] Manifest crosses as the existing projection JSON (reuse, do not redefine)
      **DoD:** Stubs generate reproducibly; `make build` includes generation; no schema
      duplicated between proto and `internal/plugin`.

### Step 2 — Host-side adapter (`grpcPlugin` implements `plugin.Plugin`)

**Goal:** A loaded subprocess looks like any other `plugin.Plugin` to the registry.
**Checklist:**

- [ ] `grpcPlugin.Manifest()` returns the manifest fetched at load
- [ ] `grpcPlugin.Routes()` returns `plugin.Route`s with gRPC-shim `Handle`/`Stream`
- [ ] `grpcPlugin.Connect()` → `grpcSession{ id }` implementing `plugin.Session`
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

### Step 4 — Brokered `NetTransport` (direct egress through the core)

**Goal:** The subprocess's driver dials targets through the core, not itself.
**Checklist:**

- [ ] Core opens a brokered `net.Conn` via `GRPCBroker`; passes `broker_dial_id`
- [ ] SDK exposes a `NetTransport` whose `DialContext` uses the brokered conn
- [ ] Egress + connection audit happen in the core, identical to in-process
- [ ] Direct transport only; agent transport explicitly unsupported in the SDK
      **DoD:** An external SQL test plugin connects to a DB **only** via the brokered
      dial; with brokering disabled it cannot reach the target (proving egress is core-owned).

### Step 5 — Server-streaming routes

**Goal:** Logs / query-result streaming for external plugins.
**Checklist:**

- [ ] `InvokeStream` maps a WS route's `StreamHandler` to a gRPC server stream
- [ ] Backpressure + cancellation propagate (client disconnect closes the stream)
- [ ] Frame format reuses the existing stream wire convention
      **DoD:** An external plugin streams a paginated/long result to the generic
      `log_stream`/results panel; closing the browser tab tears the stream down cleanly.

### Step 6 — Plugin SDK + reference external plugin

**Goal:** A third-party dev writes a plugin with near-identical DX to in-tree.
**Checklist:**

- [ ] `sdk` module: implement `Manifest()/Routes()/Connect()`, call `plugin.Serve`
- [ ] A reference external plugin (e.g. a simple REST/SQL protocol) in `examples/`
- [ ] `docs/external-plugins.md`: build, install, trust model, version policy
- [ ] Golden test: SDK round-trips a manifest → projection identical to in-process
      **DoD:** Following the doc, a clean checkout builds the example plugin and loads it
      into a dev server without touching core code.

### Step 7 — Admin surface + trust controls

**Goal:** Operators see and gate what they load.
**Checklist:**

- [ ] Admin lists loaded external plugins: name, version, declared permissions, health
- [ ] Enable/disable per plugin; disabled plugins are not spawned
- [ ] Optional binary signature/checksum verification at load (config-gated)
- [ ] Audit events for plugin load/enable/disable/crash
      **DoD:** An admin can review a plugin's declared permission/risk surface before
      enabling it, and disable a misbehaving one without a restart.

---

## 5. Compatibility & versioning

- The `.proto` + `ProtocolVersion` + manifest `APIVersion` form a **stable wire
  contract**; once published it follows the project's compat rules. Breaking
  changes require a new `ProtocolVersion` and a documented migration.
- The host refuses plugins whose handshake/`ProtocolVersion` it does not support,
  with a clear operator-facing error (never a silent skip).

## 6. Risks & open questions

- **Maintenance tax:** publishing the SDK means owning a stable plugin ABI
  indefinitely. Weigh against expected adoption before committing to Step 6.
- **Bidirectional terminals (v2):** interactive `OpenChannel`-style streams over
  gRPC add latency; decide per-demand whether external plugins ever need them.
- **Resource limits:** per-subprocess CPU/mem/FD limits and concurrency caps —
  design before exposing to untrusted code (Step 7 follow-up).
- **Single-binary identity:** installing external plugins means core + a plugins
  dir of subprocesses. This is a conscious product trade for plugin users; the
  first-party single-binary experience is unchanged.

## 7. Testing

Per [TESTING.md](TESTING.md): unit tests for the adapter/SDK round-trip
(manifest→projection parity, error normalization); an integration test that loads
a real example plugin binary, registers it, exercises a unary + a streaming route,
and verifies the brokered-dial egress path; a crash-recovery test asserting a
killed subprocess degrades to session error and restarts under backoff.

## 8. Cheaper alternative on record

If real-world demand for runtime loading proves small, the **bring-your-own-build**
model (a third-party adds an import to `all()` and runs `go build`) gives full
in-process power and a single binary at zero ongoing core cost — no proto, no SDK,
no ABI to maintain. This plan is justified only when load-without-recompile is a
firm requirement for non-developer operators.
