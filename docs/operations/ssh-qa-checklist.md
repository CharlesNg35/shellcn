# SSH Module QA Checklist

## Scope
- SSH terminal launch and reconnection
- SFTP file operations (browse, upload, download, rename, delete)
- Shared session collaboration (invite, write delegation, chat)
- Session recording enablement, retention, and playback metadata

## Preflight
- Ensure test environment has the latest database migrations applied.
- Verify protocol settings defaults (`/settings/protocols/ssh`) align with expected concurrency, idle timeout, and recording policy values.
- Confirm mock SSH/SFTP endpoints and seed identities are reachable.

## Regression Matrix
- Launch SSH session, resize terminal, and run sample commands.
- Validate SFTP tree rendering with mixed files/directories and permission errors.
- Invite participant, toggle write access, and confirm chat history surfaces in UI.
- Start, stop, and download recording while observing audit log entries.
- Exercise websocket reconnection by cycling network interface or toggling airplane mode.

## Bandwidth Throttling Scenarios
- Apply 256 Kbit/s down / 128 Kbit/s up limit; confirm terminal latency meter reflects spike and auto-backoff applies to batching.
- Upload 25 MB file via SFTP under throttled conditions; verify resumable chunks continue without corruption.
- Stream Asciinema playback while throttled to detect dropped frames or truncated segments.

## Concurrency Limit Scenarios
- Set connection limit to `1`, launch owner session, then attempt second launch and expect `limit_reached` toast plus audit entry.
- With limit `2`, launch two sessions, close one, and ensure third launch succeeds without stale locks.
- Run cleanup job (`ActiveSessionService.CleanupStale`) after forcibly terminating client and verify timeout closure event fires.

## Recording Policy Scenarios
- `Disabled`: ensure toggle hidden and `/api/session-records` stays empty.
- `Optional`: start recording mid-session, confirm consent banner shown to participants, and metadata row stores retention flag.
- `Forced`: verify session auto-records, stop endpoint returns `403`, and retention job respects admin-configured TTL.

## Negative Paths
- Attempt SFTP operations without `protocol:ssh.sftp` permission; expect `403` and no hub transfer events.
- Submit malformed chat payload (HTML injection) and confirm sanitization strips tags.
- Simulate vault identity failure to ensure launch aborts and audit log records `session.failed`.

## Observability & Metrics
- Inspect Prometheus `ssh_active_sessions` and `ssh_session_duration_seconds` updates during runs.
- Confirm realtime hub metrics (`realtime_broadcast_total`, `realtime_connections_total`) increment on join/leave actions.
- Review logs for `session.write_granted` and `sftp.transfer.*` events; no error-level entries should appear.

## Exit Criteria
- All automated tests (`go test ./...`, `pnpm test`, `pnpm build`, `pnpm lint`) pass without flaky retries.
- Manual scenarios above executed with evidence captured (screenshots, logs, metrics).
- Roadmap, implementation plan, and API docs updated to reflect coverage status.
