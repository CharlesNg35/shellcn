# Phase 2d — M-Admin · Administration foundation

M-Admin is additive to M1/M1.5. This slice adds the administration foundation
needed before real plugins: typed bootstrap config, admin user management,
invitations, and the Users view.

## Status
- [x] 2d.1 Backend — typed bootstrap config
- [x] 2d.2 Backend — admin user CRUD
- [x] 2d.3 Backend — invitations
- [x] 2d.4 Frontend — Users view

## Definition of done
Admins can configure bootstrap/runtime settings through typed config, manage
users without locking out the protected root admin, issue single-use invitation
links, and use the generic admin UI to perform those tasks.

## Deferred M-Admin Work
- Policy-rule administration
- Audit-log and per-connection activity view
- Status/health page
- Agent re-enroll/rotate history
