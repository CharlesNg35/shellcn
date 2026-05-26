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
- **Frontend: always prefer an existing PrimeVue component — for _every_ UI
  element, not just buttons.** Build UI with **PrimeVue** (unstyled + Tailwind
  pass-through preset) and **VueUse**. Before writing markup, check whether
  PrimeVue already has the component (tables → `DataTable`/`Column`, trees →
  `Tree`, tabs → `Tabs`, modals → `Dialog`, toasts → `Toast`/`useToast`, menus →
  `Menu`, overlays → `Popover`/`Drawer`, forms → `InputText`/`Select`/`Checkbox`/
  …, every clickable control → `Button`) and use it, styled via the preset.
  **Never hand-roll what a committed component covers** (no bespoke native
  `<button>`/`<input>`/`<select>`/menu/modal with ad-hoc Tailwind). Build your
  own **only when PrimeVue has no equivalent** — then justify it. **Always
  confirm the component exists and its current props/slots/events via `context7`
  + web search before wiring it — never from memory** (PrimeVue APIs change).
- **UX is first-class:** accessible (WAI-ARIA, keyboard, focus-visible),
  skeleton loading states, clear empty/error states, action feedback via toasts,
  motion that respects `prefers-reduced-motion`, dark/light theming. Keep UX in
  the generic renderer/panels — never per-plugin. (Detail in AGENTS.md.)
- **Minimal comments; small, focused components** (no god-components). DX matters.
  Comment only a non-obvious _why_. **Never** put spec/section references
  (e.g. `(v2 §14)`), task/PR references, or narration of _what_ the code does in
  source files — that metadata rots and belongs in the PR/docs, not the code.
- Work the plan in [`specs/plans/`](specs/plans/) **in phase order** (UI-first).
  Architecture lives in [`specs/v2.md`](specs/v2.md).
- After each step: **update [`checklist.md`](checklist.md)** and set the step
  file's `Status`. **Tests gate every step** ([`specs/plans/TESTING.md`](specs/plans/TESTING.md)).
- **After implementing anything, always run `make fmt`, then `make lint` and
  `make test` — all must pass before finishing.** Don't hand off unformatted code
  or failing tests/lint.
- Don't violate the architecture invariants in AGENTS.md (stateless plugins,
  manifest-driven frontend, secrets encrypted above the store, routes carry
  permission/risk/audit, lazy-load).
