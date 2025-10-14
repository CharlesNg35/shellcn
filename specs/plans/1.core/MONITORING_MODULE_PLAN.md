# Monitoring Module - Implementation Plan

**Module:** Monitoring (Metrics, Health)  
**Status:** In Progress  
**Dependencies:** Core backend, Realtime hub, Vault service, Dashboard layout

---

## Table of Contents

1. [Overview](#overview)
2. [Current State Assessment](#current-state-assessment)
3. [Requirements](#requirements)
4. [Backend Implementation Plan](#backend-implementation-plan)
   1. [Package Structure](#package-structure)
   2. [Metrics Registry & Wiring](#metrics-registry--wiring)
   3. [Instrumentation Coverage](#instrumentation-coverage)
   4. [Health & Readiness Checks](#health--readiness-checks)
   5. [API & Middleware Integration](#api--middleware-integration)
   6. [Configuration & Operations](#configuration--operations)
   7. [Testing Strategy](#testing-strategy)
5. [Frontend Implementation Plan](#frontend-implementation-plan)
6. [Documentation & DX Updates](#documentation--dx-updates)
7. [Implementation Checklist](#implementation-checklist)

---

## Overview

The Monitoring module provides Prometheus metrics, health/readiness endpoints, and an admin-facing dashboard so operators can observe system health. The backend must expose rich telemetry that aligns with our architecture specs (`specs/project/MODULE_IMPLEMENTATION.md#13-monitoring-module`), while the frontend visualizes key time-series and status indicators for administrators.

Primary goals:

- Centralize Prometheus registration and expose `/metrics` with configurable gating.
- Offer `/health`, `/health/live`, and `/health/ready` endpoints backed by dependency checks.
- Instrument critical services (auth, realtime, vault, connections, background jobs) with counters, gauges, and histograms.
- Surface metric summaries and health statuses in the admin UI.

---

## Current State Assessment

- **Metrics package:** `pkg/metrics/metrics.go#L1` defines counters/histograms (auth attempts, permission checks, session gauge, API latency, vault metrics). These are registered via `promauto` at package init and used in `internal/handlers/auth.go`, `internal/auth/session_service.go`, `internal/middleware/metrics.go`, and `internal/services/vault_service.go`. There is no module-level coordination or custom registry.
- **Metrics endpoint:** `internal/api/router.go:48` installs `middleware.Metrics()` globally, and `internal/api/router.go:296` exposes `GET /metrics` unconditionally using `promhttp.Handler()`. Config toggles under `monitoring.prometheus` are defined (`internal/app/config.go:79`) but unused.
- **Health endpoint:** `internal/handlers/health.go` returns basic database status at `/health`; there are no dedicated liveness/readiness routes or dependency checks beyond SQL ping.
- **Realtime telemetry:** `internal/realtime/hub.go` manages WebSocket sessions but does not emit metrics (e.g., connection counts, broadcast errors).
- **Background jobs:** `internal/app/maintenance.Cleaner` tasks have no metrics or heartbeat exposure.
- **Frontend:** No monitoring page exists (`web/src/pages` lacks monitoring), and navigation does not route to a monitoring view (`web/src/App.tsx`).
- **Tests:** Router tests cover `/metrics` happy path (`internal/api/router_test.go:82`) but do not assert config gating or health failure behaviour.

---

## Requirements

1. **Prometheus Integration**

   - Provide a dedicated registry with module-scoped collectors.
   - Allow optional namespacing/prefix configuration if needed.
   - Support runtime enabling/disabling of `/metrics`.

2. **Health & Readiness**

   - Return structured JSON for `/health` (basic), `/health/live` (liveness), and `/health/ready` (readiness).
   - Aggregate checks for database, Redis (if enabled), background jobs, realtime hub, and protocol drivers that expose `HealthCheck`.
   - Ensure non-ready responses surface failing component IDs and human-readable messages.

3. **Instrumentation**

   - Extend current counters/gauges (auth, permissions, sessions, vault).
   - Add realtime metrics: active websocket connections, broadcasts per stream, send failures.
   - Add job metrics: maintenance run duration/status, last success timestamp.
   - Track connection launch metrics (success/failure per protocol).
   - Record session durations and active connection counts.

4. **Frontend Dashboard**

   - Provide admin page to display health states, key counters, recent failures, and Prometheus scrape status.
   - Restrict visibility to admins (reuse permission guard).
   - Use React Query to poll health endpoints and optional lightweight metrics summaries.

5. **Config & Operations**
   - Honour `monitoring.prometheus.enabled` and `monitoring.prometheus.endpoint`.
   - Honour `monitoring.health_check.enabled` for health endpoints, with optional per-endpoint toggles if required.
   - Document scrape configuration and sample alerts.

---

## Backend Implementation Plan

### Package Structure

Create `internal/monitoring/` with:

```
internal/monitoring/
├── registry.go        # Registry builder + collector wiring
├── metrics.go         # Metric definitions and helpers
├── instrumentation.go # Recording helpers for services
├── health.go          # Aggregated liveness/readiness checks
├── checks/            # Individual dependency checkers
│   ├── database.go
│   ├── redis.go
│   ├── realtime.go
│   └── maintenance.go
└── middleware.go      # (Optional) gin middleware wrappers
```

- Move existing metric vectors from `pkg/metrics` into `internal/monitoring/metrics.go` keeping names consistent; export interfaces so current callers can migrate incrementally.
- Provide a transitional shim in `pkg/metrics` that proxies to the new package (soft deprecate) to minimize refactor risk if needed.

### Metrics Registry & Wiring

1. Build a `Registry` struct wrapping `*prometheus.Registry` with methods:
   - `New()` to construct registry, register default Go collectors (process, go runtime).
   - `Register(collector prometheus.Collector)` for custom collectors.
   - `Handler()` returning an `http.Handler` for the configured registry.
2. Support optional push of default metrics:
   - Auth attempts, permission checks, sessions, API latency, vault operations, realtime stats, maintenance jobs, connection launches.
   - Provide helper functions (e.g., `monitoring.RecordAuthAttempt(result string)`).
3. Update `cmd/server/bootstrap.go` to instantiate the registry and pass through to router builder.
4. Update `internal/api/router.go` to conditionally mount `/metrics`:
   - Use `cfg.Monitoring.Prometheus.Enabled` and `cfg.Monitoring.Prometheus.Endpoint`.
   - Inject handler from registry rather than `promhttp.Handler()` global default.

### Instrumentation Coverage

1. **HTTP Layer:** Keep `middleware.Metrics()` but change it to depend on the monitoring registry (inject histogram rather than global var). Option: store histogram in monitoring package and expose `MonitorHTTP()` middleware builder that closes over registry metrics.
2. **Auth & Permissions:** Update references in `internal/handlers/auth.go`, `internal/middleware/permission.go`, `internal/auth/session_service.go` to use new helpers.
3. **Vault Service:** Replace direct imports of `pkg/metrics` with `internal/monitoring`.
4. **Connections:**
   - Instrument `internal/services/protocol_service.go` and connection handlers to record launch successes/failures per protocol.
   - Emit gauges for active connections using realtime session tracking.
5. **Realtime Hub:** Add counters/gauges inside `internal/realtime/hub.go` for:
   - Active connections (gauge).
   - Subscribe/unsubscribe totals per stream.
   - Broadcast queue drops/errors.
6. **Maintenance Jobs:** Wrap `maintenance.Cleaner.RunOnce` / `Start` with timing metrics and failure counters.
7. **Session Duration:** When sessions end (`internal/auth/session_service.go:336` etc.), record observe durations via histogram.
8. Provide integration helpers for new services (e.g., `monitoring.ObserveDuration(metric *prometheus.HistogramVec, labels ...)`).

### Health & Readiness Checks

1. Implement `Checker` interface in `internal/monitoring/health.go`:

```go
type ProbeResult struct {
    Component string        `json:"component"`
    Status    string        `json:"status"` // up|down|degraded
    Details   string        `json:"details,omitempty"`
    Duration  time.Duration `json:"duration"`
}

type Check func(ctx context.Context) ProbeResult
```

2. Provide `Manager` with registries for liveness and readiness checks and aggregated evaluation.
3. Implement default checks:
   - Database ping (`checks/database.go`) using `PingContext`.
   - Redis ping if enabled.
   - Background worker heartbeat (Cleaner last run timestamp).
   - Realtime hub connectivity (e.g., active goroutines or queue depth).
   - Protocol drivers health using `drivers.Driver.HealthCheck`.
4. Update `registerHealthRoutes` to mount:
   - `/health` (basic alias to readiness with reduced payload).
   - `/health/live` (includes liveness checks).
   - `/health/ready` (comprehensive readiness).
   - Respect `cfg.Monitoring.Health.Enabled`; if disabled, respond 404 or skip route.
5. Ensure responses follow consistent schema with `success`, `status`, `checks` arrays, and HTTP status codes (`200` vs `503`).

### API & Middleware Integration

1. Update `internal/api/router.go` to accept monitoring registry/health manager via constructor parameters (break glass by injecting from bootstrap).
2. Move current `registerHealthRoutes` logic to use new manager and config gating.
3. Update tests (`internal/api/router_test.go`) to cover:
   - Metrics route disabled -> 404.
   - Custom endpoint path.
   - Health readiness failure when DB ping errors.
4. Update `cmd/server/bootstrap.go` to wire:
   - Monitoring registry creation.
   - Health manager registration with DB, Redis, Cleaner, Protocol services.
   - Provide instrumentation hooks to services (e.g., pass monitoring collectors when constructing realtime hub or session service).

### Configuration & Operations

1. Extend `internal/app/config.go` structures if additional knobs needed (e.g., `monitoring.health_check.endpoints.ready_path`).
2. Update sample config (`internal/app/testdata/config.yaml`, `.env.example` if present, docs) to reflect toggles.
3. Fail fast in bootstrap when metrics endpoint path collides with API routes.
4. Provide environment variables for enabling/disabling.
5. Document recommended scrape interval, example `prometheus.yml`, and alert suggestions in `/docs`.

### Testing Strategy

- **Unit Tests:**
  - Registry creation and collector registration.
  - Health check functions with simulated failures (DB error, Redis timeout, stale maintenance timestamp).
  - Monitoring middleware capturing path templating.
- **Integration Tests:**
  - Extend `internal/api/router_test.go` to assert metrics output contains new series.
  - Add tests for `/health/live` & `/health/ready` statuses using Gin testserver.
- **Concurrency/Load Testing (Optional):**
  - Add benchmark or stress tests for realtime metrics (ensure thread safety).

---

## Frontend Implementation Plan

1. **Navigation & Permissions**

   - Embed monitoring UI as a new tab within `web/src/pages/settings/Security.tsx`, mirroring the pattern used in `web/src/pages/settings/Users.tsx`.
   - Ensure tab visibility is limited to administrators via `PermissionGuard` (e.g., inherit existing security page permissions).
   - Update any tab configuration/constants so the Security page recognizes the new Monitoring tab and default tab selection logic remains intact.

2. **API Client**

   - Create `web/src/lib/api/monitoring.ts` with:
     - `fetchHealth()` -> GET `/health/ready`.
     - `fetchLiveness()` -> GET `/health/live`.
     - Optional: `fetchMetricsSummary()` that hits a backend summary endpoint (if built) or parses Prometheus metrics for highlighted values.
   - Use `web/src/hooks` to expose `useHealthStatus` + `useLivenessStatus` (React Query, polling interval 15-30s).

3. **UI Components**

   - Implement `web/src/pages/settings/Monitoring.tsx` showing:
     - Health status cards (overall, DB, Redis, realtime, maintenance).
     - Active session counts & connection metrics (via summary endpoint or derived store).
     - Chart placeholders that link to Prometheus/Grafana (for now simple sparklines or numeric values).
   - Reuse existing `StatusBadge`, `Card`, `Table` components for consistency.

4. **Error Handling & Loading States**

   - Display fallback messaging if monitoring endpoints disabled (HTTP 404 -> "Monitoring disabled").
   - Provide CTA to docs for enabling metrics.

5. **Testing**
   - Add unit tests for hooks (mock API responses) using Vitest.
   - Add React Testing Library test ensuring page renders health sections and handles degraded statuses.

---

## Documentation & DX Updates

- Update `docs/dockers.md` monitoring section with new endpoints and environment variables.
- Add operations guide for configuring Prometheus/Grafana dashboards (`docs/operations/monitoring.md`).
- Update README or admin guide to mention `/health/live` and `/health/ready`.
- Provide sample alert rules for high auth failures, low readiness, or high latency.
- Document metrics naming conventions and label cardinality guidelines for contributors.

---

## Implementation Checklist

- [x] Create `internal/monitoring` package with registry, metrics, instrumentation helpers.
- [x] Migrate existing metric definitions from `pkg/metrics` and update call sites.
- [x] Inject monitoring registry into router; gate `/metrics` route by config.
- [x] Implement health manager with liveness/readiness endpoints and dependency checks.
- [x] Instrument realtime hub, protocol services, maintenance cleaner, and session lifecycle.
- [x] Add backend tests covering metrics endpoint toggles and health failures.
- [x] Add frontend monitoring page, API clients, hooks, and tests.
- [ ] Update documentation and sample configuration for monitoring features.
- [ ] Provide migration notes (if any) for existing deployments.
