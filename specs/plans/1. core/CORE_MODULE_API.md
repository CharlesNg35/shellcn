# Core Module API Documentation

**Base URL:** `https://{host}:{port}` (default `http://localhost:8000`)
**Version:** v1 (stabilised once backend reaches GA)
**Content Type:** `application/json` unless otherwise specified

---

## 1. Conventions

### 1.1 Authentication

- **Public endpoints:**

  - `POST /api/auth/login`
  - `POST /api/auth/refresh`
  - `GET /api/auth/providers`
  - `GET /api/auth/providers/:type/login`
  - `GET /api/auth/providers/:type/callback`
  - `GET /api/auth/providers/:type/metadata`
  - `GET /api/setup/status`
  - `POST /api/setup/initialize`
  - `GET /health`
  - `GET /metrics` (Prometheus)

- **Protected endpoints:** All other endpoints require a bearer token in the `Authorization` header:

  ```
  Authorization: Bearer <access-token>
  ```

- Access tokens are short-lived JWTs (default 15 minutes), refresh tokens are stored server-side via the session service (default 7 days).

### 1.2 Response Envelope

Every endpoint returns the standard envelope defined in `pkg/response`:

**Success Response:**

```json
{
  "success": true,
  "data": {...}
}
```

**Success with Pagination:**

```json
{
  "success": true,
  "data": [...],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 45,
    "total_pages": 3
  }
}
```

**Error Response:**

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

**Root/Superuser Bypass:** Users with `is_root=true` bypass all permission checks.

---

## 2. Authentication & Session Management

### 2.1 Authentication Endpoints

| Method | Path                                 | Description                                                                                             | Permission    | Handler                      |
| ------ | ------------------------------------ | ------------------------------------------------------------------------------------------------------- | ------------- | ---------------------------- |
| POST   | `/api/auth/login`                    | Authenticate with username/password, optional MFA challenge. Returns access/refresh tokens and profile. | Public        | `AuthHandler.Login`          |
| POST   | `/api/auth/refresh`                  | Exchange refresh token for new access token pair.                                                       | Public        | `AuthHandler.Refresh`        |
| GET    | `/api/auth/providers`                | List enabled external providers with UI metadata (for login page).                                      | Public        | `ProviderHandler.ListPublic` |
| GET    | `/api/auth/providers/:type/login`    | Initiate SSO redirect (OIDC/SAML).                                                                      | Public        | `SSOHandler.Begin`           |
| GET    | `/api/auth/providers/:type/callback` | SSO callback handler.                                                                                   | Public        | `SSOHandler.Callback`        |
| GET    | `/api/auth/providers/:type/metadata` | Provider metadata (e.g., SAML SP metadata XML).                                                         | Public        | `SSOHandler.Metadata`        |
| GET    | `/api/auth/me`                       | Current user profile, roles, and permissions.                                                           | Authenticated | `AuthHandler.Me`             |
| POST   | `/api/auth/logout`                   | Invalidate current refresh token and session.                                                           | Authenticated | `AuthHandler.Logout`         |

**Sample** — Login Request:

```http
POST /api/auth/login
Content-Type: application/json

{
  "identifier": "alice",      // username or email
  "password": "Secret123!",
  "mfa_token": "123456"        // optional, required if MFA enabled
}
```

**Sample** — Login Response:

```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "c509fffa-...",
    "expires_in": 900,
    "user": {
      "id": "usr_01H...",
      "username": "alice",
      "email": "alice@example.com",
      "first_name": "Alice",
      "last_name": "Smith",
      "is_root": false,
      "is_active": true,
      "roles": [
        {
          "id": "admin",
          "name": "Administrator",
          "description": "Full system access"
        }
      ],
      "permissions": ["user.view", "user.create", "org.manage", "..."]
    }
  }
}
```

**Sample** — Refresh Token Request:

```http
POST /api/auth/refresh
Content-Type: application/json

{
  "refresh_token": "c509fffa-..."
}
```

**Sample** — Get Current User:

```http
GET /api/auth/me
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

Response:

```json
{
  "success": true,
  "data": {
    "id": "usr_01H...",
    "username": "alice",
    "email": "alice@example.com",
    "first_name": "Alice",
    "last_name": "Smith",
    "is_root": false,
    "is_active": true,
    "organization": {
      "id": "org_01H...",
      "name": "Acme Corp"
    },
    "teams": [
      {
        "id": "team_01H...",
        "name": "Engineering"
      }
    ],
    "roles": [...],
    "permissions": [...]
  }
}
```

### 2.2 Session Management

| Method | Path                       | Description                                 | Permission    | Handler                         |
| ------ | -------------------------- | ------------------------------------------- | ------------- | ------------------------------- |
| GET    | `/api/sessions/me`         | List active sessions for the current user.  | Authenticated | `SessionHandler.ListMySessions` |
| POST   | `/api/sessions/revoke/:id` | Revoke a single session by ID.              | Authenticated | `SessionHandler.Revoke`         |
| POST   | `/api/sessions/revoke_all` | Revoke all other sessions (except current). | Authenticated | `SessionHandler.RevokeAll`      |

**Sample** — List My Sessions:

```http
GET /api/sessions/me
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": "sess_01H...",
      "user_id": "usr_01H...",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0...",
      "device_name": "Chrome on MacOS",
      "created_at": "2025-10-08T10:00:00Z",
      "last_used_at": "2025-10-08T14:30:00Z",
      "expires_at": "2025-10-15T10:00:00Z",
      "is_current": true
    },
    {
      "id": "sess_02H...",
      "user_id": "usr_01H...",
      "ip_address": "192.168.1.101",
      "user_agent": "Mozilla/5.0...",
      "device_name": "Firefox on Windows",
      "created_at": "2025-10-07T09:00:00Z",
      "last_used_at": "2025-10-07T18:00:00Z",
      "expires_at": "2025-10-14T09:00:00Z",
      "is_current": false
    }
  ]
}
```

---

## 3. Setup Workflow

| Method | Path                    | Description                                            | Permission                           | Handler                   |
| ------ | ----------------------- | ------------------------------------------------------ | ------------------------------------ | ------------------------- |
| GET    | `/api/setup/status`     | Returns `"pending"` until first admin exists.          | Public                               | `SetupHandler.Status`     |
| POST   | `/api/setup/initialize` | Creates initial root admin and bootstrap organisation. | Public (guarded by empty user table) | `SetupHandler.Initialize` |

**Sample** — Check Setup Status:

```http
GET /api/setup/status
```

Response (when setup needed):

```json
{
  "success": true,
  "data": {
    "status": "pending",
    "message": "Initial setup required"
  }
}
```

Response (when setup complete):

```json
{
  "success": true,
  "data": {
    "status": "complete",
    "message": "System is configured"
  }
}
```

**Sample** — Initialize Setup:

```http
POST /api/setup/initialize
Content-Type: application/json

{
  "username": "root",
  "email": "root@example.com",
  "password": "ChangeMe123!",
  "first_name": "System",
  "last_name": "Administrator"
}
```

Response:

```json
{
  "success": true,
  "data": {
    "user": {
      "id": "usr_01H...",
      "username": "root",
      "email": "root@example.com",
      "is_root": true,
      "is_active": true
    },
    "message": "Setup completed successfully. Please login."
  }
}
```

---

## 4. User & Identity Management

| Method | Path             | Description                                                   | Permission    | Handler              |
| ------ | ---------------- | ------------------------------------------------------------- | ------------- | -------------------- |
| GET    | `/api/users`     | Paginated list of users (query `page`, `per_page`, `search`). | `user.view`   | `UserHandler.List`   |
| GET    | `/api/users/:id` | Retrieve user details with role assignments.                  | `user.view`   | `UserHandler.Get`    |
| POST   | `/api/users`     | Create user, roles, and optional activation toggle.           | `user.create` | `UserHandler.Create` |

**Sample** — List Users:

```http
GET /api/users?page=1&per_page=20&search=alice
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": "usr_01H...",
      "username": "alice",
      "email": "alice@example.com",
      "first_name": "Alice",
      "last_name": "Smith",
      "is_root": false,
      "is_active": true,
      "organization_id": "org_01H...",
      "created_at": "2025-10-01T10:00:00Z",
      "last_login_at": "2025-10-08T14:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

**Sample** — Create User:

```http
POST /api/users
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "username": "bob",
  "email": "bob@example.com",
  "password": "SecurePass123!",
  "first_name": "Bob",
  "last_name": "Johnson",
  "organization_id": "org_01H...",
  "is_active": true
}
```

### 4.1 Organisations & Teams

#### Organizations

| Method | Path                              | Description                    | Permission   | Handler                      |
| ------ | --------------------------------- | ------------------------------ | ------------ | ---------------------------- |
| GET    | `/api/orgs`                       | List organisations.            | `org.view`   | `OrganizationHandler.List`   |
| GET    | `/api/orgs/:id`                   | Get organisation detail.       | `org.view`   | `OrganizationHandler.Get`    |
| POST   | `/api/orgs`                       | Create organisation.           | `org.create` | `OrganizationHandler.Create` |
| PATCH  | `/api/orgs/:id`                   | Update display name, metadata. | `org.manage` | `OrganizationHandler.Update` |
| DELETE | `/api/orgs/:id`                   | Soft delete organisation.      | `org.manage` | `OrganizationHandler.Delete` |
| GET    | `/api/organizations/:orgID/teams` | List teams for organisation.   | `org.view`   | `TeamHandler.ListByOrg`      |

**Sample** — Create Organization:

```http
POST /api/orgs
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Engineering Department",
  "description": "Main engineering team"
}
```

#### Teams

| Method | Path                             | Description         | Permission   | Handler                    |
| ------ | -------------------------------- | ------------------- | ------------ | -------------------------- |
| GET    | `/api/teams/:id`                 | Team & members.     | `org.view`   | `TeamHandler.Get`          |
| POST   | `/api/teams`                     | Create team.        | `org.manage` | `TeamHandler.Create`       |
| PATCH  | `/api/teams/:id`                 | Rename/update team. | `org.manage` | `TeamHandler.Update`       |
| POST   | `/api/teams/:id/members`         | Append member IDs.  | `org.manage` | `TeamHandler.AddMember`    |
| DELETE | `/api/teams/:id/members/:userID` | Remove member.      | `org.manage` | `TeamHandler.RemoveMember` |
| GET    | `/api/teams/:id/members`         | List members.       | `org.view`   | `TeamHandler.ListMembers`  |

**Sample** — Create Team:

```http
POST /api/teams
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Backend Team",
  "description": "Backend developers",
  "organization_id": "org_01H..."
}
```

**Sample** — Add Team Member:

```http
POST /api/teams/team_01H.../members
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "user_ids": ["usr_01H...", "usr_02H..."]
}
```

---

## 5. Permissions & Roles

| Method | Path                                     | Description                                       | Permission          | Handler                                |
| ------ | ---------------------------------------- | ------------------------------------------------- | ------------------- | -------------------------------------- |
| GET    | `/api/permissions/registry`              | Tree of registered permissions with dependencies. | `permission.view`   | `PermissionHandler.Registry`           |
| GET    | `/api/permissions/my`                    | List permissions for current user.                | Authenticated       | `PermissionHandler.MyPermissions`      |
| GET    | `/api/permissions/roles`                 | List roles and assigned permissions.              | `permission.view`   | `PermissionHandler.ListRoles`          |
| POST   | `/api/permissions/roles`                 | Create role.                                      | `permission.manage` | `PermissionHandler.CreateRole`         |
| PATCH  | `/api/permissions/roles/:id`             | Update role name/description.                     | `permission.manage` | `PermissionHandler.UpdateRole`         |
| DELETE | `/api/permissions/roles/:id`             | Delete role (prevent delete of system roles).     | `permission.manage` | `PermissionHandler.DeleteRole`         |
| POST   | `/api/permissions/roles/:id/permissions` | Replace permission set for role.                  | `permission.manage` | `PermissionHandler.SetRolePermissions` |

**Sample** — Get Permission Registry:

```http
GET /api/permissions/registry
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": {
    "user.view": {
      "id": "user.view",
      "module": "core",
      "description": "View users",
      "depends_on": []
    },
    "user.create": {
      "id": "user.create",
      "module": "core",
      "description": "Create new users",
      "depends_on": ["user.view"]
    },
    "user.edit": {
      "id": "user.edit",
      "module": "core",
      "description": "Edit user details",
      "depends_on": ["user.view"]
    },
    "user.delete": {
      "id": "user.delete",
      "module": "core",
      "description": "Delete users",
      "depends_on": ["user.view", "user.edit"]
    }
  }
}
```

**Sample** — List Roles:

```http
GET /api/permissions/roles
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": "admin",
      "name": "Administrator",
      "description": "Full system access",
      "is_system": true,
      "permissions": [
        {
          "id": "user.view",
          "module": "core",
          "description": "View users"
        },
        {
          "id": "user.create",
          "module": "core",
          "description": "Create new users"
        }
      ],
      "created_at": "2025-10-01T10:00:00Z"
    }
  ]
}
```

**Sample** — Create Role:

```http
POST /api/permissions/roles
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Developer",
  "description": "Development team access",
  "is_system": false
}
```

**Sample** — Set Role Permissions:

```http
POST /api/permissions/roles/role_01H.../permissions
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "permissions": ["user.view", "org.view", "audit.view"]
}
```

---

## 6. Audit & Security

| Method | Path                  | Description                                                                             | Permission       | Handler                 |
| ------ | --------------------- | --------------------------------------------------------------------------------------- | ---------------- | ----------------------- |
| GET    | `/api/audit`          | Paginated audit events (`page`, `per_page`, `actor`, `action`, `result`, `from`, `to`). | `audit.view`     | `AuditHandler.List`     |
| GET    | `/api/audit/export`   | CSV export filtered by same parameters.                                                 | `audit.export`   | `AuditHandler.Export`   |
| GET    | `/api/security/audit` | Security-focused view (failed logins, privilege escalation attempts).                   | `security.audit` | `SecurityHandler.Audit` |

**Sample** — List Audit Logs:

```http
GET /api/audit?page=1&per_page=50&action=user.create&result=success&from=2025-10-01&to=2025-10-08
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": "audit_01H...",
      "user_id": "usr_01H...",
      "username": "alice",
      "action": "user.create",
      "resource": "user:usr_02H...",
      "result": "success",
      "ip_address": "192.168.1.100",
      "user_agent": "Mozilla/5.0...",
      "metadata": {
        "username": "bob",
        "email": "bob@example.com"
      },
      "created_at": "2025-10-08T10:30:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "per_page": 50,
    "total": 1,
    "total_pages": 1
  }
}
```

**Sample** — Export Audit Logs (CSV):

```http
GET /api/audit/export?action=auth.login&result=failure&from=2025-10-01&to=2025-10-08
Authorization: Bearer <access-token>
```

Response: CSV file download with headers:

```
id,user_id,username,action,resource,result,ip_address,user_agent,created_at
audit_01H...,usr_01H...,alice,auth.login,,failure,192.168.1.100,Mozilla/5.0...,2025-10-08T10:30:00Z
```

**Sample** — Security Audit:

```http
GET /api/security/audit
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": {
    "failed_logins": [
      {
        "username": "admin",
        "ip_address": "192.168.1.200",
        "attempts": 5,
        "last_attempt": "2025-10-08T14:30:00Z"
      }
    ],
    "permission_denials": [
      {
        "user_id": "usr_01H...",
        "username": "bob",
        "permission": "user.delete",
        "count": 3,
        "last_attempt": "2025-10-08T12:00:00Z"
      }
    ],
    "suspicious_activities": []
  }
}
```

Audit events include metadata such as `trace_id`, `actor`, `ip`, `resource`, `changes`, and `result`.

---

## 7. Authentication Provider Administration

**IMPORTANT:** All authentication providers are configured via UI by admins, not config files. Local auth is always enabled by default.

Administrative provider endpoints live under `/api/auth/providers`.

| Method | Path                                  | Description                                                         | Permission          | Handler                               |
| ------ | ------------------------------------- | ------------------------------------------------------------------- | ------------------- | ------------------------------------- |
| GET    | `/api/auth/providers/all`             | List provider configs and statuses (admin view).                    | `permission.view`   | `ProviderHandler.ListAll`             |
| GET    | `/api/auth/providers/enabled`         | Enabled providers for UI toggles.                                   | `permission.view`   | `ProviderHandler.GetEnabled`          |
| POST   | `/api/auth/providers/local/settings`  | Update local auth options (password policies, registration).        | `permission.manage` | `ProviderHandler.UpdateLocalSettings` |
| POST   | `/api/auth/providers/:type/configure` | Persist provider-specific configuration payload.                    | `permission.manage` | `ProviderHandler.Configure`           |
| POST   | `/api/auth/providers/:type/enable`    | Toggle provider enablement (`{"enabled":true}`).                    | `permission.manage` | `ProviderHandler.SetEnabled`          |
| POST   | `/api/auth/providers/:type/test`      | Connectivity test (OIDC discovery, SAML metadata parse, LDAP bind). | `permission.manage` | `ProviderHandler.TestConnection`      |

**Sample** — List All Providers (Admin):

```http
GET /api/auth/providers/all
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": [
    {
      "id": "prov_01H...",
      "type": "local",
      "name": "Local Authentication",
      "enabled": true,
      "allow_registration": false,
      "description": "Username and password authentication",
      "icon": "key",
      "created_at": "2025-10-01T10:00:00Z"
    },
    {
      "id": "prov_02H...",
      "type": "oidc",
      "name": "Google SSO",
      "enabled": true,
      "config": {
        "issuer": "https://accounts.google.com",
        "client_id": "123456789.apps.googleusercontent.com",
        "redirect_url": "https://shellcn.example.com/api/auth/providers/oidc/callback",
        "scopes": ["openid", "profile", "email"]
      },
      "description": "Sign in with Google",
      "icon": "google",
      "created_at": "2025-10-02T14:00:00Z"
    },
    {
      "id": "prov_03H...",
      "type": "invite",
      "name": "Email Invitation",
      "enabled": false,
      "require_email_verification": true,
      "description": "Invite users via email",
      "icon": "mail",
      "created_at": "2025-10-01T10:00:00Z"
    }
  ]
}
```

**Sample** — Configure OIDC Provider:

```http
POST /api/auth/providers/oidc/configure
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Google SSO",
  "enabled": true,
  "config": {
    "issuer": "https://accounts.google.com",
    "client_id": "123456789.apps.googleusercontent.com",
    "client_secret": "GOCSPX-...",
    "redirect_url": "https://shellcn.example.com/api/auth/providers/oidc/callback",
    "scopes": ["openid", "profile", "email"]
  }
}
```

**Sample** — Configure SAML Provider:

```http
POST /api/auth/providers/saml/configure
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Okta SAML",
  "enabled": true,
  "config": {
    "metadata_url": "https://dev-123456.okta.com/app/exk.../sso/saml/metadata",
    "entity_id": "https://shellcn.example.com",
    "sso_url": "https://dev-123456.okta.com/app/exk.../sso/saml",
    "certificate": "-----BEGIN CERTIFICATE-----\n...",
    "attribute_mapping": {
      "email": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
      "first_name": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
      "last_name": "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname"
    }
  }
}
```

**Sample** — Configure LDAP Provider:

```http
POST /api/auth/providers/ldap/configure
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "name": "Active Directory",
  "enabled": true,
  "config": {
    "host": "ldap.example.com",
    "port": 389,
    "base_dn": "dc=example,dc=com",
    "bind_dn": "cn=admin,dc=example,dc=com",
    "bind_password": "secret",
    "user_filter": "(uid={username})",
    "use_tls": true,
    "skip_verify": false,
    "attribute_mapping": {
      "username": "uid",
      "email": "mail",
      "first_name": "givenName",
      "last_name": "sn"
    }
  }
}
```

**Sample** — Update Local Settings:

```http
POST /api/auth/providers/local/settings
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "allow_registration": true
}
```

**Sample** — Enable/Disable Provider:

```http
POST /api/auth/providers/oidc/enable
Authorization: Bearer <access-token>
Content-Type: application/json

{
  "enabled": true
}
```

**Sample** — Test Provider Connection:

```http
POST /api/auth/providers/ldap/test
Authorization: Bearer <access-token>
```

Response:

```json
{
  "success": true,
  "data": {
    "status": "success",
    "message": "Successfully connected to LDAP server and authenticated",
    "details": {
      "server": "ldap.example.com:389",
      "tls": true,
      "bind_successful": true
    }
  }
}
```

**Configuration schemas** (payload `config` field):

- **OIDC**: `issuer`, `client_id`, `client_secret`, `redirect_url`, `scopes[]`
- **SAML**: `metadata_url`, `entity_id`, `sso_url`, `certificate`, `private_key`, `attribute_mapping{}`
- **LDAP**: `host`, `port`, `base_dn`, `bind_dn`, `bind_password`, `user_filter`, `use_tls`, `skip_verify`, `attribute_mapping{}`

See `internal/models/auth_provider.go` for complete JSON shapes.

---

## 8. Protocol Catalog & Connections

### 8.1 Protocol Catalog

| Method | Path                       | Description                                                           | Permission          | Handler                       |
| ------ | -------------------------- | --------------------------------------------------------------------- | ------------------- | ----------------------------- |
| GET    | `/api/protocols`           | Return the full driver catalog (driver + config enablement metadata). | `connection.view`   | `ProtocolHandler.ListAll`     |
| GET    | `/api/protocols/available` | Return only protocols available to the calling user.                  | `connection.view`   | `ProtocolHandler.ListForUser` |
| POST   | `/api/protocols/:id/test`  | Trigger a driver self-test/diagnostic (writes audit trail).           | `connection.manage` | `ProtocolHandler.TestDriver`  |

**ProtocolInfo Example**

```json
{
  "id": "docker",
  "name": "Docker",
  "module": "docker",
  "description": "Manage Docker hosts and containers",
  "category": "container",
  "icon": "Container",
  "default_port": 2376,
  "sort_order": 20,
  "features": ["terminal", "metrics"],
  "capabilities": {
    "terminal": true,
    "desktop": false,
    "file_transfer": false,
    "clipboard": false,
    "session_recording": false,
    "metrics": true,
    "reconnect": true,
    "extras": {
      "logs": true,
      "exec": true
    }
  },
  "driver_enabled": true,
  "config_enabled": true,
  "available": true
}
```

> **Permission profiles:** Every protocol driver registers `{driver}.connect`, `{driver}.manage`, and optional feature/admin scopes (e.g., `kubernetes.exec`, `docker.logs`, `database.query.read`). These depend on the core `connection.*` permissions defined in `internal/permissions/core.go`. See `specs/project/PROTOCOL_DRIVER_STANDARDS.md` for the driver contract.

### 8.2 Connections API

| Method | Path                           | Description                                                    | Permission                               | Handler                              |
| ------ | ------------------------------ | -------------------------------------------------------------- | ---------------------------------------- | ------------------------------------ |
| GET    | `/api/connections`             | List connections visible to the caller (supports filters).     | `connection.view`                        | `ConnectionHandler.List`             |
| GET    | `/api/connections/:id`         | Retrieve a specific connection with targets + visibility data. | `connection.view`                        | `ConnectionHandler.Get`              |
| POST   | `/api/connections`             | Create a connection (settings, visibility, targets).           | `connection.manage`                      | `ConnectionHandler.Create`           |
| PATCH  | `/api/connections/:id`         | Update connection metadata/settings/targets.                   | `connection.manage`                      | `ConnectionHandler.Update`           |
| DELETE | `/api/connections/:id`         | Delete a connection.                                           | `connection.manage`                      | `ConnectionHandler.Delete`           |
| POST   | `/api/connections/:id/share`   | Replace visibility ACLs (org/team/user scopes).                | `connection.share`                       | `ConnectionHandler.UpdateVisibility` |
| POST   | `/api/connections/:id/preview` | Run a dry-run or configuration validation.                     | `connection.launch` + `{driver}.connect` | `ConnectionHandler.LaunchPreview`    |

**Supported query parameters for `GET /api/connections`:**

- `protocol_id`: filter by driver.
- `organization_id`, `team_id`: scope to tenant subset.
- `search`: case-insensitive substring match across name, host, tags, metadata.
- `include=targets,visibility`: opt-in expansions (default `targets`).
- `page`, `per_page`: pagination controls (standard envelope).

**Connection payload**

```json
{
  "id": "conn_01J4TF5YBHW",
  "name": "Production Cluster",
  "description": "Primary Kubernetes control plane",
  "protocol_id": "kubernetes",
  "organization_id": "org_acme",
  "team_id": "team_platform",
  "owner_user_id": "usr_root",
  "metadata": {
    "tags": ["prod", "critical"],
    "favorite": true
  },
  "settings": {
    "context": "prod-main",
    "namespace": "platform",
    "api_server": "https://k8s.acme.io:6443"
  },
  "secret_id": "vault_secret_admin",
  "last_used_at": "2025-10-09T14:22:00Z",
  "targets": [
    {
      "id": "target_primary",
      "host": "k8s.acme.io",
      "port": 6443,
      "labels": {
        "role": "control-plane",
        "region": "us-east-1"
      },
      "ordering": 0
    }
  ],
  "visibility": [
    {
      "connection_id": "conn_01J4TF5YBHW",
      "organization_id": "org_acme",
      "team_id": null,
      "user_id": null,
      "permission_scope": "view"
    },
    {
      "connection_id": "conn_01J4TF5YBHW",
      "organization_id": null,
      "team_id": "team_platform",
      "user_id": null,
      "permission_scope": "use"
    }
  ]
}
```

> **Identity integration:** Drivers declare credential requirements (SSH key, kubeconfig, database DSN). The Identity service satisfies these bindings via `secret_id` or settings. The preview endpoint allows overriding identities for diagnostics.

### 8.3 Notifications

| Method | Path                            | Description                                      | Permission            | Handler                           |
| ------ | ------------------------------- | ------------------------------------------------ | --------------------- | --------------------------------- |
| GET    | `/api/notifications`            | List notifications for current user (paginated). | `notification.view`   | `NotificationHandler.List`        |
| POST   | `/api/notifications/read-all`   | Mark all notifications as read.                  | `notification.view`   | `NotificationHandler.MarkAllRead` |
| POST   | `/api/notifications`            | Create a notification (system/admin).            | `notification.manage` | `NotificationHandler.Create`      |
| POST   | `/api/notifications/:id/read`   | Mark one notification as read.                   | `notification.view`   | `NotificationHandler.MarkRead`    |
| POST   | `/api/notifications/:id/unread` | Mark one notification as unread.                 | `notification.view`   | `NotificationHandler.MarkUnread`  |
| DELETE | `/api/notifications/:id`        | Delete a notification.                           | `notification.view`   | `NotificationHandler.Delete`      |

- WebSocket stream: `GET /ws/notifications` (upgrade). Emits real-time notifications for the authenticated user.
- Pagination follows the standard envelope (`meta.page`, `meta.per_page`, `meta.total`).

---

## 9. Health & Observability

| Method | Path       | Description                                                      | Permission                            | Handler            |
| ------ | ---------- | ---------------------------------------------------------------- | ------------------------------------- | ------------------ |
| GET    | `/health`  | Basic process health (database connectivity).                    | Public                                | `handlers.Health`  |
| GET    | `/metrics` | Prometheus metrics (must be protected at ingress in production). | Public (recommend reverse-proxy auth) | Prometheus handler |

**Sample** — Health Check:

```http
GET /health
```

Response (healthy):

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "database": "connected",
    "timestamp": "2025-10-08T14:30:00Z"
  }
}
```

Response (unhealthy):

```json
{
  "success": false,
  "error": {
    "code": "health.database_error",
    "message": "Database connection failed"
  }
}
```

**Sample** — Prometheus Metrics:

```http
GET /metrics
```

Response (Prometheus text format):

```
# HELP shellcn_api_requests_total Total number of API requests
# TYPE shellcn_api_requests_total counter
shellcn_api_requests_total{method="GET",path="/api/users",status="200"} 1234

# HELP shellcn_api_latency_seconds API request latency
# TYPE shellcn_api_latency_seconds histogram
shellcn_api_latency_seconds_bucket{method="GET",path="/api/users",le="0.005"} 100
shellcn_api_latency_seconds_bucket{method="GET",path="/api/users",le="0.01"} 200
shellcn_api_latency_seconds_bucket{method="GET",path="/api/users",le="0.025"} 300
shellcn_api_latency_seconds_sum{method="GET",path="/api/users"} 12.5
shellcn_api_latency_seconds_count{method="GET",path="/api/users"} 1234

# HELP go_goroutines Number of goroutines
# TYPE go_goroutines gauge
go_goroutines 42
```

Metrics include request latency histograms (`shellcn_api_latency_seconds`), request counters, WebSocket connections, and Go runtime stats.

---

## 10. Error Catalogue (excerpt)

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

## 11. Webhooks & Future Work

Webhooks are not part of Phase 1. When implemented they will emit signed JSON payloads for audit events and session lifecycle changes. Subscribe to roadmap updates for schema details.

---

## 12. API Summary by Module

### Core Authentication (Public)

- `POST /api/auth/login` - Login with credentials
- `POST /api/auth/refresh` - Refresh access token
- `GET /api/auth/providers` - List enabled providers
- `GET /api/auth/providers/:type/login` - SSO redirect
- `GET /api/auth/providers/:type/callback` - SSO callback
- `GET /api/auth/providers/:type/metadata` - Provider metadata

### Core Authentication (Protected)

- `GET /api/auth/me` - Current user profile
- `POST /api/auth/logout` - Logout

### Setup (Public)

- `GET /api/setup/status` - Check setup status
- `POST /api/setup/initialize` - Initialize first admin

### Users

- `GET /api/users` - List users
- `GET /api/users/:id` - Get user
- `POST /api/users` - Create user

### Organizations

- `GET /api/orgs` - List organizations
- `GET /api/orgs/:id` - Get organization
- `POST /api/orgs` - Create organization
- `PATCH /api/orgs/:id` - Update organization
- `DELETE /api/orgs/:id` - Delete organization
- `GET /api/organizations/:orgID/teams` - List teams by org

### Teams

- `GET /api/teams/:id` - Get team
- `POST /api/teams` - Create team
- `PATCH /api/teams/:id` - Update team
- `POST /api/teams/:id/members` - Add members
- `DELETE /api/teams/:id/members/:userID` - Remove member
- `GET /api/teams/:id/members` - List members

### Permissions & Roles

- `GET /api/permissions/registry` - Permission registry
- `GET /api/permissions/my` - My permissions
- `GET /api/permissions/roles` - List roles
- `POST /api/permissions/roles` - Create role
- `PATCH /api/permissions/roles/:id` - Update role
- `DELETE /api/permissions/roles/:id` - Delete role
- `POST /api/permissions/roles/:id/permissions` - Set role permissions

### Sessions

- `GET /api/sessions/me` - List my sessions
- `POST /api/sessions/revoke/:id` - Revoke session
- `POST /api/sessions/revoke_all` - Revoke all sessions

### Notifications

- `GET /api/notifications` - List notifications
- `POST /api/notifications/read-all` - Mark all read
- `POST /api/notifications/:id/read` - Mark one read
- `POST /api/notifications/:id/unread` - Mark one unread
- `DELETE /api/notifications/:id` - Delete notification
- WebSocket: `GET /ws/notifications` - Real-time stream

### Audit & Security

- `GET /api/audit` - List audit logs
- `GET /api/audit/export` - Export audit logs (CSV)
- `GET /api/security/audit` - Security audit view

### Auth Provider Administration

- `GET /api/auth/providers/all` - List all providers (admin)
- `GET /api/auth/providers/enabled` - List enabled providers
- `POST /api/auth/providers/local/settings` - Update local settings
- `POST /api/auth/providers/:type/configure` - Configure provider
- `POST /api/auth/providers/:type/enable` - Enable/disable provider
- `POST /api/auth/providers/:type/test` - Test provider connection

### Health & Observability

- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

---

## 13. Change Log

- **2025-10-09** — Added Notifications section and documented `/api/permissions/my`; verified WebSocket `/ws/notifications` endpoint for real-time updates.

- **2025-10-08** — Comprehensive update with actual implementation details, complete request/response samples, handler references, and implementation checklist based on `internal/api/router.go` and route files.
- **2024-10-08** — Initial draft covering core authentication, identity, permissions, and provider administration APIs for Phase 7 deliverables.
