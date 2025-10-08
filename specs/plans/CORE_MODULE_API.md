# Core Module API Documentation

**Base URL:** `https://{host}:{port}` (default `http://localhost:8080`)  
**Version:** v1 (stabilised once backend reaches GA)  
**Content Type:** `application/json` unless otherwise specified

---

## 1. Conventions

### 1.1 Authentication

- Public endpoints: `POST /api/auth/login`, `POST /api/auth/refresh`, `GET /api/auth/providers`, `GET /api/setup/status`, `POST /api/setup/initialize`, SSO redirects (`/api/auth/providers/:type/login|callback|metadata`), and `GET /health`.
- All other endpoints require a bearer token in the `Authorization` header:  
  `Authorization: Bearer <access-token>`
- Access tokens are short-lived JWTs (`auth.jwt.ttl`), refresh tokens are stored server-side via the session service.

### 1.2 Response Envelope

Every endpoint returns the standard envelope defined in `pkg/response`:

```json
{
  "success": true,
  "data": {...},
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 45,
    "total_pages": 3
  }
}
```

Errors always include a machine-readable code:

```json
{
  "success": false,
  "error": {
    "code": "auth.invalid_credentials",
    "message": "Username or password is incorrect"
  }
}
```

### 1.3 Permissions

Protected routes use `middleware.RequirePermission`. Permission IDs follow dot notation (see `internal/permissions/core.go`). The **Permission** column in the route tables below lists the minimum requirement; `*.manage` implies related view permissions through dependency resolution.

---

## 2. Authentication & Session Management

| Method | Path                                 | Description                                                                                             | Permission                           |
| ------ | ------------------------------------ | ------------------------------------------------------------------------------------------------------- | ------------------------------------ |
| POST   | `/api/auth/login`                    | Authenticate with username/password, optional MFA challenge. Returns access/refresh tokens and profile. | Public                               |
| POST   | `/api/auth/refresh`                  | Exchange refresh token for new access token pair.                                                       | Public                               |
| GET    | `/api/auth/providers`                | List enabled external providers with UI metadata.                                                       | Public                               |
| GET    | `/api/auth/providers/:type/login`    | Initiate SSO redirect (OIDC/SAML/LDAP test).                                                            | Public                               |
| GET    | `/api/auth/providers/:type/callback` | SSO callback handler.                                                                                   | Public                               |
| GET    | `/api/auth/providers/:type/metadata` | Provider metadata (e.g., SAML SP metadata XML).                                                         | Public                               |
| GET    | `/api/auth/me`                       | Current user profile, roles, and permissions.                                                           | `session.active` (implicit via auth) |
| POST   | `/api/auth/logout`                   | Invalidate current refresh token and session.                                                           | `session.active`                     |

**Sample** — Login:

```http
POST /api/auth/login
Content-Type: application/json

{
  "username": "alice",
  "password": "Secret123!",
  "mfa_token": "123456"   // optional
}
```

Response:

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "c509fffa-...",
    "expires_in": 3600,
    "user": {
      "id": "usr_01H...",
      "username": "alice",
      "email": "alice@example.com",
      "roles": ["admin"],
      "permissions": ["user.view", "org.manage", "..."]
    }
  }
}
```

### 2.1 Sessions

| Method | Path                       | Description                                | Permission                                |
| ------ | -------------------------- | ------------------------------------------ | ----------------------------------------- |
| GET    | `/api/sessions/me`         | List active sessions for the current user. | `session.view` (granted to account owner) |
| POST   | `/api/sessions/revoke/:id` | Revoke a single session by ID.             | `session.manage` (owner or admin)         |
| POST   | `/api/sessions/revoke_all` | Revoke all other sessions.                 | `session.manage`                          |

---

## 3. Setup Workflow

| Method | Path                    | Description                                            | Permission                           |
| ------ | ----------------------- | ------------------------------------------------------ | ------------------------------------ |
| GET    | `/api/setup/status`     | Returns `"pending"` until first admin exists.          | Public                               |
| POST   | `/api/setup/initialize` | Creates initial root admin and bootstrap organisation. | Public (guarded by empty user table) |

Payload:

```json
{
  "username": "root",
  "email": "root@example.com",
  "password": "ChangeMe123!"
}
```

---

## 4. User & Identity Management

| Method | Path             | Description                                                   | Permission    |
| ------ | ---------------- | ------------------------------------------------------------- | ------------- |
| GET    | `/api/users`     | Paginated list of users (query `page`, `per_page`, `search`). | `user.view`   |
| GET    | `/api/users/:id` | Retrieve user details with role assignments.                  | `user.view`   |
| POST   | `/api/users`     | Create user, roles, and optional activation toggle.           | `user.create` |

### 4.1 Organisations & Teams

| Method | Path                              | Description                    | Permission   |
| ------ | --------------------------------- | ------------------------------ | ------------ |
| GET    | `/api/orgs`                       | List organisations.            | `org.view`   |
| GET    | `/api/orgs/:id`                   | Get organisation detail.       | `org.view`   |
| POST   | `/api/orgs`                       | Create organisation.           | `org.create` |
| PATCH  | `/api/orgs/:id`                   | Update display name, metadata. | `org.manage` |
| DELETE | `/api/orgs/:id`                   | Soft delete organisation.      | `org.manage` |
| GET    | `/api/organizations/:orgID/teams` | List teams for organisation.   | `org.view`   |

**Teams**

| Method | Path                             | Description         | Permission   |
| ------ | -------------------------------- | ------------------- | ------------ |
| GET    | `/api/teams/:id`                 | Team & members.     | `org.view`   |
| POST   | `/api/teams`                     | Create team.        | `org.manage` |
| PATCH  | `/api/teams/:id`                 | Rename/update team. | `org.manage` |
| POST   | `/api/teams/:id/members`         | Append member IDs.  | `org.manage` |
| DELETE | `/api/teams/:id/members/:userID` | Remove member.      | `org.manage` |
| GET    | `/api/teams/:id/members`         | List members.       | `org.view`   |

Pagination for listing endpoints returns the `meta` section described earlier.

---

## 5. Permissions & Roles

| Method | Path                                     | Description                                       | Permission          |
| ------ | ---------------------------------------- | ------------------------------------------------- | ------------------- |
| GET    | `/api/permissions/registry`              | Tree of registered permissions with dependencies. | `permission.view`   |
| GET    | `/api/permissions/roles`                 | List roles and assigned permissions.              | `permission.view`   |
| POST   | `/api/permissions/roles`                 | Create role.                                      | `permission.manage` |
| PATCH  | `/api/permissions/roles/:id`             | Update role name/description.                     | `permission.manage` |
| DELETE | `/api/permissions/roles/:id`             | Delete role (prevent delete of system roles).     | `permission.manage` |
| POST   | `/api/permissions/roles/:id/permissions` | Replace permission set for role.                  | `permission.manage` |

---

## 6. Audit & Security

| Method | Path                  | Description                                                                             | Permission       |
| ------ | --------------------- | --------------------------------------------------------------------------------------- | ---------------- |
| GET    | `/api/audit`          | Paginated audit events (`page`, `per_page`, `actor`, `action`, `result`, `from`, `to`). | `audit.view`     |
| GET    | `/api/audit/export`   | CSV export filtered by same parameters.                                                 | `audit.export`   |
| GET    | `/api/security/audit` | Security-focused view (failed logins, privilege escalation attempts).                   | `security.audit` |

Audit events include metadata such as `trace_id`, `actor`, `ip`, `resource`, `changes`, and `result`.

---

## 7. Authentication Provider Administration

Administrative provider endpoints live under `/api/auth/providers`.

| Method | Path                                  | Description                                                         | Permission          |
| ------ | ------------------------------------- | ------------------------------------------------------------------- | ------------------- |
| GET    | `/api/auth/providers/all`             | List provider configs and statuses.                                 | `permission.view`   |
| GET    | `/api/auth/providers/enabled`         | Enabled providers for UI toggles.                                   | `permission.view`   |
| POST   | `/api/auth/providers/local/settings`  | Update local auth options (password policies, invites).             | `permission.manage` |
| POST   | `/api/auth/providers/:type/configure` | Persist provider-specific configuration payload.                    | `permission.manage` |
| POST   | `/api/auth/providers/:type/enable`    | Toggle provider enablement (`{"enabled":true}`).                    | `permission.manage` |
| POST   | `/api/auth/providers/:type/test`      | Connectivity test (OIDC discovery, SAML metadata parse, LDAP bind). | `permission.manage` |

**Configuration schemas** (payload `data` examples):

- **OIDC**: issuer URL, client ID, redirect URL, scopes, optional secret reference.
- **SAML**: metadata URL or inline SSO URL, ACS URL, certificates, attribute mapping.
- **LDAP**: host/port, bind DN, TLS options, search filter, attribute mapping.

See `internal/models/auth_provider.go` for JSON shapes.

---

## 8. Health & Observability

| Method | Path       | Description                                                      | Permission                            |
| ------ | ---------- | ---------------------------------------------------------------- | ------------------------------------- |
| GET    | `/health`  | Basic process health (database connectivity).                    | Public                                |
| GET    | `/metrics` | Prometheus metrics (must be protected at ingress in production). | Public (recommend reverse-proxy auth) |

Metrics include request latency histograms (`shellcn_api_latency_seconds`) and Go runtime stats.

---

## 9. Error Catalogue (excerpt)

| Code                       | HTTP | Meaning                            | Notes                                       |
| -------------------------- | ---- | ---------------------------------- | ------------------------------------------- |
| `auth.invalid_credentials` | 401  | Username/password mismatch.        | Triggers lockout after configured attempts. |
| `auth.mfa_required`        | 401  | MFA token required.                | Response contains recovery paths.           |
| `auth.token_revoked`       | 401  | Refresh token no longer valid.     | Client must prompt for login.               |
| `permission.denied`        | 403  | Missing permission.                | Response includes missing ID.               |
| `resource.not_found`       | 404  | Entity not found.                  | Applies to users, orgs, teams, roles.       |
| `validation.failed`        | 422  | Request payload validation failed. | `data` object includes field errors.        |
| `internal.server_error`    | 500  | Unexpected error.                  | Logged with correlation ID.                 |

---

## 10. Webhooks & Future Work

Webhooks are not part of Phase 1. When implemented they will emit signed JSON payloads for audit events and session lifecycle changes. Subscribe to roadmap updates for schema details.

---

## 11. Change Log

- **2024-10-08** — Initial draft covering core authentication, identity, permissions, and provider administration APIs for Phase 7 deliverables.
