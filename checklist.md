# ShellCN — Progress Checklist

**Living progress tracker — update after every completed step.** This is the
single source of truth for "where are we." Detailed steps (sub-task checklists +
Definitions of Done) live in [`specs/plans/`](specs/plans/); architecture in
[`specs/v2.md`](specs/v2.md); test standard in
[`specs/plans/TESTING.md`](specs/plans/TESTING.md).

_Last updated: 2026-05-25 — Phase 2 (M1) complete after audit: core runtime (plugin contract + registry, manifest validator + projection, GORM store in `internal/models`, AES-GCM secret vault, local auth + sessions + WS tickets, permission+risk Casbin authz with additive stored policies, session/channel/transport runtime, chi server + route wrapper, declared input-schema validation, multipart route binding, denied-route audit, audit + telemetry with secret-access and plugin-health wiring) proven end-to-end by the `noop` plugin. Entity package renamed `domain`→`models` (structs double as GORM models); added `svg` IconType (FE+BE). Phase 3 (M2 SSH/SFTP) next; `cmd/agent` remains Phase 4 scope._

Legend: `[ ]` todo · `[~]` in progress · `[x]` done.
A step is `[x]` only when its **tests pass**; a phase is done when all its steps are `[x]`.

## Phase 0 — Bootstrap

- [x] 0.1 Initialize Go module and repo skeleton
- [x] 0.2 Scaffold the Vue + Vite frontend
- [x] 0.3 Makefile and developer tooling

## Phase 1 — M0 · Declarative UI on fixtures (priority)

- [x] 1.1 Define the projection contract (TypeScript)
- [x] 1.2 Author fixture manifests and mock dev server
- [x] 1.3 App shell, stores, and routing
- [x] 1.4 Manifest renderer and panel dispatch
- [x] 1.5 DataSource resolver
- [x] 1.6 Declarative panels
- [x] 1.7 Stub streaming panels

## Phase 2 — M1 · Core runtime

- [x] 2.1 Package skeleton and plugin contract types
- [x] 2.2 Manifest validator and browser projection
- [x] 2.3 GORM models and store repositories
- [x] 2.4 Secret vault
- [x] 2.5 Authentication and sessions
- [x] 2.6 Authorization with Casbin
- [x] 2.7 Session, channel, and transport runtime
- [x] 2.8 chi server and route wrapper
- [x] 2.9 Audit and telemetry
- [x] 2.10 Noop plugin and end-to-end validation

## Phase 3 — M2 · SSH/SFTP reference plugin

- [ ] 3.1 SSH session and Connect
- [ ] 3.2 SSH routes and manifest
- [ ] 3.3 Wire the real terminal panel
- [ ] 3.4 Wire the real file browser panel

## Phase 4 — M3 · Docker + agent transport

- [ ] 4.1 Docker session and resource routes
- [ ] 4.2 Docker manifest (tree, resources, actions)
- [ ] 4.3 Real logs, exec, and watch streams
- [ ] 4.4 shellcn-agent binary (L4 tcp/unix)
- [ ] 4.5 Agent enrollment flow and tunnel registry
- [ ] 4.6 Wire agent transport into Docker connection

## Phase 5 — M4 · Proxmox

- [ ] 5.1 Proxmox session and API client
- [ ] 5.2 Proxmox manifest (nodes, VMs, LXC, storage)
- [ ] 5.3 Real noVNC remote-desktop panel
- [ ] 5.4 Snapshots, backups, and lifecycle actions

## Phase 6 — M5 · PostgreSQL

- [ ] 6.1 PostgreSQL session and schema browser
- [ ] 6.2 Real query editor and results panel
- [ ] 6.3 Database safety controls

## Phase 7 — M6 · Kubernetes

- [ ] 7.1 Kubernetes session and L7 agent mode
- [ ] 7.2 Workloads and core resource trees
- [ ] 7.3 Pod logs, exec, and port-forward
- [ ] 7.4 YAML editor and events

---

**On completing a step:** mark it `[x]` here, update the `_Last updated_` line,
set `Status: ✅ Done` in the step file (add date/PR), and confirm its tests pass.
