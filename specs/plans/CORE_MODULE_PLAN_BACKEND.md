# Core Module - Backend Implementation Plan

**Module:** Core (Auth, Users, Permissions)
**Status:** Required (Always Enabled)
**Dependencies:** None (Foundation Module)

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture & Project Structure](#architecture--project-structure)
3. [Database Schema & Models](#database-schema--models)
4. [Shared Packages (pkg/)](#shared-packages-pkg)
5. [Authentication System](#authentication-system)
6. [Authorization & Permission System](#authorization--permission-system)
7. [User Management](#user-management)
8. [Organization & Team Management](#organization--team-management)
9. [Session Management](#session-management)
10. [Audit Logging](#audit-logging)
11. [First-Time Setup](#first-time-setup)
12. [API Endpoints](#api-endpoints)
13. [Middleware](#middleware)
14. [Security Implementation](#security-implementation)
15. [Monitoring & Observability](#monitoring--observability)
16. [Testing Strategy](#testing-strategy)
17. [Implementation Checklist](#implementation-checklist)
18. [Authentication Flow](AUTHENTICATION_FLOW.md)

---

## Overview

The Core Module provides the foundational authentication, authorization, user management, and audit capabilities that all other modules depend on. This is the first module to implement.

### Key Features

- **Authentication:**

  - Local authentication (username/password)
  - External providers (OIDC, SAML, LDAP) - Optional
  - Multi-Factor Authentication (TOTP) - Optional
  - JWT-based session management
  - Password reset flow

- **Authorization:**

  - Role-Based Access Control (RBAC)
  - Permission registry with dependency resolution
  - Root/superuser bypass
  - Module-specific permission registration

- **User Management:**

  - User CRUD operations
  - Profile management
  - Password management
  - User activation/deactivation
  - Root user safeguards

- **Organization & Team Management:**

  - Multi-tenancy support
  - Organization hierarchy
  - Team-based access control
  - Member management

- **Session Management:**

  - Active session tracking
  - Session revocation
  - Multi-device support
  - Session sharing (for protocol sessions)

- **Audit Logging:**

  - Comprehensive event logging
  - User action tracking
  - Authentication attempts
  - Permission denials
  - Log export capabilities

- **First-Time Setup:**
  - UI-based setup wizard
  - First user creation (as superuser)
  - No default credentials
  - Auto-redirect when no users exist

---

## Architecture & Project Structure

### Package Layout

Follow the layered architecture defined in `BACKEND_PATTERNS.md`:

```
shellcn/
├── cmd/
│   └── server/
│       └── main.go                      # Application entry point
│
├── internal/                            # Private application code
│   ├── app/
│   │   ├── app.go                       # Application initialization
│   │   ├── config.go                    # Configuration management
│   │   └── server.go                    # HTTP server setup
│   │
│   ├── api/                             # API layer
│   │   ├── router.go                    # Route definitions
│   │   ├── middleware/
│   │   │   ├── auth.go                  # Authentication middleware
│   │   │   ├── cors.go                  # CORS middleware
│   │   │   ├── logger.go                # Logging middleware
│   │   │   ├── ratelimit.go             # Rate limiting
│   │   │   ├── metrics.go               # Prometheus metrics
│   │   │   ├── recovery.go              # Panic recovery
│   │   │   └── permission.go            # Permission checking
│   │   │
│   │   └── handlers/                    # HTTP handlers
│   │       ├── auth.go                  # Auth endpoints
│   │       ├── setup.go                 # First-time setup
│   │       ├── users.go                 # User management
│   │       ├── organizations.go         # Organization management
│   │       ├── teams.go                 # Team management
│   │       ├── permissions.go           # Permission management
│   │       ├── sessions.go              # Session management
│   │       ├── audit.go                 # Audit log endpoints
│   │       ├── health.go                # Health check
│   │       └── websocket.go             # WebSocket handler
│   │
│   ├── auth/                            # Authentication system
│   │   ├── auth.go                      # Core auth interface
│   │   ├── jwt.go                       # JWT implementation
│   │   ├── providers/
│   │   │   ├── local.go                 # Local auth
│   │   │   ├── oidc.go                  # OpenID Connect
│   │   │   ├── saml.go                  # SAML 2.0
│   │   │   └── ldap.go                  # LDAP/AD
│   │   │
│   │   └── mfa/
│   │       └── totp.go                  # TOTP implementation
│   │
│   ├── permissions/                     # Permission system
│   │   ├── registry.go                  # Permission registry
│   │   ├── core.go                      # Core permissions
│   │   ├── checker.go                   # Permission checker
│   │   ├── dependencies.go              # Dependency resolver
│   │   └── enforcer.go                  # Permission enforcement
│   │
│   ├── models/                          # Data models
│   │   ├── user.go
│   │   ├── organization.go
│   │   ├── team.go
│   │   ├── role.go
│   │   ├── permission.go
│   │   ├── session.go
│   │   ├── audit_log.go
│   │   ├── mfa_secret.go
│   │   └── password_reset_token.go
│   │
│   ├── database/                        # Database layer
│   │   ├── db.go                        # Database initialization
│   │   ├── sqlite.go                    # SQLite driver
│   │   ├── postgres.go                  # PostgreSQL driver (optional)
│   │   ├── mysql.go                     # MySQL driver (optional)
│   │   ├── migrations.go                # Migration runner
│   │   └── repositories/
│   │       ├── user_repository.go
│   │       ├── organization_repository.go
│   │       ├── team_repository.go
│   │       ├── role_repository.go
│   │       ├── permission_repository.go
│   │       ├── session_repository.go
│   │       └── audit_repository.go
│   │
│   └── services/                        # Business logic
│       ├── user_service.go
│       ├── auth_service.go
│       ├── organization_service.go
│       ├── team_service.go
│       ├── permission_service.go
│       ├── session_service.go
│       └── audit_service.go
│
├── pkg/                                 # PUBLIC shared packages
│   ├── logger/                          # Logging utilities
│   │   └── logger.go
│   ├── errors/                          # Error definitions
│   │   └── errors.go
│   ├── response/                        # API response helpers
│   │   └── response.go
│   ├── validator/                       # Input validation
│   │   └── validator.go
│   ├── crypto/                          # Encryption utilities
│   │   └── crypto.go
│   ├── websocket/                       # WebSocket utilities
│   │   └── websocket.go
│   ├── session/                         # Session management
│   │   └── session.go
│   └── testing/                         # Test utilities
│       └── testing.go
│
├── config/                              # Configuration files
│   └── config.yaml
│
├── go.mod
├── go.sum
└── Makefile
```

### Initialization Order

**Location:** `internal/app/app.go`

```go
package app

import (
    "log"

    // Import modules to trigger init() registration
    _ "shellcn/internal/permissions"
    _ "shellcn/internal/vault"

    "shellcn/pkg/logger"
    "shellcn/internal/database"
    "shellcn/internal/permissions"
)

type App struct {
    config *Config
    db     *gorm.DB
    server *Server
}

func New() (*App, error) {
    // 1. Load configuration
    config, err := LoadConfig()
    if err != nil {
        return nil, err
    }

    // 2. Initialize logger
    if err := logger.Init(config.LogLevel); err != nil {
        return nil, err
    }

    // 3. Initialize database
    db, err := database.Init(config.Database)
    if err != nil {
        return nil, err
    }

    // 4. Run migrations
    if err := database.AutoMigrate(db); err != nil {
        return nil, err
    }

    // 5. Validate permission dependencies
    if err := permissions.ValidateDependencies(); err != nil {
        log.Fatal("Permission dependency validation failed:", err)
    }

    logger.Info("Loaded permissions",
        zap.Int("count", len(permissions.GetAll())),
    )

    // 6. Initialize server
    server := NewServer(config, db)

    return &App{
        config: config,
        db:     db,
        server: server,
    }, nil
}

func (a *App) Start() error {
    return a.server.Run()
}

func (a *App) Shutdown() error {
    // Graceful shutdown
    return a.server.Shutdown()
}
```

---

## Database Schema & Models

### Database Support

- **Primary:** SQLite (default, embedded)
- **Optional:** PostgreSQL, MySQL (via environment configuration)

### Core Models

**Location:** `internal/models/`

#### 1. User Model (`user.go`)

```go
package models

import (
    "time"
    "gorm.io/gorm"
)

type User struct {
    ID        string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
    Username  string         `gorm:"uniqueIndex;not null" json:"username"`
    Email     string         `gorm:"uniqueIndex;not null" json:"email"`
    Password  string         `gorm:"not null" json:"-"` // Bcrypt hash

    // Profile
    FirstName string         `json:"first_name"`
    LastName  string         `json:"last_name"`
    Avatar    string         `json:"avatar"`

    // Status
    IsRoot    bool           `gorm:"default:false" json:"is_root"`
    IsActive  bool           `gorm:"default:true" json:"is_active"`

    // MFA
    MFAEnabled bool          `gorm:"default:false" json:"mfa_enabled"`
    MFASecret  *MFASecret    `gorm:"foreignKey:UserID" json:"-"`

    // Relationships
    OrganizationID *string   `gorm:"type:uuid" json:"organization_id"`
    Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`

    Teams      []Team        `gorm:"many2many:user_teams;" json:"teams,omitempty"`
    Roles      []Role        `gorm:"many2many:user_roles;" json:"roles,omitempty"`
    Sessions   []Session     `gorm:"foreignKey:UserID" json:"-"`

    // Timestamps
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

    // Audit
    LastLoginAt    *time.Time `json:"last_login_at"`
    LastLoginIP    string     `json:"last_login_ip"`
    FailedAttempts int        `gorm:"default:0" json:"-"`
    LockedUntil    *time.Time `json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
    if u.ID == "" {
        u.ID = uuid.New().String()
    }
    return nil
}
```

#### 2. Organization Model (`organization.go`)

```go
package models

type Organization struct {
    ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
    Name        string    `gorm:"not null" json:"name"`
    Description string    `json:"description"`

    // Settings
    Settings    string    `gorm:"type:json" json:"settings"` // JSON blob

    // Relationships
    Users       []User    `gorm:"foreignKey:OrganizationID" json:"users,omitempty"`
    Teams       []Team    `gorm:"foreignKey:OrganizationID" json:"teams,omitempty"`

    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

#### 3. Team Model (`team.go`)

```go
package models

type Team struct {
    ID             string       `gorm:"primaryKey;type:uuid" json:"id"`
    Name           string       `gorm:"not null" json:"name"`
    Description    string       `json:"description"`

    OrganizationID string       `gorm:"type:uuid;not null" json:"organization_id"`
    Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`

    Users          []User       `gorm:"many2many:user_teams;" json:"users,omitempty"`

    CreatedAt      time.Time    `json:"created_at"`
    UpdatedAt      time.Time    `json:"updated_at"`
}
```

#### 4. Role Model (`role.go`)

```go
package models

type Role struct {
    ID          string       `gorm:"primaryKey;type:uuid" json:"id"`
    Name        string       `gorm:"uniqueIndex;not null" json:"name"`
    Description string       `json:"description"`
    IsSystem    bool         `gorm:"default:false" json:"is_system"` // System roles can't be deleted

    Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
    Users       []User       `gorm:"many2many:user_roles;" json:"users,omitempty"`

    CreatedAt   time.Time    `json:"created_at"`
    UpdatedAt   time.Time    `json:"updated_at"`
}
```

#### 5. Permission Model (`permission.go`)

```go
package models

type Permission struct {
    ID          string    `gorm:"primaryKey" json:"id"` // e.g., "user.create"
    Module      string    `gorm:"not null;index" json:"module"`
    Description string    `json:"description"`
    DependsOn   string    `gorm:"type:json" json:"depends_on"` // JSON array of permission IDs

    Roles       []Role    `gorm:"many2many:role_permissions;" json:"roles,omitempty"`

    CreatedAt   time.Time `json:"created_at"`
}
```

#### 6. Session Model (`session.go`)

```go
package models

type Session struct {
    ID           string    `gorm:"primaryKey;type:uuid" json:"id"`
    UserID       string    `gorm:"type:uuid;not null;index" json:"user_id"`
    User         *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`

    RefreshToken string    `gorm:"uniqueIndex;not null" json:"-"`

    // Device info
    IPAddress    string    `json:"ip_address"`
    UserAgent    string    `json:"user_agent"`
    DeviceName   string    `json:"device_name"`

    // Expiry
    ExpiresAt    time.Time `gorm:"index" json:"expires_at"`
    LastUsedAt   time.Time `json:"last_used_at"`

    CreatedAt    time.Time `json:"created_at"`
    RevokedAt    *time.Time `json:"revoked_at"`
}
```

#### 7. AuditLog Model (`audit_log.go`)

```go
package models

type AuditLog struct {
    ID        string    `gorm:"primaryKey;type:uuid" json:"id"`

    // Who
    UserID    *string   `gorm:"type:uuid;index" json:"user_id"`
    User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
    Username  string    `json:"username"` // Denormalized for deleted users

    // What
    Action    string    `gorm:"not null;index" json:"action"` // e.g., "user.create"
    Resource  string    `gorm:"index" json:"resource"` // e.g., "user:123"
    Result    string    `gorm:"not null" json:"result"` // "success" or "failure"

    // Context
    IPAddress string    `json:"ip_address"`
    UserAgent string    `json:"user_agent"`
    Metadata  string    `gorm:"type:json" json:"metadata"` // JSON blob

    CreatedAt time.Time `gorm:"index" json:"created_at"`
}
```

#### 8. MFASecret Model (`mfa_secret.go`)

```go
package models

type MFASecret struct {
    ID            string    `gorm:"primaryKey;type:uuid" json:"id"`
    UserID        string    `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`

    Secret        string    `gorm:"not null" json:"-"` // Encrypted TOTP secret
    BackupCodes   string    `gorm:"type:json" json:"-"` // Encrypted JSON array

    CreatedAt     time.Time `json:"created_at"`
    LastUsedAt    *time.Time `json:"last_used_at"`
}
```

#### 9. PasswordResetToken Model (`password_reset_token.go`)

```go
package models

type PasswordResetToken struct {
    ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
    UserID    string    `gorm:"type:uuid;not null;index" json:"user_id"`
    Token     string    `gorm:"uniqueIndex;not null" json:"-"` // Hashed token

    ExpiresAt time.Time `gorm:"index" json:"expires_at"`
    UsedAt    *time.Time `json:"used_at"`

    CreatedAt time.Time `json:"created_at"`
}
```

#### 10. AuthProvider Model (`auth_provider.go`)

**IMPORTANT:** Authentication providers are configured via UI by admins, not config files.

```go
package models

type AuthProvider struct {
    ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
    Type        string    `gorm:"not null;uniqueIndex" json:"type"` // local, oidc, oauth2, saml, ldap, invite
    Name        string    `gorm:"not null" json:"name"` // Display name
    Enabled     bool      `gorm:"default:false" json:"enabled"`

    // Configuration (JSON blob, encrypted)
    Config      string    `gorm:"type:json" json:"config"` // Provider-specific config

    // Local provider settings
    AllowRegistration bool  `gorm:"default:false" json:"allow_registration"` // For local auth

    // Invite settings
    RequireEmailVerification bool `gorm:"default:true" json:"require_email_verification"` // For invite

    // Metadata
    Description string    `json:"description"`
    Icon        string    `json:"icon"` // Icon URL or name

    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    CreatedBy   string    `gorm:"type:uuid" json:"created_by"` // Admin who configured it
}

// Provider-specific config structures (stored as JSON in Config field)

// OIDCConfig for OpenID Connect
type OIDCConfig struct {
    Issuer       string `json:"issuer"`
    ClientID     string `json:"client_id"`
    ClientSecret string `json:"client_secret"` // Encrypted
    RedirectURL  string `json:"redirect_url"`
    Scopes       []string `json:"scopes"`
}

// SAMLConfig for SAML 2.0
type SAMLConfig struct {
    MetadataURL     string `json:"metadata_url"`
    EntityID        string `json:"entity_id"`
    SSOURL          string `json:"sso_url"`
    Certificate     string `json:"certificate"`
    PrivateKey      string `json:"private_key"` // Encrypted
    AttributeMapping map[string]string `json:"attribute_mapping"` // SAML attr -> user field
}

// LDAPConfig for LDAP/Active Directory
type LDAPConfig struct {
    Host            string `json:"host"`
    Port            int    `json:"port"`
    BaseDN          string `json:"base_dn"`
    BindDN          string `json:"bind_dn"`
    BindPassword    string `json:"bind_password"` // Encrypted
    UserFilter      string `json:"user_filter"`
    UseTLS          bool   `json:"use_tls"`
    SkipVerify      bool   `json:"skip_verify"`
    AttributeMapping map[string]string `json:"attribute_mapping"` // LDAP attr -> user field
}
```

#### 11. CacheEntry Model (`cache_entry.go`)

```go
package models

import "time"

type CacheEntry struct {
    Key       string    `gorm:"primaryKey;size:256"`
    Value     []byte    `gorm:"type:blob"`
    ExpiresAt time.Time `gorm:"index"`
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

This table stores cached artefacts (rate counters, session snapshots, etc.) when the platform falls back to the SQL database instead of Redis. Expired rows are lazily pruned by background workers and rate-limit operations.

### Database Migrations

**Location:** `internal/database/migrations.go`

```go
package database

import (
    "gorm.io/gorm"
    "shellcn/internal/models"
)

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{},
        &models.Organization{},
        &models.Team{},
        &models.Role{},
        &models.Permission{},
        &models.Session{},
        &models.AuditLog{},
        &models.MFASecret{},
        &models.PasswordResetToken{},
        &models.AuthProvider{},
        &models.UserInvite{},
        &models.EmailVerification{},
        &models.CacheEntry{},
    )
}

// SeedData creates initial system roles, permissions, and auth providers
func SeedData(db *gorm.DB) error {
    // Create system roles
    roles := []models.Role{
        {
            ID:          "admin",
            Name:        "Administrator",
            Description: "Full system access",
            IsSystem:    true,
        },
        {
            ID:          "user",
            Name:        "User",
            Description: "Standard user access",
            IsSystem:    true,
        },
    }

    for _, role := range roles {
        if err := db.FirstOrCreate(&role, "id = ?", role.ID).Error; err != nil {
            return err
        }
    }

    // Create default auth providers
    // Local auth is always enabled by default
    localProvider := models.AuthProvider{
        Type:              "local",
        Name:              "Local Authentication",
        Enabled:           true,
        AllowRegistration: false, // Disabled by default, admin can enable
        Description:       "Username and password authentication",
        Icon:              "key",
    }
    db.FirstOrCreate(&localProvider, "type = ?", "local")

    // Create disabled invite provider (admin can enable)
    inviteProvider := models.AuthProvider{
        Type:                     "invite",
        Name:                     "Email Invitation",
        Enabled:                  false,
        RequireEmailVerification: true,
        Description:              "Invite users via email",
        Icon:                     "mail",
    }
    db.FirstOrCreate(&inviteProvider, "type = ?", "invite")

    return nil
}
```

---

## Shared Packages (pkg/)

### 1. Logger Package (`pkg/logger/logger.go`)

```go
package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap"
)

var globalLogger *zap.Logger

func Init(level string) error {
    config := zap.NewProductionConfig()

    // Parse log level
    var zapLevel zapcore.Level
    if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
        zapLevel = zapcore.InfoLevel
    }
    config.Level = zap.NewAtomicLevelAt(zapLevel)

    logger, err := config.Build()
    if err != nil {
        return err
    }

    globalLogger = logger
    return nil
}

func Info(msg string, fields ...zap.Field) {
    globalLogger.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
    globalLogger.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
    globalLogger.Debug(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
    globalLogger.Warn(msg, fields...)
}

func WithModule(module string) *zap.Logger {
    return globalLogger.With(zap.String("module", module))
}
```

### 2. Error Package (`pkg/errors/errors.go`)

```go
package errors

import (
    "fmt"
    "net/http"
)

type AppError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    StatusCode int    `json:"-"`
    Internal   error  `json:"-"`
}

func (e *AppError) Error() string {
    if e.Internal != nil {
        return fmt.Sprintf("%s: %v", e.Message, e.Internal)
    }
    return e.Message
}

// Common errors
var (
    ErrUnauthorized = &AppError{
        Code:       "UNAUTHORIZED",
        Message:    "Authentication required",
        StatusCode: http.StatusUnauthorized,
    }

    ErrForbidden = &AppError{
        Code:       "FORBIDDEN",
        Message:    "Permission denied",
        StatusCode: http.StatusForbidden,
    }

    ErrNotFound = &AppError{
        Code:       "NOT_FOUND",
        Message:    "Resource not found",
        StatusCode: http.StatusNotFound,
    }

    ErrBadRequest = &AppError{
        Code:       "BAD_REQUEST",
        Message:    "Invalid request",
        StatusCode: http.StatusBadRequest,
    }

    ErrInternalServer = &AppError{
        Code:       "INTERNAL_SERVER_ERROR",
        Message:    "Internal server error",
        StatusCode: http.StatusInternalServerError,
    }
)

func New(code, message string, statusCode int) *AppError {
    return &AppError{
        Code:       code,
        Message:    message,
        StatusCode: statusCode,
    }
}

func Wrap(err error, message string) *AppError {
    return &AppError{
        Code:       "INTERNAL_ERROR",
        Message:    message,
        StatusCode: http.StatusInternalServerError,
        Internal:   err,
    }
}
```

### 3. Response Package (`pkg/response/response.go`)

```go
package response

import (
    "github.com/gin-gonic/gin"
    "shellcn/pkg/errors"
)

type Response struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   *ErrorInfo  `json:"error,omitempty"`
    Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorInfo struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

type Meta struct {
    Page       int `json:"page,omitempty"`
    PerPage    int `json:"per_page,omitempty"`
    Total      int `json:"total,omitempty"`
    TotalPages int `json:"total_pages,omitempty"`
}

func Success(c *gin.Context, statusCode int, data interface{}) {
    c.JSON(statusCode, Response{
        Success: true,
        Data:    data,
    })
}

func SuccessWithMeta(c *gin.Context, statusCode int, data interface{}, meta *Meta) {
    c.JSON(statusCode, Response{
        Success: true,
        Data:    data,
        Meta:    meta,
    })
}

func Error(c *gin.Context, err error) {
    appErr, ok := err.(*errors.AppError)
    if !ok {
        appErr = errors.ErrInternalServer
    }

    c.JSON(appErr.StatusCode, Response{
        Success: false,
        Error: &ErrorInfo{
            Code:    appErr.Code,
            Message: appErr.Message,
        },
    })
}
```

### 4. Crypto Package (`pkg/crypto/crypto.go`)

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"

    "golang.org/x/crypto/bcrypt"
)

// Password hashing
func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}

func VerifyPassword(hashedPassword, password string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
    return err == nil
}

// AES-256-GCM encryption
func Encrypt(plaintext, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }

    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func Decrypt(ciphertext string, key []byte) ([]byte, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return nil, err
    }

    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return nil, errors.New("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    return gcm.Open(nil, nonce, ciphertext, nil)
}

// Generate random token
func GenerateToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}
```

### 5. Mail Package (`pkg/mail/mailer.go`)

The mail package provides a thin SMTP abstraction for outbound notifications:

- `Mailer` interface with a single `Send` method accepting a context-aware message payload
- `SMTPSettings` struct mirroring runtime configuration (host, port, credentials, TLS, timeout)
- `NewSMTPMailer` constructor that validates configuration and builds a TLS-capable SMTP client
- RFC 822 message formatter that deduplicates recipients and normalises headers

This package underpins invite and email verification workflows while allowing alternative mail transports to be supplied in tests.

---

## Authentication System

### JWT Service

**Location:** `internal/auth/jwt.go`

```go
package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
    "shellcn/internal/models"
)

type JWTService struct {
    secretKey     []byte
    accessExpiry  time.Duration
    refreshExpiry time.Duration
}

type Claims struct {
    UserID   string   `json:"user_id"`
    Username string   `json:"username"`
    IsRoot   bool     `json:"is_root"`
    Permissions []string `json:"permissions"`
    jwt.RegisteredClaims
}

func NewJWTService(secretKey string) *JWTService {
    return &JWTService{
        secretKey:     []byte(secretKey),
        accessExpiry:  15 * time.Minute,
        refreshExpiry: 7 * 24 * time.Hour,
    }
}

func (s *JWTService) GenerateAccessToken(user *models.User, permissions []string) (string, error) {
    claims := &Claims{
        UserID:      user.ID,
        Username:    user.Username,
        IsRoot:      user.IsRoot,
        Permissions: permissions,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessExpiry)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
            Issuer:    "shellcn",
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.secretKey)
}

func (s *JWTService) GenerateRefreshToken() (string, error) {
    return crypto.GenerateToken(32)
}

func (s *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return s.secretKey, nil
    })

    if err != nil {
        return nil, err
    }

    if claims, ok := token.Claims.(*Claims); ok && token.Valid {
        return claims, nil
    }

    return nil, errors.New("invalid token")
}
```

### Local Authentication Provider

**Location:** `internal/auth/providers/local.go`

```go
package providers

import (
    "errors"
    "time"

    "gorm.io/gorm"
    "shellcn/internal/models"
    "shellcn/pkg/crypto"
)

type LocalProvider struct {
    db *gorm.DB
}

func NewLocalProvider(db *gorm.DB) *LocalProvider {
    return &LocalProvider{db: db}
}

func (p *LocalProvider) Authenticate(username, password string) (*models.User, error) {
    var user models.User

    // Find user
    if err := p.db.Where("username = ? OR email = ?", username, username).First(&user).Error; err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, errors.New("invalid credentials")
        }
        return nil, err
    }

    // Check if user is active
    if !user.IsActive {
        return nil, errors.New("user account is disabled")
    }

    // Check if account is locked
    if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
        return nil, errors.New("account is locked")
    }

    // Verify password
    if !crypto.VerifyPassword(user.Password, password) {
        // Increment failed attempts
        user.FailedAttempts++

        // Lock account after 5 failed attempts
        if user.FailedAttempts >= 5 {
            lockUntil := time.Now().Add(15 * time.Minute)
            user.LockedUntil = &lockUntil
        }

        p.db.Save(&user)
        return nil, errors.New("invalid credentials")
    }

    // Reset failed attempts on successful login
    user.FailedAttempts = 0
    user.LockedUntil = nil
    p.db.Save(&user)

    return &user, nil
}

func (p *LocalProvider) Register(username, email, password string) (*models.User, error) {
    // Hash password
    hashedPassword, err := crypto.HashPassword(password)
    if err != nil {
        return nil, err
    }

    user := &models.User{
        Username: username,
        Email:    email,
        Password: hashedPassword,
        IsActive: true,
    }

    if err := p.db.Create(user).Error; err != nil {
        return nil, err
    }

    return user, nil
}

func (p *LocalProvider) ChangePassword(userID, oldPassword, newPassword string) error {
    var user models.User
    if err := p.db.First(&user, "id = ?", userID).Error; err != nil {
        return err
    }

    // Verify old password
    if !crypto.VerifyPassword(user.Password, oldPassword) {
        return errors.New("invalid current password")
    }

    // Hash new password
    hashedPassword, err := crypto.HashPassword(newPassword)
    if err != nil {
        return err
    }

    user.Password = hashedPassword
    return p.db.Save(&user).Error
}
```

### MFA (TOTP) Implementation

**Location:** `internal/auth/mfa/totp.go`

```go
package mfa

import (
    "crypto/rand"
    "encoding/base32"
    "fmt"

    "github.com/pquerna/otp"
    "github.com/pquerna/otp/totp"
    "github.com/skip2/go-qrcode"

    "gorm.io/gorm"
    "shellcn/internal/models"
    "shellcn/pkg/crypto"
)

type TOTPService struct {
    db            *gorm.DB
    encryptionKey []byte
}

func NewTOTPService(db *gorm.DB, encryptionKey []byte) *TOTPService {
    return &TOTPService{
        db:            db,
        encryptionKey: encryptionKey,
    }
}

func (s *TOTPService) GenerateSecret(userID, username string) (*otp.Key, []string, error) {
    // Generate TOTP key
    key, err := totp.Generate(totp.GenerateOpts{
        Issuer:      "ShellCN",
        AccountName: username,
    })
    if err != nil {
        return nil, nil, err
    }

    // Generate backup codes
    backupCodes := make([]string, 10)
    for i := 0; i < 10; i++ {
        code, err := generateBackupCode()
        if err != nil {
            return nil, nil, err
        }
        backupCodes[i] = code
    }

    // Encrypt secret
    encryptedSecret, err := crypto.Encrypt([]byte(key.Secret()), s.encryptionKey)
    if err != nil {
        return nil, nil, err
    }

    // Hash and encrypt backup codes
    hashedCodes := make([]string, len(backupCodes))
    for i, code := range backupCodes {
        hash, _ := crypto.HashPassword(code)
        hashedCodes[i] = hash
    }

    // Store in database
    mfaSecret := &models.MFASecret{
        UserID:      userID,
        Secret:      encryptedSecret,
        BackupCodes: string(hashedCodes), // JSON
    }

    if err := s.db.Create(mfaSecret).Error; err != nil {
        return nil, nil, err
    }

    return key, backupCodes, nil
}

func (s *TOTPService) VerifyCode(userID, code string) (bool, error) {
    var mfaSecret models.MFASecret
    if err := s.db.Where("user_id = ?", userID).First(&mfaSecret).Error; err != nil {
        return false, err
    }

    // Decrypt secret
    decryptedSecret, err := crypto.Decrypt(mfaSecret.Secret, s.encryptionKey)
    if err != nil {
        return false, err
    }

    // Verify TOTP code
    valid := totp.Validate(code, string(decryptedSecret))

    if valid {
        // Update last used
        now := time.Now()
        mfaSecret.LastUsedAt = &now
        s.db.Save(&mfaSecret)
    }

    return valid, nil
}

func (s *TOTPService) GenerateQRCode(key *otp.Key) ([]byte, error) {
    return qrcode.Encode(key.String(), qrcode.Medium, 256)
}

func generateBackupCode() (string, error) {
    bytes := make([]byte, 5)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base32.StdEncoding.EncodeToString(bytes)[:8], nil
}
```

---

## Authorization & Permission System

### Permission Registry

**Location:** `internal/permissions/registry.go`

This is the global permission registry that all modules register their permissions with.

```go
package permissions

import (
    "fmt"
    "sync"
)

type Permission struct {
    ID          string
    Module      string
    DependsOn   []string
    Description string
}

type PermissionRegistry struct {
    permissions map[string]*Permission
    mu          sync.RWMutex
}

var globalRegistry = &PermissionRegistry{
    permissions: make(map[string]*Permission),
}

// Register a permission - called by modules in init()
func Register(perm *Permission) error {
    globalRegistry.mu.Lock()
    defer globalRegistry.mu.Unlock()

    if _, exists := globalRegistry.permissions[perm.ID]; exists {
        return fmt.Errorf("permission %s already registered", perm.ID)
    }

    globalRegistry.permissions[perm.ID] = perm
    return nil
}

// Get all registered permissions
func GetAll() map[string]*Permission {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    result := make(map[string]*Permission)
    for k, v := range globalRegistry.permissions {
        result[k] = v
    }
    return result
}

// Get permissions by module
func GetByModule(module string) []*Permission {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    var result []*Permission
    for _, perm := range globalRegistry.permissions {
        if perm.Module == module {
            result = append(result, perm)
        }
    }
    return result
}

// Validate all permission dependencies on startup
func ValidateDependencies() error {
    globalRegistry.mu.RLock()
    defer globalRegistry.mu.RUnlock()

    for _, perm := range globalRegistry.permissions {
        for _, dep := range perm.DependsOn {
            if _, exists := globalRegistry.permissions[dep]; !exists {
                return fmt.Errorf("permission %s depends on non-existent permission %s",
                    perm.ID, dep)
            }
        }
    }
    return nil
}
```

### Core Permissions Registration

**Location:** `internal/permissions/core.go`

```go
package permissions

func init() {
    // User Management
    Register(&Permission{
        ID:          "user.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View users",
    })

    Register(&Permission{
        ID:          "user.create",
        Module:      "core",
        DependsOn:   []string{"user.view"},
        Description: "Create new users",
    })

    Register(&Permission{
        ID:          "user.edit",
        Module:      "core",
        DependsOn:   []string{"user.view"},
        Description: "Edit user details",
    })

    Register(&Permission{
        ID:          "user.delete",
        Module:      "core",
        DependsOn:   []string{"user.view", "user.edit"},
        Description: "Delete users",
    })

    // Organization Management
    Register(&Permission{
        ID:          "org.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View organizations",
    })

    Register(&Permission{
        ID:          "org.create",
        Module:      "core",
        DependsOn:   []string{"org.view"},
        Description: "Create organizations",
    })

    Register(&Permission{
        ID:          "org.manage",
        Module:      "core",
        DependsOn:   []string{"org.view"},
        Description: "Manage organizations",
    })

    // Permission Management
    Register(&Permission{
        ID:          "permission.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View permissions",
    })

    Register(&Permission{
        ID:          "permission.manage",
        Module:      "core",
        DependsOn:   []string{"permission.view"},
        Description: "Assign/revoke permissions",
    })

    // Audit
    Register(&Permission{
        ID:          "audit.view",
        Module:      "core",
        DependsOn:   []string{},
        Description: "View audit logs",
    })

    Register(&Permission{
        ID:          "audit.export",
        Module:      "core",
        DependsOn:   []string{"audit.view"},
        Description: "Export audit logs",
    })
}
```

### Permission Checker

**Location:** `internal/permissions/checker.go`

```go
package permissions

import (
    "context"
    "gorm.io/gorm"
    "shellcn/internal/models"
)

type Checker struct {
    db *gorm.DB
}

func NewChecker(db *gorm.DB) *Checker {
    return &Checker{db: db}
}

// Check if user has permission (with dependency resolution)
func (c *Checker) Check(ctx context.Context, userID, permissionID string) (bool, error) {
    // Get user
    var user models.User
    if err := c.db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
        return false, err
    }

    // Root user bypasses all permission checks
    if user.IsRoot {
        return true, nil
    }

    // Get permission from registry
    globalRegistry.mu.RLock()
    perm, exists := globalRegistry.permissions[permissionID]
    globalRegistry.mu.RUnlock()

    if !exists {
        return false, fmt.Errorf("permission %s not found", permissionID)
    }

    // Check dependencies first
    for _, dep := range perm.DependsOn {
        hasDepPerm, err := c.hasPermission(&user, dep)
        if err != nil {
            return false, err
        }
        if !hasDepPerm {
            return false, nil // Missing dependency
        }
    }

    // Check the permission itself
    return c.hasPermission(&user, permissionID)
}

func (c *Checker) hasPermission(user *models.User, permissionID string) (bool, error) {
    for _, role := range user.Roles {
        for _, perm := range role.Permissions {
            if perm.ID == permissionID {
                return true, nil
            }
        }
    }
    return false, nil
}

// GetUserPermissions returns all permissions for a user
func (c *Checker) GetUserPermissions(userID string) ([]string, error) {
    var user models.User
    if err := c.db.Preload("Roles.Permissions").First(&user, "id = ?", userID).Error; err != nil {
        return nil, err
    }

    // Root user has all permissions
    if user.IsRoot {
        allPerms := GetAll()
        permIDs := make([]string, 0, len(allPerms))
        for id := range allPerms {
            permIDs = append(permIDs, id)
        }
        return permIDs, nil
    }

    // Collect unique permissions
    permMap := make(map[string]bool)
    for _, role := range user.Roles {
        for _, perm := range role.Permissions {
            permMap[perm.ID] = true
        }
    }

    permIDs := make([]string, 0, len(permMap))
    for id := range permMap {
        permIDs = append(permIDs, id)
    }

    return permIDs, nil
}
```

---

## User Management

### User Service

**Location:** `internal/services/user_service.go`

```go
package services

import (
    "errors"
    "gorm.io/gorm"
    "shellcn/internal/models"
    "shellcn/pkg/crypto"
)

type UserService struct {
    db            *gorm.DB
    auditService  *AuditService
}

func NewUserService(db *gorm.DB, auditService *AuditService) *UserService {
    return &UserService{
        db:           db,
        auditService: auditService,
    }
}

// Create user
func (s *UserService) Create(username, email, password string, isRoot bool) (*models.User, error) {
    // Hash password
    hashedPassword, err := crypto.HashPassword(password)
    if err != nil {
        return nil, err
    }

    user := &models.User{
        Username: username,
        Email:    email,
        Password: hashedPassword,
        IsRoot:   isRoot,
        IsActive: true,
    }

    if err := s.db.Create(user).Error; err != nil {
        return nil, err
    }

    // Audit log
    s.auditService.Log("user.create", user.ID, "success", nil)

    return user, nil
}

// Get user by ID
func (s *UserService) GetByID(id string) (*models.User, error) {
    var user models.User
    if err := s.db.Preload("Organization").Preload("Teams").Preload("Roles").
        First(&user, "id = ?", id).Error; err != nil {
        return nil, err
    }
    return &user, nil
}

// List users with pagination
func (s *UserService) List(page, perPage int, filters map[string]interface{}) ([]models.User, int64, error) {
    var users []models.User
    var total int64

    query := s.db.Model(&models.User{})

    // Apply filters
    if orgID, ok := filters["organization_id"]; ok {
        query = query.Where("organization_id = ?", orgID)
    }
    if isActive, ok := filters["is_active"]; ok {
        query = query.Where("is_active = ?", isActive)
    }

    // Count total
    query.Count(&total)

    // Paginate
    offset := (page - 1) * perPage
    if err := query.Offset(offset).Limit(perPage).
        Preload("Organization").Preload("Roles").
        Find(&users).Error; err != nil {
        return nil, 0, err
    }

    return users, total, nil
}

// Update user
func (s *UserService) Update(id string, updates map[string]interface{}) (*models.User, error) {
    var user models.User
    if err := s.db.First(&user, "id = ?", id).Error; err != nil {
        return nil, err
    }

    // Prevent modifying root status
    if _, hasRoot := updates["is_root"]; hasRoot && user.IsRoot {
        return nil, errors.New("cannot modify root status")
    }

    if err := s.db.Model(&user).Updates(updates).Error; err != nil {
        return nil, err
    }

    s.auditService.Log("user.update", user.ID, "success", updates)

    return &user, nil
}

// Delete user (soft delete)
func (s *UserService) Delete(id string) error {
    var user models.User
    if err := s.db.First(&user, "id = ?", id).Error; err != nil {
        return err
    }

    // Prevent deleting root user
    if user.IsRoot {
        return errors.New("cannot delete root user")
    }

    if err := s.db.Delete(&user).Error; err != nil {
        return err
    }

    s.auditService.Log("user.delete", user.ID, "success", nil)

    return nil
}

// Activate/Deactivate user
func (s *UserService) SetActive(id string, active bool) error {
    var user models.User
    if err := s.db.First(&user, "id = ?", id).Error; err != nil {
        return err
    }

    // Prevent deactivating root user
    if user.IsRoot && !active {
        return errors.New("cannot deactivate root user")
    }

    user.IsActive = active
    if err := s.db.Save(&user).Error; err != nil {
        return err
    }

    action := "user.activate"
    if !active {
        action = "user.deactivate"
    }
    s.auditService.Log(action, user.ID, "success", nil)

    return nil
}

// Change password
func (s *UserService) ChangePassword(id, newPassword string) error {
    hashedPassword, err := crypto.HashPassword(newPassword)
    if err != nil {
        return err
    }

    if err := s.db.Model(&models.User{}).Where("id = ?", id).
        Update("password", hashedPassword).Error; err != nil {
        return err
    }

    s.auditService.Log("user.password_change", id, "success", nil)

    return nil
}
```

---

## Organization & Team Management

### Organization Service

**Location:** `internal/services/organization_service.go`

```go
package services

import (
    "gorm.io/gorm"
    "shellcn/internal/models"
)

type OrganizationService struct {
    db           *gorm.DB
    auditService *AuditService
}

func NewOrganizationService(db *gorm.DB, auditService *AuditService) *OrganizationService {
    return &OrganizationService{
        db:           db,
        auditService: auditService,
    }
}

func (s *OrganizationService) Create(name, description string) (*models.Organization, error) {
    org := &models.Organization{
        Name:        name,
        Description: description,
    }

    if err := s.db.Create(org).Error; err != nil {
        return nil, err
    }

    s.auditService.Log("org.create", org.ID, "success", nil)

    return org, nil
}

func (s *OrganizationService) GetByID(id string) (*models.Organization, error) {
    var org models.Organization
    if err := s.db.Preload("Users").Preload("Teams").
        First(&org, "id = ?", id).Error; err != nil {
        return nil, err
    }
    return &org, nil
}

func (s *OrganizationService) List() ([]models.Organization, error) {
    var orgs []models.Organization
    if err := s.db.Find(&orgs).Error; err != nil {
        return nil, err
    }
    return orgs, nil
}
```

### Team Service

**Location:** `internal/services/team_service.go`

```go
package services

import (
    "gorm.io/gorm"
    "shellcn/internal/models"
)

type TeamService struct {
    db           *gorm.DB
    auditService *AuditService
}

func NewTeamService(db *gorm.DB, auditService *AuditService) *TeamService {
    return &TeamService{
        db:           db,
        auditService: auditService,
    }
}

func (s *TeamService) Create(orgID, name, description string) (*models.Team, error) {
    team := &models.Team{
        OrganizationID: orgID,
        Name:           name,
        Description:    description,
    }

    if err := s.db.Create(team).Error; err != nil {
        return nil, err
    }

    s.auditService.Log("team.create", team.ID, "success", nil)

    return team, nil
}

func (s *TeamService) AddMember(teamID, userID string) error {
    var team models.Team
    var user models.User

    if err := s.db.First(&team, "id = ?", teamID).Error; err != nil {
        return err
    }

    if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
        return err
    }

    if err := s.db.Model(&team).Association("Users").Append(&user); err != nil {
        return err
    }

    s.auditService.Log("team.add_member", teamID, "success", map[string]interface{}{
        "user_id": userID,
    })

    return nil
}
```

---

## Auth Provider Management

### Auth Provider Service

**Location:** `internal/services/auth_provider_service.go`

**IMPORTANT:** All authentication providers are configured via UI by admins.

```go
package services

import (
    "encoding/json"
    "errors"
    "gorm.io/gorm"
    "shellcn/internal/models"
    "shellcn/pkg/crypto"
)

type AuthProviderService struct {
    db            *gorm.DB
    auditService  *AuditService
    encryptionKey []byte
}

func NewAuthProviderService(db *gorm.DB, auditService *AuditService, encryptionKey []byte) *AuthProviderService {
    return &AuthProviderService{
        db:            db,
        auditService:  auditService,
        encryptionKey: encryptionKey,
    }
}

// List all auth providers
func (s *AuthProviderService) List() ([]models.AuthProvider, error) {
    var providers []models.AuthProvider
    if err := s.db.Find(&providers).Error; err != nil {
        return nil, err
    }

    // Don't return sensitive config data in list
    for i := range providers {
        providers[i].Config = "" // Redact config
    }

    return providers, nil
}

// Get provider by type
func (s *AuthProviderService) GetByType(providerType string) (*models.AuthProvider, error) {
    var provider models.AuthProvider
    if err := s.db.Where("type = ?", providerType).First(&provider).Error; err != nil {
        return nil, err
    }
    return &provider, nil
}

// Get enabled providers (for login page)
func (s *AuthProviderService) GetEnabled() ([]models.AuthProvider, error) {
    var providers []models.AuthProvider
    if err := s.db.Where("enabled = ?", true).Find(&providers).Error; err != nil {
        return nil, err
    }

    // Don't return sensitive config
    for i := range providers {
        providers[i].Config = ""
    }

    return providers, nil
}

// Create or update OIDC provider
func (s *AuthProviderService) ConfigureOIDC(config models.OIDCConfig, enabled bool, createdBy string) error {
    // Encrypt client secret
    encryptedSecret, err := crypto.Encrypt([]byte(config.ClientSecret), s.encryptionKey)
    if err != nil {
        return err
    }
    config.ClientSecret = encryptedSecret

    // Marshal config to JSON
    configJSON, err := json.Marshal(config)
    if err != nil {
        return err
    }

    provider := models.AuthProvider{
        Type:        "oidc",
        Name:        "OpenID Connect",
        Enabled:     enabled,
        Config:      string(configJSON),
        Description: "Single Sign-On via OpenID Connect",
        Icon:        "shield-check",
        CreatedBy:   createdBy,
    }

    // Upsert
    if err := s.db.Where("type = ?", "oidc").Assign(provider).FirstOrCreate(&provider).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.configure", "oidc", "success", map[string]interface{}{
        "enabled": enabled,
    })

    return nil
}

// Configure SAML provider
func (s *AuthProviderService) ConfigureSAML(config models.SAMLConfig, enabled bool, createdBy string) error {
    // Encrypt private key
    encryptedKey, err := crypto.Encrypt([]byte(config.PrivateKey), s.encryptionKey)
    if err != nil {
        return err
    }
    config.PrivateKey = encryptedKey

    configJSON, err := json.Marshal(config)
    if err != nil {
        return err
    }

    provider := models.AuthProvider{
        Type:        "saml",
        Name:        "SAML 2.0",
        Enabled:     enabled,
        Config:      string(configJSON),
        Description: "SAML 2.0 Single Sign-On",
        Icon:        "shield",
        CreatedBy:   createdBy,
    }

    if err := s.db.Where("type = ?", "saml").Assign(provider).FirstOrCreate(&provider).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.configure", "saml", "success", map[string]interface{}{
        "enabled": enabled,
    })

    return nil
}

// Configure LDAP provider
func (s *AuthProviderService) ConfigureLDAP(config models.LDAPConfig, enabled bool, createdBy string) error {
    // Encrypt bind password
    encryptedPassword, err := crypto.Encrypt([]byte(config.BindPassword), s.encryptionKey)
    if err != nil {
        return err
    }
    config.BindPassword = encryptedPassword

    configJSON, err := json.Marshal(config)
    if err != nil {
        return err
    }

    provider := models.AuthProvider{
        Type:        "ldap",
        Name:        "LDAP / Active Directory",
        Enabled:     enabled,
        Config:      string(configJSON),
        Description: "LDAP or Active Directory authentication",
        Icon:        "building",
        CreatedBy:   createdBy,
    }

    if err := s.db.Where("type = ?", "ldap").Assign(provider).FirstOrCreate(&provider).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.configure", "ldap", "success", map[string]interface{}{
        "enabled": enabled,
    })

    return nil
}

// Update local provider settings
func (s *AuthProviderService) UpdateLocalSettings(allowRegistration, requireEmailVerification bool) error {
    updates := map[string]any{
        "allow_registration":        allowRegistration,
        "require_email_verification": requireEmailVerification,
    }

    if err := s.db.Model(&models.AuthProvider{}).
        Where("type = ?", "local").
        Updates(updates).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.update", "local", "success", updates)

    return nil
}

// Update invite provider settings
func (s *AuthProviderService) UpdateInviteSettings(enabled, requireEmailVerification bool) error {
    updates := map[string]interface{}{
        "enabled":                      enabled,
        "require_email_verification":   requireEmailVerification,
    }

    if err := s.db.Model(&models.AuthProvider{}).
        Where("type = ?", "invite").
        Updates(updates).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.update", "invite", "success", updates)

    return nil
}

// Enable/disable provider
func (s *AuthProviderService) SetEnabled(providerType string, enabled bool) error {
    // Cannot disable local auth
    if providerType == "local" {
        return errors.New("cannot disable local authentication")
    }

    if err := s.db.Model(&models.AuthProvider{}).
        Where("type = ?", providerType).
        Update("enabled", enabled).Error; err != nil {
        return err
    }

    action := "auth_provider.enable"
    if !enabled {
        action = "auth_provider.disable"
    }

    s.auditService.Log(action, providerType, "success", nil)

    return nil
}

// Delete provider configuration
func (s *AuthProviderService) Delete(providerType string) error {
    // Cannot delete local or invite providers
    if providerType == "local" || providerType == "invite" {
        return errors.New("cannot delete system auth providers")
    }

    if err := s.db.Where("type = ?", providerType).Delete(&models.AuthProvider{}).Error; err != nil {
        return err
    }

    s.auditService.Log("auth_provider.delete", providerType, "success", nil)

    return nil
}

// Test provider connection (for LDAP)
func (s *AuthProviderService) TestConnection(providerType string) error {
    provider, err := s.GetByType(providerType)
    if err != nil {
        return err
    }

    switch providerType {
    case "ldap":
        var config models.LDAPConfig
        if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
            return err
        }

        // Decrypt bind password
        decryptedPassword, err := crypto.Decrypt(config.BindPassword, s.encryptionKey)
        if err != nil {
            return err
        }
        config.BindPassword = string(decryptedPassword)

        // Test LDAP connection
        return testLDAPConnection(config)

    case "oidc":
        // Test OIDC discovery endpoint
        var config models.OIDCConfig
        if err := json.Unmarshal([]byte(provider.Config), &config); err != nil {
            return err
        }
        return testOIDCConnection(config)

    default:
        return errors.New("connection test not supported for this provider")
    }
}

### Email-Oriented Services

- **Invite Service (`internal/services/invite_service.go`)**
  - Issues invite links with SHA-256 token hashing and expiry enforcement
  - Persists invites in `user_invites` table and records acceptance timestamps
  - Sends invitation emails through the SMTP mailer abstraction when configured
- **Email Verification Service (`internal/services/email_verification_service.go`)**
  - Generates verification tokens for local self-registration when required
  - Stores hashed tokens in `email_verifications` table with configurable lifetimes
  - Dispatches verification messages via the shared mailer infrastructure
```

---

## Session Management

### Session Service

**Location:** `internal/services/session_service.go`

```go
package services

import (
    "time"
    "gorm.io/gorm"
    "shellcn/internal/models"
    "shellcn/pkg/crypto"
)

type SessionService struct {
    db           *gorm.DB
    auditService *AuditService
}

func NewSessionService(db *gorm.DB, auditService *AuditService) *SessionService {
    return &SessionService{
        db:           db,
        auditService: auditService,
    }
}

// Create new session
func (s *SessionService) Create(userID, ipAddress, userAgent string) (*models.Session, string, error) {
    // Generate refresh token
    refreshToken, err := crypto.GenerateToken(32)
    if err != nil {
        return nil, "", err
    }

    // Hash refresh token for storage
    hashedToken, err := crypto.HashPassword(refreshToken)
    if err != nil {
        return nil, "", err
    }

    session := &models.Session{
        UserID:       userID,
        RefreshToken: hashedToken,
        IPAddress:    ipAddress,
        UserAgent:    userAgent,
        ExpiresAt:    time.Now().Add(7 * 24 * time.Hour),
        LastUsedAt:   time.Now(),
    }

    if err := s.db.Create(session).Error; err != nil {
        return nil, "", err
    }

    s.auditService.Log("session.create", session.ID, "success", map[string]interface{}{
        "user_id": userID,
        "ip":      ipAddress,
    })

    return session, refreshToken, nil
}

// Validate refresh token
func (s *SessionService) Validate(refreshToken string) (*models.Session, error) {
    var sessions []models.Session
    if err := s.db.Where("revoked_at IS NULL AND expires_at > ?", time.Now()).
        Find(&sessions).Error; err != nil {
        return nil, err
    }

    // Check each session's hashed token
    for _, session := range sessions {
        if crypto.VerifyPassword(session.RefreshToken, refreshToken) {
            // Update last used
            session.LastUsedAt = time.Now()
            s.db.Save(&session)
            return &session, nil
        }
    }

    return nil, errors.New("invalid refresh token")
}

// List user sessions
func (s *SessionService) ListByUser(userID string) ([]models.Session, error) {
    var sessions []models.Session
    if err := s.db.Where("user_id = ? AND revoked_at IS NULL", userID).
        Order("last_used_at DESC").
        Find(&sessions).Error; err != nil {
        return nil, err
    }
    return sessions, nil
}

// Revoke session
func (s *SessionService) Revoke(sessionID string) error {
    now := time.Now()
    if err := s.db.Model(&models.Session{}).
        Where("id = ?", sessionID).
        Update("revoked_at", now).Error; err != nil {
        return err
    }

    s.auditService.Log("session.revoke", sessionID, "success", nil)

    return nil
}

// Revoke all user sessions
func (s *SessionService) RevokeAllByUser(userID string) error {
    now := time.Now()
    if err := s.db.Model(&models.Session{}).
        Where("user_id = ? AND revoked_at IS NULL", userID).
        Update("revoked_at", now).Error; err != nil {
        return err
    }

    s.auditService.Log("session.revoke_all", userID, "success", nil)

    return nil
}

// Cleanup expired sessions (background job)
func (s *SessionService) CleanupExpired() error {
    return s.db.Where("expires_at < ?", time.Now()).
        Delete(&models.Session{}).Error
}
```

---

## Audit Logging

### Audit Service

**Location:** `internal/services/audit_service.go`

```go
package services

import (
    "encoding/json"
    "time"
    "gorm.io/gorm"
    "shellcn/internal/models"
)

type AuditService struct {
    db *gorm.DB
}

func NewAuditService(db *gorm.DB) *AuditService {
    return &AuditService{db: db}
}

// Log audit event
func (s *AuditService) Log(action, resource, result string, metadata map[string]interface{}) error {
    metadataJSON, _ := json.Marshal(metadata)

    log := &models.AuditLog{
        Action:   action,
        Resource: resource,
        Result:   result,
        Metadata: string(metadataJSON),
    }

    // Get user from context if available
    // This would be set by middleware

    return s.db.Create(log).Error
}

// List audit logs with filters
func (s *AuditService) List(page, perPage int, filters map[string]interface{}) ([]models.AuditLog, int64, error) {
    var logs []models.AuditLog
    var total int64

    query := s.db.Model(&models.AuditLog{})

    // Apply filters
    if userID, ok := filters["user_id"]; ok {
        query = query.Where("user_id = ?", userID)
    }
    if action, ok := filters["action"]; ok {
        query = query.Where("action = ?", action)
    }
    if result, ok := filters["result"]; ok {
        query = query.Where("result = ?", result)
    }
    if startDate, ok := filters["start_date"]; ok {
        query = query.Where("created_at >= ?", startDate)
    }
    if endDate, ok := filters["end_date"]; ok {
        query = query.Where("created_at <= ?", endDate)
    }

    // Count total
    query.Count(&total)

    // Paginate
    offset := (page - 1) * perPage
    if err := query.Offset(offset).Limit(perPage).
        Order("created_at DESC").
        Preload("User").
        Find(&logs).Error; err != nil {
        return nil, 0, err
    }

    return logs, total, nil
}

// Export audit logs (for compliance)
func (s *AuditService) Export(filters map[string]interface{}) ([]models.AuditLog, error) {
    var logs []models.AuditLog

    query := s.db.Model(&models.AuditLog{})

    // Apply same filters as List
    if userID, ok := filters["user_id"]; ok {
        query = query.Where("user_id = ?", userID)
    }
    // ... other filters

    if err := query.Order("created_at DESC").
        Preload("User").
        Find(&logs).Error; err != nil {
        return nil, err
    }

    return logs, nil
}

// Cleanup old logs (retention policy)
func (s *AuditService) CleanupOld(retentionDays int) error {
    cutoff := time.Now().AddDate(0, 0, -retentionDays)
    return s.db.Where("created_at < ?", cutoff).
        Delete(&models.AuditLog{}).Error
}
```

---

## First-Time Setup

### Setup Handler

**Location:** `internal/api/handlers/setup.go`

The setup wizard is only accessible when no users exist in the system.

```go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "shellcn/internal/models"
    "shellcn/internal/services"
    "shellcn/pkg/response"
)

type SetupHandler struct {
    userService *services.UserService
    db          *gorm.DB
}

func NewSetupHandler(userService *services.UserService, db *gorm.DB) *SetupHandler {
    return &SetupHandler{
        userService: userService,
        db:          db,
    }
}

// Check if setup is needed
func (h *SetupHandler) CheckSetupNeeded(c *gin.Context) {
    var count int64
    h.db.Model(&models.User{}).Count(&count)

    response.Success(c, http.StatusOK, gin.H{
        "setup_needed": count == 0,
    })
}

// Create first user (root/superuser)
func (h *SetupHandler) CreateFirstUser(c *gin.Context) {
    // Check if users already exist
    var count int64
    h.db.Model(&models.User{}).Count(&count)

    if count > 0 {
        response.Error(c, errors.New("setup already completed"))
        return
    }

    var req struct {
        Username  string `json:"username" binding:"required"`
        Email     string `json:"email" binding:"required,email"`
        Password  string `json:"password" binding:"required,min=8"`
        FirstName string `json:"first_name"`
        LastName  string `json:"last_name"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, err)
        return
    }

    // Create root user
    user, err := h.userService.Create(req.Username, req.Email, req.Password, true)
    if err != nil {
        response.Error(c, err)
        return
    }

    // Update profile
    if req.FirstName != "" || req.LastName != "" {
        h.userService.Update(user.ID, map[string]interface{}{
            "first_name": req.FirstName,
            "last_name":  req.LastName,
        })
    }

    response.Success(c, http.StatusCreated, gin.H{
        "user": user,
        "message": "Setup completed successfully",
    })
}
```

**Frontend Integration:**

- Check `/api/setup/check` on app load
- If `setup_needed: true`, redirect to `/setup` page
- Setup page calls `/api/setup/complete` with first user details
- After successful setup, redirect to login page

---

## API Endpoints

### Router Configuration

**Location:** `internal/api/router.go`

```go
package api

import (
    "github.com/gin-gonic/gin"
    "shellcn/internal/api/handlers"
    "shellcn/internal/api/middleware"
)

func SetupRouter(
    authHandler *handlers.AuthHandler,
    setupHandler *handlers.SetupHandler,
    userHandler *handlers.UserHandler,
    // ... other handlers
    authMiddleware *middleware.AuthMiddleware,
    permMiddleware *middleware.PermissionMiddleware,
) *gin.Engine {
    r := gin.New()

    // Global middleware
    r.Use(gin.Recovery())
    r.Use(middleware.Logger())
    r.Use(middleware.CORS())
    r.Use(middleware.Metrics())

    // Public routes
    public := r.Group("/api")
    {
        // Setup (only when no users exist)
        public.GET("/setup/check", setupHandler.CheckSetupNeeded)
        public.POST("/setup/complete", setupHandler.CreateFirstUser)

        // Auth
        public.POST("/auth/login", authHandler.Login)
        public.POST("/auth/refresh", authHandler.Refresh)
        public.POST("/auth/password-reset/request", authHandler.RequestPasswordReset)
        public.POST("/auth/password-reset/confirm", authHandler.ConfirmPasswordReset)

        // Auth providers (public - for login page to show enabled providers)
        public.GET("/auth/providers", authProviderHandler.GetEnabled)

        // Health
        public.GET("/health", handlers.Health)
    }

    // Protected routes
    protected := r.Group("/api")
    protected.Use(authMiddleware.RequireAuth())
    {
        // Auth (authenticated)
        protected.POST("/auth/logout", authHandler.Logout)
        protected.GET("/auth/me", authHandler.GetCurrentUser)
        protected.POST("/auth/mfa/setup", authHandler.SetupMFA)
        protected.POST("/auth/mfa/verify", authHandler.VerifyMFA)
        protected.POST("/auth/mfa/disable", authHandler.DisableMFA)

        // Users
        users := protected.Group("/users")
        {
            users.GET("", permMiddleware.Require("user.view"), userHandler.List)
            users.GET("/:id", permMiddleware.Require("user.view"), userHandler.Get)
            users.POST("", permMiddleware.Require("user.create"), userHandler.Create)
            users.PUT("/:id", permMiddleware.Require("user.edit"), userHandler.Update)
            users.DELETE("/:id", permMiddleware.Require("user.delete"), userHandler.Delete)
            users.POST("/:id/activate", permMiddleware.Require("user.edit"), userHandler.Activate)
            users.POST("/:id/deactivate", permMiddleware.Require("user.edit"), userHandler.Deactivate)
        }

        // Organizations
        orgs := protected.Group("/organizations")
        {
            orgs.GET("", permMiddleware.Require("org.view"), orgHandler.List)
            orgs.GET("/:id", permMiddleware.Require("org.view"), orgHandler.Get)
            orgs.POST("", permMiddleware.Require("org.create"), orgHandler.Create)
            orgs.PUT("/:id", permMiddleware.Require("org.manage"), orgHandler.Update)
            orgs.DELETE("/:id", permMiddleware.Require("org.manage"), orgHandler.Delete)
        }

        // Teams
        teams := protected.Group("/teams")
        {
            teams.GET("", permMiddleware.Require("org.view"), teamHandler.List)
            teams.POST("", permMiddleware.Require("org.manage"), teamHandler.Create)
            teams.POST("/:id/members", permMiddleware.Require("org.manage"), teamHandler.AddMember)
            teams.DELETE("/:id/members/:user_id", permMiddleware.Require("org.manage"), teamHandler.RemoveMember)
        }

        // Permissions
        perms := protected.Group("/permissions")
        {
            perms.GET("", permMiddleware.Require("permission.view"), permHandler.List)
            perms.GET("/my", permHandler.GetMyPermissions) // No permission required
            perms.POST("/roles/:role_id", permMiddleware.Require("permission.manage"), permHandler.AssignToRole)
        }

        // Sessions
        sessions := protected.Group("/sessions")
        {
            sessions.GET("", sessionHandler.ListMySessions) // Own sessions
            sessions.DELETE("/:id", sessionHandler.Revoke)
        }

        // Audit logs
        audit := protected.Group("/audit")
        {
            audit.GET("", permMiddleware.Require("audit.view"), auditHandler.List)
            audit.GET("/export", permMiddleware.Require("audit.export"), auditHandler.Export)
        }

        // Auth provider management (admin only)
        providers := protected.Group("/auth/providers")
        providers.Use(permMiddleware.Require("permission.manage"))
        {
            providers.GET("/all", authProviderHandler.ListAll)
            providers.GET("/:type", authProviderHandler.Get)
            providers.POST("/oidc", authProviderHandler.ConfigureOIDC)
            providers.POST("/saml", authProviderHandler.ConfigureSAML)
            providers.POST("/ldap", authProviderHandler.ConfigureLDAP)
            providers.PUT("/local", authProviderHandler.UpdateLocal)
            providers.PUT("/invite", authProviderHandler.UpdateInvite)
            providers.PUT("/:type/enable", authProviderHandler.Enable)
            providers.PUT("/:type/disable", authProviderHandler.Disable)
            providers.POST("/:type/test", authProviderHandler.TestConnection)
            providers.DELETE("/:type", authProviderHandler.Delete)
        }
    }

    // Metrics endpoint (Prometheus)
    r.GET("/metrics", gin.WrapH(promhttp.Handler()))

    return r
}
```

### Complete API Endpoint List

#### Public Endpoints

| Method | Path                               | Description               |
| ------ | ---------------------------------- | ------------------------- |
| GET    | `/api/setup/check`                 | Check if setup is needed  |
| POST   | `/api/setup/complete`              | Complete first-time setup |
| POST   | `/api/auth/login`                  | Login with credentials    |
| POST   | `/api/auth/refresh`                | Refresh access token      |
| POST   | `/api/auth/password-reset/request` | Request password reset    |
| POST   | `/api/auth/password-reset/confirm` | Confirm password reset    |
| GET    | `/api/health`                      | Health check              |

#### Protected Endpoints

**Authentication:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| POST | `/api/auth/logout` | - | Logout current session |
| GET | `/api/auth/me` | - | Get current user |
| POST | `/api/auth/mfa/setup` | - | Setup MFA |
| POST | `/api/auth/mfa/verify` | - | Verify MFA code |
| POST | `/api/auth/mfa/disable` | - | Disable MFA |

**Users:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/users` | `user.view` | List users |
| GET | `/api/users/:id` | `user.view` | Get user details |
| POST | `/api/users` | `user.create` | Create user |
| PUT | `/api/users/:id` | `user.edit` | Update user |
| DELETE | `/api/users/:id` | `user.delete` | Delete user |
| POST | `/api/users/:id/activate` | `user.edit` | Activate user |
| POST | `/api/users/:id/deactivate` | `user.edit` | Deactivate user |

**Organizations:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/organizations` | `org.view` | List organizations |
| GET | `/api/organizations/:id` | `org.view` | Get organization |
| POST | `/api/organizations` | `org.create` | Create organization |
| PUT | `/api/organizations/:id` | `org.manage` | Update organization |
| DELETE | `/api/organizations/:id` | `org.manage` | Delete organization |

**Teams:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/teams` | `org.view` | List teams |
| POST | `/api/teams` | `org.manage` | Create team |
| POST | `/api/teams/:id/members` | `org.manage` | Add team member |
| DELETE | `/api/teams/:id/members/:user_id` | `org.manage` | Remove team member |

**Permissions:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/permissions` | `permission.view` | List all permissions |
| GET | `/api/permissions/my` | - | Get my permissions |
| POST | `/api/permissions/roles/:role_id` | `permission.manage` | Assign permissions to role |

**Sessions:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/sessions` | - | List my sessions |
| DELETE | `/api/sessions/:id` | - | Revoke session |

**Audit:**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/audit` | `audit.view` | List audit logs |
| GET | `/api/audit/export` | `audit.export` | Export audit logs |

**Auth Providers (UI Configuration):**
| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| GET | `/api/auth/providers` | - | List enabled providers (public, for login page) |
| GET | `/api/auth/providers/all` | `permission.manage` | List all providers (admin) |
| GET | `/api/auth/providers/:type` | `permission.manage` | Get provider config |
| POST | `/api/auth/providers/oidc` | `permission.manage` | Configure OIDC provider |
| POST | `/api/auth/providers/saml` | `permission.manage` | Configure SAML provider |
| POST | `/api/auth/providers/ldap` | `permission.manage` | Configure LDAP provider |
| PUT | `/api/auth/providers/local` | `permission.manage` | Update local settings |
| PUT | `/api/auth/providers/invite` | `permission.manage` | Update invite settings |
| PUT | `/api/auth/providers/:type/enable` | `permission.manage` | Enable provider |
| PUT | `/api/auth/providers/:type/disable` | `permission.manage` | Disable provider |
| POST | `/api/auth/providers/:type/test` | `permission.manage` | Test provider connection |
| DELETE | `/api/auth/providers/:type` | `permission.manage` | Delete provider config |

---

## Middleware

### Authentication Middleware

**Location:** `internal/api/middleware/auth.go`

```go
package middleware

import (
    "strings"
    "github.com/gin-gonic/gin"
    "shellcn/internal/auth"
    "shellcn/pkg/errors"
    "shellcn/pkg/response"
)

type AuthMiddleware struct {
    jwtService *auth.JWTService
}

func NewAuthMiddleware(jwtService *auth.JWTService) *AuthMiddleware {
    return &AuthMiddleware{jwtService: jwtService}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract token from Authorization header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            response.Error(c, errors.ErrUnauthorized)
            c.Abort()
            return
        }

        // Parse Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            response.Error(c, errors.ErrUnauthorized)
            c.Abort()
            return
        }

        token := parts[1]

        // Validate token
        claims, err := m.jwtService.ValidateAccessToken(token)
        if err != nil {
            response.Error(c, errors.ErrUnauthorized)
            c.Abort()
            return
        }

        // Set user info in context
        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("is_root", claims.IsRoot)
        c.Set("permissions", claims.Permissions)

        c.Next()
    }
}
```

### Permission Middleware

**Location:** `internal/api/middleware/permission.go`

```go
package middleware

import (
    "github.com/gin-gonic/gin"
    "shellcn/internal/permissions"
    "shellcn/pkg/errors"
    "shellcn/pkg/response"
)

type PermissionMiddleware struct {
    checker *permissions.Checker
}

func NewPermissionMiddleware(checker *permissions.Checker) *PermissionMiddleware {
    return &PermissionMiddleware{checker: checker}
}

func (m *PermissionMiddleware) Require(permissionID string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Get user from context (set by auth middleware)
        userID, exists := c.Get("user_id")
        if !exists {
            response.Error(c, errors.ErrUnauthorized)
            c.Abort()
            return
        }

        // Check if root user (bypass permission check)
        isRoot, _ := c.Get("is_root")
        if isRoot.(bool) {
            c.Next()
            return
        }

        // Check permission
        hasPermission, err := m.checker.Check(c.Request.Context(), userID.(string), permissionID)
        if err != nil {
            response.Error(c, err)
            c.Abort()
            return
        }

        if !hasPermission {
            response.Error(c, errors.ErrForbidden)
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### Other Middleware

**CORS Middleware** (`internal/api/middleware/cors.go`):

```go
func CORS() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    }
}
```

**Logger Middleware** (`internal/api/middleware/logger.go`):

```go
func Logger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        c.Next()

        duration := time.Since(start)

        logger.Info("HTTP Request",
            zap.String("method", c.Request.Method),
            zap.String("path", path),
            zap.Int("status", c.Writer.Status()),
            zap.Duration("duration", duration),
            zap.String("ip", c.ClientIP()),
        )
    }
}
```

**Rate Limiting Middleware** (`internal/api/middleware/ratelimit.go`):

```go
func RateLimit() gin.HandlerFunc {
    // Use golang.org/x/time/rate
    limiter := rate.NewLimiter(rate.Limit(100), 200) // 100 req/sec, burst 200

    return func(c *gin.Context) {
        if !limiter.Allow() {
            response.Error(c, errors.New("RATE_LIMIT", "Too many requests", 429))
            c.Abort()
            return
        }
        c.Next()
    }
}
```

---

## Security Implementation

### Password Security

1. **Hashing Algorithm:** bcrypt with cost factor 10+
2. **Password Requirements:**

   - Minimum 8 characters
   - At least one uppercase letter
   - At least one lowercase letter
   - At least one number
   - At least one special character (optional but recommended)

3. **Password Reset Flow:**
   - Generate secure random token
   - Hash token before storing
   - Set expiration (1 hour)
   - Send email with reset link
   - Validate token on reset
   - Mark token as used

### Session Security

1. **JWT Tokens:**

   - Access token: 15 minutes expiry
   - Refresh token: 7 days expiry
   - Stored in httpOnly cookies (frontend)
   - Include user permissions in claims

2. **Session Management:**
   - Track device/IP/User-Agent
   - Allow session revocation
   - Automatic cleanup of expired sessions
   - Support multi-device sessions

### MFA Security

1. **TOTP Implementation:**

   - Use standard TOTP algorithm (RFC 6238)
   - 30-second time step
   - 6-digit codes
   - QR code generation for easy setup

2. **Backup Codes:**
   - Generate 10 backup codes
   - Hash before storage
   - One-time use
   - Allow regeneration

### Encryption

1. **Vault Credentials:**

   - AES-256-GCM encryption
   - Master key from environment variable
   - Key derivation using Argon2id
   - Unique nonce per encryption

2. **Sensitive Data:**
   - MFA secrets encrypted at rest
   - Password reset tokens hashed
   - Audit logs include IP addresses

### Security Headers

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-XSS-Protection", "1; mode=block")
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("Content-Security-Policy", "default-src 'self'")
        c.Next()
    }
}
```

---

## Monitoring & Observability

### Prometheus Metrics

**Location:** `pkg/metrics/metrics.go`

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Authentication metrics
    AuthAttempts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "shellcn_auth_attempts_total",
            Help: "Total number of authentication attempts",
        },
        []string{"result"}, // success, failure
    )

    // Permission checks
    PermissionChecks = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "shellcn_permission_checks_total",
            Help: "Total number of permission checks",
        },
        []string{"permission", "result"},
    )

    // Active sessions
    ActiveSessions = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "shellcn_active_sessions",
            Help: "Number of active sessions",
        },
    )

    // API latency
    APILatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "shellcn_api_latency_seconds",
            Help:    "API endpoint latency",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method", "path", "status"},
    )
)
```

### Structured Logging

Use `pkg/logger` with module-specific loggers:

```go
logger := logger.WithModule("auth")
logger.Info("User logged in",
    zap.String("user_id", user.ID),
    zap.String("ip", ipAddress),
)
```

### Health Check

**Location:** `internal/api/handlers/health.go`

```go
func Health(c *gin.Context) {
    // Check database connection
    sqlDB, err := db.DB()
    if err != nil {
        c.JSON(503, gin.H{"status": "unhealthy", "database": "error"})
        return
    }

    if err := sqlDB.Ping(); err != nil {
        c.JSON(503, gin.H{"status": "unhealthy", "database": "down"})
        return
    }

    c.JSON(200, gin.H{
        "status": "healthy",
        "database": "up",
        "version": "1.0.0",
    })
}
```

---

## Testing Strategy

### Unit Tests

**Test Coverage Goals:** ≥80% for core packages

#### Service Layer Tests

**Example:** `internal/services/user_service_test.go`

```go
package services_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "shellcn/internal/services"
    "shellcn/pkg/testing"
)

func TestUserService_Create(t *testing.T) {
    db := testing.SetupTestDB(t)
    defer testing.TeardownTestDB(t, db)

    auditService := services.NewAuditService(db)
    userService := services.NewUserService(db, auditService)

    user, err := userService.Create("testuser", "test@example.com", "password123", false)

    assert.NoError(t, err)
    assert.NotNil(t, user)
    assert.Equal(t, "testuser", user.Username)
    assert.NotEmpty(t, user.ID)
}

func TestUserService_Delete_RootUser(t *testing.T) {
    db := testing.SetupTestDB(t)
    defer testing.TeardownTestDB(t, db)

    auditService := services.NewAuditService(db)
    userService := services.NewUserService(db, auditService)

    // Create root user
    user, _ := userService.Create("root", "root@example.com", "password123", true)

    // Attempt to delete root user
    err := userService.Delete(user.ID)

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "cannot delete root user")
}
```

#### Permission System Tests

**Example:** `internal/permissions/checker_test.go`

```go
func TestChecker_RootUserBypass(t *testing.T) {
    db := testing.SetupTestDB(t)
    defer testing.TeardownTestDB(t, db)

    // Create root user
    rootUser := &models.User{
        Username: "root",
        IsRoot:   true,
    }
    db.Create(rootUser)

    checker := permissions.NewChecker(db)

    // Root user should have all permissions
    hasPermission, err := checker.Check(context.Background(), rootUser.ID, "any.permission")

    assert.NoError(t, err)
    assert.True(t, hasPermission)
}

func TestChecker_DependencyResolution(t *testing.T) {
    // Test that permission dependencies are properly resolved
    // user.delete depends on user.view and user.edit
    // User should have all three permissions to delete
}
```

#### Authentication Tests

**Example:** `internal/auth/jwt_test.go`

```go
func TestJWTService_GenerateAndValidate(t *testing.T) {
    jwtService := auth.NewJWTService("test-secret-key")

    user := &models.User{
        ID:       "user-123",
        Username: "testuser",
        IsRoot:   false,
    }

    permissions := []string{"user.view", "user.create"}

    // Generate token
    token, err := jwtService.GenerateAccessToken(user, permissions)
    assert.NoError(t, err)
    assert.NotEmpty(t, token)

    // Validate token
    claims, err := jwtService.ValidateAccessToken(token)
    assert.NoError(t, err)
    assert.Equal(t, user.ID, claims.UserID)
    assert.Equal(t, user.Username, claims.Username)
    assert.Equal(t, permissions, claims.Permissions)
}
```

### Integration Tests

**Test HTTP endpoints with real database**

**Example:** `internal/api/handlers/auth_test.go`

```go
func TestAuthHandler_Login(t *testing.T) {
    // Setup test server
    router, db := testing.SetupTestServer(t)
    defer testing.TeardownTestDB(t, db)

    // Create test user
    hashedPassword, _ := crypto.HashPassword("password123")
    user := &models.User{
        Username: "testuser",
        Email:    "test@example.com",
        Password: hashedPassword,
        IsActive: true,
    }
    db.Create(user)

    // Test login
    w := httptest.NewRecorder()
    body := `{"username":"testuser","password":"password123"}`
    req, _ := http.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    router.ServeHTTP(w, req)

    assert.Equal(t, 200, w.Code)

    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)

    assert.True(t, response["success"].(bool))
    assert.NotEmpty(t, response["data"].(map[string]interface{})["access_token"])
}
```

### Test Utilities

**Location:** `pkg/testing/testing.go`

```go
package testing

import (
    "testing"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "shellcn/internal/database"
)

func SetupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatal(err)
    }

    // Run migrations
    if err := database.AutoMigrate(db); err != nil {
        t.Fatal(err)
    }

    return db
}

func TeardownTestDB(t *testing.T, db *gorm.DB) {
    sqlDB, _ := db.DB()
    sqlDB.Close()
}

func SetupTestServer(t *testing.T) (*gin.Engine, *gorm.DB) {
    db := SetupTestDB(t)

    // Initialize services
    // Initialize handlers
    // Setup router

    return router, db
}
```

### Contract Tests

Test critical contracts:

- JWT token structure
- API response format
- Permission dependency rules
- MFA enrollment flow

### Static Analysis

```bash
# Run golangci-lint
golangci-lint run ./...

# Run race detector
go test -race ./...

# Check coverage
go test -cover ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

---

## Implementation Checklist

### Phase 1: Foundation (Week 1)

- [x] **Project Setup**

  - [x] Initialize Go module
  - [x] Setup directory structure
  - [x] Configure Makefile
  - [x] Setup CI/CD pipeline

- [x] **Shared Packages**

  - [x] Implement `pkg/logger`
  - [x] Implement `pkg/errors`
  - [x] Implement `pkg/response`
  - [x] Implement `pkg/crypto`
  - [x] Implement `pkg/validator`
  - [x] Write tests for shared packages

- [x] **Database Layer**
  - [x] Define all GORM models
  - [x] Implement database initialization
  - [x] Create migration system
  - [x] Setup SQLite driver
  - [x] Write model tests

### Phase 2: Authentication (Week 2)

- [x] **JWT Service**

  - [x] Implement token generation
  - [x] Implement token validation
  - [x] Write JWT tests

- [x] **Local Auth Provider**

  - [x] Implement login
  - [x] Implement password hashing
  - [x] Implement account lockout
  - [x] Write auth provider tests

- [x] **Session Management**

  - [x] Implement session service
  - [x] Implement refresh token flow
  - [x] Implement session revocation
  - [x] Introduce cache abstraction with Redis preference and SQL fallback
  - [x] Write session tests

- [x] **MFA (Optional)**
  - [x] Implement TOTP service
  - [x] Implement QR code generation
  - [x] Implement backup codes
  - [x] Write MFA tests

### Phase 3: Authorization (Week 3)

- [x] **Permission System**

  - [x] Implement permission registry
  - [x] Register core permissions
  - [x] Implement permission checker
  - [x] Implement dependency resolver
  - [x] Write permission tests

- [x] **Permission Service**
  - [x] Implement role management
  - [x] Implement permission assignment
  - [x] Write permission service tests

### Phase 4: Core Services (Week 4)

- [x] **User Service**

  - [x] Implement CRUD operations
  - [x] Implement activation/deactivation
  - [x] Implement password management
  - [x] Write user service tests

- [x] **Organization Service**

  - [x] Implement CRUD operations
  - [x] Write organization service tests

- [x] **Team Service**

  - [x] Implement team management
  - [x] Implement member management
  - [x] Write team service tests

- [x] **Audit Service**

  - [x] Implement audit logging
  - [x] Implement log filtering
  - [x] Implement log export
  - [x] Write audit service tests

- [x] **Auth Provider Service**
  - [x] Implement provider CRUD
  - [x] Implement OIDC configuration
  - [x] Implement SAML configuration
  - [x] Implement LDAP configuration
  - [x] Implement local/invite settings
  - [x] Implement connection testing
  - [x] Write auth provider service tests

### Phase 5: API Layer (Week 5)

- [x] **Middleware**

  - [x] Implement auth middleware
  - [x] Implement permission middleware
  - [x] Implement CORS middleware
  - [x] Implement logger middleware
  - [x] Implement rate limiting backed by shared cache with SQL fallback
  - [x] Write middleware tests

- [x] **Handlers**

  - [x] Implement auth handlers
  - [x] Implement setup handler
  - [x] Implement user handlers
  - [x] Implement organization handlers
  - [x] Implement team handlers
  - [x] Implement permission handlers
  - [x] Implement session handlers
  - [x] Implement audit handlers
  - [x] Implement auth provider handlers
  - [x] Write handler integration tests

- [x] **Router**
  - [x] Configure all routes
  - [x] Setup route groups
  - [x] Apply middleware
  - [x] Write router tests

### Phase 6: Security & Monitoring (Week 6)

- [x] **Security**

  - [x] Implement security headers
  - [x] Implement CSRF protection
  - [x] Implement input validation
  - [x] Security audit

- [x] **Monitoring**

  - [x] Implement Prometheus metrics
  - [x] Implement health check
  - [x] Setup structured logging
  - [x] Configure log levels

- [x] **Background Jobs**
  - [x] Implement session cleanup
  - [x] Implement audit log retention
  - [x] Implement token cleanup

### Phase 7: Testing & Documentation (Week 7)

- [ ] **Testing**

  - [ ] Achieve 80%+ test coverage  
    - Add a CI coverage gate (`go test ./... -coverprofile=coverage.out` + `go tool cover -func=coverage.out`) targeting ≥80% overall and ≥70% per package.  
    - Backfill missing unit tests for core services (`internal/services/*`), permission checker edge cases (`internal/permissions/checker.go`), and routing helpers (`internal/api/routes_setup.go`) using the existing `testutil` fixtures.  
    - Include regression cases for first-user bootstrap, session revocation, permission dependency denial, and audit logging so critical flows stay protected.  
    - Progress: expanded coverage for SMTP validation + dial/auth flows, provider registry & OIDC/SAML/LDAP factories, runtime defaults, and logger helpers (overall coverage ~55%).

  - [ ] Run integration tests  
    - Use `testutil.NewServer` to stand up an in-memory stack (SQLite + mocked Redis) and run end-to-end tests against `internal/api/handlers/*`.  
    - Cover authentication (login, refresh, logout), org/team CRUD, permission assignment, setup wizard, and audit export flows.  
    - Seed fixtures via repository layer helpers and assert HTTP responses, database mutations, emitted audit events, and background job queues.

  - [ ] Run contract tests  
    - Validate JSON envelope contract (`success`, `error`, `data`) and pagination schema for every authenticated endpoint using golden responses.  
    - Verify JWT claims (subject, permissions, expiry) and permission dependency rules using table-driven tests.  
    - Add contract checks for external provider enablement API to guarantee consistent configuration payloads for the frontend.

  - [ ] Performance testing  
    - Stress the hottest endpoints (`POST /api/auth/login`, `GET /api/users`, `GET /api/permissions/graph`) with `hey` or `vegeta` at target RPS baselines (P50 <100ms, P95 <300ms).  
    - Capture CPU/memory profiles with `go test -bench` + `pprof` while running load to identify bottlenecks in session and permission checks.  
    - Document tuning knobs (database pool, Redis TTLs, rate limiter burst) and feed results into the deployment guide.

  - [ ] Security testing  
    - Execute `golangci-lint`, `gosec ./...`, and `staticcheck ./...` in CI; triage and fix any findings.  
    - Run dependency vulnerability scans via `govulncheck ./...` and ensure Makefile target exists.  
    - Perform manual penetration checklist: JWT tampering, privilege escalation, rate-limit bypass, MFA recovery abuse, and misconfigured external providers.

- [ ] **Documentation**
  - [ ] API documentation (specs/plans/CORE_MODULE_API.md)  
    - Produce an OpenAPI 3.1 spec for all core endpoints covering auth, users, permissions, sessions, audit, organizations, teams, and providers.  
    - Include request/response schemas, error codes, permission requirements, and example payloads aligned with contract tests.  
    - Generate both human-readable markdown and JSON/YAML artefacts; link publishing steps (Redocly/Stoplight) from the README.

  - [ ] Deployment CI/CD (GHCR image on tag/manual dispatch)  
    - Extend GitHub Actions workflow to build multi-arch images, run the full test suite (unit + integration), and push to `ghcr.io/shellcn/core`.  
    - Store GHCR credentials in repository secrets, sign images with cosign, and attach SBOM (syft) before release.  
    - Provide rollback guidance and tag naming conventions (e.g., `core-vMAJOR.MINOR.PATCH`).

  - [ ] Configuration guide  
    - Document every `config.yaml` and ENV option from `internal/app/config.go`, including Redis fallback behavior, feature toggles, and module enablement.  
    - Add examples for single-node (SQLite) and production (Postgres + Redis + TLS) setups with sample `docker-compose` snippets.  
    - Highlight sensitive values (JWT secret, vault key) and include rotation/backup recommendations.

  - [ ] Troubleshooting guide  
    - Catalogue common failure scenarios: failed first-user setup, database connectivity, Redis fallback degradation, authentication provider misconfiguration, rate limit lockouts.  
    - Provide log snippets, diagnostic commands (`make debug`, `kubectl logs`, `go tool pprof`), and resolution steps.  
    - Append support escalation checklist (evidence to gather, metrics dashboards, audit log exports).

### Phase 8: External Auth Providers (Optional - Week 8)

- [ ] **Shared SSO Foundation**

  - [ ] Finalize `internal/auth/providers` interfaces so external providers plug into existing session issuance pipeline (`SessionService.CreateForSubject`)
  - [ ] Extend provider registry to expose provider metadata (type, display name, button label, enabled status, test capability)
  - [ ] Implement unified SSO callback handler flow that maps external identities to local users (by email) with optional auto-provision toggle
  - [ ] Persist provider enablement flags, secrets, and mapping rules via `AuthProviderService` using encrypted storage helpers
  - [ ] Add audit events for provider enable/disable, configuration updates, connection tests, and login attempts

- [ ] **OIDC Provider**

  - [ ] Implement full OIDC authorization code flow with PKCE, nonce handling, and state validation (uses `coreos/go-oidc` + `golang.org/x/oauth2`)
  - [ ] Support discovery (`/.well-known/openid-configuration`), metadata caching, and automatic JWKS refresh/rotation with background cache invalidation
  - [ ] Map standard claims (`sub`, `email`, `given_name`, `family_name`, `preferred_username`) to local user fields with configurable claim keys
  - [ ] Handle user provisioning rules (auto-create with default role, require invite, or deny unknown accounts)
  - [ ] Expose `/api/auth/providers/oidc/login` + callback endpoint and route wiring, including error redirect contract for frontend
  - [ ] Write tests: unit coverage for token validation and claim mapping, service integration test faking OIDC provider, and handler tests using httptest

- [ ] **SAML Provider**

  - [ ] Implement SAML Service Provider configuration (entity ID, ACS URL, metadata generation) using `crewjam/saml`
  - [ ] Support metadata ingestion (IdP metadata URL/upload) with certificate parsing, signature verification, and clock skew handling
  - [ ] Map attributes (NameID, email, first/last name, groups) via configurable attribute statements and enforce required attributes
  - [ ] Add ACS endpoint that exchanges SAML assertions for local sessions, including replay detection, audience validation, and optional forceAuthn
  - [ ] Provide IdP-initiated login support and SP metadata download endpoint for administrators
  - [ ] Write tests: assertion validation unit tests, metadata parsing tests, handler integration test with signed sample assertions

- [ ] **LDAP Provider**
  - [ ] Implement LDAP bind/authenticate flow with support for StartTLS, skip-verify toggle, and connection pooling/back-off
  - [ ] Support both simple bind (user DN template) and search+bind (search filter) strategies configurable via UI
  - [ ] Map LDAP attributes (uid, mail, givenName, sn, memberOf) into local user profile and optional role assignment rules
  - [ ] Implement scheduled sync job (optional) to pre-create or disable users based on LDAP search filters
  - [ ] Add `/api/auth/providers/ldap/test` command handler to validate connectivity, bind credentials, and sample attribute mapping
  - [ ] Write tests: mock LDAP server interaction tests (using `go-ldap` in-memory server), service tests for attribute mapping, and handler tests for test endpoint

---

## Dependencies

### Required Go Packages

```go
require (
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.0.0
    github.com/google/uuid v1.3.0
    github.com/prometheus/client_golang v1.16.0
    github.com/pquerna/otp v1.4.0
    github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
    go.uber.org/zap v1.25.0
    golang.org/x/crypto v0.12.0
    golang.org/x/time v0.3.0
    gorm.io/driver/sqlite v1.5.3
    gorm.io/driver/postgres v1.5.2
    gorm.io/driver/mysql v1.5.1
    gorm.io/gorm v1.25.4
)
```

### Optional Packages (for external auth)

```go
require (
    github.com/coreos/go-oidc/v3 v3.6.0
    github.com/crewjam/saml v0.4.13
    github.com/go-ldap/ldap/v3 v3.4.5
)
```

---

## Configuration

### Environment Variables

```bash
# Server
SERVER_PORT=8080
SERVER_HOST=0.0.0.0

# Database
DB_DRIVER=sqlite # sqlite, postgres, mysql
DB_PATH=./data/shellcn.db # for SQLite
DB_HOST=localhost # for Postgres/MySQL
DB_PORT=5432
DB_NAME=shellcn
DB_USER=shellcn
DB_PASSWORD=secret

# JWT
JWT_SECRET=your-secret-key-change-this
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h

# Email
EMAIL_SMTP_ENABLED=false
EMAIL_SMTP_HOST=smtp.example.com
EMAIL_SMTP_PORT=587
EMAIL_SMTP_USERNAME=mailer
EMAIL_SMTP_PASSWORD=super-secret
EMAIL_SMTP_FROM=no-reply@example.com
EMAIL_SMTP_USE_TLS=true
EMAIL_SMTP_TIMEOUT=10s

# Vault Encryption
VAULT_ENCRYPTION_KEY=your-32-byte-encryption-key

# Logging
LOG_LEVEL=info # debug, info, warn, error

# Features
ENABLE_MFA=true

# NOTE: Authentication providers (OIDC, SAML, LDAP) are configured via UI
# by administrators, not through environment variables. See Auth Provider Management.
```

---

## Summary

This implementation plan provides a complete roadmap for building the Core Module backend. The module follows best practices from `BACKEND_PATTERNS.md` and provides:

1. **Robust Authentication:** Local auth, JWT sessions, optional MFA
2. **Flexible Authorization:** RBAC with permission dependencies, root user bypass
3. **Complete User Management:** CRUD, activation, password management
4. **Multi-tenancy:** Organizations and teams
5. **Audit Trail:** Comprehensive logging of all actions
6. **Security:** Encryption, secure sessions, password policies
7. **Observability:** Prometheus metrics, structured logging, health checks
8. **Testability:** High test coverage, integration tests, test utilities

The implementation is designed to be extended by other modules while maintaining security and performance standards.

---

**Next Steps:**

1. Review this plan with the team
2. Set up development environment
3. Begin Phase 1 implementation
4. Coordinate with frontend team for API contract alignment
