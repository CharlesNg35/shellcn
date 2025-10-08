# ShellCN Platform - Shared Patterns & Guidelines

This document defines common patterns, packages, conventions, and best practices that ALL modules must follow for consistency and maintainability.

---

## Table of Contents

1. [Technology Stack Versions](#technology-stack-versions)
2. [Backend Patterns](#backend-patterns)
3. [Frontend Patterns](#frontend-patterns)
4. [Database Patterns](#database-patterns)
5. [API Conventions](#api-conventions)
6. [Authentication & Authorization](#authentication--authorization)
7. [Error Handling](#error-handling)
8. [Testing Standards](#testing-standards)
9. [Security Patterns](#security-patterns)
10. [WebSocket Patterns](#websocket-patterns)
11. [Logging & Monitoring](#logging--monitoring)
12. [Code Style & Formatting](#code-style--formatting)
13. [Documentation Standards](#documentation-standards)
14. [Build & Deployment](#build--deployment)

---

## 1. Technology Stack Versions

### Backend (Go)

**Always check for latest versions before implementation!**

```go
// go.mod
module github.com/yourusername/shellcn

go 1.21

require (
    // Web Framework
    github.com/gin-gonic/gin v1.10.0

    // Database
    gorm.io/gorm v1.25.5
    gorm.io/driver/sqlite v1.5.4
    gorm.io/driver/postgres v1.5.4
    gorm.io/driver/mysql v1.5.2

    // Authentication
    github.com/golang-jwt/jwt/v5 v5.2.0
    golang.org/x/crypto v0.17.0

    // OIDC/SAML/LDAP
    github.com/coreos/go-oidc/v3 v3.9.0
    golang.org/x/oauth2 v0.15.0
    github.com/crewjam/saml v0.4.14
    github.com/go-ldap/ldap/v3 v3.4.6

    // MFA
    github.com/pquerna/otp v1.4.0
    github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e

    // WebSocket
    github.com/gorilla/websocket v1.5.1

    // SSH & SFTP
    golang.org/x/crypto/ssh v0.17.0
    github.com/pkg/sftp v1.13.6

    // Telnet
    github.com/ziutek/telnet v0.0.0-20180329124119-c3b780dc415b

    // Docker
    github.com/docker/docker v24.0.7+incompatible
    github.com/docker/go-connections v0.4.0

    // Kubernetes
    k8s.io/client-go v0.29.0
    k8s.io/api v0.29.0
    k8s.io/apimachinery v0.29.0

    // Databases
    github.com/go-sql-driver/mysql v1.7.1
    github.com/lib/pq v1.10.9
    go.mongodb.org/mongo-driver v1.13.1
    github.com/redis/go-redis/v9 v9.3.1

    // File Sharing
    github.com/hirochachacha/go-smb2 v1.1.0
    github.com/aws/aws-sdk-go-v2 v1.24.0
    google.golang.org/api v0.154.0

    // Monitoring
    github.com/prometheus/client_golang v1.18.0

    // Logging
    go.uber.org/zap v1.26.0

    // Configuration
    github.com/spf13/viper v1.18.2

    // Utilities
    github.com/google/uuid v1.5.0
    golang.org/x/sync v0.5.0
    github.com/robfig/cron/v3 v3.0.1

    // Testing
    github.com/stretchr/testify v1.8.4
    github.com/golang/mock v1.6.0
)
```

### Frontend (React + TypeScript)

**Check npmjs.com for latest versions!**

```json
{
  "name": "shellcn-web",
  "version": "1.0.0",
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0",
    "react-router": "^7.6.2",

    "@radix-ui/react-dialog": "^1.0.5",
    "@radix-ui/react-dropdown-menu": "^2.0.6",
    "@radix-ui/react-select": "^2.0.0",
    "@radix-ui/react-tabs": "^1.0.4",
    "@radix-ui/react-tooltip": "^1.0.7",

    "tailwindcss": "^4.1.0",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.1.0",
    "lucide-react": "^0.460.0",

    "xterm": "^5.5.0",
    "xterm-addon-fit": "^0.10.0",
    "xterm-addon-web-links": "^0.11.0",
    "xterm-addon-search": "^0.15.0",

    "@tanstack/react-query": "^5.59.0",
    "zustand": "^5.0.0",
    "axios": "^1.7.0",
    "socket.io-client": "^4.8.0",

    "react-hook-form": "^7.53.0",
    "zod": "^3.23.0",
    "@hookform/resolvers": "^3.9.0",

    "react-dropzone": "^14.3.0",
    "@tanstack/react-table": "^8.20.0",
    "date-fns": "^4.1.0",
    "sonner": "^1.7.0"
  },
  "devDependencies": {
    "vite": "^7.0.0",
    "@vitejs/plugin-react": "^4.3.0",
    "typescript": "^5.7.0",

    "eslint": "^9.15.0",
    "@typescript-eslint/parser": "^8.15.0",
    "@typescript-eslint/eslint-plugin": "^8.15.0",
    "prettier": "^3.3.0",

    "vitest": "^2.1.0",
    "@testing-library/react": "^16.0.0",
    "@testing-library/jest-dom": "^6.1.5",
    "@testing-library/user-event": "^14.5.1"
  }
}
```

### Rust FFI Modules

**Check crates.io for latest versions!**

```toml
# rust-modules/rdp/Cargo.toml
[package]
name = "rdp-ffi"
version = "0.1.0"
edition = "2021"

[lib]
crate-type = ["staticlib"]

[dependencies]
ironrdp = "0.1"  # CHECK: https://crates.io/crates/ironrdp
tokio = { version = "1.35", features = ["full"] }

[build-dependencies]
cbindgen = "0.29"  # CHECK: https://crates.io/crates/cbindgen
```

---

## 2. Backend Patterns

### 2.1 Project Structure (Strict Convention)

```
internal/
├── app/
│   ├── app.go              # Application initialization
│   ├── config.go           # Configuration management
│   └── server.go           # HTTP server setup
│
├── api/
│   ├── router.go           # Route definitions
│   ├── middleware/         # All middleware
│   │   ├── auth.go
│   │   ├── cors.go
│   │   ├── logger.go
│   │   ├── ratelimit.go
│   │   ├── metrics.go
│   │   ├── recovery.go
│   │   └── permission.go
│   │
│   └── handlers/           # HTTP handlers
│       ├── auth.go
│       ├── setup.go
│       ├── users.go
│       └── [module]_handler.go
│
├── permissions/
│   ├── registry.go         # Global registry
│   └── core.go             # Core permissions
│
├── vault/
│   ├── vault.go
│   ├── encryption.go
│   ├── identity.go
│   └── permissions.go
│
├── modules/
│   ├── common/
│   │   ├── session.go      # Common session interface
│   │   ├── recorder.go
│   │   └── pool.go
│   │
│   ├── [protocol]/
│   │   ├── [protocol].go   # Main client
│   │   ├── session.go      # Session management
│   │   ├── permissions.go  # Module permissions
│   │   └── handler.go      # WebSocket/HTTP handler
│
├── models/                 # GORM models
├── database/              # Database layer
├── services/              # Business logic
└── utils/                 # Shared utilities
```

### 2.2 Permission Registration Pattern

**EVERY module MUST register permissions using `init()`:**

```go
// internal/modules/ssh/permissions.go
package ssh

import "shellcn/internal/permissions"

func init() {
    permissions.Register(&permissions.Permission{
        ID:          "ssh.connect",
        Module:      "ssh",
        DependsOn:   []string{"vault.view"},
        Description: "Connect to SSH servers",
    })

    permissions.Register(&permissions.Permission{
        ID:          "ssh.execute",
        Module:      "ssh",
        DependsOn:   []string{"ssh.connect"},
        Description: "Execute commands",
    })
}
```

### 2.3 Handler Pattern (Standard)

**ALL handlers MUST follow this pattern:**

```go
// internal/api/handlers/[module]_handler.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "shellcn/internal/services"
    "shellcn/internal/models"
)

type ModuleHandler struct {
    service *services.ModuleService
}

func NewModuleHandler(service *services.ModuleService) *ModuleHandler {
    return &ModuleHandler{service: service}
}

// GET /api/module/resource
func (h *ModuleHandler) List(c *gin.Context) {
    userID := c.GetString("user_id")

    resources, err := h.service.List(userID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": err.Error(),
        })
        return
    }

    c.JSON(http.StatusOK, resources)
}

// POST /api/module/resource
func (h *ModuleHandler) Create(c *gin.Context) {
    userID := c.GetString("user_id")

    var req models.CreateResourceRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error": "Invalid request",
        })
        return
    }

    resource, err := h.service.Create(userID, &req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "error": err.Error(),
        })
        return
    }

    c.JSON(http.StatusCreated, resource)
}
```

### 2.4 Service Pattern (Business Logic)

```go
// internal/services/[module]_service.go
package services

import (
    "shellcn/internal/models"
    "shellcn/internal/database/repositories"
)

type ModuleService struct {
    repo *repositories.ModuleRepository
}

func NewModuleService(repo *repositories.ModuleRepository) *ModuleService {
    return &ModuleService{repo: repo}
}

func (s *ModuleService) Create(userID string, req *models.CreateRequest) (*models.Resource, error) {
    // Validation
    if err := s.validate(req); err != nil {
        return nil, err
    }

    // Business logic
    resource := &models.Resource{
        UserID: userID,
        Name:   req.Name,
    }

    // Persist
    if err := s.repo.Create(resource); err != nil {
        return nil, err
    }

    return resource, nil
}
```

### 2.5 Repository Pattern (Data Access)

```go
// internal/database/repositories/[module]_repository.go
package repositories

import (
    "gorm.io/gorm"
    "shellcn/internal/models"
)

type ModuleRepository struct {
    db *gorm.DB
}

func NewModuleRepository(db *gorm.DB) *ModuleRepository {
    return &ModuleRepository{db: db}
}

func (r *ModuleRepository) Create(resource *models.Resource) error {
    return r.db.Create(resource).Error
}

func (r *ModuleRepository) FindByID(id string) (*models.Resource, error) {
    var resource models.Resource
    err := r.db.Where("id = ?", id).First(&resource).Error
    return &resource, err
}

func (r *ModuleRepository) List(userID string) ([]*models.Resource, error) {
    var resources []*models.Resource
    err := r.db.Where("user_id = ?", userID).Find(&resources).Error
    return resources, err
}
```

### 2.6 Model Pattern (GORM)

```go
// internal/models/[resource].go
package models

import (
    "time"
    "github.com/google/uuid"
    "gorm.io/gorm"
)

type Resource struct {
    ID        string    `gorm:"primaryKey" json:"id"`
    UserID    string    `gorm:"index" json:"user_id"`
    Name      string    `gorm:"not null" json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate hook
func (r *Resource) BeforeCreate(tx *gorm.DB) error {
    if r.ID == "" {
        r.ID = uuid.New().String()
    }
    return nil
}

// TableName override
func (Resource) TableName() string {
    return "resources"
}
```

### 2.7 Configuration Pattern

```go
// internal/app/config.go
package app

import (
    "github.com/spf13/viper"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Vault    VaultConfig
    Modules  ModulesConfig
}

type ServerConfig struct {
    Port int    `mapstructure:"port"`
    Host string `mapstructure:"host"`
}

type DatabaseConfig struct {
    Driver string `mapstructure:"driver"`
    SQLite SQLiteConfig
    Postgres PostgresConfig
}

func LoadConfig() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath(".")
    viper.AddConfigPath("./config")

    // Environment variables
    viper.SetEnvPrefix("SHELLCN")
    viper.AutomaticEnv()

    // Defaults
    viper.SetDefault("server.port", 8000)
    viper.SetDefault("database.driver", "sqlite")

    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }

    var config Config
    if err := viper.Unmarshal(&config); err != nil {
        return nil, err
    }

    return &config, nil
}
```

---

## 3. Frontend Patterns

### 3.1 Project Structure (Strict Convention)

```
web/src/
├── main.tsx
├── App.tsx
│
├── pages/                      # Page components
│   ├── Dashboard.tsx
│   ├── Login.tsx
│   ├── Setup.tsx
│   │
│   ├── settings/
│   │   ├── Identities.tsx
│   │   ├── Profile.tsx
│   │   └── Security.tsx
│   │
│   └── [module]/
│       ├── ConnectionList.tsx
│       ├── NewConnection.tsx
│       └── Session.tsx
│
├── components/                 # Reusable components
│   ├── ui/                     # Base UI (shadcn)
│   │   ├── button.tsx
│   │   ├── input.tsx
│   │   ├── dialog.tsx
│   │   └── select.tsx
│   │
│   ├── terminal/
│   │   ├── Terminal.tsx
│   │   └── TerminalToolbar.tsx
│   │
│   └── [module]/
│       └── [ModuleComponent].tsx
│
├── hooks/                      # Custom hooks
│   ├── useAuth.ts
│   ├── usePermissions.ts
│   ├── useSettings.ts
│   └── use[Module].ts
│
├── lib/                        # Utilities
│   ├── api/
│   │   ├── client.ts           # Axios client
│   │   ├── auth.ts
│   │   └── [module].ts
│   │
│   └── utils.ts
│
├── store/                      # Zustand stores
│   ├── authStore.ts
│   ├── settingsStore.ts
│   └── [module]Store.ts
│
└── types/                      # TypeScript types
    ├── api.ts
    ├── models.ts
    └── [module].ts
```

### 3.2 API Client Pattern (Axios)

```typescript
// lib/api/client.ts
import axios from 'axios';

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL || 'http://localhost:8000/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor (add auth token)
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('access_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor (handle errors)
apiClient.interceptors.response.use(
  (response) => response,
  async (error) => {
    if (error.response?.status === 401) {
      // Token expired, redirect to login
      localStorage.removeItem('access_token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export default apiClient;
```

### 3.3 React Query Pattern

```typescript
// hooks/useConnections.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { connectionsApi } from '@/lib/api/connections';
import type { Connection, CreateConnectionRequest } from '@/types/models';

export function useConnections() {
  return useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  });
}

export function useConnection(id: string) {
  return useQuery({
    queryKey: ['connections', id],
    queryFn: () => connectionsApi.get(id),
    enabled: !!id,
  });
}

export function useCreateConnection() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateConnectionRequest) => connectionsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['connections'] });
    },
  });
}
```

### 3.4 Zustand Store Pattern

```typescript
// store/settingsStore.ts
import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface TerminalPreferences {
  fontSize: number;
  fontFamily: string;
  cursorStyle: 'block' | 'underline' | 'bar';
  theme: string;
}

interface SettingsState {
  terminal: TerminalPreferences;
  updateTerminalPreferences: (prefs: Partial<TerminalPreferences>) => void;
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      terminal: {
        fontSize: 14,
        fontFamily: 'Menlo, Monaco, monospace',
        cursorStyle: 'block',
        theme: 'dark',
      },
      updateTerminalPreferences: (prefs) =>
        set((state) => ({
          terminal: { ...state.terminal, ...prefs },
        })),
    }),
    {
      name: 'settings-storage',
    }
  )
);
```

### 3.5 Permission Hook Pattern

```typescript
// hooks/usePermissions.ts
import { useAuth } from './useAuth';

export function useHasPermission(permission: string): boolean {
  const { user } = useAuth();

  // Root user has all permissions
  if (user?.isRoot) return true;

  // Check user's permission list
  return user?.permissions?.includes(permission) || false;
}

export function useHasAnyPermission(permissions: string[]): boolean {
  const { user } = useAuth();
  if (user?.isRoot) return true;
  return permissions.some(perm => user?.permissions?.includes(perm));
}

export function useHasAllPermissions(permissions: string[]): boolean {
  const { user } = useAuth();
  if (user?.isRoot) return true;
  return permissions.every(perm => user?.permissions?.includes(perm));
}
```

### 3.6 Component Pattern (Permission-Based Rendering)

```typescript
// components/UserManagement.tsx
import { useHasPermission } from '@/hooks/usePermissions';
import { Button } from '@/components/ui/button';

export function UserManagement() {
  const canCreate = useHasPermission('user.create');
  const canDelete = useHasPermission('user.delete');

  return (
    <div>
      <h1>User Management</h1>

      {canCreate && (
        <Button onClick={handleCreate}>Create User</Button>
      )}

      {canDelete && (
        <Button variant="destructive" onClick={handleDelete}>
          Delete User
        </Button>
      )}
    </div>
  );
}
```

### 3.7 Form Pattern (React Hook Form + Zod)

```typescript
// components/forms/ConnectionForm.tsx
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';

const connectionSchema = z.object({
  name: z.string().min(1, 'Name is required'),
  host: z.string().min(1, 'Host is required'),
  port: z.number().min(1).max(65535),
  identityId: z.string().optional(),
});

type ConnectionFormData = z.infer<typeof connectionSchema>;

export function ConnectionForm() {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<ConnectionFormData>({
    resolver: zodResolver(connectionSchema),
  });

  const onSubmit = (data: ConnectionFormData) => {
    console.log(data);
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)}>
      <input {...register('name')} />
      {errors.name && <span>{errors.name.message}</span>}

      <input {...register('host')} />
      {errors.host && <span>{errors.host.message}</span>}

      <button type="submit">Submit</button>
    </form>
  );
}
```

---

## 4. Database Patterns

### 4.1 Migration Pattern (GORM AutoMigrate)

```go
// internal/database/db.go
package database

import (
    "gorm.io/gorm"
    "shellcn/internal/models"
)

func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        // Core models
        &models.User{},
        &models.Organization{},
        &models.Team{},
        &models.Role{},
        &models.Permission{},
        &models.Session{},
        &models.AuditLog{},

        // Vault models
        &models.Identity{},
        &models.SSHKey{},
        &models.IdentityShare{},
        &models.VaultKey{},

        // Connection models
        &models.Connection{},
        &models.SharedSession{},
        &models.Notification{},
    )
}
```

### 4.2 Encryption Pattern (Vault)

```go
// internal/vault/encryption.go
package vault

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "errors"
    "io"

    "golang.org/x/crypto/argon2"
)

type Encryptor struct {
    masterKey []byte
}

func NewEncryptor(masterKey string) (*Encryptor, error) {
    // Derive key using Argon2id
    salt := []byte("shellcn-vault-salt") // Use unique salt per deployment
    key := argon2.IDKey([]byte(masterKey), salt, 1, 64*1024, 4, 32)

    return &Encryptor{masterKey: key}, nil
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(e.masterKey)
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

    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
    data, err := base64.StdEncoding.DecodeString(ciphertext)
    if err != nil {
        return "", err
    }

    block, err := aes.NewCipher(e.masterKey)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", errors.New("ciphertext too short")
    }

    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }

    return string(plaintext), nil
}
```

---

## 5. API Conventions

### 5.1 REST API Routes (Standard Pattern)

```
# Authentication
POST   /api/auth/login
POST   /api/auth/logout
POST   /api/auth/refresh
GET    /api/auth/me

# Setup
GET    /api/setup/required
POST   /api/setup/first-user

# Users
GET    /api/users
POST   /api/users
GET    /api/users/:id
PUT    /api/users/:id
DELETE /api/users/:id

# Vault (Identities)
GET    /api/vault/identities
POST   /api/vault/identities
GET    /api/vault/identities/:id
PUT    /api/vault/identities/:id
DELETE /api/vault/identities/:id
POST   /api/vault/identities/:id/share

# SSH Keys
GET    /api/vault/ssh-keys
POST   /api/vault/ssh-keys
DELETE /api/vault/ssh-keys/:id

# Connections (Generic)
GET    /api/connections
POST   /api/connections
GET    /api/connections/:id
PUT    /api/connections/:id
DELETE /api/connections/:id

# Module-specific
GET    /api/ssh/connections
POST   /api/ssh/connections
WS     /ws/ssh/:id

GET    /api/docker/hosts
POST   /api/docker/hosts
GET    /api/docker/hosts/:id/containers
POST   /api/docker/hosts/:id/containers/:cid/exec

# Health & Metrics
GET    /health
GET    /health/ready
GET    /health/live
GET    /metrics
```

### 5.2 Response Format (Standard)

```go
// Success response
{
  "data": { ... },
  "message": "Optional success message"
}

// Error response
{
  "error": "Error message",
  "code": "ERROR_CODE", // Optional
  "details": { ... }    // Optional
}

// Paginated response
{
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

---

## 6. Authentication & Authorization

### 6.1 JWT Token Pattern

```go
// internal/auth/jwt.go
package auth

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
    UserID   string   `json:"user_id"`
    Username string   `json:"username"`
    IsRoot   bool     `json:"is_root"`
    Permissions []string `json:"permissions"`
    jwt.RegisteredClaims
}

func GenerateToken(userID, username string, isRoot bool, permissions []string) (string, error) {
    claims := &Claims{
        UserID:   userID,
        Username: username,
        IsRoot:   isRoot,
        Permissions: permissions,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
```

### 6.2 Auth Middleware Pattern

```go
// internal/api/middleware/auth.go
package middleware

import (
    "net/http"
    "strings"
    "github.com/gin-gonic/gin"
    "shellcn/internal/auth"
)

func AuthRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Authorization header required",
            })
            c.Abort()
            return
        }

        tokenString := strings.TrimPrefix(authHeader, "Bearer ")
        claims, err := auth.ValidateToken(tokenString)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "Invalid token",
            })
            c.Abort()
            return
        }

        c.Set("user_id", claims.UserID)
        c.Set("username", claims.Username)
        c.Set("is_root", claims.IsRoot)
        c.Set("permissions", claims.Permissions)
        c.Next()
    }
}
```

---

## 7. Error Handling

### 7.1 Custom Errors

```go
// internal/utils/errors.go
package utils

import "errors"

var (
    ErrNotFound          = errors.New("resource not found")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrForbidden         = errors.New("forbidden")
    ErrInvalidInput      = errors.New("invalid input")
    ErrDuplicateResource = errors.New("resource already exists")
    ErrInternalError     = errors.New("internal server error")
)
```

### 7.2 Error Response Helper

```go
// internal/api/handlers/errors.go
package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "shellcn/internal/utils"
)

func HandleError(c *gin.Context, err error) {
    switch err {
    case utils.ErrNotFound:
        c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
    case utils.ErrUnauthorized:
        c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
    case utils.ErrForbidden:
        c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
    case utils.ErrInvalidInput:
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
    case utils.ErrDuplicateResource:
        c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
    default:
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
    }
}
```

---

## 8. Testing Standards

### 8.1 Backend Testing (Go)

```go
// internal/services/module_service_test.go
package services

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type MockModuleRepository struct {
    mock.Mock
}

func (m *MockModuleRepository) Create(resource *models.Resource) error {
    args := m.Called(resource)
    return args.Error(0)
}

func TestModuleService_Create(t *testing.T) {
    mockRepo := new(MockModuleRepository)
    service := NewModuleService(mockRepo)

    req := &models.CreateRequest{
        Name: "Test Resource",
    }

    mockRepo.On("Create", mock.Anything).Return(nil)

    resource, err := service.Create("user-123", req)

    assert.NoError(t, err)
    assert.NotNil(t, resource)
    assert.Equal(t, "Test Resource", resource.Name)
    mockRepo.AssertExpectations(t)
}
```

### 8.2 Frontend Testing (Vitest + React Testing Library)

```typescript
// components/__tests__/Button.test.tsx
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { Button } from '../ui/button';

describe('Button', () => {
  it('renders with text', () => {
    render(<Button>Click me</Button>);
    expect(screen.getByText('Click me')).toBeInTheDocument();
  });

  it('calls onClick when clicked', () => {
    const handleClick = vi.fn();
    render(<Button onClick={handleClick}>Click me</Button>);

    fireEvent.click(screen.getByText('Click me'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });
});
```

---

## 9. Security Patterns

### 9.1 Password Hashing (bcrypt)

```go
import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(hash), err
}

func VerifyPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
```

### 9.2 Input Validation

```go
// Use Gin binding
type CreateUserRequest struct {
    Username string `json:"username" binding:"required,min=3,max=50"`
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

func (h *UserHandler) Create(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // ...
}
```

---

## 10. WebSocket Patterns

### 10.1 Backend WebSocket Handler

```go
// internal/modules/ssh/handler.go
package ssh

import (
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "net/http"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // Configure properly in production
    },
}

func (h *SSHHandler) WebSocketHandler(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    connectionID := c.Param("id")
    userID := c.GetString("user_id")

    session, err := h.service.Connect(userID, connectionID)
    if err != nil {
        conn.WriteJSON(map[string]string{"error": err.Error()})
        return
    }
    defer session.Close()

    // Read from WebSocket, write to SSH
    go func() {
        for {
            _, message, err := conn.ReadMessage()
            if err != nil {
                break
            }
            session.Write(message)
        }
    }()

    // Read from SSH, write to WebSocket
    for {
        data, err := session.Read()
        if err != nil {
            break
        }
        conn.WriteMessage(websocket.BinaryMessage, data)
    }
}
```

### 10.2 Frontend WebSocket Hook

```typescript
// hooks/useSSHWebSocket.ts
import { useEffect, useRef } from 'react';

export function useSSHWebSocket(connectionId: string, onData: (data: ArrayBuffer) => void) {
  const ws = useRef<WebSocket | null>(null);

  useEffect(() => {
    const token = localStorage.getItem('access_token');
    ws.current = new WebSocket(
      `ws://localhost:8000/ws/ssh/${connectionId}?token=${token}`
    );

    ws.current.binaryType = 'arraybuffer';

    ws.current.onmessage = (event) => {
      onData(event.data);
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      ws.current?.close();
    };
  }, [connectionId, onData]);

  const send = (data: string | ArrayBuffer) => {
    ws.current?.send(data);
  };

  return { send };
}
```

---

## 11. Logging & Monitoring

### 11.1 Structured Logging (Zap)

```go
// internal/utils/logger.go
package utils

import "go.uber.org/zap"

var Logger *zap.Logger

func InitLogger() {
    var err error
    Logger, err = zap.NewProduction()
    if err != nil {
        panic(err)
    }
}

// Usage
utils.Logger.Info("User logged in",
    zap.String("user_id", userID),
    zap.String("ip", ip),
)
```

### 11.2 Prometheus Metrics

```go
// internal/monitoring/metrics.go
package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    HTTPRequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    ActiveConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "active_connections",
            Help: "Active connections by protocol",
        },
        []string{"protocol"},
    )
)
```

---

## 12. Code Style & Formatting

### 12.1 Go

- Use `gofmt` for formatting
- Use `golangci-lint` for linting
- Follow standard Go naming conventions
- Export only what's necessary

### 12.2 TypeScript

```json
// .prettierrc
{
  "semi": true,
  "singleQuote": true,
  "tabWidth": 2,
  "trailingComma": "es5"
}
```

### 12.3 File Naming

- Go: `snake_case.go` (e.g., `user_service.go`)
- TypeScript: `PascalCase.tsx` for components, `camelCase.ts` for utilities

---

## 13. Documentation Standards

### 13.1 Go Comments

```go
// UserService provides user management functionality.
// It handles user creation, authentication, and profile updates.
type UserService struct {
    repo *repositories.UserRepository
}

// Create creates a new user with the given request data.
// It validates the input, hashes the password, and persists the user.
// Returns the created user or an error if validation fails.
func (s *UserService) Create(req *CreateUserRequest) (*models.User, error) {
    // Implementation
}
```

### 13.2 TypeScript JSDoc

```typescript
/**
 * Hook for managing SSH connections.
 *
 * @returns Query result with connections data
 *
 * @example
 * ```tsx
 * function ConnectionList() {
 *   const { data, isLoading } = useConnections();
 *   // ...
 * }
 * ```
 */
export function useConnections() {
  // Implementation
}
```

---

## 14. Build & Deployment

### 14.1 Build Process

```bash
# 1. Build Rust FFI modules
cd rust-modules/rdp && cargo build --release
cd ../vnc && cargo build --release

# 2. Build frontend
cd web && pnpm install && pnpm run build

# 3. Build Go binary (with embedded frontend)
CGO_ENABLED=1 go build -o shellcn ./cmd/server
```

### 14.2 Makefile (Standard)

```makefile
.PHONY: build test clean run

build:
	@echo "Building Rust FFI modules..."
	cd rust-modules/rdp && cargo build --release
	cd rust-modules/vnc && cargo build --release

	@echo "Building frontend..."
	cd web && pnpm install && pnpm run build

	@echo "Building Go binary..."
	CGO_ENABLED=1 go build -o shellcn ./cmd/server

test:
	go test ./...
	cd web && pnpm test

run:
	./shellcn

clean:
	rm -f shellcn
	rm -rf web/dist
	cd rust-modules/rdp && cargo clean
	cd rust-modules/vnc && cargo clean
```

---

## Summary Checklist

When implementing a new module, ensure:

- ✅ Follow project structure exactly
- ✅ Register permissions in `init()`
- ✅ Use standard handler/service/repository pattern
- ✅ Follow API route conventions
- ✅ Implement proper error handling
- ✅ Add comprehensive tests
- ✅ Use permission-based UI rendering
- ✅ Apply user preferences (no hardcoded settings)
- ✅ Use structured logging
- ✅ Add Prometheus metrics
- ✅ Document all exported functions
- ✅ Check for latest library versions
- ✅ Follow security patterns (encryption, validation)

---

**End of Shared Patterns & Guidelines**
