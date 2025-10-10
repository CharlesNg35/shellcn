# Active Connections – Implementation Plan

## Objective

Expose live connection activity ("**active connections**") across the platform and surface it in the UI (`useActiveConnections` hook) so that users can see which connections are currently in use. This relies on the **Connect Hub** (our realtime WebSocket layer) to broadcast session lifecycle updates.

**Key Concept:** A **connection** is a stored entity that references a protocol driver (SSH, VNC, Docker, Kubernetes, Database, etc.). An **active connection** means a user has launched that connection and a live session exists (terminal open, desktop session running, database query executing, etc.).

---

## 🔑 Critical Session Constraints

### Session Uniqueness: Per User AND Per Connection

**IMPORTANT:** Active sessions are tracked **per user AND per connection**, NOT per protocol:

1. **One Session Per User Per Connection**
   - ✅ User A can have ONE active session on "prod-server-01" (SSH)
   - ✅ User B can ALSO have ONE active session on "prod-server-01" (SSH) **simultaneously**
   - ❌ User A CANNOT have TWO active sessions on "prod-server-01" at the same time
   - ✅ User A CAN have sessions on BOTH "prod-server-01" (SSH) AND "staging-db" (Database)

2. **Composite Unique Key: `(user_id, connection_id)`**
   - Sessions are uniquely identified by the combination of user and connection
   - NOT by protocol type (multiple connections can use the same protocol)
   - The same connection can have multiple active sessions from different users

3. **Session Visibility Rules**
   - **Regular Users:** See ONLY their own active sessions
   - **Admins:** See ALL active sessions across all users, WITH username/user info displayed
   - **Team Members:** See sessions from other team members on shared team connections (optional based on permissions)

### Example Scenarios

**Scenario 1: Multiple Users, Same Connection**
```
Connection: "prod-server-01" (SSH)
- Session 1: User Alice → ACTIVE ✅
- Session 2: User Bob → ACTIVE ✅
- Session 3: User Alice (second attempt) → REJECTED ❌ (already has active session)
```

**Scenario 2: Single User, Multiple Connections**
```
User: Alice
- "prod-server-01" (SSH) → ACTIVE ✅
- "staging-db" (PostgreSQL) → ACTIVE ✅
- "k8s-cluster" (Kubernetes) → ACTIVE ✅
- "prod-server-01" (second attempt) → REJECTED ❌ (already has active session on this connection)
```

**Scenario 3: Admin View vs Regular User View**
```
Admin sees:
- prod-server-01 (3 active sessions)
  - Alice (started 10m ago)
  - Bob (started 5m ago)
  - Charlie (started 2m ago)

Alice sees (regular user):
- prod-server-01 (1 active session)
  - My session (started 10m ago)
```

---

## Critical Distinction: Protocols vs Connections vs Active Sessions

### ❌ **WRONG Understanding** (What the Original Plan Said)

"Show protocols in the sidebar" - This would mean showing "SSH", "Docker", "Kubernetes" as active items.

### ✅ **CORRECT Understanding** (What We Actually Need)

The platform has **three distinct concepts**:

1. **Protocols** (`/api/protocols`)
   - **What:** Driver catalog (SSH, Docker, Kubernetes, RDP, etc.)
   - **Purpose:** List available protocol drivers the user can use
   - **Used for:** Connection creation dropdown, capabilities metadata
   - **Example:** "SSH protocol supports terminal, file transfer, clipboard"

2. **Connections** (`/api/connections`)
   - **What:** Stored connection entities (configuration + settings)
   - **Purpose:** Reusable connection profiles with credentials
   - **Used for:** Connections list page, quick launch
   - **Example:** "Production Server (SSH) - 192.168.1.100:22"

3. **Active Sessions** (`/api/connections/active` - NEW!)
   - **What:** Live runtime sessions currently executing
   - **Purpose:** Show what's running RIGHT NOW
   - **Used for:** Sidebar "Active Sessions", live status badges
   - **Example:** "Production Server (SSH) - Started 5 mins ago by Alice"

### Sidebar Sections (After Implementation)

```
┌─────────────────────────┐
│ SHELLCN                │
├─────────────────────────┤
│ Dashboard              │
│ Connections            │
├─────────────────────────┤
│ PROTOCOLS              │  ← EXISTING (shows connection counts)
│ ► All Connections (23) │
│ ► SSH (8)              │
│ ► Docker (7)           │
│ ► Kubernetes (5)       │
│ ► Database (3)         │
├─────────────────────────┤
│ ACTIVE SESSIONS        │  ← NEW (shows live sessions)
│ ● prod-server-01       │     (2 sessions)
│ ● staging-db           │     (1 session)
│ ● k8s-cluster-main     │     (3 sessions)
└─────────────────────────┘
```

**Key Points:**

- **Protocols section** = Static list showing "how many SSH/Docker/K8s connections exist"
- **Active Sessions section** = Dynamic list showing "which specific connections are running NOW"
- **NOT the same thing!** Protocols don't "go active" - connections do!

---

## Architecture Overview

### Current System Components

The platform already has the following infrastructure in place:

1. **Connection Entities** (`internal/models/connection.go`)
   - Stored connection configurations for various protocols
   - Fields: `ID`, `Name`, `ProtocolID`, `TeamID`, `OwnerUserID`, `Settings`, `LastUsedAt`
   - Managed via `ConnectionService` (`internal/services/connection_service.go`)

2. **Protocol Drivers** (`internal/drivers/driver.go`)
   - Driver interface with `Launcher` capability
   - `SessionRequest` and `SessionHandle` for runtime sessions
   - Drivers: SSH, Telnet, RDP, VNC, Docker, Kubernetes, Database clients

3. **Realtime WebSocket Hub** (`internal/realtime/hub.go`)
   - Multiplexed WebSocket streams for real-time updates
   - Stream-based pub/sub: `BroadcastToUser`, `BroadcastStream`
   - Existing streams: `notifications` (defined in `internal/realtime/streams.go`)
   - WebSocket endpoints: `GET /ws` and `GET /ws/:stream`

4. **Frontend Hooks**
   - `useConnections` - fetches stored connection entities
   - `useConnectionSummary` - protocol-grouped connection counts
   - `useAvailableProtocols` - available protocol drivers

---

## Backend Implementation

### 1. Session Lifecycle Tracking

**Option A: In-Memory Registry (Recommended for MVP)**

Instead of a database table, maintain an in-memory session registry:

```go
// internal/services/active_session_service.go
package services

import (
    "sync"
    "time"

    "github.com/charlesng35/shellcn/internal/realtime"
)

type ActiveSessionRecord struct {
    ID           string    `json:"id"`            // Session UUID
    ConnectionID string    `json:"connection_id"` // FK to connections table
    UserID       string    `json:"user_id"`       // User who owns this session
    UserName     string    `json:"user_name"`     // Username for admin display
    TeamID       *string   `json:"team_id"`
    ProtocolID   string    `json:"protocol_id"`   // ssh, docker, kubernetes, etc.
    StartedAt    time.Time `json:"started_at"`
    LastSeenAt   time.Time `json:"last_seen_at"`

    // Optional metadata
    Host         string `json:"host,omitempty"`
    Port         int    `json:"port,omitempty"`
}

type ActiveSessionService struct {
    mu       sync.RWMutex
    sessions map[string]*ActiveSessionRecord // Key: session ID
    userConnIndex map[string]string          // Key: "userID:connectionID" -> session ID
    hub      *realtime.Hub
}

func NewActiveSessionService(hub *realtime.Hub) *ActiveSessionService {
    return &ActiveSessionService{
        sessions: make(map[string]*ActiveSessionRecord),
        userConnIndex: make(map[string]string),
        hub:      hub,
    }
}

func (s *ActiveSessionService) RegisterSession(session *ActiveSessionRecord) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Check if user already has an active session on this connection
    indexKey := fmt.Sprintf("%s:%s", session.UserID, session.ConnectionID)
    if existingSessionID, exists := s.userConnIndex[indexKey]; exists {
        return fmt.Errorf("user %s already has an active session (%s) on connection %s",
            session.UserID, existingSessionID, session.ConnectionID)
    }

    // Register the session
    s.sessions[session.ID] = session
    s.userConnIndex[indexKey] = session.ID

    // Broadcast session.opened event
    s.hub.BroadcastStream("connection.sessions", realtime.Message{
        Event: "session.opened",
        Data:  session,
    })

    return nil
}

func (s *ActiveSessionService) UnregisterSession(sessionID string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    session, exists := s.sessions[sessionID]
    if !exists {
        return
    }

    // Remove from both indexes
    delete(s.sessions, sessionID)
    indexKey := fmt.Sprintf("%s:%s", session.UserID, session.ConnectionID)
    delete(s.userConnIndex, indexKey)

    // Broadcast session.closed event
    s.hub.BroadcastStream("connection.sessions", realtime.Message{
        Event: "session.closed",
        Data: map[string]any{
            "id":            session.ID,
            "connection_id": session.ConnectionID,
            "user_id":       session.UserID,
        },
    })
}

func (s *ActiveSessionService) Heartbeat(sessionID string) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if session, exists := s.sessions[sessionID]; exists {
        session.LastSeenAt = time.Now()
    }
}

func (s *ActiveSessionService) ListActive(userID string, teamIDs []string, isAdmin bool) []*ActiveSessionRecord {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var results []*ActiveSessionRecord
    for _, session := range s.sessions {
        // Admin sees ALL sessions across all users
        if isAdmin {
            results = append(results, session)
            continue
        }

        // Regular users see ONLY their own sessions
        if session.UserID == userID {
            results = append(results, session)
            continue
        }

        // Optional: Team members can see each other's sessions on shared team connections
        // Uncomment below if you want team visibility
        /*
        if session.TeamID != nil {
            for _, teamID := range teamIDs {
                if *session.TeamID == teamID {
                    results = append(results, session)
                    break
                }
            }
        }
        */
    }
    return results
}

// HasActiveSession checks if a user already has an active session on a connection
func (s *ActiveSessionService) HasActiveSession(userID, connectionID string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()

    indexKey := fmt.Sprintf("%s:%s", userID, connectionID)
    _, exists := s.userConnIndex[indexKey]
    return exists
}

// Cleanup expired sessions (run periodically)
func (s *ActiveSessionService) CleanupStale(gracePeriod time.Duration) {
    s.mu.Lock()
    defer s.mu.Unlock()

    now := time.Now()
    for sessionID, session := range s.sessions {
        if now.Sub(session.LastSeenAt) > gracePeriod {
            // Remove from both indexes
            delete(s.sessions, sessionID)
            indexKey := fmt.Sprintf("%s:%s", session.UserID, session.ConnectionID)
            delete(s.userConnIndex, indexKey)

            s.hub.BroadcastStream("connection.sessions", realtime.Message{
                Event: "session.closed",
                Data: map[string]any{
                    "id":            sessionID,
                    "connection_id": session.ConnectionID,
                    "user_id":       session.UserID,
                    "reason":        "timeout",
                },
            })
        }
    }
}
```

**Option B: Database Table (For Persistence)**

If persistence is required (audit, recovery after restart):

```sql
-- Migration: add connection_sessions table
CREATE TABLE connection_sessions (
    id TEXT PRIMARY KEY,
    connection_id TEXT NOT NULL REFERENCES connections(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id),
    user_name TEXT NOT NULL,
    team_id TEXT REFERENCES teams(id),
    protocol_id TEXT NOT NULL,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    host TEXT,
    port INTEGER,
    metadata TEXT,
    INDEX idx_connection_sessions_connection (connection_id),
    INDEX idx_connection_sessions_user (user_id),
    INDEX idx_connection_sessions_active (last_seen_at),
    UNIQUE INDEX idx_user_connection_unique (user_id, connection_id)  -- Enforce one session per user per connection
);
```

**Recommendation:** Start with **Option A (In-Memory)** for simplicity. Add database persistence later if needed for audit trails.

---

### Session Uniqueness Enforcement

**CRITICAL:** Both implementations MUST enforce the `(user_id, connection_id)` uniqueness constraint:

- **In-Memory:** Use `userConnIndex` map with composite key `"userID:connectionID"`
- **Database:** Use `UNIQUE INDEX idx_user_connection_unique (user_id, connection_id)`

**Before launching a connection:**
```go
// Check if user already has active session
if sessionService.HasActiveSession(userID, connectionID) {
    return errors.New("You already have an active session on this connection")
}
```

---

### 2. Driver Integration

Each protocol driver must emit session lifecycle events:

```go
// Example: SSH Driver
package ssh

import (
    "context"

    "github.com/charlesng/shellcn/internal/drivers"
    "github.com/charlesng/shellcn/internal/services"
)

type SSHDriver struct {
    sessionService *services.ActiveSessionService
}

func (d *SSHDriver) Launch(ctx context.Context, req drivers.SessionRequest) (drivers.SessionHandle, error) {
    // 1. Check if user already has an active session on this connection
    if d.sessionService.HasActiveSession(req.UserID, req.ConnectionID) {
        return nil, errors.New("you already have an active session on this connection")
    }

    // 2. Establish SSH connection
    conn, err := d.connect(req)
    if err != nil {
        return nil, err
    }

    // 3. Register active session
    sessionID := generateUUID()
    if err := d.sessionService.RegisterSession(&services.ActiveSessionRecord{
        ID:           sessionID,
        ConnectionID: req.ConnectionID,
        UserID:       req.UserID,
        UserName:     req.UserName,  // Include username for admin display
        ProtocolID:   req.ProtocolID,
        StartedAt:    time.Now(),
        LastSeenAt:   time.Now(),
        Host:         req.Settings["host"].(string),
        Port:         req.Settings["port"].(int),
    }); err != nil {
        conn.Close()
        return nil, fmt.Errorf("failed to register session: %w", err)
    }

    // 4. Return session handle
    return &SSHSession{
        id:             sessionID,
        conn:           conn,
        sessionService: d.sessionService,
    }, nil
}

type SSHSession struct {
    id             string
    conn           *ssh.Client
    sessionService *services.ActiveSessionService
}

func (s *SSHSession) Close(ctx context.Context) error {
    // Unregister session
    s.sessionService.UnregisterSession(s.id)

    // Close SSH connection
    return s.conn.Close()
}
```

**All drivers** (Docker, Kubernetes, VNC, RDP, Database) follow the same pattern:
- Call `RegisterSession` on connection start
- Optionally send `Heartbeat` for long-lived sessions
- Call `UnregisterSession` on connection close

---

### 3. REST API Endpoints

Add to `internal/api/routes_connections.go`:

```go
func registerConnectionRoutes(api *gin.RouterGroup, handler *handlers.ConnectionHandler, checker *permissions.Checker) {
    connections := api.Group("/connections")
    {
        connections.GET("", middleware.RequirePermission(checker, "connection.view"), handler.List)
        connections.GET("/summary", middleware.RequirePermission(checker, "connection.view"), handler.Summary)
        connections.GET("/active", middleware.RequirePermission(checker, "connection.view"), handler.ListActive) // NEW
        connections.GET("/:id", middleware.RequirePermission(checker, "connection.view"), handler.Get)
    }
}
```

Handler implementation (`internal/handlers/connections.go`):

```go
func (h *ConnectionHandler) ListActive(c *gin.Context) {
    userID := c.GetString("user_id") // From auth middleware

    // Get user's teams and role
    var user models.User
    if err := h.db.Preload("Teams").Preload("Roles").First(&user, "id = ?", userID).Error; err != nil {
        response.Error(c, errors.ErrUnauthorized)
        return
    }

    // Check if user is admin
    isAdmin := false
    for _, role := range user.Roles {
        if role.Name == "admin" || role.Name == "super_admin" {
            isAdmin = true
            break
        }
    }

    teamIDs := make([]string, len(user.Teams))
    for i, team := range user.Teams {
        teamIDs[i] = team.ID
    }

    // Filters
    protocolID := c.Query("protocol_id")
    teamID := c.Query("team_id")

    // Get active sessions
    // Regular users see only their own sessions
    // Admins see ALL sessions with user information
    sessions := h.activeSessionService.ListActive(userID, teamIDs, isAdmin)

    // Apply filters
    var filtered []*services.ActiveSessionRecord
    for _, session := range sessions {
        if protocolID != "" && session.ProtocolID != protocolID {
            continue
        }
        if teamID != "" {
            if teamID == "personal" && session.TeamID != nil {
                continue
            }
            if teamID != "personal" && (session.TeamID == nil || *session.TeamID != teamID) {
                continue
            }
        }
        filtered = append(filtered, session)
    }

    response.Success(c, filtered)
}
```

---

### 4. WebSocket Stream

Add to `internal/realtime/streams.go`:

```go
const (
    StreamNotifications      = "notifications"
    StreamConnectionSessions = "connection.sessions" // NEW
)
```

**Event Types:**
- `session.opened` - Connection session started
- `session.closed` - Connection session ended
- `session.heartbeat` - Periodic keepalive (optional)

**Event Payload:**
```json
{
  "stream": "connection.sessions",
  "event": "session.opened",
  "data": {
    "id": "sess_abc123",
    "connection_id": "conn_xyz789",
    "user_id": "usr_001",
    "team_id": "team_platform",
    "protocol_id": "ssh",
    "started_at": "2025-10-10T14:22:00Z",
    "last_seen_at": "2025-10-10T14:22:00Z",
    "host": "prod-server-01",
    "port": 22
  }
}
```

---

### 5. Permissions & Visibility

Active connection visibility follows existing `connection.view` permission with role-based filtering:

```go
// Session Visibility Rules:
//
// REGULAR USERS:
// - See ONLY their own active sessions
// - Cannot see sessions from other users (even on same connection)
// - Cannot see username of other users
//
// ADMINS:
// - See ALL active sessions across all users
// - See username/user_id for each session
// - Can identify which user owns each session
// - Useful for monitoring and support
//
// OPTIONAL - TEAM MEMBERS:
// - Can optionally see sessions from team members on shared team connections
// - Controlled by permission check (currently disabled in code)
```

**Session Display by Role:**

```go
// Regular User View (user_id: "usr_alice")
// Only sees own sessions
[
  {
    "id": "sess_001",
    "connection_id": "conn_prod_server",
    "user_id": "usr_alice",
    "user_name": "alice",  // Own username (safe to show)
    "started_at": "..."
  }
]

// Admin View (user_id: "usr_admin", role: "admin")
// Sees ALL sessions with user information
[
  {
    "id": "sess_001",
    "connection_id": "conn_prod_server",
    "user_id": "usr_alice",
    "user_name": "alice",  // Shows username for monitoring
    "started_at": "..."
  },
  {
    "id": "sess_002",
    "connection_id": "conn_prod_server",
    "user_id": "usr_bob",
    "user_name": "bob",    // Shows username for monitoring
    "started_at": "..."
  },
  {
    "id": "sess_003",
    "connection_id": "conn_staging_db",
    "user_id": "usr_charlie",
    "user_name": "charlie",
    "started_at": "..."
  }
]
```

---

### 6. Audit Trail

Session lifecycle events should write to audit log:

```go
// On session.opened
audit.Log(&models.AuditLog{
    UserID:       session.UserID,
    Action:       "connection.session.opened",
    ResourceType: "connection",
    ResourceID:   session.ConnectionID,
    Details: map[string]any{
        "session_id":  session.ID,
        "protocol_id": session.ProtocolID,
        "host":        session.Host,
        "port":        session.Port,
    },
})

// On session.closed
audit.Log(&models.AuditLog{
    UserID:       session.UserID,
    Action:       "connection.session.closed",
    ResourceType: "connection",
    ResourceID:   session.ConnectionID,
    Details: map[string]any{
        "session_id": session.ID,
        "duration":   time.Since(session.StartedAt).String(),
    },
})
```

---

## Frontend Implementation

### 1. Hook: `useActiveConnections`

Create `web/src/hooks/useActiveConnections.ts`:

**🔥 RECOMMENDED: Polling Approach (Simpler, Good Enough)**

```typescript
import { useQuery, type UseQueryOptions } from '@tanstack/react-query'
import { apiClient } from '@/lib/api/client'
import { unwrapResponse } from '@/lib/api/http'
import type { ApiResponse } from '@/types/api'
import { ApiError } from '@/lib/api/http'

export interface ActiveConnectionSession {
  id: string
  connection_id: string
  user_id: string
  user_name: string           // Username for admin display
  team_id?: string | null
  protocol_id: string
  started_at: string
  last_seen_at: string
  host?: string
  port?: number
}

interface UseActiveConnectionsOptions {
  protocol_id?: string
  team_id?: string
  enabled?: boolean
  refetchInterval?: number // Poll interval in ms (default: 10 seconds)
}

type QueryOptions = Omit<
  UseQueryOptions<ActiveConnectionSession[], ApiError>,
  'queryKey' | 'queryFn'
>

export function useActiveConnections(options: UseActiveConnectionsOptions = {}) {
  const {
    protocol_id,
    team_id,
    enabled = true,
    refetchInterval = 10_000, // Poll every 10 seconds
    ...queryOptions
  } = options

  return useQuery<ActiveConnectionSession[], ApiError>({
    queryKey: ['connections', 'active', { protocol_id, team_id }],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (protocol_id) params.set('protocol_id', protocol_id)
      if (team_id) params.set('team_id', team_id)

      const response = await apiClient.get<ApiResponse<ActiveConnectionSession[]>>(
        '/connections/active',
        { params }
      )
      return unwrapResponse(response)
    },
    enabled,
    staleTime: 5_000,       // Consider stale after 5 seconds
    refetchInterval,        // Auto-refresh every 10 seconds
    refetchOnWindowFocus: true, // Refresh when tab gains focus
    ...queryOptions,
  })
}
```

**Why Polling is Better for Active Connections:**

1. ✅ **Simplicity** - No WebSocket connection management
2. ✅ **React Query handles everything** - Auto-refetch, caching, error handling
3. ✅ **Good enough** - 10-second delay is acceptable for session tracking
4. ✅ **Works everywhere** - No WebSocket infrastructure issues
5. ✅ **Low overhead** - Active sessions endpoint is lightweight

**⚠️ OPTIONAL: WebSocket Approach (If Real-time is Critical)**

<details>
<summary>Click to see WebSocket implementation (only if polling isn't good enough)</summary>

```typescript
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useWebSocket } from '@/hooks/useWebSocket'

export function useActiveConnections(options: UseActiveConnectionsOptions = {}) {
  const { protocol_id, team_id, enabled = true } = options

  // 1. Fetch initial active sessions via REST
  const { data, isLoading, error, refetch } = useQuery<ActiveConnectionSession[]>({
    queryKey: ['connections', 'active', { protocol_id, team_id }],
    queryFn: async () => {
      const params = new URLSearchParams()
      if (protocol_id) params.set('protocol_id', protocol_id)
      if (team_id) params.set('team_id', team_id)

      const response = await fetch(`/api/connections/active?${params}`)
      if (!response.ok) throw new Error('Failed to fetch active connections')

      const result = await response.json()
      return result.data
    },
    enabled,
    staleTime: 30_000, // 30 seconds
  })

  // 2. Subscribe to real-time updates via WebSocket
  const { lastMessage } = useWebSocket({
    stream: 'connection.sessions',
    enabled,
  })

  // 3. Merge real-time updates with cached data
  const sessions = useMemo(() => {
    if (!data) return []

    let updated = [...data]

    if (lastMessage?.event === 'session.opened') {
      const session = lastMessage.data as ActiveConnectionSession
      if (!updated.find(s => s.id === session.id)) {
        updated.push(session)
      }
    } else if (lastMessage?.event === 'session.closed') {
      const { id } = lastMessage.data as { id: string }
      updated = updated.filter(s => s.id !== id)
    }

    return updated.filter(session => {
      if (protocol_id && session.protocol_id !== protocol_id) return false
      if (team_id === 'personal' && session.team_id != null) return false
      if (team_id && team_id !== 'personal' && session.team_id !== team_id) return false
      return true
    })
  }, [data, lastMessage, protocol_id, team_id])

  return {
    sessions,
    isLoading,
    error,
    refetch,
  }
}
```

**When to use WebSocket instead:**
- Real-time collaboration (multiple users on same connection)
- Immediate feedback required (< 1 second latency)
- High-frequency session changes

</details>

---

### 2. WebSocket Hook

Create `web/src/hooks/useWebSocket.ts`:

```typescript
import { useEffect, useState, useRef } from 'react'
import { useAuth } from '@/hooks/useAuth'

interface WebSocketMessage {
  stream: string
  event: string
  data: any
  meta?: Record<string, any>
}

interface UseWebSocketOptions {
  stream: string
  enabled?: boolean
}

export function useWebSocket({ stream, enabled = true }: UseWebSocketOptions) {
  const { user } = useAuth()
  const [lastMessage, setLastMessage] = useState<WebSocketMessage | null>(null)
  const [isConnected, setIsConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!enabled || !user) return

    const token = localStorage.getItem('access_token')
    if (!token) return

    const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws?streams=${stream}&token=${token}`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      setIsConnected(true)
      console.log(`[WebSocket] Connected to stream: ${stream}`)
    }

    ws.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data) as WebSocketMessage
        if (message.stream === stream) {
          setLastMessage(message)
        }
      } catch (error) {
        console.error('[WebSocket] Failed to parse message:', error)
      }
    }

    ws.onerror = (error) => {
      console.error('[WebSocket] Error:', error)
    }

    ws.onclose = () => {
      setIsConnected(false)
      console.log(`[WebSocket] Disconnected from stream: ${stream}`)
    }

    return () => {
      ws.close()
    }
  }, [stream, enabled, user])

  return {
    lastMessage,
    isConnected,
  }
}
```

---

### 3. Sidebar Integration

Update `web/src/components/layout/Sidebar.tsx`:

**IMPORTANT:** The sidebar currently shows "Protocols" section with connection counts (how many SSH connections exist, how many Docker connections, etc.). This is CORRECT and should STAY.

**ADD a NEW section** for Active Connections (live sessions):

```tsx
// Import the hooks
import { useActiveConnections } from '@/hooks/useActiveConnections'
import { useConnections } from '@/hooks/useConnections'

// Inside the Sidebar component

// 1. Keep existing Protocol Summary (connection counts by protocol)
const { data: protocolSummary, isLoading: summaryLoading } = useConnectionSummary(undefined, {
  enabled: hasPermission(PERMISSIONS.CONNECTION.VIEW),
})
// ... existing code for protocol summary ...

// 2. ADD new Active Connections section
const { sessions } = useActiveConnections({
  enabled: hasPermission(PERMISSIONS.CONNECTION.VIEW),
})

// Fetch connection details for active sessions
const activeConnectionIds = useMemo(() => {
  return [...new Set(sessions.map(s => s.connection_id))]
}, [sessions])

const { data: connectionsResult } = useConnections(
  { include: 'targets' },
  { enabled: activeConnectionIds.length > 0 }
)
const connections = useMemo(() => connectionsResult?.data ?? [], [connectionsResult?.data])

// Build lookup: connection_id -> connection name
const connectionLookup = useMemo(() => {
  return connections.reduce<Record<string, string>>((acc, conn) => {
    acc[conn.id] = conn.name
    return acc
  }, {})
}, [connections])

// Group active sessions by connection
const activeConnectionsGrouped = useMemo(() => {
  const grouped = new Map<string, {
    connection_id: string
    connection_name: string
    protocol_id: string
    session_count: number
    sessions: ActiveConnectionSession[]  // Store sessions for admin tooltip
  }>()

  sessions.forEach(session => {
    const existing = grouped.get(session.connection_id)
    if (existing) {
      existing.session_count++
      existing.sessions.push(session)
    } else {
      grouped.set(session.connection_id, {
        connection_id: session.connection_id,
        connection_name: connectionLookup[session.connection_id] || session.connection_id,
        protocol_id: session.protocol_id,
        session_count: 1,
        sessions: [session],
      })
    }
  })

  return Array.from(grouped.values())
}, [sessions, connectionLookup])

// Check if current user is admin
const { user } = useAuth()
const isAdmin = user?.roles?.some(role => role.name === 'admin' || role.name === 'super_admin')

// Render NEW section (BELOW the Protocols section)
{hasPermission(PERMISSIONS.CONNECTION.VIEW) && (
  <div className="space-y-2">
    <div className="flex items-center justify-between px-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
      <span>Active Sessions</span>
      <Badge variant="outline" className="text-[10px]">
        {sessions.length}
      </Badge>
    </div>
    {activeConnectionsGrouped.length === 0 ? (
      <div className="rounded-md border border-dashed border-border/60 px-3 py-4 text-center text-xs text-muted-foreground">
        No active sessions
      </div>
    ) : (
      <div className="space-y-1">
        {activeConnectionsGrouped.map((item) => (
          <Tooltip key={item.connection_id}>
            <TooltipTrigger asChild>
              <NavLink
                to={`/connections/${item.connection_id}`}
                className={({ isActive }) =>
                  cn(
                    'flex items-center justify-between rounded-md px-3 py-2 text-sm font-medium transition',
                    isActive
                      ? 'bg-primary text-primary-foreground shadow'
                      : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                  )
                }
              >
                <span className="flex items-center gap-2">
                  <span className="h-2 w-2 rounded-full bg-green-500 animate-pulse" />
                  <span className="truncate">{item.connection_name}</span>
                </span>
                {item.session_count > 1 && (
                  <Badge variant="secondary" className="text-[10px]">
                    {item.session_count}
                  </Badge>
                )}
              </NavLink>
            </TooltipTrigger>
            {/* Show session details in tooltip (usernames visible only to admins) */}
            {isAdmin && item.session_count > 1 && (
              <TooltipContent side="right" className="max-w-xs">
                <div className="space-y-1">
                  <p className="font-semibold text-xs mb-2">Active Users:</p>
                  {item.sessions.map((session) => (
                    <div key={session.id} className="flex items-center gap-2 text-xs">
                      <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
                      <span>{session.user_name}</span>
                      <span className="text-muted-foreground">
                        ({formatDistanceToNow(new Date(session.started_at))} ago)
                      </span>
                    </div>
                  ))}
                </div>
              </TooltipContent>
            )}
          </Tooltip>
        ))}
      </div>
    )}
  </div>
)}
```

**Key Features:**

1. ✅ **Keep existing "Protocols" section** - shows connection counts (e.g., "SSH: 5 connections")
2. ✅ **Add NEW "Active Sessions" section** - shows live sessions (e.g., "prod-server-01 (SSH) - Live")
3. ✅ **Display connection NAMES** not protocol names (e.g., "Production Server" not "SSH")
4. ✅ **Show session count per connection** (e.g., if 2 users are using same connection)
5. ✅ **Admin-only tooltip** - Hover over connections with multiple sessions to see which users are active (admins only)
6. ✅ **Session uniqueness enforced** - One session per user per connection (enforced at backend)

---

### 4. Connections Page Enhancement

Add "Active" filter to `web/src/pages/connections/Connections.tsx`:

```tsx
// Add state
const [showActiveOnly, setShowActiveOnly] = useState(false)

// Fetch active sessions
const { sessions: activeSessions } = useActiveConnections()

// Filter connections
const filteredConnections = useMemo(() => {
  let filtered = connections.filter(/* existing filters */)

  if (showActiveOnly) {
    const activeConnectionIds = new Set(activeSessions.map(s => s.connection_id))
    filtered = filtered.filter(conn => activeConnectionIds.has(conn.id))
  }

  return filtered
}, [connections, showActiveOnly, activeSessions])

// Add toggle button
<div className="flex items-center gap-2">
  <Button
    variant={showActiveOnly ? 'default' : 'outline'}
    size="sm"
    onClick={() => setShowActiveOnly(!showActiveOnly)}
  >
    {showActiveOnly ? 'Show All' : 'Active Only'}
  </Button>
</div>
```

---

### 5. Connection Card Badge

Update `web/src/components/connections/ConnectionCard.tsx`:

```tsx
import { useActiveConnections } from '@/hooks/useActiveConnections'

export function ConnectionCard({ connection, ... }: ConnectionCardProps) {
  const { sessions } = useActiveConnections()
  const { user } = useAuth()
  const isAdmin = user?.roles?.some(role => role.name === 'admin' || role.name === 'super_admin')

  const activeSessions = useMemo(() => {
    return sessions.filter(s => s.connection_id === connection.id)
  }, [sessions, connection.id])

  const isActive = activeSessions.length > 0
  const sessionCount = activeSessions.length

  return (
    <div className="...">
      {/* Existing card content */}

      {isActive && (
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="absolute top-2 right-2">
              <Badge variant="success" className="flex items-center gap-1 cursor-help">
                <span className="h-2 w-2 rounded-full bg-green-500 animate-pulse" />
                Live {sessionCount > 1 && `(${sessionCount})`}
              </Badge>
            </div>
          </TooltipTrigger>
          {/* Admin tooltip showing active usernames */}
          {isAdmin && sessionCount > 0 && (
            <TooltipContent>
              <div className="space-y-1">
                <p className="font-semibold text-xs">Active Users:</p>
                {activeSessions.map((session) => (
                  <div key={session.id} className="text-xs">
                    • {session.user_name} ({formatDistanceToNow(new Date(session.started_at))} ago)
                  </div>
                ))}
              </div>
            </TooltipContent>
          )}
        </Tooltip>
      )}
    </div>
  )
}
```

---

## Data Retention & Cleanup

### In-Memory Approach

```go
// Run periodic cleanup in app initialization
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        activeSessionService.CleanupStale(5 * time.Minute)
    }
}()
```

### Database Approach

```go
// Cleanup job (runs every 5 minutes)
func CleanupStaleSessions(db *gorm.DB, gracePeriod time.Duration) error {
    threshold := time.Now().Add(-gracePeriod)

    return db.Where("last_seen_at < ?", threshold).
        Delete(&models.ConnectionSession{}).Error
}
```

---

## Testing Strategy

### Backend Tests

1. **Unit Tests** (`internal/services/active_session_service_test.go`)
   ```go
   func TestRegisterSession(t *testing.T) { ... }
   func TestRegisterSession_DuplicateUserConnection_ReturnsError(t *testing.T) {
       // Test that registering the same user+connection twice fails
   }
   func TestUnregisterSession(t *testing.T) { ... }
   func TestUnregisterSession_RemovesFromBothIndexes(t *testing.T) {
       // Test that both sessions map and userConnIndex are cleaned up
   }
   func TestHasActiveSession(t *testing.T) { ... }
   func TestListActive_FiltersByUser(t *testing.T) { ... }
   func TestListActive_AdminSeesAllSessions(t *testing.T) {
       // Test that admins see all sessions with usernames
   }
   func TestListActive_RegularUserSeesOnlyOwn(t *testing.T) {
       // Test that regular users only see their own sessions
   }
   func TestCleanupStale(t *testing.T) { ... }
   func TestMultipleUsersOnSameConnection(t *testing.T) {
       // Test that Alice and Bob can both have sessions on same connection
   }
   ```

2. **Integration Tests** (`internal/handlers/connections_test.go`)
   ```go
   func TestListActive_ReturnsActiveSessionsForUser(t *testing.T) { ... }
   func TestListActive_FiltersTeamSessions(t *testing.T) { ... }
   func TestListActive_AdminSeesUsernames(t *testing.T) {
       // Test admin gets user_name field in response
   }
   func TestLaunchConnection_RejectsDuplicateSession(t *testing.T) {
       // Test that launching connection twice for same user fails
   }
   ```

3. **WebSocket Tests** (`internal/realtime/hub_test.go`)
   ```go
   func TestBroadcastConnectionSessions(t *testing.T) { ... }
   ```

### Frontend Tests

1. **Hook Tests** (`web/src/hooks/useActiveConnections.test.ts`)
   ```typescript
   test('merges WebSocket updates with initial data', () => { ... })
   test('filters by protocol_id', () => { ... })
   test('removes closed sessions', () => { ... })
   ```

2. **Component Tests** (`web/src/components/layout/Sidebar.test.tsx`)
   ```typescript
   test('displays active connection count', () => { ... })
   test('shows "No active connections" when empty', () => { ... })
   ```

---

## Rollout Checklist

### Phase 1: Backend Foundation
- [x] ~~Add `ActiveSessionService` with in-memory registry~~
- [ ] Integrate session lifecycle events in SSH driver (reference implementation)
- [ ] Add `GET /api/connections/active` endpoint
- [ ] Add `connection.sessions` WebSocket stream
- [ ] Write unit tests for session service
- [ ] Add audit logging for session events

### Phase 2: Driver Integration
- [ ] Integrate SSH driver (done in Phase 1)
- [ ] Integrate Docker driver
- [ ] Integrate Kubernetes driver
- [ ] Integrate RDP driver
- [ ] Integrate Database drivers
- [ ] Add periodic cleanup job

### Phase 3: Frontend
- [ ] Create `useWebSocket` hook
- [ ] Create `useActiveConnections` hook
- [ ] Update Sidebar with active connections section
- [ ] Add active badge to ConnectionCard
- [ ] Add "Active Only" filter to Connections page
- [ ] Write frontend tests

### Phase 4: QA & Launch
- [ ] Test session uniqueness constraint (one session per user per connection)
- [ ] Test multiple users on same connection (Alice + Bob on "prod-server-01")
- [ ] Test single user on multiple connections (Alice on SSH + Docker + K8s)
- [ ] Test admin view shows all sessions with usernames
- [ ] Test regular user view shows only own sessions
- [ ] Test duplicate session rejection (error message when user tries to launch twice)
- [ ] Test WebSocket reconnection behavior (if using WebSocket)
- [ ] Test permission filtering (team vs personal)
- [ ] Load test with 50+ active sessions across multiple users
- [ ] Document API endpoints in CORE_MODULE_API.md

---

## API Reference Summary

### REST Endpoints

| Method | Path                       | Description                        | Permission       |
|--------|----------------------------|------------------------------------|------------------|
| GET    | `/api/connections/active`  | List active connection sessions    | `connection.view` |

**Query Parameters:**
- `protocol_id` - Filter by protocol (ssh, docker, kubernetes, etc.)
- `team_id` - Filter by team (`personal` or team UUID)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "id": "sess_abc123",
      "connection_id": "conn_xyz789",
      "user_id": "usr_001",
      "user_name": "alice",
      "team_id": "team_platform",
      "protocol_id": "ssh",
      "started_at": "2025-10-10T14:22:00Z",
      "last_seen_at": "2025-10-10T14:22:00Z",
      "host": "prod-server-01",
      "port": 22
    }
  ]
}
```

**Notes:**
- Regular users only see their own sessions in the response
- Admins see all sessions across all users (with `user_name` visible for each)

### WebSocket Stream

**URL:** `ws://localhost:8000/ws?streams=connection.sessions&token=<access_token>`

**Events:**
- `session.opened` - New connection session started
- `session.closed` - Connection session ended
- `session.heartbeat` - Periodic keepalive (optional)

---

## Notes

- **No `connection_sessions` database table** - Use in-memory registry for MVP (add persistence later if needed)
- **Driver responsibility** - Each protocol driver emits session lifecycle events
- **Polling-first approach** - Use React Query polling (10s interval) for simplicity
- **Permission-aware** - Users only see sessions for connections they can access
- **Protocol-agnostic** - Works with SSH, Docker, Kubernetes, Database, RDP, VNC, etc.

### Important UI Clarifications

- ✅ **Keep existing "Protocols" sidebar section** - Shows connection counts per protocol (e.g., "SSH: 8 connections")
  - Route: `/connections?protocol_id=ssh` (filters connection list by protocol)
  - Data source: `GET /api/connections/summary` (existing endpoint)

- ✅ **Add NEW "Active Sessions" sidebar section** - Shows live connection sessions
  - Displays connection NAMES (e.g., "prod-server-01") not protocol types
  - Links to: `/connections/{connection_id}` (specific connection detail)
  - Data source: `GET /api/connections/active` (new endpoint) + optional WebSocket

- ❌ **DO NOT replace protocols section** - They serve different purposes!
  - Protocols = "what drivers exist and how many connections use them"
  - Active Sessions = "which specific connections are running right now"

### 🔑 Critical Session Constraints (Summary)

**Session Uniqueness:**
- ✅ **One session per (user, connection) pair** - Composite unique key enforced
- ✅ **Multiple users can use same connection** - Alice and Bob can both connect to "prod-server-01"
- ❌ **One user CANNOT have multiple sessions on same connection** - Second launch attempt is rejected

**Session Visibility:**
- 👤 **Regular Users:** See ONLY their own sessions (no visibility into other users)
- 👑 **Admins:** See ALL sessions across all users WITH usernames (for monitoring/support)
- 🏢 **Team Members:** Optionally see team sessions (currently disabled, can be enabled)

**UI Indicators:**
- 💚 **Green pulse dot** - Connection has active sessions
- 🔢 **Session count badge** - Shows "(2)" if multiple users on same connection
- 🛠️ **Admin tooltip** - Hover to see which users are active (admins only)
- 🚫 **Error on duplicate** - "You already have an active session on this connection"

---

## Future Enhancements

1. **Session Details Modal** - Click active connection to see:
   - User who started it
   - Start time / duration
   - Host/port details
   - "Join Session" button (for shared sessions)

2. **Session Metrics** - Track:
   - Average session duration per protocol
   - Peak concurrent sessions
   - Most active connections

3. **Session Recording** - Link active sessions to session recordings

4. **Desktop Notifications** - Browser notifications when:
   - Team member starts a connection
   - Shared session becomes available

5. **Database Persistence** - Migrate from in-memory to database for:
   - Historical session data
   - Recovery after server restart
   - Long-term analytics
