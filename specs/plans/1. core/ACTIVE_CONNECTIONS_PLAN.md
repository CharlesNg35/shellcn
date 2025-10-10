# Active Connections – Implementation Notes

## Objective

Expose live connection activity ("active connections") across the platform and surface it in the UI (`useActiveConnections` hook) so that users can see what is currently in use. This relies on the **Connect Hub** (our realtime event layer) to broadcast session lifecycle updates.

---

## Backend

### 1. Session Telemetry

- Introduce a lightweight `connection_sessions` table (or in-memory registry with persistent tail) keyed by:
  - `id` (UUID)
  - `connection_id`
  - `user_id`
  - `team_id` (nullable)
  - `protocol_id`
  - `started_at`
  - `last_seen_at` (heartbeat, allows pruning)
- Every protocol driver (SSH, RDP, K8s, etc.) fires `session.open` / `session.close` events through the Connect Hub. The Connection runtime is responsible for:
  - Emitting an `open` event when the tunnel succeeds.
  - Periodically emitting a heartbeat (`session.ping`) for long-lived sessions (optional but recommended).
  - Emitting `close` when the connection ends or drops.

### 2. Connect Hub Integration

- Extend the hub with a `connection.sessions` stream:
  - Clients subscribe to `ws://.../ws/connection.sessions`.
  - Payload includes the session DTO above plus `action` (`opened`, `closed`, `heartbeat`).
- The hub relays events to subscribers and can optionally persist them via the session service described above.

### 3. REST API

- `GET /api/connections/active`
  - Filters: `team_id` (UUID or `personal`), `protocol_id`, `user_id` (admin only), pagination.
  - Default response returns currently-open sessions plus metadata (connection name, team name, protocol icon hints).
- `GET /api/connections/summary` (already documented) should accept `active_only=true` once the session layer is in place so we can fetch both "all connections" and "active connections" summaries.

### 4. Permissions & Audit

- Session visibility follows the same `connection.view` guard as the connection list.
- When a session opens/closes, write an audit entry (`connection.session.open`, `connection.session.close`) so administrators can review activity.

---

## Frontend

### 1. Hook: `useActiveConnections`

- Lives in `web/src/hooks/useActiveConnections.ts`.
- Responsibilities:
  - Subscribe to the Connect Hub stream via `useRealtime` (or lightweight websocket helper).
  - Maintain a React Query cache keyed by filters (`team`, `protocol`).
  - Fallback to polling `GET /api/connections/active` on mount (or when the socket drops).
  - Expose `{ sessions, isLoading, isStale }`.

### 2. Sidebar Integration

- Replace the current protocol summary placeholder with a compact list:
  - Show top N active connections grouped by protocol.
  - Each entry links to `/connections?team=...&protocol_id=...&filter=active`.
  - Empty state explains that starting a connection will populate the list.

### 3. Connections Page

- Add an "Active" toggle/segmented control:
  - `All connections` vs `Active connections`.
  - When in "Active" mode, hide stored-but-idle entries.
- Cards display "Live since <timestamp>" and "User: <username>" badges.

### 4. Teams Page

- Leverage the same hook with `team_id` filter; optionally show a mini table of active sessions for that team underneath members/roles.

### 5. UX & Accessibility

- Session updates should animate smoothly (e.g., fade in/out).
- Keep data resilient during socket reconnects: show a subtle banner if stale.
- Ensure screen readers announce changes (ARIA live region in the sidebar list).

---

## Data Retention & Cleanup

- Sessions should expire automatically if `last_seen_at` is older than a grace period (e.g., 5 minutes).
- A periodic job can remove closed/expired sessions to keep the table lean.

---

## Testing Strategy

1. Unit tests for the session service (open/close/heartbeat flow).
2. Websocket integration tests (ensure the hub broadcasts to multiple subscribers).
3. Frontend:
   - Mocked hub stream for the hook (Vitest) to validate optimistic updates.
   - Cypress scenario to ensure the sidebar updates when a session opens/closes.

---

## Rollout Checklist

1. Ship backend session persistence + Connect Hub events.
2. Build REST endpoint and update API documentation.
3. Add `useActiveConnections` hook.
4. Wire sidebar and Connections page UI.
5. QA with at least two protocol drivers (SSH and RDP) to confirm cross-driver behaviour.
