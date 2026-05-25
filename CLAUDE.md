# CLAUDE.md

ShellCN — a single-binary **Go + embedded-Vue infrastructure access gateway**.
Every protocol is a first-party Go plugin that declares a versioned manifest; the
frontend renders any plugin **generically** (zero per-plugin UI).

**@AGENTS.md is the full working agreement** (project, workflow, architecture
invariants, code style, verification rules) and applies here in full. Read it.

## Non-negotiables (full detail in AGENTS.md)

- **Verify libraries, APIs, and practices via `context7` + web search before
  using them — never from memory.** Prefer existing maintained packages over
  building from scratch.
- **Minimal comments; small, focused components** (no god-components). DX matters.
- Work the plan in [`specs/plans/`](specs/plans/) **in phase order** (UI-first).
  Architecture lives in [`specs/v2.md`](specs/v2.md).
- After each step: **update [`checklist.md`](checklist.md)** and set the step
  file's `Status`. **Tests gate every step** ([`specs/plans/TESTING.md`](specs/plans/TESTING.md)).
- Don't violate the architecture invariants in AGENTS.md (stateless plugins,
  manifest-driven frontend, secrets encrypted above the store, routes carry
  permission/risk/audit, lazy-load).
