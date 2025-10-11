# Realtime WebSocket Architecture

## Overview

ShellCN exposes a single authenticated WebSocket entry point that multiplexes realtime features (notifications today, chat/SSH/video in future) over one connection per browser session. The goal is to minimise open sockets, standardise message envelopes, and provide a consistent upgrade/authentication workflow for backend services and frontend consumers.

```
Client ──┐
         │  GET /ws?token=…&streams=notifications,chat
Gin ─> RealtimeHandler ─> Realtime Hub ─> Stream handlers (notifications, chat, …)
```

## Connection Lifecycle

1. **Upgrade** – Clients hit `GET /ws` (alias `/ws/:stream`) and include:
   - `token` (or `access_token`) query parameter with the current access token, or an `Authorization: Bearer <token>` header.
   - `streams` query parameter (comma separated or repeated) listing the logical streams required (e.g. `notifications`, `chat`).
2. **Authentication** – `RealtimeHandler` validates the token via `JWTService`. Unauthorised requests return `401`.
3. **Registration** – The request upgrades through `gorilla/websocket` and registers the connection + user ID + initial stream subscriptions with the shared hub.
4. **Multiplexing** – Services push events through the hub:
   - `BroadcastToUser(stream, userID, message)` – targeted delivery.
   - `BroadcastToUsers(stream, []userIDs, message)` – fan-out to specific users.
   - `BroadcastStream(stream, message)` – stream-wide broadcast.
5. **Heartbeat** – The hub emits ping frames every 54 seconds (`pingPeriod`) and enforces a 60 second `pong` timeout to detect dead clients.
6. **Control Messages** – Clients can send JSON control frames:
   ```json
   { "action": "subscribe", "streams": ["chat"] }
   { "action": "unsubscribe", "streams": ["notifications"] }
   { "action": "ping" }
   ```
   Unsupported payloads are ignored with a server log entry. Future control verbs should use the same envelope.

## Message Envelope

All server-to-client messages share a canonical structure:

```json
{
  "stream": "notifications",
  "event": "notification.created",
  "data": { "... stream specific payload ..." },
  "meta": { "... optional metadata ..." }
}
```

- `stream` – lower-case identifier (see [Stream Catalog](#stream-catalog)).
- `event` – semantic event name scoped to the stream.
- `data` – structured payload owned by the stream producer.
- `meta` – optional key/value metadata.

Client code should first check `stream` before handling the event.

## Backend Integration

1. **Create a stream constant** in `internal/realtime/streams.go`.
2. **Broadcast** using the hub (`*realtime.Hub`), injecting it where needed:
   ```go
   hub.BroadcastToUser(
       realtime.StreamNotifications,
       userID,
       realtime.Message{
           Event: "notification.created",
           Data:  payload, // struct matching JSON contract
       },
   )
   ```
3. **Subscriber services** should depend on the hub instead of provisioning their own WebSocket stack. This preserves multiplexing and shared heartbeat support.
4. **Token extraction** is centralised in `RealtimeHandler`. If you need custom auth logic, extend the handler rather than re-implementing a bespoke endpoint.

### Notifications Example

- Stream: `notifications`
- Event schema (`NotificationEventPayload` in Go / `NotificationEventData` in TS) includes:
  - `notification` – the full DTO when available.
  - `notification_id` – fallback ID for deletes/markers.
- Events: `notification.created`, `notification.updated`, `notification.read`, `notification.deleted`, `notification.read_all`.

## Frontend Integration

1. Build URLs with `buildWebSocketUrl('/ws', { token, streams: 'notifications' })`.
2. Use the shared `useWebSocket` hook with `parseJson` enabled (default) so that payloads arrive as typed objects.
3. Handle messages via the `RealtimeMessage<T>` interface (`web/src/types/realtime.ts`) and gate by `message.stream`.
4. To subscribe to additional streams on demand, send a control message:
   ```ts
   socket.send(JSON.stringify({ action: 'subscribe', streams: ['chat'] }))
   ```
5. When multiple modules need realtime data (e.g., notifications + chat), prefer a single connection and distribute messages through local event emitters or React context.

## Stream Catalog

| Stream ID       | Purpose                  | Current Status |
|-----------------|--------------------------|----------------|
| `notifications` | User notification events | Implemented    |
| `chat`          | Team/user chat (future)  | Reserved       |
| `ssh`           | Interactive SSH (future) | Reserved       |
| `rdp`           | Remote desktop (future)  | Reserved       |

Add new streams by defining a constant and documenting the required event shapes within this file.

## Operational Notes

- **Backpressure**: Each connection has a buffered send channel (`64` messages). Overflow drops the client and the hub logs a warning. Producers should keep payloads small and avoid bursty fan-out without rate control.
- **Security**: Origin checks allow same-origin or loopback. If you deploy behind a different domain, update the `CheckOrigin` function to reflect allowed hosts.
- **Scaling**: The hub is currently in-process and keyed by user ID. For multi-node deployments, layer a pub/sub backend (Redis, NATS, etc.) that fan-outs events to each node’s hub instance.
- **Binary Streams**: High-throughput binary protocols (e.g., screen sharing) may require dedicated sockets with tuned buffers. Those should still authenticate via `RealtimeHandler`, but can negotiate a different sub-protocol or route if needed.

## Future Enhancements

- **Session-Aware Routing**: Attach session IDs to `meta` so that frontends can correlate events with specific remote sessions.
- **Per-Stream Authorization**: Add hook points for stream-level permission checks.
- **Presence & Diagnostics**: Expose hub metrics (connected clients per stream) and implement admin endpoints for observability.
