# Phase 2c — M1.6 · Session recording foundation

**Status:** ☐ Not started · **Milestone:** M1.6 · **Depends on:** Phase 2b (M1.5)
**Spec refs:** [v2 §9.5](../../v2.md), [v2 §12.2](../../v2.md)

M1 built the core stream wrapper; M1.5 makes the platform usable. **M1.6 adds
recording as a generic, opt-in platform capability before the first real terminal
plugin lands.** Recording is never inferred from `terminal` or `remote_desktop`
panel type alone. A plugin declares what it can record, and connection creation
shows recording options only when the selected plugin supports them.

Two recording classes are supported:

- **Terminal/event recording** — SSH, Docker exec, Kubernetes exec, telnet,
  serial. Stored as asciicast v2 (`.cast`) so playback is lightweight and
  protocol-neutral.
- **Desktop/graphical recording** — VNC now, RDP/Guacamole later. Stored through
  a generic recording contract, with either browser canvas capture (`webm`) or a
  plugin-native recorder where the protocol stack supports one.

## Steps

- [ ] 2c.1 Recording manifest contract + connection policy
- [ ] 2c.2 Recording storage, metadata, retention, and authorization
- [ ] 2c.3 Core stream recording wrapper and lifecycle
- [ ] 2c.4 Terminal asciicast recorder and playback
- [ ] 2c.5 Desktop/graphical recording framework
- [ ] 2c.6 Recording APIs and frontend management UI

## Definition of done (phase exit)

A plugin can declare recording support without adding frontend code. A user
creating a connection sees an **off-by-default** "auto record" option only for
supported recording classes. When enabled, terminal streams produce asciicast v2
recordings and graphical streams can create browser-captured WebM recordings or
delegate to a plugin-native recorder. Admins can list/view all recordings; normal
users can list/view only recordings for sessions they created, unless a manage
grant or explicit policy allows broader access. Tests prove recording is opt-in,
not panel-type automatic.

## Out of scope

- Protocol-specific SSH/VNC implementation details: those land with their plugin
  phases, using this foundation.
- Compliance-grade desktop recording for protocols where only browser canvas
  capture exists. Browser capture is useful, but not authoritative if the client
  is untrusted.
- Full recording export/transcoding pipeline beyond first playback/download.
