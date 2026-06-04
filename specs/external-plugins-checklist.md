# External (out-of-tree) plugins — implementation checklist

Companion task list for [`external-plugins.md`](./external-plugins.md). Check items
off as they land. Each step is done only when its tests pass and `make fmt &&
make lint && make test` are green. Section refs (§) point at the plan.
**Full capability parity, all in v1 — no feature is first-party-only, nothing deferred.**

---

## Step 0 — Extract the public contract into the nested `sdk/` module — §3.8

**Prerequisite for Steps 1–8.** One-time packaging refactor, no behavior change. **Done.**

- [x] Create `sdk/` nested module (`go.mod` = `github.com/charlesng35/shellcn/sdk`).
      Deps so far: `go-playground/validator/v10`; grpc + go-plugin added in Step 1.
- [x] Move the **entire** `internal/plugin` package → `sdk/plugin` (contract +
      `registry`/`validate`/`projection`/credential-resolution). `internal/plugin`
      removed — plugins import **only** `sdk/plugin`, never `internal/*`. (Whole-package
      move avoids a `package plugin` self-collision and is purely mechanical.)
- [x] Define lean `plugin.User` (id, username, displayName, roles), `plugin.AuditResult`
      (+constants), `plugin.Snippet`; decouple `RequestContext` from `internal/models`.
      Server maps at the boundary: `toPluginUser`, `snippetBridge`, audit-hook
      `plugin.AuditResult → models.AuditResult` (`internal/server/plugin_bridge.go`).
- [x] Rewrite imports `…/internal/plugin` → `…/sdk/plugin` repo-wide (329 non-test +
      tests); `models.Audit*`/`Snippet` → `plugin.*` in plugins; lean-type swap at
      test `NewRequestContext` sites.
- [x] `sdk/plugin` has **zero** `internal/*` imports (verified); root `go.mod`
      require + `replace ./sdk`; `go.work` (`use . ./sdk`); Makefile `PKG` +
      `GO_SOURCE_DIRS` include `sdk`. Tag `sdk/vX.Y.Z` deferred until the wire ABI lands.
- [x] **DoD met:** `go build`/`go vet`/`golangci-lint`/`go test` green across both
      modules — **73 pkgs pass, 0 fail**, incl. moved `sdk/plugin` contract tests;
      builds with **and** without the workspace (`GOWORK=off`); zero behavior change.

## Step 0.5 — Built-ins are SDK-only (no `internal/*`), enforced — **Done**

Built-ins are the reference for out-of-tree plugins, which (being a separate
module) cannot import `internal/*`. Enforce the same on built-ins so they stay a
faithful template.

- [x] Moved gateway-owned constants (`AgentBinary`, `AgentImageLatest`,
      `AgentInternalAddress`, `DefaultClientName`) to `sdk/plugin`; `internal/app`
      aliases them (single source of truth, core unchanged); 13 plugin prod files
      use `plugin.*`, drop `internal/app`.
- [x] Added `sdk/plugintest` (`DirectTransport`, `TransportFunc`); rewrote ~26
      plugin test files off `internal/transport` + `internal/models.Connection`.
- [x] Relocated `plugins/docker/enrollment_test.go` → `internal/service`
      (`service_test`, imports `plugins/docker`) — it tests the enrollment service,
      not the plugin contract.
- [x] **`plugins/` is now 100% free of `internal/*` (prod AND test).**
- [x] **depguard** rule `plugins-sdk-only` in `.golangci.yml` bans
      `github.com/charlesng35/shellcn/internal` from `plugins/**` — verified it
      fires on a planted import; lint clean on real code.
- [x] Gate green: build/vet/lint/test pass on both modules.

## Step 1 — Wire contract (`.proto` for `Plugin` + `Host`) + stubs — §3.4 — **Done**

- [x] `sdk/proto/pluginv1/plugin.proto` (package `pluginv1` — flat, version-suffixed;
      the redundant org/area nesting was dropped since the module path already says
      `shellcn`) with **both** services: `Plugin` (served by plugin) and `Host`
      (served by core: `DialTarget`/`HTTPProxyEndpoint`/`OpenHTTPConn`/`Audit`).
      Self-contained (local `Empty`, no well-known-type imports → reproducible
      offline). The whole wire contract lives **inside the SDK module** (source +
      config + gen) so it travels with `go get`.
- [x] `Plugin` service: `GetManifest`, `Connect`, `HealthCheck`, `Close`,
      `Invoke`, `OpenStream`, `OpenChannel`, `ServeHTTPProxy`.
      Byte-streams ride raw brokered conns named by `BrokerRef.broker_id`.
- [x] **buf** generation: `sdk/buf.yaml` (BASIC lint) + `sdk/buf.gen.yaml` (managed
      mode, `go_package_prefix`) → stubs at `sdk/gen/pluginv1` (package `pluginv1`,
      imported directly), checked in. `make proto` (`cd sdk && buf generate`).
- [x] Handshake (`sdk/grpcplugin`): `Handshake` (magic cookie) + `ProtocolVersion` + `PluginName` dispense key.
- [x] Manifest crosses as JSON bytes (`Manifest.json`); contract owned by Go types
      in `sdk/plugin`, not duplicated in protobuf.
- [x] sdk deps: grpc 1.67.0, protobuf 1.36.11, go-plugin 1.8.0.
- [x] **DoD met:** `buf generate` reproducible (no diff on regen); build/lint/test
      green both modules (root 72 ok/0 fail, sdk ok); `make proto` works.

## Step 2 — Host-side adapter (`grpcPlugin` implements `plugin.Plugin`) — §3.1 — **Done**

Manifest round-trip via approach (A): `Panel`/`Action` decode `PanelConfig` by
panel type (`sdk/plugin/config_json.go`); `Route` JSON-tagged, funcs `json:"-"`.
Bundle codec in `sdk/grpcplugin`. Adapter in `internal/extplugin`.

- [x] `grpcPlugin.Manifest()` returns the manifest decoded from the subprocess
      (full round-trip incl. nested panel configs, `AgentProfile`, `Recording`).
- [x] `grpcPlugin.Routes()` returns `plugin.Route`s with gRPC-shim `Handle`
      (forwards to `Invoke`); `Stream` returns `ErrNotSupported` until Step 5.
- [x] `grpcPlugin.Connect()` → `grpcSession{ id }` implementing `plugin.Session`
      (`HealthCheck`/`Close`; `OpenChannel` → `ErrNotSupported` until Step 5).
      `HTTPProxy` deferred to Step 6.
- [x] Subprocess errors normalize to `plugin.Err*` via `grpcplugin.ErrorFromStatus`
      (+ symmetric `StatusFromError` for the serve side).
- [x] `RequestContext.Params()`/`Body()` accessors added for the Invoke shim.
- [x] **DoD met:** bufconn test registers the adapter through `Registry.Register`;
      projection is **byte-identical** to in-process `BuildProjection`; a unary
      `Invoke` round-trips; a gRPC `NotFound` normalizes to `plugin.ErrNotFound`.
      Build/lint/test green both modules (root 73 ok/0 fail, sdk ok).

## Step 3 — Discovery + lifecycle manager — §3.1 — **Done**

Also delivered the plugin-side serve glue (minimal Step 7) so a real subprocess
exists to spawn: `sdk/grpcplugin/server.go` (PluginServer + session registry),
`sdk/grpcplugin/goplugin.go` (`GoPlugin` GRPCServer/GRPCClient bridge + `Plugins`),
`sdk/serve.go` (`sdk.Serve` — the plugin `main` entry).

- [x] `Manager` scans `plugins.d/`; one `goplugin.Client` subprocess per binary.
- [x] `goplugin.NewClient` with `AllowedProtocols=[gRPC]`, `AutoMTLS=true`, handshake.
- [x] Each spawned plugin wraps via `extplugin.New` and registers into the **same**
      `Registry` (validation gates bad manifests).
- [x] `Close` kills all subprocesses; per-plugin load failures skipped (joined
      errors returned) so one bad plugin can't block the rest.
- [x] Restart-on-crash with bounded backoff: a supervisor goroutine per plugin
      polls `client.Exited()` and respawns (200ms→30s backoff), swapping the live
      gRPC client via a `clientRef` so the registered manifest/routes are
      undisturbed. Verified by a test that crashes the subprocess (`os.Exit`) and
      asserts a fresh `Connect`+`Invoke` recovers.
- [x] **DoD met (end-to-end):** test builds a real plugin binary (`testdata/
demoplugin`, `sdk.Serve`), `Manager.LoadAll` spawns + registers it, then
      `Connect`+`HealthCheck`+`Invoke` round-trip **over the live gRPC
      subprocess**; `Close` is clean. Build/lint/test green both modules (root 73
      ok/0 fail, sdk ok).

## Step 4 — Brokered egress through the core (L4 + L7, direct + agent) — §3.5 — **Done**

Mechanism: `grpcPlugin.Connect` serves a per-connection **`Host`** service (backed
by the core's `cfg.Net`) on a brokered id, passed to the plugin as
`host_broker_id`; the plugin builds a `brokerTransport` whose `DialContext` calls
`Host.DialTarget`. Bytes ride a raw `Conn.Pipe` stream wrapped as `net.Conn`
(`sdk/grpcplugin/conn.go` `streamConn`/`connBridge`).

- [x] `Host.DialTarget` dials via the connection's `NetTransport` and brokers a
      `net.Conn` back. Direct **and** agent are automatic — `cfg.Net` is whatever
      the core wired (L4 tcp/unix); the plugin code is identical.
- [x] SDK `NetTransport`: `DialContext` → `Host.DialTarget` → brokered `Conn.Pipe`.
      `HTTP()` returns `ok=false` (use `DialContext`), exactly like the core's
      `Direct` transport — so HTTP-over-L4 works for direct/agent plugins.
- [x] **L7 egress is wired:** `Host.OpenHTTPConn` brokers a conn to a core-run
      reverse proxy (`grpcplugin.NewHTTPProxyBridge`, served over `Conn.Pipe` with
      a `singleConnListener`) that applies `cfg.Net`'s RoundTripper — so agent
      `http_proxy` auth injection works. SDK `brokerTransport.HTTP()` returns that
      L7 client. Tested: `TestPluginL7ThroughCore` (plugin fetches an HTTP target
      through the core's reverse proxy). The reverse-proxy primitive is reused by
      Step 6.
- [x] **`Host.Audit` forwards end-to-end:** the plugin's `rc.Audit(...)` →
      `Host.Audit` (per-connection host client) → core `AuditFunc`
      (`Manager` `WithAudit`). Tested: `TestPluginAuditForwardsToCore`. (The core
      `AuditFunc` connects to the real audit writer when the server adopts the
      Manager — a startup-wiring step, not a plugin gap.)
- [x] **Egress stays in the core:** the plugin never dials targets itself.
- [x] **DoD met (end-to-end):** `TestPluginEgressThroughCore` — a real subprocess
      plugin echoes bytes off a TCP target **through the core's transport**; with
      **no** core transport it cannot reach the target. Build/lint/test green both
      modules (root 73 ok/0 fail, sdk ok). (Agent path shares the identical
      `cfg.Net` code path; a live agent tunnel isn't stood up in the unit test.)

## Step 5 — Streaming parity + recording — §3.5 — **Done**

Unified the byte-pipe: one `grpcplugin.pipeServer` + shared `ServeConn`/`DialConn`
back every brokered conn (terminal, channel, dial, HTTP proxy). Dropped the
redundant `InvokeServerStream`/`Frame` — every WS route is bidi in the Go contract,
so `OpenStream` covers terminal/exec **and** logs/results alike.

- [x] `OpenStream` control plane + **raw brokered conn** data plane → bidi WS routes
      (terminal/exec stdin/stdout, and logs/results which just write). Plugin runs
      `route.Stream(rc, clientStream)` over the brokered conn; host bridges it to
      the browser `ClientStream`.
- [x] `OpenChannel` → impl `Session.OpenChannel` bridged to a raw brokered conn,
      wrapped host-side as `plugin.Channel` (`grpcChannel`).
- [x] Cancellation propagates: when the browser `ClientStream.Context()` is done,
      the host closes the brokered conn → the plugin handler's reads EOF and
      `route.Stream` returns. Tested (`TestPluginBidiStream` ends on disconnect).
- [x] **Recording is transparent:** the core bridges the browser `ClientStream`
      (which the server already wraps with the recorder) to the plugin, so it
      records external streams byte-for-byte with no plugin-system code — the host
      is the byte-pump in the middle.
- [x] **DoD met (end-to-end):** `TestPluginBidiStream` (a real subprocess plugin
      echoes over a brokered stream, tears down on disconnect) and
      `TestPluginOpenChannel` (channel echo). Build/lint/test green both modules
      (root 73 ok/0 fail, sdk ok). (Resize/exit-status are app-level frames the
      handler reads from the same stream — no extra wire surface needed.)

## Step 6 — HTTPProxy parity ("open in browser", incl. WebSocket) — §3.5 — **Done**

`grpcSession` now implements `plugin.HTTPProxy` (`internal/extplugin/proxy.go`):
it calls `Plugin.ServeHTTPProxy` (→ `BrokerRef`), hijacks the browser conn, writes
the request, and raw-pipes both ways. Plugin side (`sdk/grpcplugin/proxy.go`)
serves the impl's `ServeHTTPProxy` via `http.Server` over the brokered conn
(`singleConnListener`).

- [x] Core hijacks the browser conn and bridges raw bytes to a brokered conn the
      plugin's `ServeHTTPProxy` serves. (authn/authz/prefix-strip stay in the core's
      existing proxy route, unchanged — `grpcSession` is just the `HTTPProxy` impl.)
- [x] Redirects, assets, and **WebSocket upgrades** pass through: after the request
      it's a raw byte bridge, so a 101 + WS frames (or any streamed body) flow
      unchanged. The plugin's `http.Server` supports hijack, so the impl's reverse
      proxy upgrades natively.
- [x] CSRF-exemption etc. live on the core's proxy route — no change for external.
- [x] **DoD met (end-to-end):** `TestPluginHTTPProxy` — `client → core (hijack) →
brokered conn → plugin reverse-proxy → upstream via cfg.Net → back`, asserting
      the proxied body. Exercises proxy **and** core egress together. Build/lint/test
      green both modules (root 73 ok/0 fail, sdk ok). (HTTP GET tested; WS uses the
      identical raw-byte path after the upgrade request.)

## Step 7 — Plugin SDK + reference external plugin — §3.1 — **Done**

- [x] `sdk.Serve(p)` is the `main` entry; a plugin implements `Manifest()/Routes()/
Connect()` against `sdk/plugin`. (Built in Step 3.)
- [x] **Reference plugin `examples/memo`** — its **own Go module** depending only on
      the SDK (no core/`internal/`): in-memory notes with a manifest (table panel +
      create form + row delete), unary CRUD routes, and session state. README +
      `replace` documenting the out-of-tree pattern. The capability matrix
      (streaming/agent/proxy/recording) lives in the docs + the testdata demo;
      `memo` is the idiomatic copy-me starting point.
- [x] `docs/external-plugins.md`: write/build (cross-compile matrix, pure-Go),
      install (`plugins.d/`), capability table, the egress rule (`cfg.Net`), trust
      model, agent-transport acknowledgement, versioning.
- [x] **Golden test** (`sdk/grpcplugin/codec_test.go`): `EncodeManifest` →
      `DecodeManifest` → projection **byte-identical** to in-process.
- [x] **DoD met:** `TestExampleMemoLoads` builds `examples/memo` as the standalone
      module it is (`GOWORK=off`) and loads it via the `Manager`; CRUD round-trips
      over the live subprocess — **no core changes**. `make build` of the example
      verified `GOWORK=off` (lean dep tree: grpc/go-plugin/validator only).
- [x] **Fixed a `-race` bug** found by CI: `bridgeStream` closed the brokered conn
      from two goroutines → concurrent `grpc CloseSend`. `streamConn.Close` is now
      `sync.Once`-guarded (idempotent, correct `net.Conn` hygiene). Full `-race`
      gate green both modules (root 73 ok/0 fail/**0 races**, sdk ok).

## Step 8 — Admin surface + trust controls — §3.6 — **Mostly done**

The trust boundary is the operator-controlled `plugins.dir` plus the availability
policy. Speculative hardening (per-plugin agent-transport acknowledgement, sidecar
checksums, runtime subprocess kill/respawn, per-subprocess resource limits) was
intentionally left out — it added surface and friction without moving that
boundary. Real binary-integrity verification (config-pinned digest or signature)
remains a clean option if a concrete need arises.

**Server-integration glue (was the missing prerequisite) — Done.** The `Manager`
is now wired into startup: new `plugins.dir` config (default `plugins.d`,
overridable via `config.yaml`/`SHELLCN_PLUGINS_DIR`) drives
`extplugin.NewManager` → `LoadAll(ctx, reg)` (registers external plugins into the
**same** registry as the built-ins) → `defer Close()` on shutdown, all in
`cmd/server/main.go` `run()`. The Manager's `WithAudit` forwards plugin
stream-internal audit to the core writer. `Manager.Loaded()` exposes
name/path/health for the admin surface.

**Protocol availability (admin surface) — Done.** An admin "Protocols" page
(`web/src/views/ProtocolsView.vue`, route `settings/protocols`, admin-gated on
the router **and** the backend `requireAdmin`) lists built-in and external
protocols in two tabs with title/icon/version/transports and, for external,
live health. Each protocol has a 3-state availability — **enabled** (all),
**admin_only** (admins only), **disabled** (nobody) — persisted in
`models.ProtocolSetting` via `service.ProtocolService`. Default (no row) =
enabled, so behavior is unchanged until an admin acts.

- [x] Admin lists loaded plugins: name, title, **version**, **transports**,
      **health** (built-in + external, split by tab).
- [x] **Availability per protocol** (enabled / admin*only / disabled), enforced
      end-to-end: `/plugins` catalog is filtered per user; the single chokepoint
      `acquireSession` blocks an unavailable protocol for HTTP routes, WS streams,
      the open-in-browser proxy, **and** AI `InvokeRoute`, returning a clear
      `403 … this protocol is not available`; connection-create is validated too.
      Existing connections still render (sidebar icon comes from the connection
      summary) — only *connecting* errors. *(This is a richer policy than the
      original "disabled = not spawned": disabling hides + blocks use without
      killing the subprocess, so the admin can re-enable without a restart.)\_
- [x] **Capability review:** each row shows the protocol's declared **route risk
      levels** (distinct, sorted) and **recording classes** alongside transports,
      so an admin reviews the surface before exposing it.
- [~] **Checksum/signature verification at load** — dropped. A `<binary>.sha256`
  sidecar next to the binary is a circular trust anchor (anyone who can write
  the binary can rewrite the sidecar) and corruption already fails the go-plugin
  handshake. If integrity is ever required, do it properly: pin the expected
  digest in `config.yaml` (trusted source) or verify a real signature
  (cosign/minisign) against a pinned public key.
- [x] **Audit events:** availability changes are audited as `protocol.availability`
      (with the acting admin). _(Load/crash lifecycle auditing was dropped — `LoadAll`
      runs on every startup, so it would append a duplicate `plugin.load` row per
      restart; the slog startup log already covers boot-time load/skip.)_
- [x] **Tests:** `internal/service/protocols_test.go` (`Allows`,
      Set/States/Allowed, invalid-state rejection);
      `internal/server/protocols_test.go` (admin-only gate; disabled hidden+blocked
      +restore; admin_only visibility/connect; capability surface). Gate green:
      `make fmt && make lint && make test` (Go `-race` + 398 vitest + 18 e2e).
- [~] **Agent-transport acknowledgement** — dropped as over-engineering. The
  operator-controlled `plugins.dir` and the availability policy already gate
  untrusted code; a separate per-plugin ack added friction without meaningful
  additional safety.
- [x] **DoD:** an admin can review a plugin's capability surface (risk + recording + transports) and disable a misbehaving plugin without a restart (via
      availability). Disabling is a policy (hidden + connect-blocked), not a process
      kill — the idle subprocess stays running and re-enables instantly; to fully
      unload an external plugin, remove its binary from `plugins.dir` and restart.

## Cross-cutting (apply across steps) — §3.2, §3.7, §5

- [x] **Raw conns for bytes, gRPC for control:** every byte-stream (terminal,
      desktop, `OpenChannel`, HTTPProxy, L4 dial) rides a raw brokered `net.Conn`,
      never per-frame protobuf. (Steps 4–6.)
- [x] **Egress stays in the core:** plugins never dial targets themselves; all
      reach via `Host.DialTarget`/`OpenHTTPConn` (direct or agent). (Step 4.)
- [x] **Permissions are core-enforced:** `route_id` is a handle; the core resolves
      `Permission`/`Risk`/`AuditEvent`/`Input` and runs the wrapper before `Invoke`.
      Protocol availability is enforced at the same boundary (`acquireSession`).
- [x] Versioning: handshake magic cookie + `ProtocolVersion` refuse an
      unsupported plugin at load; manifest `APIVersion` is validated by the
      registry. Marketplace manifests do not pin the SDK module version or
      duplicate display metadata. (Step 1.)
- [x] **Out of core scope (by design):** per-subprocess resource limits are an
      ops concern (run the gateway under a container/systemd slice), and the
      cross-compile build matrix belongs to each plugin author's own repo —
      `docs/external-plugins.md` documents the matrix + pure-Go preference, and a
      wrong-arch binary fails the handshake cleanly.
- [x] **Code rules (AGENTS.md):** libs verified; PrimeVue-only admin UI via the
      preset (`Tabs`/`DataTable`/`Column`/`Select`/`Breadcrumb`); **pnpm**; small
      focused units; minimal _why_-only comments, **no** spec/PR refs in source;
      plugin-agnostic core (availability keys on the manifest name, no per-plugin
      special-casing).
- [x] **Gate green:** `make fmt && make lint && make test` (Go `-race`, 0 lint
      issues both modules, 398 vitest + 18 e2e).
- [x] **Write _and execute_** integration tests: a real example plugin binary is
      loaded and exercised for unary, bidi stream, `OpenChannel`, open-in-browser,
      direct egress, L7-through-core, audit forwarding, and crash → restart under
      backoff (`internal/extplugin/*_test.go`); plus the availability enforcement
      tests above. _(A live agent tunnel shares the identical `cfg.Net` path but
      isn't stood up in unit tests.)_
- [ ] Update `specs/project.md` with the external-plugin architecture once stable.

## Pre-build confirmations — §1, §3, §6

- [x] Scope: keep the 40 first-party plugins compiled in; external = out-of-process
      only. Pure OSS extensibility (no commercial/licensing, not for size reduction).
- [x] **Full capability parity, all in v1** — no v2, nothing first-party-only.
- [x] Mechanism: `hashicorp/go-plugin`, **gRPC only**, `AutoMTLS`, `GRPCBroker`.
- [x] Packaging: **nested SDK module** `github.com/charlesng35/shellcn/sdk` (in this
      repo, NOT under `internal/`), holding the public contract (`sdk/plugin`) +
      serve glue; core + 40 built-ins import the contract from there; tagging
      `sdk/vX.Y.Z` pending the first release.
- [x] Egress + audit owned by the core via the `Host` service.
- [x] Accept the maintenance tax: a stable, full-surface plugin ABI owned indefinitely.
- [x] Accept the trade: installing external plugins means core + a `plugins.d/` of
      subprocesses (first-party single-binary experience unchanged).
- [x] Cheaper alternative on record: bring-your-own-build (add import to `all()`,
      `go build`) if runtime-loading demand proves small.
