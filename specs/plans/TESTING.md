# Testing standard

**Tests are a release gate.** A step is not `✅` until its tests are written,
green, and meaningful — a step's **Definition of done** always implies its tests.
A milestone (phase) cannot be marked done until its end-to-end tests pass.

## Layers

- **Go unit** — table-driven, fast, no network/DB. In-memory store fakes (§14.1).
  Cover: manifest validation, browser projection, schema/`Condition` evaluation,
  secret encrypt/redact, policy + risk matrix, transport dialers, route wrapper,
  pagination/cursor logic.
- **Plugin tests** — every plugin package ships `*_test.go` using the
  `plugintest` harness (fake `RequestContext` / `Session` / `NetTransport`), so
  routes + manifest are exercised **with no real infrastructure**.
- **Contract / golden** — a golden test asserting the Go manifest **projection**
  equals the TypeScript `projection.ts` shape, so the FE/BE contract can't drift.
  Goldens are updated deliberately, reviewed in the diff.
- **Cross-DB integration (store)** — the store suite runs against **SQLite +
  Postgres + MySQL** via testcontainers (§11.1). **SQLite is the per-PR gate**
  (fast, embedded); **Postgres + MySQL run nightly** (and as an M1 hardening
  task) so the matrix doesn't slow early iteration.
- **Integration (plugin ↔ real target)** — testcontainers/dockertest for SSH,
  Docker, Postgres; recorded fixtures where a live target is impractical
  (Proxmox, cloud APIs).
- **Frontend unit/component** — Vitest + Vue Test Utils: panels, `SchemaForm`
  conditions/validators, `DataSource` resolver, **icon rendering**
  (name/url/base64/emoji + fallback), lazy panel loading.
- **Frontend e2e** — Playwright drives the fixture-backed app (M0) and later the
  real binary: golden path + key edge cases per panel. **Streaming panels
  (terminal/VNC/logs/query) are validated by real e2e at their plugin milestone —
  a mock WebSocket is never acceptance.**
- **Security** — authz deny-by-default; WS ticket validity/expiry/single-use;
  secrets never serialized/logged; SSRF/egress allow-deny; destructive-action
  gating; unused enrollment-token expiry and enrolled-agent reconnect/revoke.
- **Race/leak** — `go test -race`; sessions/channels close cleanly (no goroutine
  leaks); idle timeout reclaims.

## CI gates (every PR)

- `build` + `lint` (golangci-lint, gofumpt) + `go test -race ./...`
- Cross-DB store matrix (SQLite/Postgres/MySQL)
- Frontend unit + e2e (fixtures)
- Coverage floor on core packages (`plugin`, `store`, `auth`, `policy`,
  `secrets`, `transport`) — target **≥ 80%**; plugins cover their route handlers.

## Per-step rule

Each step file's checklist **includes its tests**, and the step's status flips to
`✅` only when they pass. When a step touches a streaming panel, its real e2e at
the relevant milestone is the acceptance — not the M0 stub.
