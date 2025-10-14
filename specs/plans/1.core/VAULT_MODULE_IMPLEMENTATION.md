# Vault Module Implementation Specification

## Overview

The Vault module introduces encrypted credential storage, identity sharing, and UI tooling that unlock the remote protocol roadmap. This document summarizes the backend and frontend work required to evolve the current placeholders (`web/src/pages/settings/Identities.tsx`) into production-ready workflows while integrating with existing services, permissions, and telemetry pipelines.

## Terminology

- **Vault** – The secure service responsible for encrypting, governing access to, and auditing credential payloads.
- **Identity** – A single credential record stored in the vault (for example an SSH key pair, database username/password, API token, or kubeconfig) that can be attached to connections or shared. Identities have scopes: `global` (reusable across connections), `team` (shared within a team), or `connection` (bound to a single connection).
- **Credential Template** – A schema describing the fields, validation rules, and protocol compatibility for a given identity type.

## Scope & Goals

- Deliver encrypted storage for multi-protocol credentials (passwords, SSH keys, API tokens, kubeconfigs) using Argon2id key derivation.
- Provide CRUD APIs for identities, credential payloads (SSH, databases, infrastructure), and share rules with audit coverage.
- Surface vault management UI (lists, forms, sharing) and embed selectors in connection flows.
- Enforce permission-aware access across users, teams, and shared identities.
- **Single source of truth:** All connections MUST reference an identity via `identity_id` (no inline secrets in `Connection.Settings`).
- Produce observability, background maintenance, and documentation supporting operations.

## Non-Goals

- Building a full zero-knowledge escrow workflow or per-user passphrase UX.
- Delivering protocol-specific secret adapters beyond the shared identity selector.
- Shipping cloud KMS integrations or external secret engines in this iteration.
- Replacing existing authentication provider encryption (handled by `AuthProviderService`).
- Credential rotation workflows (UI/API or scheduled) — out of scope for MVP; credentials can be updated manually as needed.
- **Multi-key grace periods** – not planned for the initial release; revisit if compliance demands it.

## Architecture Principles

### Single Source of Truth

**Every connection MUST reference an Identity. No exceptions.**

```
Connection.identity_id → Identity (scope: global|team|connection)
                              ↓
                        Encrypted credentials
```

**Benefits:**

- ✅ All credentials encrypted consistently
- ✅ Unified audit trail
- ✅ Simpler driver code (always fetch from `identity_id`)
- ✅ No dual-storage complexity
- ✅ Better security (no accidental plaintext in settings)

### Identity Scopes

| Scope        | Description                 | Use Case                              | Visible in Identities List? |
| ------------ | --------------------------- | ------------------------------------- | --------------------------- |
| `global`     | Reusable across connections | Production SSH key used by 10 servers | ✅ Yes                      |
| `team`       | Shared within a team        | Team database credentials             | ✅ Yes                      |
| `connection` | Bound to single connection  | One-off dev server password           | ❌ No (auto-created)        |

**Connection-scoped identities:**

- Automatically created when user enters ad-hoc credentials during connection creation
- Transparently managed by the vault (user never sees them in Identities list)
- Deleted when parent connection is deleted (cascade)
- Name format: `"{Connection.Name} (auto)"` for admin traceability

## Dependencies & Existing Assets

- `pkg/crypto/crypto.go` already exposes AES-256-GCM helpers but lacks Argon2 key derivation.
- `internal/api/router.go` decodes `VAULT_ENCRYPTION_KEY` and wires services during startup.
- `internal/database/migrations.go` controls AutoMigrate registration for all models.
- `internal/services/connection_service.go` exposes `IdentityID *string` ready for vault integration.
- `internal/services/auth_provider_service.go` demonstrates encryption pattern (OIDC/SAML/LDAP secrets).
- `web/src/lib/api/client.ts`, `web/src/hooks/useUsers.ts`, and related modules show React Query and toast patterns to mirror.
- `web/src/lib/navigation.ts` reserves `/settings/identities` route.

## Backend Implementation

### Encryption & Key Management (Simplified)

\*\*Phase 1 (MVP): Single Active Key

- Add an Argon2id helper in `pkg/crypto` to derive a 32-byte key from `VAULT_ENCRYPTION_KEY`.
- Create `internal/vault/encryption.go` that wraps the existing AES-GCM helpers with the derived key.
- Update `internal/api/router.go` to load the master key, derive the runtime key, and inject a `VaultCrypto` into the vault service.
- Extend configuration validation so operators get a clear error if `VAULT_ENCRYPTION_KEY` is missing or too short.
- Document the importance of securing `VAULT_ENCRYPTION_KEY` (e.g., managed secret store) in `docs/operations/vault-secrets.md`.

### Data Model

**Core Models Overview**

- `Identity`: encrypted payload + metadata (scope, owner, optional team/connection binding, usage counters). Enforce `connection_id` only when scope is `connection`.
- `IdentityShare`: user/team grants with permission level (`use`, `view_metadata`, `edit`) and optional expiry; ensure uniqueness per principal.
- `CredentialTemplate`: driver-provided schema (`fields` with type/validation/input_modes, `compatible_protocols`, `version`, `deprecated_after`).
- `Connection`: rename `secret_id` → `identity_id` (nullable until migration runs) and cascade delete connection-scoped identities.

**Schema Migration Checklist**

- Rename `connections.secret_id` → `connections.identity_id`.
- Add `identities.scope` (default `global`) and `identities.connection_id` (unique when present).
- Backfill connection records to reference identities (create connection-scoped identities as needed).
- Register new models/fields in `internal/database/migrations.go` and extend tests.

### Service Layer & Business Logic

**VaultService Responsibilities**

- Constructor validates the encryption key, instantiates `VaultCrypto`, and wires audit + permission helpers.
- Create/update flows enforce permissions, validate against the credential template, encrypt payload JSON, persist the record, and audit the action.
- Read flows verify access (owner, share, or admin), decrypt via `VaultCrypto`, update usage stats, and audit access.
- Helpers cover connection-scoped provisioning, share lifecycle, cascade deletes, and metadata updates (usage count, last used).

**ConnectionService Integration**

- Creation flow accepts either an existing `identity_id` or inline credentials.
  - For inline credentials, call `VaultService.CreateIdentity` with `scope=connection`; after the connection is saved, update the identity with `connection_id`.
  - Reject requests that attempt to store secrets in `Connection.Settings` (validation helper).
- Update flow mirrors the same rules: existing identities must be accessible, and inline credential edits should replace the connection-scoped identity via the vault service instead of storing secrets in settings.
- Deletion should cascade: removing a connection triggers `vaultService.DeleteIdentity` when the linked identity is connection scoped.

**Template Sync Integration**

- Extend `ProtocolCatalogService.Sync()` to upsert credential templates for every registered driver.
- Store `required_fields`, `compatible_protocols`, `version`, and `deprecated_after` in `credential_templates`.
- Skip drivers that do not declare a template; log mismatches for observability.

### HTTP Layer

**Handlers (`internal/handlers/vault.go`):**

- `GET /api/vault/identities`: list identities visible to the requester; supports protocol filtering and optional inclusion of connection-scoped entries.
- `POST /api/vault/identities`: create a new identity using vault service validation/encryption.
- `GET /api/vault/templates`: return credential templates + version metadata for UI consumption.
- `POST /api/vault/identities/:id/shares` and `DELETE /api/vault/shares/:id`: manage sharing lifecycle.
- Handlers delegate to `VaultService` for permission checks, standardized error responses, and auditing.

**Route Registration**

- Mount vault routes under `/api/vault` with auth + rate limiting middleware.
- Expose CRUD endpoints for identities, read-only templates endpoint, and share management endpoints.
- Ensure router wiring happens after permission checker and vault service are initialized.

### Background Tasks & Metrics

- Maintenance job to purge expired shares (daily cron)
- Cleanup orphaned connection-scoped identities (weekly)
- Prometheus metrics: `vault_identities_total{scope}`, `vault_credentials_accessed_total`, `vault_shares_active`, `vault_encryption_errors_total`
- Audit logging for all vault operations

## Frontend Implementation

### Data Contracts & Client Layer

- Mirror backend models in TypeScript (`Identity`, `IdentityShare`, `CredentialTemplate`, `CredentialField`) including scope, ownership, version, deprecation metadata, and `inputModes` hints so fields can render as text, file upload, etc.
- Surface usage analytics (`usageCount`, `lastUsedAt`, `connectionCount`) so UI components can visualise identity utilisation.
- Expose typed API helpers in `web/src/lib/api/vault.ts` for identity CRUD, template fetch, and share management.
- Provide React Query hooks (`useIdentities`, `useIdentity`, `useIdentityMutations`, `useCredentialTemplates`, `useIdentitySharing`) that handle caching, optimistic updates, and toast notifications via existing utilities.

### UI & Workflows

- Settings ▸ Identities page: list/search/filter identities, distinguish team vs global scopes, hide connection-scoped records, surface create/edit/share dialogs.
- Identity selector: filters by protocol compatibility, shows scope badges, offers inline creation when the viewer has `vault.create`.
- Connection forms: radio toggle between saved identity and ad-hoc credentials; credential fields render dynamically based on template metadata (e.g., allow paste or file upload for the same secret), and submitting inline credentials triggers backend creation of a connection-scoped identity.
- Identity detail: display metadata, usage history, associated connections, and share management UI.
- Sensitive credential fields must never be rendered back to the browser—even with full permissions. Forms should only allow overwriting secrets; non-sensitive metadata may be shown when permitted.
- Global integration: display identity labels in connection cards, show "shared via identity" badges, and ensure breadcrumbs/sidebar highlight vault entry points.

### Integration Touchpoints

- Fetch templates dynamically per protocol and render credential fields accordingly.
- Auto-provision connection-scoped identities when inline credentials are submitted (backend handles persistence, frontend resets form state).
- Auto-share relevant identities when connections grant launch permissions so recipients can launch without manual credential duplication.
- Invalidate relevant queries after identity or share mutations to keep UI consistent.

## Permissions & Access Control

**Permission Scopes**

- Backend registers `vault.view`, `vault.create`, `vault.edit`, `vault.delete`, `vault.share`, `vault.use_shared`, `vault.manage_all` (see `internal/permissions/core.go`).
- Seed default roles: `admin` receives `vault.manage_all`; `user` receives `vault.use_shared`.
- Enforce permissions in handlers and React components via `PermissionGuard` and the shared constants map.

**Access Control Logic:**

- Owner can always access their identities
- Team members can access team-scoped identities
- Share recipients can use shared identities (decrypt for connection launch)
- `vault.view_shared_metadata` allows viewing name/type (not secrets)
- Connection-scoped identities follow connection ownership rules
- Root users bypass all checks

## Testing & QA

### Unit Tests

- `pkg/crypto/kdf_test.go` – Argon2 derivation
- `internal/vault/encryption_test.go` – Encrypt/decrypt with tampering
- `internal/services/vault_service_test.go` – CRUD, sharing, permissions
- `internal/handlers/vault_test.go` – HTTP endpoints, validation

### Integration Tests

- Identity lifecycle (create → share → use → delete)
- Connection creation with inline credentials (auto-creates scoped identity)
- Cascade delete (connection deletion removes scoped identity)
- Permission enforcement across all endpoints

### Frontend Tests

- Component tests for IdentitySelector, IdentityForm
- Hook tests for useIdentities, useCreateIdentity
- Cypress E2E:
  - Create global identity → use in connection
  - Enter ad-hoc credentials → verify scoped identity created
  - Share identity → recipient can launch connection

## Observability & Security Notes

### Security Checklist

- ✅ Secrets never logged (mask in audit entries)
- ✅ Encryption key stored in environment (not config files)
- ✅ Argon2 key derivation (memory-hard)
- ✅ Rate limiting on vault endpoints (100 req/min)
- ✅ CSRF protection enabled
- ✅ Frontend never caches decrypted secrets
- ✅ UI never renders decrypted secrets; write flows allow overwrite without exposing existing values
- ✅ Copy/download restricted to explicit user gestures

### Audit Logging

- `vault.identity.created` (scope, type)
- `vault.credentials.accessed` (user, connection)
- `vault.identity.shared` (recipient, permission)
- `vault.identity.deleted`
- `vault.share.revoked`

### Metrics

- `vault_identities_total{scope}`
- `vault_credentials_accessed_total`
- `vault_shares_active`
- `vault_encryption_errors_total`

## Operational Runbooks

### Key Rotation

- Credential rotation workflows are out of scope for MVP; identities are updated manually as needed through administrative processes.
- `VAULT_ENCRYPTION_KEY` is expected to remain stable. If rotation of the master key is ever required, follow a dedicated security runbook outside this implementation scope.

### Backup & Recovery

- Schedule database backups (identities remain encrypted at rest) and store the vault key in a secure secret store/KMS.
- Recovery steps: restore database snapshot, restore/export the vault key, restart application, verify by listing identities.

## Risks & Mitigation

| Risk                                | Likelihood | Impact | Mitigation                                  |
| ----------------------------------- | ---------- | ------ | ------------------------------------------- |
| Encryption key leaked               | Low        | High   | Store in KMS, restrict access, audit usage  |
| Connection-scoped identity orphaned | Medium     | Low    | Weekly cleanup job, cascade delete          |
| Template version mismatch           | Low        | Medium | Validation on save, migration warnings      |
| Share permission confusion          | Medium     | Low    | Clear UI labels, permission tooltips        |
| Performance with 10k+ identities    | Low        | Medium | Index on scope/owner, pagination, caching   |

## Success Criteria

**MVP Launch:**

- ✅ All connections reference identities (no inline secrets)
- ✅ Users can create/edit/delete global identities
- ✅ Connection-scoped identities auto-created for ad-hoc credentials
- ✅ Sharing works for users and teams
- ✅ Audit logs capture all vault access
- ✅ Zero secrets logged in plaintext
