# Protocol Driver Standards

This document defines the contract that every protocol driver (formerly "module") must follow across backend, frontend, and specification layers. It complements `specs/plans/1. core/CONNECTION_PROTOCOLS_PLAN.md` and supersedes legacy module-language inside historical docs.

## 1. Driver Taxonomy & Naming

- **Driver ID**: lower-case, hyphen-less identifier (e.g. `ssh`, `docker`, `kubernetes`). IDs become permission prefixes, connection protocol ids, and filesystem folder names under `specs/project/drivers/<driver-id>.md`.
- **Title**: human readable string displayed in UI tab labels ("Kubernetes", "Docker Engine").
- **Category**: standard categories `terminal`, `desktop`, `container`, `database`, `file_share`, `vm`, `network`. Custom categories must be documented and added to UI icon mapping.
- **Module Field**: persisted value mirroring configuration namespace (current config keys under `Config.Modules`). Prefer the driver ID unless multiple sub-protocols share the same driver (database family).

## 2. Specification Layout

Each driver receives its own spec file under `specs/project/drivers/<driver-id>.md` with the following sections:

1. **Overview** – summary, target infrastructure, backend driver type (native, FFI, proxy).
2. **Connection Schema** – base settings persisted in `connections.settings` (host, port, namespace, context, path, etc.). Include `required?`, `default`, `validation`, and the capability flag(s) each property unlocks.
3. **Identity Requirements** – identities or vault credentials needed (e.g. SSH key, kubeconfig, Docker TLS cert). Specify secret schema keys so Credential Vault integration can be automated.
4. **Permission Profile** – list of permission ids (base + optional). Align with section 4 below.
5. **Frontend Contract** – form panels, quick actions, optional wizards, capability-specific UI toggles.
6. **Testing Guidance** – driver-specific fixtures, integration tests, and mocks.
7. **Future Enhancements** – optional roadmap for driver-specific features.

## 3. Driver Registration Pipeline

1. Driver package implements interfaces in `internal/drivers/driver.go`.
2. Driver registers with `drivers.Registry` during bootstrap (`drivers.MustRegister`).
3. `protocols.Registry.SyncFromDrivers` ingests descriptors & capabilities.
4. `ProtocolCatalogService.Sync` persists metadata + enablement state.
5. `protocols.RegisterDriverPermissions` (see section 4) registers permission ids before `permissions.Sync`.
6. Frontend fetches protocol catalog and driver schema from `/api/protocols`.

## 4. Permission Model

| Layer               | Responsibility                                                            |
| ------------------- | ------------------------------------------------------------------------- |
| `connection.view`   | Grants access to protocol/connection catalog routes.                      |
| `connection.launch` | Required to start or preview sessions.                                    |
| `connection.manage` | Required for CRUD operations on connections and driver advanced settings. |
| `connection.share`  | Required for editing visibility ACLs.                                     |

Every driver defines a `PermissionProfile` struct:

```go
type PermissionProfile struct {
    BaseConnect   string   // defaults to "{driver}.connect"
    Manage        string   // defaults to "{driver}.manage"
    FeatureScopes []string // optional extras like "kubernetes.exec"
    AdminScopes   []string // optional admin extras like "kubernetes.cluster.admin"
}
```

Rules:

- Base connect scopes depend on `connection.launch`.
- Manage + admin scopes depend on `connection.manage`.
- Feature scopes depend on `connection.launch` unless they mutate state; mutating scopes must depend on `connection.manage`.
- Shared database drivers may add child scopes (e.g. `database.mysql.connect`, `database.redis.manage`). These still register under the database driver spec file.

Permission registration occurs in driver init:

```go
permissions.Register(&permissions.Permission{ID: profile.BaseConnect, Module: driverID, DependsOn: []string{"connection.launch"}})
```

## 5. Connection Schema Requirements

- Store driver settings as JSON on `Connection.Settings`. Drivers supply a JSON schema via `drivers.SchemaProvider` describing field names, types, validation, and whether a field is identity-backed.
- `Connection.Metadata` holds UI-only preferences (favorite tags, color). Do not duplicate driver settings in metadata.
- Provide a helper `DriverConfig.Normalize(settings map[string]any) (map[string]any, error)` to coerce defaults, merge ports, and handle capability toggles.
- Drivers that require multiple targets (e.g., Kubernetes API + kubeconfig) should use `ConnectionTargets` to persist per-cluster endpoints.

## 6. Identity & Credential Vault Integration

- Drivers declare required secret slots (e.g., `ssh.key`, `ssh.password`, `kubeconfig`, `docker.cert`).
- Secrets either reference `vault.Credential` IDs or embed inline encrypted payloads.
- `Identity` feature (future) must map to driver requirements using the same key names to allow auto-binding.
- `ProtocolService` and UI should surface missing credential requirements so users can attach identities before launching.

## 7. Frontend Contract

- `/api/protocols` returns `ProtocolInfo` with `capabilities` and `features` arrays. UI uses this to display capability chips and to decide which tabs (terminal, desktop, metrics) to show.
- `/api/protocols/:id` (future) will include configuration schema for driver forms.
- Frontend state hooks (`useUserProtocols`, `useConnections`) cache responses and filter by permission-derived availability.
- React components should rely on `capabilities` when toggling UI actions (e.g., show "File Transfer" if `file_transfer` in features).

## 8. Testing Expectations

- **Unit Tests**: driver-specific packages should test descriptor registration, capability responses, permission profile registration, and config validation.
- **Integration Tests**: cover `ProtocolService`, handler endpoints, and driver health sync (mocking registries where needed).
- **Frontend Tests**: ensure the Connections page renders capability chips, disables launch buttons when permissions are missing, and respects category filters.

## 9. Example Permission Profiles

| Driver     | Base Connect         | Manage              | Feature Scopes                                | Admin Scopes               |
| ---------- | -------------------- | ------------------- | --------------------------------------------- | -------------------------- |
| SSH        | `ssh.connect`        | `ssh.manage`        | `ssh.sftp`, `ssh.port_forward`                | `ssh.global.manage`        |
| Kubernetes | `kubernetes.connect` | `kubernetes.manage` | `kubernetes.exec`, `kubernetes.port_forward`  | `kubernetes.cluster.admin` |
| Docker     | `docker.connect`     | `docker.manage`     | `docker.logs`, `docker.exec`                  | `docker.stack.deploy`      |
| Database   | `database.connect`   | `database.manage`   | `database.query.read`, `database.query.write` | `database.cluster.manage`  |

## 10. Migration Notes

- Historical references to the "Core Module" now map to the "Core Protocol Driver Set". Where documentation still mentions modules, annotate them with the new term on sight to maintain clarity.
- New drivers must include their spec document _before_ code merges.
- Any config change that toggles driver availability must update the relevant spec sections (config schema + permission updates).

---

**Status**: Draft ready for implementation guidance.
**Maintainer**: Core platform architecture team.
