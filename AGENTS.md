# AGENTS.md — Working agreement for ShellCN

## Project

ShellCN is a self-hosted **infrastructure access gateway / operations cockpit**: a
single Go binary with an embedded Vue 3 frontend that brokers secure, audited
access to SSH, SFTP, Docker, Kubernetes, Proxmox, databases, remote desktops, and
more. Every protocol is a **first-party, compiled-in Go plugin** that declares a
**versioned manifest** (config schema, layout, resources, actions, streams,
routes). The **core** owns rendering, security, sessions, transport, and audit.
The **frontend is a universal renderer** driven entirely by the manifest
projection — **adding a plugin requires zero frontend changes.**

**Authoritative docs (read before coding):**

- [`specs/v2.md`](specs/v2.md) — architecture (source of truth).
- [`specs/plans/`](specs/plans/) — phased, numbered build steps (each with a
  sub-task checklist + Definition of Done).
- [`specs/plans/TESTING.md`](specs/plans/TESTING.md) — testing standard.
- [`specs/plugins.md`](specs/plugins.md) — plugin roadmap.
- [`checklist.md`](checklist.md) — **living progress tracker.**

## How to work here

1. Read `specs/v2.md` (relevant section) + the current phase's step files first.
2. Follow the **phase order** (UI-first: M0 declarative UI on fixtures → M1 core →
   M2 SSH → …). Don't jump ahead.
3. After finishing a step: tick its sub-tasks, set the step file's
   **`Status: ✅ Done`**, and **update `checklist.md`**. Keep them in sync, always.
4. A step is done only when its **tests pass**; a phase only when its e2e is green.

## Verify before you build (IMPORTANT)

- Before using **any** library, framework, API, or "best practice," **verify it
  with `context7` (library docs) and web search.** Use current/latest docs and
  practices — do **not** rely on training memory for library APIs; they change.
- Prefer **existing, maintained packages** (npm / Go) over building from scratch.
  Check the package's current docs via context7 first.

## Code style

- **Minimal comments.** Write self-documenting code; comment only a non-obvious
  _why_ (a constraint, an invariant, a workaround). No verbose or obvious
  comments, no narrating what the code does.
- **Small, focused units.** No god-components, no mixing concerns in one file —
  split into small components + composables (frontend) and small packages +
  functions (Go). DX matters.
- **Reuse over reinvention** (see above).
- **Latest practices, verified** (see above).

## Architecture invariants (don't violate)

- Plugins are **stateless singletons**; all per-connection state lives in the
  `Session` (mutex-guard lazily-opened sub-clients).
- Plugins ship **manifest + route handlers only** — never UI, HTTP plumbing,
  auth, or storage.
- The **frontend never special-cases a plugin** — it renders whatever the
  manifest projection declares (panels, tabs, tree, actions).
- **Secrets** are encrypted above the store (store sees ciphertext); never
  returned to the client or logged.
- Every route carries **permission + risk + audit**; the core wrapper enforces
  authn → authz → validate → audit → handler.
- **Lazy-load** heavy panels/data; first paint stays constant regardless of catalog size.

## Commands

- `make build` — vite build → embed → `go build` (single binary)
- `make dev-web` / `make dev-api` — Vite HMR / Go server (`--dev`, proxies `/api`)
- `make test` · `make lint` · `make gen` (ent / codegen) · `make migrate`

## Stack

**Backend:** Go — chi (router), ent (cross-DB store), Casbin (RBAC), pure-Go
drivers (modernc sqlite / pgx / mysql), coder/websocket.
**Frontend:** Vue 3 + Vite + TypeScript + Pinia + PrimeVue (unstyled) + Tailwind +
xterm.js + noVNC + Monaco.
