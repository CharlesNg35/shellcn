# Authentication Provider UI Configuration

**Status:** Required Feature  
**Priority:** High  
**Module:** Core

---

## Overview

All authentication providers (OIDC, SAML, LDAP) are configured through the admin UI, **NOT** through configuration files or environment variables. This provides a better user experience and allows administrators to manage authentication methods without requiring server restarts or file system access.

---

## Supported Authentication Providers

### 1. **Local Authentication** (Always Enabled)
- **Type:** `local`
- **Description:** Username and password authentication
- **Settings:**
  - Allow Registration: Enable/disable user self-registration
  - Require Email Confirmation: When enabled, newly registered users must confirm their email via verification link before activation
- **Notes:** Cannot be disabled (always available as fallback)

### 2. **Email Invitation** (Optional)
- **Type:** `invite`
- **Description:** Invite users via email
- **Settings:**
  - Enabled: Yes/No
  - Require Email Verification: Yes/No
- **Notes:** System provider, cannot be deleted

### 3. **OpenID Connect (OIDC)** (Optional)
- **Type:** `oidc`
- **Description:** Single Sign-On via OpenID Connect
- **Configuration:**
  - Issuer URL
  - Client ID
  - Client Secret (encrypted)
  - Redirect URL
  - Scopes (e.g., `openid profile email`)
- **Examples:** Google, Azure AD, Okta, Keycloak

### 4. **SAML 2.0** (Optional)
- **Type:** `saml`
- **Description:** SAML 2.0 Single Sign-On
- **Configuration:**
  - Metadata URL
  - Entity ID
  - SSO URL
  - Certificate
  - Private Key (encrypted)
  - Attribute Mapping (SAML attributes → user fields)
- **Examples:** Azure AD, Okta, OneLogin

### 5. **LDAP / Active Directory** (Optional)
- **Type:** `ldap`
- **Description:** LDAP or Active Directory authentication
- **Configuration:**
  - Host
  - Port
  - Base DN
  - Bind DN
  - Bind Password (encrypted)
  - User Filter
  - Use TLS
  - Skip Certificate Verification
  - Attribute Mapping (LDAP attributes → user fields)
- **Examples:** Active Directory, OpenLDAP, FreeIPA

---

## Database Schema

### AuthProvider Model

```go
type AuthProvider struct {
    ID          string    `gorm:"primaryKey;type:uuid"`
    Type        string    `gorm:"not null;uniqueIndex"` // local, oidc, oauth2, saml, ldap, invite
    Name        string    `gorm:"not null"`
    Enabled     bool      `gorm:"default:false"`
    Config      string    `gorm:"type:json"` // Provider-specific config (encrypted)
    
    // Local provider settings
    AllowRegistration bool  `gorm:"default:false"`
    
    // Invite settings
    RequireEmailVerification bool `gorm:"default:true"`
    
    Description string
    Icon        string
    CreatedAt   time.Time
    UpdatedAt   time.Time
    CreatedBy   string    `gorm:"type:uuid"` // Admin who configured it
}
```

---

## Backend Implementation

### API Endpoints

**Public (for login page):**
- `GET /api/auth/providers` - List enabled providers

**Admin Only (requires `permission.manage`):**
- `GET /api/auth/providers/all` - List all providers
- `GET /api/auth/providers/:type` - Get provider config
- `POST /api/auth/providers/oidc` - Configure OIDC
- `POST /api/auth/providers/saml` - Configure SAML
- `POST /api/auth/providers/ldap` - Configure LDAP
- `PUT /api/auth/providers/local` - Update local settings
- `PUT /api/auth/providers/invite` - Update invite settings
- `PUT /api/auth/providers/:type/enable` - Enable provider
- `PUT /api/auth/providers/:type/disable` - Disable provider
- `POST /api/auth/providers/:type/test` - Test connection
- `DELETE /api/auth/providers/:type` - Delete provider

### Service Layer

**Location:** `internal/services/auth_provider_service.go`

Key methods:
- `List()` - List all providers (redact sensitive config)
- `GetEnabled()` - Get enabled providers (for login page)
- `ConfigureOIDC()` - Configure OIDC provider
- `ConfigureSAML()` - Configure SAML provider
- `ConfigureLDAP()` - Configure LDAP provider
- `UpdateLocalSettings()` - Update local auth settings
- `UpdateInviteSettings()` - Update invite settings
- `SetEnabled()` - Enable/disable provider
- `TestConnection()` - Test provider connection (LDAP, OIDC)

### Security

1. **Encryption:** All sensitive fields (client secrets, private keys, passwords) are encrypted using AES-256-GCM before storage
2. **Permissions:** Only admins with `permission.manage` can configure providers
3. **Audit Logging:** All provider configuration changes are logged
4. **Validation:** Provider configs are validated before saving
5. **Local Auth Protection:** Local authentication cannot be disabled (always available as fallback)

---

## Frontend Implementation

### Auth Providers Page

**Location:** `src/pages/settings/AuthProviders.tsx`

**Features:**
- Grid view of all available providers
- Visual status indicators (enabled/disabled, configured/not configured)
- Configure button for each provider
- Enable/disable toggle switches
- Modal forms for configuration

### Provider Configuration Forms

Each provider type has its own configuration form:

1. **LocalSettingsForm** - Allow registration toggle
2. **InviteSettingsForm** - Enable/disable, email verification
3. **OIDCConfigForm** - OIDC configuration
4. **SAMLConfigForm** - SAML configuration
5. **LDAPConfigForm** - LDAP configuration

### Components

**ProviderCard:**
- Shows provider name, icon, description
- Status badge (Active/Inactive)
- Configuration status (Configured/Not configured)
- Enable/disable toggle
- Configure button

### API Integration

**Location:** `src/lib/api/authProviders.ts`

Methods:
- `getEnabled()` - For login page
- `getAll()` - For admin page
- `configureOIDC()` - Configure OIDC
- `configureSAML()` - Configure SAML
- `configureLDAP()` - Configure LDAP
- `updateLocal()` - Update local settings
- `updateInvite()` - Update invite settings
- `enable()` - Enable provider
- `disable()` - Disable provider
- `testConnection()` - Test connection
- `delete()` - Delete provider

---

## User Experience Flow

### Admin Configuring OIDC

1. Navigate to **Settings → Auth Providers**
2. Click **Configure** on the OIDC card
3. Fill in the configuration form:
   - Issuer URL (e.g., `https://accounts.google.com`)
   - Client ID
   - Client Secret
   - Redirect URL (auto-filled)
   - Scopes (default: `openid profile email`)
4. Optionally check "Enable this provider immediately"
5. Click **Save Configuration**
6. Provider is now configured and can be enabled/disabled with toggle

### User Logging In

1. Navigate to login page
2. See all enabled authentication providers:
   - Local login form (always visible)
   - SSO buttons for enabled providers (OIDC, SAML)
3. Click on desired provider
4. Redirected to provider's login page
5. After successful authentication, redirected back to ShellCN

---

## Migration from Config Files

**Old Approach (NOT USED):**
```bash
# Environment variables
ENABLE_OIDC=true
OIDC_ISSUER=https://accounts.google.com
OIDC_CLIENT_ID=xxx
OIDC_CLIENT_SECRET=yyy
```

**New Approach (CURRENT):**
- All configuration done through UI
- No environment variables needed
- No server restart required
- Changes take effect immediately
- Audit trail of all changes

---

## Benefits

1. **User-Friendly:** No need to edit config files or environment variables
2. **No Server Restart:** Changes take effect immediately
3. **Audit Trail:** All configuration changes are logged
4. **Security:** Sensitive data encrypted at rest
5. **Flexibility:** Enable/disable providers on the fly
6. **Multi-Admin:** Multiple admins can manage providers
7. **Testing:** Built-in connection testing for LDAP and OIDC
8. **Self-Service:** Admins don't need server access

---

## Implementation Checklist

### Backend
- [x] Create AuthProvider model
- [x] Add migration for auth_providers table
- [x] Implement AuthProviderService
- [x] Create API endpoints
- [x] Add permission checks (`permission.manage`)
- [x] Implement encryption for sensitive fields
- [x] Add audit logging
- [x] Implement connection testing
- [x] Seed default providers (local, invite)

### Frontend
- [x] Create AuthProviders page
- [x] Create ProviderCard component
- [x] Create configuration forms for each provider type
- [x] Implement authProviders API module
- [x] Create useAuthProviders hook
- [x] Add route to settings
- [x] Add navigation link in sidebar
- [x] Implement enable/disable toggle
- [x] Add permission guard

### Testing
- [ ] Test provider configuration
- [ ] Test enable/disable functionality
- [ ] Test connection testing
- [ ] Test encryption/decryption
- [ ] Test permission checks
- [ ] Test audit logging
- [ ] E2E test: Configure OIDC provider
- [ ] E2E test: Login with OIDC

---

## Notes

- Local authentication is always enabled and cannot be disabled
- System providers (local, invite) cannot be deleted
- All sensitive configuration data is encrypted before storage
- Only admins with `permission.manage` can configure providers
- Provider configuration changes are immediately effective (no restart needed)
- The login page dynamically shows enabled providers
