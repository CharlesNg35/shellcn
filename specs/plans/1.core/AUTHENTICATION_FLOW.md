# Authentication Flow Overview

This document describes how ShellCN authenticates users across all supported login mechanisms. It ties together the UI, API, services, and provider integrations implemented in the backend.

---

## Entry Points

### Login Page

- Always renders the local username/password form.
- Dynamically fetches `/api/auth/providers` to determine which external providers are available.
  - Each provider comes with a `type`, `flow`, `allow_registration`, and display metadata.
  - `flow = password` → handled directly via `/api/auth/login`.
  - `flow = redirect` → use begin/callback endpoints.

### API Endpoints

- `POST /api/auth/login`
  - `provider` omitted or `local`: local authentication.
  - `provider = ldap`: LDAP password validation + auto-provisioning.
- `GET /api/auth/providers/:type/login`
  - Begins redirect-based SSO flows (OIDC, SAML).
- `GET /api/auth/providers/:type/callback`
  - Handles the provider response, issues session tokens, and redirects back to the UI.
- `GET /api/auth/providers/:type/metadata`
  - SAML SP metadata for IdP configuration.
- `POST /api/auth/refresh`
  - Rotates refresh tokens.
- `POST /api/auth/logout`
  - Revokes the active session.

---

## Route Reference

| Route                                 | Method | Audience                            | Purpose                                                      |
| ------------------------------------- | ------ | ----------------------------------- | ------------------------------------------------------------ |
| `/api/auth/login`                     | `POST` | Public                              | Local / LDAP login (select via `provider`)                   |
| `/api/auth/refresh`                   | `POST` | Public                              | Rotate refresh token for JWT                                 |
| `/api/auth/logout`                    | `POST` | Authenticated                       | Revoke active session                                        |
| `/api/auth/providers`                 | `GET`  | Public                              | List login-page providers with `flow` + `allow_registration` |
| `/api/auth/providers/:type/login`     | `GET`  | Public                              | Begin redirect SSO (OIDC/SAML)                               |
| `/api/auth/providers/:type/callback`  | `GET`  | Public                              | Complete redirect SSO                                        |
| `/api/auth/providers/:type/metadata`  | `GET`  | Public                              | SAML SP metadata (XML)                                       |
| `/api/auth/providers/enabled`         | `GET`  | Authenticated (`permission.view`)   | Admin view of enabled providers (without secret config)      |
| `/api/auth/providers/all`             | `GET`  | Authenticated (`permission.view`)   | Admin list of all providers                                  |
| `/api/auth/providers/:type/configure` | `POST` | Authenticated (`permission.manage`) | Configure provider secret + enablement                       |
| `/api/auth/providers/:type/enable`    | `POST` | Authenticated (`permission.manage`) | Toggle enabled flag                                          |
| `/api/auth/providers/:type/test`      | `POST` | Authenticated (`permission.manage`) | Connection test (LDAP/OIDC)                                  |
| `/api/auth/providers/local/settings`  | `POST` | Authenticated (`permission.manage`) | Update local auth registration rules                         |

---

## Authentication Paths

### 1. Local Username/Password

1. User submits credentials via UI → `POST /api/auth/login`.
2. `AuthHandler.handleLocalLogin` constructs a `LocalProvider` and verifies the password, handling account lockouts and activity updates.
3. On success, `SessionService.CreateSession` issues access/refresh tokens.
4. Response payload includes `tokens`, user profile, and current permissions.

### 2. LDAP Directory

1. UI sends `POST /api/auth/login` with `{ provider: "ldap", identifier, password }`.
2. `AuthProviderService.LoadLDAPConfig` returns the decrypted LDAP settings.
3. `LDAPAuthenticator` binds with the service account (optional), searches the user, and binds with their password.
4. Returned attributes are mapped to the `Identity` struct.
5. `SSOManager.Resolve` ensures a local user exists (auto-provision if enabled) and delegates session issuance to `SessionService`.

### 3. OIDC Redirect Flow

1. UI or frontend route hits `GET /api/auth/providers/oidc/login`.
2. `SSOHandler.Begin`:
   - Generates state + PKCE challenge.
   - Persists flow metadata using `StateCodec`.
   - Redirects to the provider’s authorization URL via `OIDCProvider.Begin`.
3. User authenticates with IdP and is redirected to `/api/auth/providers/oidc/callback`.
4. `SSOHandler.Callback`:
   - Validates state & nonce.
   - Exchanges code for tokens via `OIDCProvider.Callback`.
   - Maps claims to the ShellCN identity.
5. `SSOManager.Resolve` upserts/loads the user and issues platform tokens.

### 4. SAML Redirect Flow

1. UI triggers `GET /api/auth/providers/saml/login`.
2. `SSOHandler.Begin` calls `SAMLProvider.Begin`, producing an AuthnRequest (HTTP-Redirect binding) and state.
3. User signs in at the IdP, which posts the assertion to `/api/auth/providers/saml/callback`.
4. `SAMLProvider.Callback` validates the response via `ServiceProvider.ParseResponse`.
5. `SSOManager.Resolve` maps SAML attributes to a local user and issues tokens.

---

## Session Issuance & Metadata

- `SessionService.CreateSession` is the core token generator.
- External flows use `SessionService.CreateForSubject`, which merges provider metadata (e.g., `sso_provider`, `sso_subject`) into JWT claims.
- Token pair: access JWT (short-lived) + refresh token stored in DB/cache.
- `SessionService.RefreshSession` rotates refresh tokens with revocation checks.
- `SessionService.RevokeSession` and `SessionService.CleanupExpired` maintain hygiene.

---

## Identity Resolution & Provisioning

- `SSOManager.Resolve`:
  - Looks up existing users by email (case-insensitive).
  - Auto-provisions new users if the provider allows it, generating a random placeholder password.
  - Updates last login timestamp/IP.
- Attribute mapping per provider:
  - OIDC: configurable via claim keys (`email`, `given_name`, etc.).
  - SAML: `AttributeMapping` (e.g., `email`, `first_name`, `last_name`, `groups`).
  - LDAP: attribute mapping for `email`, `display_name`, `groups`, `username`.
- Group information is stored in the JWT metadata (`sso_groups`) for feature-specific logic (future use).

---

## Configuration & Security

- Auth provider configurations are stored in the `auth_providers` table with secrets encrypted using the vault key.
- `AuthProviderService.Configure*` methods handle encryption and audit logging.
- Providers can be enabled/disabled and flagged for self-registration.
- `AuthProviderService.GetEnabledPublic` returns only enabled providers plus local auth for the login page.
- SAML metadata endpoint allows IdPs to fetch SP info (`/api/auth/providers/saml/metadata`).

### Determining Provider Availability

- **Login UI** should read `GET /api/auth/providers` and filter on:
  - `enabled` and `flow` to decide rendering.
  - For the **local** provider, `allow_registration = true` means public self-registration is available (`POST /api/auth/register` to be implemented by frontend).
  - For external providers (OIDC/SAML/LDAP), `allow_registration` governs _auto-provisioning_ on first login rather than exposing a public signup form.
- **Admin UI** can use:
  - `GET /api/auth/providers/enabled` for an overview without secrets.
  - `GET /api/auth/providers/all` or the configure endpoints to retrieve specific stored settings (redacted).
- **Enabling/Disabling** is performed via `POST /api/auth/providers/:type/enable` with `{ "enabled": true/false }`.
- **Configuration** requires `POST /api/auth/providers/:type/configure` passing the provider-specific `config` payload plus `allow_registration`.
- **Public Registration** is controlled per provider:
  - Local provider toggled via `POST /api/auth/providers/local/settings`.
  - External providers (OIDC/SAML/LDAP) accept an `allow_registration` flag during configuration. When true, `SSOManager.Resolve` will auto-provision new accounts on first login.

---

## Error Handling & Metrics

- All login paths increment `metrics.AuthAttempts` with success/failure labels.
- Standard HTTP responses:
  - `401 Unauthorized` for invalid credentials or user disabled.
  - `400 Bad Request` for malformed requests.
  - `500 Internal Server Error` for unexpected issues (audit logged).
- Redirect flows fall back to `error_redirect` (default `/login?error=sso_failed`).

---

## Future Enhancements

- Support additional redirect providers (custom IdPs) by registering new `Descriptor` implementations.
- Enrich frontend SSO UI with provider status, self-registration indicators, and matching error states.
- Extend SAML support with Artifact binding and signed AuthnRequests if needed.
