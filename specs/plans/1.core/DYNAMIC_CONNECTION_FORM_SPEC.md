# Dynamic Connection Form & Schema Registry

## 1. Background

### 1.1 Existing Implementation (Audit Summary)

- **Server-side workflow**
  - `internal/services/connection_service.go` accepts arbitrary `settings map[string]any` and persists it as JSON. Optional `connection_targets` are only created if the API payload already provides them; no validation or normalisation exists.
  - SSH launch (`internal/handlers/ssh_session.go`) calls `resolveHostPort`, which falls back to `connection.targets[0]` or `settings.host`. Concurrency, SFTP, terminal overrides, and recording toggles read raw JSON without schema enforcement.
  - No shared abstraction exists for other protocol handlers, meaning each new driver must replicate parsing logic.
- **Driver registry**
  - `drivers.Registry` already supplies metadata, capabilities, and credential templates. `ProtocolCatalogService.Sync` persists these descriptors to `connection_protocols` and `credential_templates`.
- **Frontend workflow**
  - `web/src/components/connections/ConnectionFormModal.tsx` mixes universal fields (name, folder, team, identity, icon) with SSH-specific toggles. Adding a protocol requires editing this monolithic component, which contradicts the registry-driven pattern adopted for credentials (`CredentialTemplate`).

### 1.2 Problem Statement

- We must support many protocol types (SSH, RDP, Kubernetes, databases, etc.) as outlined in `specs/project/project_spec.md`. Hard-coding per-driver UI is unmaintainable.
- Backend services lack canonical schemas for connection settings, leading to brittle launch-time parsing and inconsistent validation.
- We need a registry-driven contract so drivers publish configuration templates similar to credential templates.

### 1.3 Goal

Make the connection form _schema driven_. Drivers declare their configuration requirements, the backend persists and validates them, and the frontend renders a single dynamic form that works for existing and upcoming protocols.

## 2. Objectives & Non‑Goals

| Objective           | Details                                                                                                                                 |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| Dynamic schema      | Drivers register a typed template describing connection fields (labels, input type, defaults, validation, storage binding).             |
| Backend enforcement | `ConnectionService` validates create/update payloads against the template and normalises values (including defaults).                   |
| Launch readiness    | Session launch handlers receive a sanitised config without guessing between `settings` vs `targets`.                                    |
| Registry alignment  | Reuse the existing driver registry pattern (similar to `CredentialTemplate`). Sync templates during protocol catalog sync.              |
| Frontend reuse      | A single React form component renders any driver by reading the template from the API.                                                  |
| Shared fields       | Folder/team assignment, icon, color, identity selector, and description remain common across all protocols with permission-aware logic. |

Non-goals for this iteration:

- Building every future protocol driver (we only model the contracts and update SSH as the reference implementation).
- Removing `connection.targets` table (we continue supporting explicit targets while offering storage bindings from templates).
- Replacing the credential template system (the new connection template complements it).

## 3. Driver Contract Additions

### 3.1 Interface

Add a new interface in `internal/drivers/connection_template.go`:

```go
type ConnectionTemplater interface {
    ConnectionTemplate() (*ConnectionTemplate, error)
}
```

```go
type ConnectionTemplate struct {
    DriverID    string
    Version     string
    DisplayName string
    Description string
    Sections    []ConnectionSection
    Metadata    map[string]any
}

type ConnectionSection struct {
    ID          string
    Label       string
    Description string
    Fields      []ConnectionField
}

type ConnectionField struct {
    Key          string
    Label        string
    Type         string
    Required     bool
    Default      any
    Placeholder  string
    HelpText     string
    Options      []ConnectionOption
    Binding      ConnectionBinding
    Validation   map[string]any
    Dependencies []FieldDependency
}

type ConnectionOption struct {
    Value string
    Label string
}

type ConnectionBinding struct {
    Target   string
    Path     string
    Index    int
    Property string
}

type FieldDependency struct {
    Field string
    Equals any
}
```

- `Type` drives frontend widgets; predefined types include `string`, `multiline`, `number`, `boolean`, `select`, `target_host`, `target_port`, `json`, etc.
- `Binding` instructs backend/front-end where to store output: e.g. SSH host → `{Target:"target", Index:0, Property:"host"}`; concurrency override → `{Target:"settings", Path:"session.concurrent_limit"}`.
- `Dependencies` allow conditional visibility/enabled status without wiring extra props in React.
- Drivers may attach `Metadata` (e.g., grouping hints, icon suggestions).

### 3.2 SSH Reference Template

During driver init:

```go
func NewSSHDriver() *Driver {
    tpl := &drivers.ConnectionTemplate{
        DriverID:    "ssh",
        Version:     "2025-01-01",
        DisplayName: "SSH Connection",
        Sections: []drivers.ConnectionSection{
            {
                ID:    "endpoint",
                Label: "Endpoint",
                Fields: []drivers.ConnectionField{
                    {
                        Key:      "host",
                        Label:    "Host",
                        Type:     drivers.ConnectionFieldTypeString,
                        Required: true,
                        Validation: map[string]any{"pattern": hostnamePattern},
                        Binding: drivers.ConnectionBinding{
                            Target:   drivers.BindingTargetConnectionTarget,
                            Index:    0,
                            Property: "host",
                        },
                    },
                    {
                        Key:      "port",
                        Label:    "Port",
                        Type:     drivers.ConnectionFieldTypeNumber,
                        Default:  22,
                        Validation: map[string]any{"min": 1, "max": 65535},
                        Binding: drivers.ConnectionBinding{
                            Target:   drivers.BindingTargetConnectionTarget,
                            Index:    0,
                            Property: "port",
                        },
                    },
                },
            },
            {
                ID:    "session",
                Label: "Session Behaviour",
                Fields: []drivers.ConnectionField{
                    {
                        Key:      "concurrent_limit",
                        Label:    "Concurrent Sessions",
                        Type:     drivers.ConnectionFieldTypeNumber,
                        Default:  0,
                        Validation: map[string]any{"min": 0, "max": 1000},
                        Binding: drivers.ConnectionBinding{
                            Target: drivers.BindingTargetSettings,
                            Path:   "session.concurrent_limit",
                        },
                    },
                    // Additional fields…
                },
            },
        },
    }
    return newDriver(desc, caps, tpl)
}
```

`newDriver` would embed the template so `ConnectionTemplate()` returns it.

## 4. Persistence & Registry Sync

1. **New model/table**: `connection_templates` (similar to `credential_templates`):

| Column                        | Notes                                                 |
| ----------------------------- | ----------------------------------------------------- |
| `driver_id`                   | Lowercase driver id (`ssh`).                          |
| `version`                     | Semver/date string, allows versioning and migrations. |
| `display_name`, `description` | UI metadata.                                          |
| `sections`                    | JSON array of sections/fields.                        |
| `metadata`                    | Optional JSON.                                        |
| `hash`                        | Deterministic hash for change detection.              |

2. Extend `ProtocolCatalogService.Sync`:

   - Detect drivers implementing `ConnectionTemplater`.
   - Marshal template + persist to `connection_templates` with `OnConflict` upsert (matching driver/version).
   - Validate schema integrity: ensure at least one section & field, required bindings present, no duplicate keys.

3. Add GORM model validations (similar to `CredentialTemplate.BeforeSave`).

4. Consider storing a `current` pointer per driver (latest version). Simplest approach: clients request latest by sorting `created_at DESC`.

## 5. API Surface

### 5.1 Endpoints

- `GET /api/protocols/:id/connection-template`
  - Returns `{ template: ConnectionTemplateDTO }`.
  - DTO flattening: sections, fields, driver metadata, version, `requires_identity` flag (see §5.2).
- Optionally embed a minimal snapshot in `/api/protocols/available` (just `version` + `has_template`) so UI can prefetch lazily.

### 5.2 DTO Enhancements

- Extend `ProtocolInfo` response with:
  - `default_port` (already present) – keep.
  - `connection_template_version` (latest version string or null).
  - `identity_required` boolean (derived from template metadata or driver capability).

### 5.3 Frontend Hook

- Create `useConnectionTemplate(protocolId)` in `web/src/hooks` to hit the new endpoint.
- Cache per protocol/version using React Query.

## 6. Connection Create/Update Pipeline

1. **Payload structure**: continue accepting `metadata`, `settings`, but add `fields` map coming from the dynamic form. Example:

```json
{
  "name": "Prod SSH",
  "protocol_id": "ssh",
  "fields": {
    "host": "prod.example.com",
    "port": 22,
    "concurrent_limit": 2,
    "recording_enabled": true
  },
  "identity_id": "vault-uuid"
}
```

2. `ConnectionHandler.Create` / `Update`:

   - Load template via `connectionTemplateSvc.Resolve(protocolID)` (see §6.3).
   - Validate `fields` against template:
     - Required keys present.
     - Type coercion (string → number).
     - Regex/range checks from `Validation`.
     - Apply defaults for missing optional values.
   - Materialise storage outputs:
     - Build `settings` map, `metadata` map, and `[]ConnectionTarget`.
     - Clean old targets on update before inserting new ones.
   - Call driver `ValidateConfig(ctx, normalisedSettings)` if it implements `drivers.Validator`.
   - Persist connection with resulting JSON + targets.

3. **Template resolution service**:

```go
type ConnectionTemplateService struct {
    db *gorm.DB
    driverRegistry *drivers.Registry
}

func (s *ConnectionTemplateService) Resolve(ctx context.Context, protocolID string) (*drivers.ConnectionTemplate, error)
```

Resolution order:

1.  Lookup cached template in DB (latest version).
2.  If missing/outdated, query driver registry directly, persist, then return.
3.  Handle protocols without templates by returning `nil`.

4.  **Validation errors**: return structured errors referencing field keys (e.g., `field.host.required`).

## 7. Session Launch Flow Improvements

Current `SSHSessionHandler` performs ad-hoc extraction:

```go
host, port, hostErr := resolveHostPort(connDTO, settings)
```

With templates:

1. Add `ConnectionConfig` helper in `services`:

```go
type ConnectionConfig struct {
    Settings map[string]any
    Targets  []models.ConnectionTarget
}

func (s *ConnectionTemplateService) MaterialiseConfig(connection services.ConnectionDTO) (ConnectionConfig, error)
```

2. The helper ensures required target entries exist (host/port). If missing, reject launch early with clear error.
3. `ssh_session` (and future launch handlers) consume `config.Settings` to populate `SessionRequest`.
4. For features like session overrides (`concurrent_limit`, `enable_sftp`), rely on settings populated by template rather than scattered keys.

## 8. Frontend Form Builder

1. **Component composition**:

   - Break the experience into focused components to avoid monolithic files:
     - `ConnectionFormShell` handles shared chrome (title, icon/color picker, identity + folder/team selectors, permission gating).
     - `ConnectionTemplateForm` renders template-driven sections/fields.
     - `ConnectionFormActions` encapsulates submit/reset buttons and submission state.
   - Shared components (folder picker, team selector, identity selector) remain reusable imports to keep consistency across the app.

2. **Form layout**:

   - Render sections sequentially with optional accordion/tabs (metadata-driven).
   - Field components keyed by `field.Type`.
     - `string` → `<Input/>`.
     - `multiline` → `<Textarea/>`.
     - `number` → `<Input type="number">`.
     - `boolean` → `<Switch/>`.
     - `select` → `<Select/>`.
     - `target_host` / `hostname` → `<Input>` with validation message from template (pattern).
   - Support advanced group toggles by checking section metadata (e.g., `section.Metadata["collapsedDefault"]`).

3. **State management**:

   - Build a single `formValues` object initialised with template defaults + existing connection values.
   - When editing, map existing `settings`, `metadata`, and `targets` back to template keys using the same binding logic (inverse mapping helper shared with backend).

4. **Validation**:

   - Client-side constraints derived from template (`min`, `max`, `pattern`).
   - Display backend validation errors by matching `field_errors` map keyed by `field.Key`.

5. **Submission**:

   - Transform `formValues` → `fields` map.
   - Keep existing identity/folder/team/metadata flows (identity selection stays outside template for now but flagged as future metadata).
   - `useConnectionMutations` post/put to backend with new payload shape.

6. **UI polish**:
   - Keep consistent icon/color selectors (template metadata can eventually hide them per driver).
   - Session recording toggle for SSH becomes a template field; remove hard-coded component.

## 9. Migration & Backfill

1. Write a one-off migration:
   - For existing SSH connections, copy `targets[0]` host/port into new template-driven fields (`settings.host` or create targets if empty).
   - Ensure `connection.settings.recording_enabled`, `concurrent_limit`, etc. remain intact; template just formalises them.
2. After rollout the old manual form code paths can be removed.

## 10. Testing & Observability

- **Backend**
  - Unit tests for `ConnectionTemplateService` (resolve, defaulting, binding).
  - Connection service tests covering create/update with template enforcement, as well as driver validation errors.
  - SSH launch handler test verifying host/port extraction uses template output.
- **Frontend**
  - Component tests for the dynamic form builder rendering various field types.
  - Mutation tests verifying payload transformation for create/update.
  - Cypress (or Playwright) smoke to ensure connection creation works end-to-end with the new schema.
- **Monitoring**
  - Add structured audit when templates change (`protocol.catalog.sync`).
  - Log validation failures with field identifiers to ease troubleshooting.

## 11. Launch Flow & Service Integration

- **Connection service**
  - Extend `Create`/`Update` workflow to:
    - Resolve the template for `input.ProtocolID`.
    - Validate the incoming `fields` payload and derive `settings`, `metadata`, and `[]ConnectionTarget`.
    - Persist normalised targets using the existing `ConnectionTarget` model (replace previous delete-and-replace block).
    - Invoke `drivers.Validator` when implemented to allow extra driver-specific checks.
  - Store the applied template version in `connection.metadata.template_version` for future migrations.
- **Session launch handlers**
  - Introduce `services.ConnectionTemplateService.MaterialiseConfig(dto ConnectionDTO)` to produce a deterministic `SessionConfig` (settings + targets + metadata).
  - Refactor `internal/handlers/ssh_session.go` to consume this helper instead of `resolveHostPort`; ensure error responses reference missing template fields (`field.host.required`).
  - Update other protocol launch handlers as they are implemented to follow the same pattern.
- **Realtime / active sessions**
  - Enrich `ActiveSessionRecord.Metadata` with template-bound values (host, port, cluster name, database, etc.) so UI chips/reports remain protocol-agnostic.
  - Ensure session sharing and recording infrastructure continues to receive the same metadata; no functional changes expected beyond using the normalised config.

## 12. Open Questions

1. **Field Types**: Do we need composite types (arrays, key/value tags) in v1? Proposal: start with scalar + list-of-targets via metadata; iterate later.
2. **Identity Requirements**: drivers like Telnet may not need vault identities. Template metadata could include `requires_identity` to toggle the selector.
3. **Target cardinality**: Some protocols need multiple endpoints (e.g., read/write). Template should allow multiple `target` bindings referencing different indices.
4. **Version upgrades**: When driver template version bumps, how do we migrate existing connections? Plan: store last applied version per connection (in `metadata`) to gracefully prompt for review.

---

**Next Steps**

1. Implement driver interface + template structs (`internal/drivers`).
2. Create `connection_templates` model + migration.
3. Update `ProtocolCatalogService` to persist templates.
4. Build `ConnectionTemplateService` and integrate with connection create/update handlers.
5. Adjust launch flow to consume normalised config.
6. Ship frontend dynamic form builder + hooks.
7. Backfill existing SSH connections and retire hard-coded fields.

- **Shared Base Fields**: The platform keeps universal inputs (name, description, folder, team, icon, color, identity selector). These remain outside driver templates but are composed into the same form experience.
  - Team assignment respects permission checks (`team.manage`, `connection.manage`). The payload continues to include `team_id` only when callers are authorised.
  - Folder selection stays available for all protocols; we keep existing folder hooks and slot the picker into the new layout.
  - Identity selector remains a shared component. Template metadata may later declare `requires_identity` to hide the selector when not needed.

📌 **Spec cross-reference**

- Update `specs/project/PROTOCOL_DRIVER_STANDARDS.md` to include the `ConnectionTemplater` contract, template conventions, and guidance on shared vs driver-specific fields. Each driver spec (`specs/project/drivers/<id>.md`) should document its template schema alongside credential requirements.
