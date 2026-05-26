# ShellCN — Implementation Plans

Step-by-step build plan. Architecture lives in [`../v2.md`](../v2.md); this folder
is the execution tracker. Work proceeds **UI-first**: Phase 1 (M0) proves the
declarative renderer on fixtures, Phase 2 (M1) brings up the real core, Phase 3+
add plugins.

## How to use this tracker

- Each **phase** is a folder; each **step** is a numbered `.md` with its own
  sub-task checklist and a **Definition of Done**.
- When a step's DoD is met: tick its box below, set the step file's **Status** to
  `✅ Done` (add date + PR), and tick its internal sub-tasks.
- A phase is **done** when all its steps are `✅`.

## Cross-cutting (applies to every step)

- **Tests are a gate** — no step is `✅` without its tests; no phase is done
  without its e2e. See [`TESTING.md`](TESTING.md) for the layers and CI gates.
- **Icons are structured** — every icon field uses the `Icon{ Type, Value }` type
  (`name`/`url`/`base64`/`emoji`/`svg`; inline `svg` is DOMPurify-sanitized), v2 §5.1.
- **Lazy-load by default** — code-split heavy panels, fetch projections/data on
  demand, connect sessions lazily (v2 §12.1). First paint stays constant.
- **Panel set grows in core** — specialized panels such as `graph`/`trace`/`kv`/
  `http_client` are lazy-loaded core renderers (v2 §6.2); plugins still ship
  only manifests, route handlers, and generic payloads.

## Status legend

`☐` not started · `🚧` in progress · `✅` done

## Milestone mapping

| Phase | Milestone (v2 §15) | Theme                                 |
| ----- | ------------------ | ------------------------------------- |
| 0     | —                  | Bootstrap                             |
| 1     | M0                 | Declarative UI on fixtures (priority) |
| 2     | M1                 | Core runtime                          |
| 2b    | M1.5               | Platform management (auth UI, connection/credential CRUD, sharing) — v2 §12.2 |
| 2c    | M1.6               | Session recording foundation (plugin-declared, opt-in) — v2 §9.5 |
| 2d    | M-Admin            | Administration foundation (config, users, invitations) — v2 §12.2 |
| 3     | M2                 | SSH/SFTP reference plugin             |
| 4     | M3                 | Docker + agent transport validation/hardening (L4) |
| 5     | M4                 | Proxmox (VNC)                         |
| 6     | M5                 | PostgreSQL                            |
| 7     | M6                 | Kubernetes (L7 agent)                 |

## Progress

Tracked in the repo-root **[`checklist.md`](../../checklist.md)** (the single
living tracker — update it after every completed step). Per-step detail lives in
the phase folders below; each step file also carries its own `Status` line.
