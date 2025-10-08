# ShellCN Platform - Backend Shared Patterns

This document defines **shared, reusable backend patterns** that ALL modules must follow. Focus is on code reusability, consistency, and the `pkg/` folder for shared utilities.

---

## Table of Contents

1. [Technology Stack](#1-technology-stack)
2. [Project Structure](#2-project-structure)
3. [Shared Packages (pkg/)](#3-shared-packages-pkg)
4. [Permission System](#4-permission-system)
5. [Layered Architecture](#5-layered-architecture)
6. [Database Patterns](#6-database-patterns)
7. [API Patterns](#7-api-patterns)
8. [WebSocket Patterns](#8-websocket-patterns)
9. [Security & Encryption](#9-security--encryption)
10. [Error Handling](#10-error-handling)
11. [Logging & Monitoring](#11-logging--monitoring)
12. [Testing Standards](#12-testing-standards)
13. [Configuration Management](#13-configuration-management)
14. [Build & Deployment](#14-build--deployment)
15. [Module Implementation Checklist](#15-module-implementation-checklist)

---

## 1. Technology Stack

### Core Backend (Go 1.21+)

**IMPORTANT: Always check for latest versions before implementation!**

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
    gorm.io/driver/postgres v1.5.4  // Optional
    gorm.io/driver/mysql v1.5.2     // Optional

    // Authentication
    github.com/golang-jwt/jwt/v5 v5.2.0
    golang.org/x/crypto v0.17.0

    // WebSocket
    github.com/gorilla/websocket v1.5.1

    // Monitoring
    github.com/prometheus/client_golang v1.18.0

    // Logging
    go.uber.org/zap v1.26.0

    // Configuration
    github.com/spf13/viper v1.18.2

    // Utilities
    github.com/google/uuid v1.5.0
    golang.org/x/sync v0.5.0

    // Testing
    github.com/stretchr/testify v1.8.4
    github.com/golang/mock v1.6.0
)
```

---

## 2. Project Structure

```
shellcn/
├── cmd/
│   └── server/
│       └── main.go              # Application entry point
│
├── internal/                    # Private application code
│   ├── app/                     # Application setup
│   ├── api/                     # API layer (handlers, middleware, routes)
│   ├── auth/                    # Authentication system
│   ├── permissions/             # Permission registry
│   ├── vault/                   # Credential vault
│   ├── monitoring/              # Monitoring & health checks
│   ├── modules/                 # Protocol modules (ssh, docker, k8s, etc.)
│   ├── models/                  # Data models
│   ├── database/                # Database layer
│   └── services/                # Business logic
│
├── pkg/                         # PUBLIC shared packages (reusable)
│   ├── logger/                  # Logging utilities
│   ├── errors/                  # Error definitions & helpers
│   ├── validator/               # Input validation
│   ├── crypto/                  # Encryption utilities
│   ├── websocket/               # WebSocket utilities
│   ├── session/                 # Session management
│   ├── response/                # API response helpers
│   ├── config/                  # Configuration helpers
│   └── testing/                 # Test utilities
│
├── rust-modules/                # Rust FFI modules
├── web/                         # Frontend
├── config/                      # Configuration files
├── scripts/                     # Build & deployment scripts
├── Makefile                     # Build automation
└── go.mod
```

---

## 3. Shared Packages (pkg/)

**The `pkg/` folder contains reusable utilities that ALL modules can import.**

### 3.1 Logger Package

**Location:** `pkg/logger/logger.go`

```go
package logger

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

var Log *zap.Logger

// Initialize logger
func Init(level string) error {
    config := zap.NewProductionConfig()

    // Parse log level
    switch level {
    case "debug":
        config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
    case "info":
        config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
    case "warn":
        config.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
    case "error":
        config.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
    }

    var err error
    Log, err = config.Build()
    return err
}

// Shorthand functions
func Info(msg string, fields ...zap.Field) {
    Log.Info(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
    Log.Error(msg, fields...)
}

func Debug(msg string, fields ...zap.Field) {
    Log.Debug(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
    Log.Fatal(msg, fields...)
}

// Module-specific logger
func WithModule(module string) *zap.Logger {
    return Log.With(zap.String("module", module))
}
```

**Usage:**

```go
import "shellcn/pkg/logger"

logger.Init("info")
logger.Info("SSH connection established",
    zap.String("user_id", userID),
    zap.String("host", host),
)

// Module-specific logger
sshLogger := logger.WithModule("ssh")
sshLogger.Error("Connection failed", zap.Error(err))
```

### 3.2 Error Package

**Location:** `pkg/errors/errors.go`

```go
package errors

import (
    "errors"
    "fmt"
)

// Standard errors
var (
    ErrNotFound          = errors.New("resource not found")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrForbidden         = errors.New("forbidden")
    ErrInvalidInput      = errors.New("invalid input")
    ErrDuplicateResource = errors.New("resource already exists")
    ErrInternalError     = errors.New("internal server error")
    ErrConnectionFailed  = errors.New("connection failed")
    ErrSessionExpired    = errors.New("session expired")
)

// Wrap error with context
func Wrap(err error, message string) error {
    if err == nil {
        return nil
    }
    return fmt.Errorf("%s: %w", message, err)
}

// Create custom error
func New(message string) error {
    return errors.New(message)
}

// Check if error is specific type
func Is(err, target error) bool {
    return errors.Is(err, target)
}
```

**Usage:**

```go
import "shellcn/pkg/errors"

if user == nil {
    return nil, errors.ErrNotFound
}

if err != nil {
    return errors.Wrap(err, "failed to connect to SSH server")
}

if errors.Is(err, errors.ErrUnauthorized) {
    // Handle unauthorized
}
```

### 3.3 Response Package

**Location:** `pkg/response/response.go`

```go
package response

import (
    "net/http"
    "github.com/gin-gonic/gin"
    "shellcn/pkg/errors"
)

// Success response
func OK(c *gin.Context, data interface{}) {
    c.JSON(http.StatusOK, gin.H{
        "data": data,
    })
}

// Created response
func Created(c *gin.Context, data interface{}) {
    c.JSON(http.StatusCreated, gin.H{
        "data": data,
    })
}

// Error response with automatic status code
func Error(c *gin.Context, err error) {
    statusCode := getStatusCode(err)
    c.JSON(statusCode, gin.H{
        "error": err.Error(),
    })
}

// Error with custom message
func ErrorMessage(c *gin.Context, statusCode int, message string) {
    c.JSON(statusCode, gin.H{
        "error": message,
    })
}

// Paginated response
func Paginated(c *gin.Context, data interface{}, page, pageSize, total int) {
    c.JSON(http.StatusOK, gin.H{
        "data": data,
        "pagination": gin.H{
            "page":        page,
            "page_size":   pageSize,
            "total":       total,
            "total_pages": (total + pageSize - 1) / pageSize,
        },
    })
}

func getStatusCode(err error) int {
    switch {
    case errors.Is(err, errors.ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, errors.ErrUnauthorized):
        return http.StatusUnauthorized
    case errors.Is(err, errors.ErrForbidden):
        return http.StatusForbidden
    case errors.Is(err, errors.ErrInvalidInput):
        return http.StatusBadRequest
    case errors.Is(err, errors.ErrDuplicateResource):
        return http.StatusConflict
    default:
        return http.StatusInternalServerError
    }
}
```

**Usage:**

```go
import "shellcn/pkg/response"

func (h *Handler) List(c *gin.Context) {
    items, err := h.service.List()
    if err != nil {
        response.Error(c, err)
        return
    }
    response.OK(c, items)
}

func (h *Handler) ListPaginated(c *gin.Context) {
    items, total, err := h.service.ListPaginated(page, pageSize)
    if err != nil {
        response.Error(c, err)
        return
    }
    response.Paginated(c, items, page, pageSize, total)
}
```

### 3.4 Validator Package

**Location:** `pkg/validator/validator.go`

```go
package validator

import (
    "fmt"
    "regexp"
    "shellcn/pkg/errors"
)

// Validate required string
func Required(value, fieldName string) error {
    if value == "" {
        return fmt.Errorf("%s is required", fieldName)
    }
    return nil
}

// Validate port number
func Port(port int) error {
    if port < 1 || port > 65535 {
        return errors.New("invalid port number")
    }
    return nil
}

// Validate hostname
func Hostname(host string) error {
    if host == "" {
        return errors.New("hostname is required")
    }
    // Basic hostname validation
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, host)
    if !matched {
        return errors.New("invalid hostname")
    }
    return nil
}

// Validate IP address
func IPAddress(ip string) error {
    matched, _ := regexp.MatchString(`^(\d{1,3}\.){3}\d{1,3}$`, ip)
    if !matched {
        return errors.New("invalid IP address")
    }
    return nil
}

// Validate string length
func Length(value string, min, max int, fieldName string) error {
    length := len(value)
    if length < min || length > max {
        return fmt.Errorf("%s must be between %d and %d characters", fieldName, min, max)
    }
    return nil
}
```

**Usage:**

```go
import "shellcn/pkg/validator"

func (s *Service) Validate(req *CreateRequest) error {
    if err := validator.Required(req.Name, "name"); err != nil {
        return err
    }
    if err := validator.Hostname(req.Host); err != nil {
        return err
    }
    if err := validator.Port(req.Port); err != nil {
        return err
    }
    return nil
}
```

### 3.5 Crypto Package

**Location:** `pkg/crypto/crypto.go`

```go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "io"
    "golang.org/x/crypto/argon2"
    "golang.org/x/crypto/bcrypt"
)

// Password hashing
func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    return string(hash), err
}

func VerifyPassword(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

// AES-256-GCM encryption
type Encryptor struct {
    key []byte
}

func NewEncryptor(masterKey string, salt []byte) *Encryptor {
    key := argon2.IDKey([]byte(masterKey), salt, 1, 64*1024, 4, 32)
    return &Encryptor{key: key}
}

func (e *Encryptor) Encrypt(plaintext string) (string, error) {
    block, err := aes.NewCipher(e.key)
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

    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    return string(plaintext), err
}
```

**Usage:**

```go
import "shellcn/pkg/crypto"

// Hash password
hash, _ := crypto.HashPassword("mypassword")

// Verify password
valid := crypto.VerifyPassword("mypassword", hash)

// Encrypt data
encryptor := crypto.NewEncryptor(masterKey, salt)
encrypted, _ := encryptor.Encrypt("secret data")
decrypted, _ := encryptor.Decrypt(encrypted)
```

### 3.6 WebSocket Package

**Location:** `pkg/websocket/websocket.go`

```go
package websocket

import (
    "github.com/gorilla/websocket"
    "net/http"
    "time"
)

var DefaultUpgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        return true // Configure in production
    },
}

// WebSocket connection wrapper
type Conn struct {
    *websocket.Conn
}

// Write with timeout
func (c *Conn) WriteWithTimeout(messageType int, data []byte, timeout time.Duration) error {
    c.SetWriteDeadline(time.Now().Add(timeout))
    return c.WriteMessage(messageType, data)
}

// Read with timeout
func (c *Conn) ReadWithTimeout(timeout time.Duration) (int, []byte, error) {
    c.SetReadDeadline(time.Now().Add(timeout))
    return c.ReadMessage()
}

// Ping-pong keepalive
func (c *Conn) StartKeepalive(interval time.Duration, done chan struct{}) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    c.SetPongHandler(func(string) error {
        c.SetReadDeadline(time.Now().Add(interval * 2))
        return nil
    })

    for {
        select {
        case <-ticker.C:
            if err := c.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
                return
            }
        case <-done:
            return
        }
    }
}
```

**Usage:**

```go
import "shellcn/pkg/websocket"

ws, err := websocket.DefaultUpgrader.Upgrade(w, r, nil)
if err != nil {
    return
}
conn := &websocket.Conn{Conn: ws}

// Start keepalive
done := make(chan struct{})
go conn.StartKeepalive(30*time.Second, done)

// Write with timeout
conn.WriteWithTimeout(websocket.BinaryMessage, data, 5*time.Second)
```

### 3.7 Session Package

**Location:** `pkg/session/session.go`

```go
package session

import (
    "sync"
    "time"
)

// Session interface - all protocol sessions implement this
type Session interface {
    ID() string
    Read() ([]byte, error)
    Write([]byte) error
    Close() error
    IsAlive() bool
}

// Session manager
type Manager struct {
    sessions map[string]Session
    mu       sync.RWMutex
}

func NewManager() *Manager {
    return &Manager{
        sessions: make(map[string]Session),
    }
}

func (m *Manager) Add(id string, session Session) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.sessions[id] = session
}

func (m *Manager) Get(id string) (Session, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    session, ok := m.sessions[id]
    return session, ok
}

func (m *Manager) Remove(id string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    if session, ok := m.sessions[id]; ok {
        session.Close()
        delete(m.sessions, id)
    }
}

func (m *Manager) CleanupStale(timeout time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    for id, session := range m.sessions {
        if !session.IsAlive() {
            session.Close()
            delete(m.sessions, id)
        }
    }
}
```

**Usage:**

```go
import "shellcn/pkg/session"

// In module code
type SSHSession struct {
    id     string
    client *ssh.Client
    // ...
}

func (s *SSHSession) ID() string { return s.id }
func (s *SSHSession) Read() ([]byte, error) { /* ... */ }
func (s *SSHSession) Write(data []byte) error { /* ... */ }
func (s *SSHSession) Close() error { /* ... */ }
func (s *SSHSession) IsAlive() bool { /* ... */ }

// Use session manager
manager := session.NewManager()
manager.Add(sessionID, sshSession)
```

### 3.8 Testing Package

**Location:** `pkg/testing/testing.go`

```go
package testing

import (
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
    "testing"
)

// Setup test database
func SetupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to create test database: %v", err)
    }
    return db
}

// Cleanup test database
func CleanupTestDB(db *gorm.DB) {
    sqlDB, _ := db.DB()
    sqlDB.Close()
}

// Assert helper
func AssertNoError(t *testing.T, err error) {
    if err != nil {
        t.Fatalf("Expected no error, got: %v", err)
    }
}

func AssertError(t *testing.T, err error) {
    if err == nil {
        t.Fatal("Expected error, got nil")
    }
}
```

---

## 4. Permission System

### Global Permission Registry

**Location:** `internal/permissions/registry.go`

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

type Registry struct {
    permissions map[string]*Permission
    mu          sync.RWMutex
}

var global = &Registry{
    permissions: make(map[string]*Permission),
}

// Register permission (called in init())
func Register(perm *Permission) error {
    global.mu.Lock()
    defer global.mu.Unlock()

    if _, exists := global.permissions[perm.ID]; exists {
        return fmt.Errorf("permission %s already registered", perm.ID)
    }

    global.permissions[perm.ID] = perm
    return nil
}

// Get all permissions
func GetAll() map[string]*Permission {
    global.mu.RLock()
    defer global.mu.RUnlock()

    result := make(map[string]*Permission)
    for k, v := range global.permissions {
        result[k] = v
    }
    return result
}

// Validate dependencies on startup
func ValidateDependencies() error {
    global.mu.RLock()
    defer global.mu.RUnlock()

    for _, perm := range global.permissions {
        for _, dep := range perm.DependsOn {
            if _, exists := global.permissions[dep]; !exists {
                return fmt.Errorf("permission %s depends on non-existent permission %s", perm.ID, dep)
            }
        }
    }
    return nil
}
```

### Module Permission Registration

**Every module must register permissions in `init()`:**

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

---

## 5. Layered Architecture

**ALL modules MUST follow this layered architecture:**

```
Handler Layer (API)
       ↓
Service Layer (Business Logic)
       ↓
Repository Layer (Data Access)
       ↓
Database (GORM)
```

### Example: Standard CRUD Pattern

```go
// Handler
func (h *Handler) Create(c *gin.Context) {
    var req Request
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, errors.ErrInvalidInput)
        return
    }

    userID := c.GetString("user_id")
    resource, err := h.service.Create(userID, &req)
    if err != nil {
        response.Error(c, err)
        return
    }

    response.Created(c, resource)
}

// Service
func (s *Service) Create(userID string, req *Request) (*Model, error) {
    // Validation
    if err := validator.Required(req.Name, "name"); err != nil {
        return nil, err
    }

    // Business logic
    resource := &Model{
        UserID: userID,
        Name:   req.Name,
    }

    // Persist
    if err := s.repo.Create(resource); err != nil {
        return nil, err
    }

    return resource, nil
}

// Repository
func (r *Repository) Create(resource *Model) error {
    return r.db.Create(resource).Error
}
```

---

## 6. Database Patterns

### GORM Model Pattern

```go
type Model struct {
    ID        string    `gorm:"primaryKey" json:"id"`
    UserID    string    `gorm:"index" json:"user_id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

func (m *Model) BeforeCreate(tx *gorm.DB) error {
    if m.ID == "" {
        m.ID = uuid.New().String()
    }
    return nil
}

func (Model) TableName() string {
    return "models"
}
```

### Migration Pattern

```go
// internal/database/db.go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{},
        &models.Connection{},
        // ... all models
    )
}
```

---

## 7. API Patterns

### Middleware Pattern

```go
// internal/api/middleware/auth.go
func AuthRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := extractToken(c)
        claims, err := validateToken(token)
        if err != nil {
            response.Error(c, errors.ErrUnauthorized)
            c.Abort()
            return
        }

        c.Set("user_id", claims.UserID)
        c.Set("is_root", claims.IsRoot)
        c.Next()
    }
}

func RequirePermission(perm string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("user_id")
        isRoot := c.GetBool("is_root")

        if isRoot {
            c.Next()
            return
        }

        if !hasPermission(userID, perm) {
            response.Error(c, errors.ErrForbidden)
            c.Abort()
            return
        }

        c.Next()
    }
}
```

### Route Registration Pattern

```go
// internal/api/router.go
func SetupRoutes(r *gin.Engine) {
    api := r.Group("/api")
    api.Use(middleware.AuthRequired())

    {
        api.GET("/resource",
            middleware.RequirePermission("resource.view"),
            handler.List,
        )
        api.POST("/resource",
            middleware.RequirePermission("resource.create"),
            handler.Create,
        )
    }
}
```

---

## 8. WebSocket Patterns

### Standard WebSocket Handler

```go
import "shellcn/pkg/websocket"

func (h *Handler) WebSocketHandler(c *gin.Context) {
    ws, err := websocket.DefaultUpgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    conn := &websocket.Conn{Conn: ws}
    defer conn.Close()

    // Bidirectional communication
    done := make(chan struct{})

    go func() {
        for {
            _, msg, err := conn.ReadMessage()
            if err != nil {
                break
            }
            // Handle message
        }
        close(done)
    }()

    <-done
}
```

---

## 9. Security & Encryption

### Use Shared Crypto Package

```go
import "shellcn/pkg/crypto"

// Password hashing
hash, _ := crypto.HashPassword(password)
valid := crypto.VerifyPassword(password, hash)

// Data encryption
encryptor := crypto.NewEncryptor(masterKey, salt)
encrypted, _ := encryptor.Encrypt("secret")
decrypted, _ := encryptor.Decrypt(encrypted)
```

---

## 10. Error Handling

### Use Shared Error Package

```go
import (
    "shellcn/pkg/errors"
    "shellcn/pkg/response"
)

func (h *Handler) Get(c *gin.Context) {
    resource, err := h.service.Get(id)
    if err != nil {
        response.Error(c, err)  // Auto-detects status code
        return
    }
    response.OK(c, resource)
}

func (s *Service) Validate(req *Request) error {
    if req.Name == "" {
        return errors.ErrInvalidInput
    }
    return nil
}
```

---

## 11. Logging & Monitoring

### Use Shared Logger Package

```go
import (
    "shellcn/pkg/logger"
    "go.uber.org/zap"
)

logger.Info("Connection established",
    zap.String("user_id", userID),
    zap.String("protocol", "ssh"),
)

logger.Error("Connection failed",
    zap.Error(err),
    zap.String("host", host),
)

// Module-specific logger
sshLogger := logger.WithModule("ssh")
sshLogger.Debug("Sending data", zap.Int("bytes", len(data)))
```

### Prometheus Metrics Pattern

```go
// internal/monitoring/metrics.go
var (
    RequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "http_requests_total",
            Help: "Total HTTP requests",
        },
        []string{"method", "path", "status"},
    )

    ConnectionsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "connections_active",
            Help: "Active connections",
        },
        []string{"protocol"},
    )
)

// Usage in modules
monitoring.ConnectionsActive.WithLabelValues("ssh").Inc()
monitoring.ConnectionsActive.WithLabelValues("ssh").Dec()
```

---

## 12. Testing Standards

### Use Shared Testing Package

```go
import (
    "testing"
    "shellcn/pkg/testing"
    "github.com/stretchr/testify/assert"
)

func TestService_Create(t *testing.T) {
    db := testing.SetupTestDB(t)
    defer testing.CleanupTestDB(db)

    service := NewService(db)
    result, err := service.Create(req)

    testing.AssertNoError(t, err)
    assert.NotNil(t, result)
}
```

---

## 13. Configuration Management

### Viper Pattern

```go
// internal/app/config.go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Modules  map[string]ModuleConfig
}

func LoadConfig() (*Config, error) {
    viper.SetConfigName("config")
    viper.SetConfigType("yaml")
    viper.AddConfigPath("./config")

    viper.SetEnvPrefix("SHELLCN")
    viper.AutomaticEnv()

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

## 14. Build & Deployment

### Use Makefile

```bash
# Build everything
make build

# Run tests
make test

# Development mode
make dev

# Clean
make clean
```

See `Makefile` in project root for all commands.

---

## 15. Module Implementation Checklist

Before implementing a new module, ensure:

### ✅ Use Shared Packages

- [ ] Use `pkg/logger` for logging
- [ ] Use `pkg/errors` for error handling
- [ ] Use `pkg/response` for API responses
- [ ] Use `pkg/validator` for input validation
- [ ] Use `pkg/crypto` for encryption
- [ ] Use `pkg/websocket` for WebSocket connections (if needed)
- [ ] Use `pkg/session` for session management (if needed)

### ✅ Follow Patterns

- [ ] Register permissions in `init()` function
- [ ] Follow layered architecture (Handler → Service → Repository)
- [ ] Use GORM models with UUID generation
- [ ] Apply middleware (AuthRequired, RequirePermission)
- [ ] Use structured logging with module name
- [ ] Handle errors using shared error package
- [ ] Write unit tests with shared testing utilities

### ✅ Code Quality

- [ ] Document all exported functions (GoDoc)
- [ ] Write tests (coverage > 70%)
- [ ] Run `make lint` before commit
- [ ] Run `make test` before commit
- [ ] Check for latest library versions

---

**All modules MUST reuse the shared `pkg/` utilities to ensure consistency and reduce code duplication.**

---

**End of Backend Shared Patterns**
