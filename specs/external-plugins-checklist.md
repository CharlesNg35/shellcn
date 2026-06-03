# External (out-of-tree) plugins — implementation checklist

Companion task list for [`external-plugins.md`](./external-plugins.md). Check items
off as they land. Each step is done only when its tests pass and `make fmt &&
make lint && make test` are green. Section refs (§) point at the plan.
**Full capability parity, all in v1 — no feature is first-party-only, nothing deferred.**

---

## Step 0 — Extract the public contract into the nested `sdk/` module — §3.8

**Prerequisite for Steps 1–8.** One-time packaging refactor, no behavior change.

- [ ] Create `sdk/` nested module (`go.mod` = `github.com/charlesng35/shellcn/sdk`,
      minimal deps: grpc, go-plugin, contract types only).
- [ ] Move contract files (`manifest`, `schema`, `ui`, `route`, `session`,
      `category`, `recording`, `credentials`, `errors`, `sort`, `filter`,
      `response`) into `sdk/plugin`; keep `registry`/`validate`/`projection`/
      credential-resolution in `internal/plugin`.
- [ ] Define lean `contract.User` (id, username, roles); decouple `RequestContext`
      from `internal/models`; core maps `models.User → contract.User`.
- [ ] Rewrite imports `…/internal/plugin` → `…/sdk/plugin` across core + 40 plugins
      (~329 files; mechanical `gofmt -r`/sed pass).
- [ ] `sdk/plugin` has **zero** `internal/*` imports; core requires the `sdk` module;
      tagged `sdk/vX.Y.Z` so the wire/ABI version travels with it.
- [ ] **DoD:** `make fmt && make lint && make test` green with **zero behavior
      change**; the 40 built-ins compile against `sdk/plugin`.

## Step 1 — Wire contract (`.proto` for `Plugin` + `Host`) + stubs — §3.4

- [ ] `proto/plugin/v1/plugin.proto` with **both** services: `Plugin` (served by
      plugin) and `Host` (served by core: `DialTarget`/`HTTPProxyEndpoint`/`Audit`).
- [ ] `Plugin` service: `GetManifest`, `Connect`, `HealthCheck`, `Close`,
      `Invoke`, `InvokeServerStream`, `OpenStream` (bidi control), `OpenChannel`,
      `ServeHTTPProxy`.
- [ ] Buf (or protoc) generation wired into the build; checked-in generated stubs.
- [ ] Handshake config (magic cookie + `ProtocolVersion`) in a shared package.
- [ ] Manifest crosses as the existing projection JSON (reuse, do **not** redefine).
- [ ] **DoD:** stubs generate reproducibly; `make build` includes generation; no
      schema duplicated between proto and `internal/plugin`.

## Step 2 — Host-side adapter (`grpcPlugin` implements `plugin.Plugin`) — §3.1

- [ ] `grpcPlugin.Manifest()` returns the manifest fetched at load (incl.
      `AgentProfile`, `Recording`).
- [ ] `grpcPlugin.Routes()` returns `plugin.Route`s with gRPC-shim `Handle`/`Stream`.
- [ ] `grpcPlugin.Connect()` → `grpcSession{ id }` implementing `plugin.Session`
      (`HealthCheck`/`OpenChannel`/`Close`) **and** `plugin.HTTPProxy`.
- [ ] Subprocess errors normalize to the core's `plugin.Err*` sentinels.
- [ ] Crash/exit surfaces as session error, not a core panic.
- [ ] **DoD:** a trivial in-repo test plugin registers through `Registry.Register`
      and its projection is byte-identical to an equivalent in-process plugin.

## Step 3 — Discovery + lifecycle manager — §3.1

- [ ] Scan a configured `plugins.d/` dir; one subprocess per plugin binary.
- [ ] `plugin.NewClient` with `AllowedProtocols=[gRPC]`, `AutoMTLS=true`, handshake.
- [ ] Register each into the **same** `Registry` (validation gates bad manifests).
- [ ] Restart-on-crash with bounded backoff; surfaced in admin/health.
- [ ] Clean shutdown kills all subprocesses; no zombies.
- [ ] **DoD:** dropping a built plugin binary into `plugins.d/` and restarting
      makes the new protocol appear in the catalog with **zero** code change.

## Step 4 — Brokered egress through the core (L4 + L7, direct + agent) — §3.5

- [ ] `Host.DialTarget` dials via the connection's `NetTransport` and brokers a
      `net.Conn`; works for **direct and agent (L4 tcp/unix)** unchanged.
- [ ] `Host.HTTPProxyEndpoint` runs a per-session forward proxy applying the core
      RoundTripper; covers **L7 direct and agent (http_proxy)**.
- [ ] SDK `NetTransport`: `DialContext` → `DialTarget`; `HTTP()` → proxy endpoint.
- [ ] `Host.Audit` hook records stream-internal operations (parity with `AuditHook`).
- [ ] Egress + connection audit happen in the core, identical to in-process.
- [ ] **DoD:** an external plugin reaches a DB over **direct** and the **same**
      plugin reaches it through an **enrolled agent** with no plugin code change;
      with brokering disabled it cannot reach the target.

## Step 5 — Streaming parity + recording — §3.5

- [ ] `InvokeServerStream` for logs/results → generic `log_stream`/results panels.
- [ ] `OpenStream` control plane + **raw brokered conn** data plane → interactive
      terminal/exec (`stdin`/`stdout`, resize, exit-status).
- [ ] `OpenChannel` → raw brokered conn wrapped as `plugin.Channel`; session pinned.
- [ ] Backpressure + cancellation propagate (client disconnect tears down cleanly).
- [ ] Core records external streams via its byte-bridge; authoritative recording
      via a declared server-stream.
- [ ] **DoD:** an external plugin serves a live exec terminal (working resize) and a
      followed log stream; both produce a recording identical in shape to a
      built-in plugin's.

## Step 6 — HTTPProxy parity ("open in browser", incl. WebSocket) — §3.5

- [ ] Core authn/authz, strips prefix, hijacks the browser conn, bridges to a
      brokered conn the plugin's `ServeHTTPProxy` serves.
- [ ] Redirects, assets, and **WebSocket upgrades** pass through (raw bytes).
- [ ] CSRF-exempt proxy subtree handled as for built-ins.
- [ ] **DoD:** a generated "open in browser" link to an external plugin's upstream
      loads a full web UI including a working WebSocket, through the brokered proxy.

## Step 7 — Plugin SDK + reference external plugin — §3.1

- [ ] `sdk` module: implement `Manifest()/Routes()/Connect()`, call `sdk.Serve`.
- [ ] Reference plugin in `examples/` exercising **unary + terminal + agent
      transport + open-in-browser + recording** (proves parity end-to-end).
- [ ] `docs/external-plugins.md`: build, install, trust model, version policy,
      cross-compile matrix, agent-transport acknowledgement.
- [ ] Golden test: SDK round-trips a manifest → projection identical to in-process.
- [ ] **DoD:** following the doc, a clean checkout builds the example plugin and
      loads it into a dev server, demonstrating every capability, without touching
      core code.

## Step 8 — Admin surface + trust controls — §3.6

- [ ] Admin lists loaded external plugins: name, version, declared permissions,
      transports, health.
- [ ] Enable/disable per plugin; disabled plugins are not spawned.
- [ ] Explicit per-plugin **agent-transport acknowledgement** before it may enroll.
- [ ] Optional binary signature/checksum verification at load (config-gated).
- [ ] Audit events for plugin load/enable/disable/crash.
- [ ] **DoD:** an admin can review a plugin's full capability/permission surface,
      must acknowledge agent transport before it tunnels, and can disable a
      misbehaving plugin without a restart.

## Cross-cutting (apply across steps) — §3.2, §3.7, §5

- [ ] **Raw conns for bytes, gRPC for control:** every byte-stream (terminal,
      desktop, `OpenChannel`, HTTPProxy, L4 dial) rides a raw brokered `net.Conn`,
      never per-frame protobuf.
- [ ] **Egress stays in the core:** plugins never dial targets themselves; all
      reach via `Host.DialTarget`/`HTTPProxyEndpoint` (direct or agent).
- [ ] **Permissions are core-enforced:** `route_id` is a handle; the core resolves
      `Permission`/`Risk`/`AuditEvent`/`Input` and runs the wrapper before `Invoke`.
- [ ] Versioning: `.proto` + `ProtocolVersion` + manifest `APIVersion` form a
      stable wire contract; host refuses unsupported versions with a clear error.
- [ ] Per-subprocess resource limits (CPU/mem/FD) + concurrency caps.
- [ ] Cross-compile build matrix (`linux/amd64,arm64`, `darwin/arm64`,
      `windows/amd64`); pure-Go drivers preferred; wrong-arch fails handshake cleanly.
- [ ] **Code rules (AGENTS.md):** verify every lib/API via context7 + websearch;
      PrimeVue-only admin UI via the preset; VueUse; **pnpm** (never npm); small
      focused units; minimal comments (non-obvious _why_ only), **no** spec/PR refs
      in source; plugin-agnostic core (the adapter must not special-case any plugin).
- [ ] **Gate green:** `make fmt && make lint && make test`.
- [ ] **Write _and execute_** integration tests (load a real example plugin binary;
      exercise unary, server-stream, exec terminal + resize, `OpenChannel`,
      open-in-browser WebSocket, direct egress, agent egress; assert a recording is
      produced; crash → session error → restart under backoff).
- [ ] Update `specs/project.md` with the external-plugin architecture once stable.

## Pre-build confirmations — §1, §3, §6

- [ ] Scope: keep the 40 first-party plugins compiled in; external = out-of-process
      only. Pure OSS extensibility (no commercial/licensing, not for size reduction).
- [ ] **Full capability parity, all in v1** — no v2, nothing first-party-only.
- [ ] Mechanism: `hashicorp/go-plugin`, **gRPC only**, `AutoMTLS`, `GRPCBroker`.
- [ ] Packaging: **nested SDK module** `github.com/charlesng35/shellcn/sdk` (in this
      repo, NOT under `internal/`), holding the public contract (`sdk/plugin`) +
      serve glue; core + 40 built-ins import the contract from there; versioned
      `sdk/vX.Y.Z`.
- [ ] Egress + audit owned by the core via the `Host` service; agent transport is
      gated by an operator acknowledgement (trust control, not a capability cut).
- [ ] Accept the maintenance tax: a stable, full-surface plugin ABI owned indefinitely.
- [ ] Accept the trade: installing external plugins means core + a `plugins.d/` of
      subprocesses (first-party single-binary experience unchanged).
- [ ] Cheaper alternative on record: bring-your-own-build (add import to `all()`,
      `go build`) if runtime-loading demand proves small.
