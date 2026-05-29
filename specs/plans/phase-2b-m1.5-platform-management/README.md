# Phase 2b — M1.5 · Platform management

**Status:** ✅ Done · **Milestone:** M1.5 · **Depends on:** Phase 2 (M1)
**Spec ref:** [v2 §12.2](../../project.md)

M0 built the manifest-driven renderer; M1 built the core runtime + plugin route
dispatch. **M1.5 makes the platform usable end-to-end without fixtures:** a user
can sign in, create/edit/share connections and reusable credentials through the
UI, and reach them. It adds the **control-plane CRUD endpoints** these need (only
list/read exists today) and the **platform UI** (auth gate, connection form,
credential management, sharing) — all consistent with the architecture:

- The "add/edit connection" form is **manifest-driven** — pick a protocol, render
  its projected `config` schema with the existing generic `SchemaForm`, submit.
  No per-plugin management code (v2 §12.2).
- Secrets stay **write-only**: edit forms show `set`/`not set` + "replace", never
  the value (v2 §9.3).
- Every new endpoint keeps the **authn → authz → audit** guarantees (v2 §4, §9).

## Steps

- [x] 2b.1 Backend — connection CRUD endpoints (create/update/delete, schema-validated, secret-encrypted, authz'd)
- [x] 2b.2 Backend — credential CRUD + rotation endpoints (write-only secret material)
- [x] 2b.3 Backend — sharing grants endpoints (connection + credential; use/manage)
- [x] 2b.4 Frontend — auth/session gate + global error/authz UX (login, logout, CSRF, 401→login)
- [x] 2b.5 Frontend — connection management UI (manifest-driven create/edit/delete + transport selector)
- [x] 2b.6 Frontend — credential management + sharing UI (create/rotate/delete, grant use/manage)

## Definition of done (phase exit)

A fresh instance: log in → add a connection (form rendered from a plugin's
`config` schema, inline secrets or a referenced credential) → it appears and
opens → create + share a reusable credential → all state-changing calls carry
CSRF, 401 redirects to login, 403/validation errors surface as toasts. Backend
CRUD covered by unit/integration tests; FE covered by component + e2e tests.
Connection grant semantics are explicit: `use` opens/uses, `manage` edits/shares/deletes.
Credential grants are separate from connection grants: shared connections can
use their already-bound credentials without exposing credential records, while
credential sharing remains managed from the credentials page.
Credential deletion is blocked while referenced.

## Out of scope here → **M-Admin** (later, additive, v2 §12.2)

Detailed when reached; needs its own control-plane endpoints:

- User / role management + role assignment.
- Policy-rule admin UI (additive stored `role + permission + risk`).
- Audit-log view (filters: user / connection / route / risk / result) + per-connection activity panel.
- Light status page (gateway health, live session/channel counts) — **not** a Grafana replacement.
- Agent management polish (re-enroll/rotate token, disconnect/offline history, artifact-URL + copy-env/download-manifest variants).

These are **operate-it** surfaces, not blockers for first real use. Backend-only
M1 behaviors (wrapper validation, denied-route audit, stored-policy loading,
secret-access metrics) need **no** UI — they are test-verified.
