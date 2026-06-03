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

- [x] `proto/shellcn/plugin/v1/plugin.proto` with **both** services: `Plugin`
      (served by plugin) and `Host` (served by core: `DialTarget`/
      `HTTPProxyEndpoint`/`Audit`). Self-contained (local `Empty`, no
      well-known-type imports → fully reproducible offline).
- [x] `Plugin` service: `GetManifest`, `Connect`, `HealthCheck`, `Close`,
      `Invoke`, `InvokeServerStream`, `OpenStream`, `OpenChannel`, `ServeHTTPProxy`.
      Byte-streams ride raw brokered conns named by `BrokerRef.broker_id`.
- [x] **buf** generation: `buf.yaml` (BASIC lint) + `buf.gen.yaml` (managed mode,
      `go_package_prefix`) → stubs at `sdk/gen/shellcn/plugin/v1` (package
      `pluginv1`), checked in. `make proto` + `make tools` (buf, protoc-gen-go*).
- [x] Handshake (`sdk/grpcplugin`): `Handshake` (magic cookie) + `ProtocolVersion`
      + `PluginName` dispense key.
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
