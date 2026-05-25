# ShellCN — Progress Checklist

**Living progress tracker — update after every completed step.** This is the
single source of truth for "where are we." Detailed steps (sub-task checklists +
Definitions of Done) live in [`specs/plans/`](specs/plans/); architecture in
[`specs/v2.md`](specs/v2.md); test standard in
[`specs/plans/TESTING.md`](specs/plans/TESTING.md).

_Last updated: 2026-05-25 — Phase 3 (M2 SSH/SFTP) complete. Core runtime and platform management remain complete; the shipped placeholder `noop` plugin has been removed. SSH/SFTP are now real first-party plugins: `ssh` provides terminal + SFTP files + tunnels + snippets, `sftp` provides file-only access, and both share the same SSH/SFTP session + route implementation. **Next: Phase 4 (M3 Docker + agent transport).**_

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

## Phase 2b — M1.5 · Platform management (make it usable)

_Done — control-plane CRUD + platform UI (spec [v2 §12.2](specs/v2.md), steps [phase-2b](specs/plans/phase-2b-m1.5-platform-management/)). Connection/credential CRUD + sharing endpoints with authn→authz→audit; auth gate + global error UX; manifest-driven connection create/edit/delete; credential management + sharing UI. All secrets write-only end to end._

- [x] 2b.1 Backend — connection CRUD endpoints (schema-validated, secret-encrypted, authz'd)
- [x] 2b.2 Backend — credential CRUD + rotation (write-only secret material)
- [x] 2b.3 Backend — sharing grants endpoints (connection + credential; use/manage)
- [x] 2b.4 Frontend — auth/session gate + global error/authz UX (login, CSRF, 401→login, logout)
- [x] 2b.5 Frontend — connection management UI (manifest-driven create/edit/delete + transport selector)
- [x] 2b.6 Frontend — credential management + sharing UI (create/rotate/delete, grant use/manage)

## Phase 2c — M1.6 · Session recording foundation

_Done — recording is a generic, plugin-declared, off-by-default platform capability (spec [v2 §9.5](specs/v2.md), steps [phase-2c](specs/plans/phase-2c-m1.6-session-recording/)). Plugins declare recordable stream classes (`terminal`/`desktop`) + formats via `RecordingCapability`; connections carry a per-class policy (`disabled`/`manual`/`auto`=forced). The core stream wrapper taps recordable WS streams (forced denies the stream up front if it can't start; manual start/stops; bounded buffering never blocks the live stream). Terminal → asciicast v2; desktop → browser `webm_canvas` chunk uploads (non-authoritative). Metadata in a new `Recording` model + `RecordingStore`; bytes in a replaceable `BlobStore` (local FS default). Role-aware list/get/content/delete APIs (admin all + per-user drill-down; non-admins only their own recordings), retention OFF by default (`config.recordings`), cleanup job when enabled. Frontend: Recordings view + asciinema/WebM players, per-panel REC state + manual start/stop, connection create/edit policy options only when the plugin declares support._

- [x] 2c.1 Recording manifest contract + connection policy
- [x] 2c.2 Recording storage, metadata, retention, and authorization
- [x] 2c.3 Core stream recording wrapper and lifecycle
- [x] 2c.4 Terminal asciicast recorder and playback
- [x] 2c.5 Desktop/graphical recording framework
- [x] 2c.6 Recording APIs and frontend management UI

## Phase 2d — M-Admin · Administration foundation

_Done — user/role management + invitations + the config foundation they need
(spec [v2 §12.2](specs/v2.md), [v2 §9.1](specs/v2.md), steps
[phase-2d](specs/plans/phase-2d-m-admin/)). SMTP is bootstrap config
(`config.email.*`), not a stored table; invitations always yield a copyable
link, with email as a best-effort extra when SMTP is enabled._

- [x] 2d.1 Backend — typed bootstrap config (`internal/config`, Viper: `config.yaml` + `SHELLCN_*` env + flag overrides; master key unified)
- [x] 2d.2 Backend — admin user CRUD (`/api/admin/users`) with root-admin protection (root never deleted/locked out; only root deletes admins); audited
- [x] 2d.3 Backend — invitations create/list/revoke (`/api/admin/invitations`) + public lookup/accept (`/api/invitations/{token}`, single-use); config-driven SMTP via `internal/email`
- [x] 2d.4 Frontend — Users view (Users · Invitations tabs): create/edit/delete users, invite → copyable link, revoke, public accept page, admin-only nav + email status in Settings

> **Still M-Admin (later):** policy-rule admin (`role+permission+risk`), audit-log view + per-connection activity, light status page (health/plugin-health/session counts), agent re-enroll/rotate + history.

## Phase 3 — M2 · SSH/SFTP reference plugin

_Done — SSH and SFTP are separate compiled-in plugins with shared SSH/SFTP session and file-route code. `ssh` exposes Terminal, Files, Tunnels, and Snippets; `sftp` exposes the same generic file browser only. SSH/SFTP auth supports password, private key, and stored credential without extra trust or SSH-agent configuration. SFTP opens lazily over the same SSH client, guarded by the session mutex. Terminal streaming is real xterm.js ↔ `ssh.shell` with resize control frames; file browser routes implement list/read/download/upload/mkdir/rename/delete with core-streamed downloads and audit/authz wrapper coverage. The shipped placeholder `noop` plugin was removed; server e2e now uses an internal test-only plugin._

- [x] 3.1 SSH session and Connect
- [x] 3.2 SSH routes and manifest
- [x] 3.3 Wire the real terminal panel
- [x] 3.4 Wire the real file browser panel

## Phase 4 — M3 · Docker + agent transport

- [ ] 4.1 Docker session and resource routes
- [ ] 4.2 Docker manifest (tree, resources, actions)
- [ ] 4.3 Real logs, exec, and watch streams
- [ ] 4.4 Harden shellcn-agent L4 tcp/unix against Docker
- [ ] 4.5 Harden enrollment/tunnel registry with Docker agent mode
- [ ] 4.6 Wire and prove agent transport in Docker connection

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
