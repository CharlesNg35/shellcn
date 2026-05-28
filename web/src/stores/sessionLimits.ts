// Routed connection workspaces cached by Vue KeepAlive in the app shell.
export const KEEP_ALIVE_CONNECTION_WORKSPACES_MAX = 6;

// Backend plugin sessions the browser will actively heartbeat at once.
// Browser-side active session cap; backend allows more for multi-tab/API safety.
export const MAX_LIVE_CONNECTION_SESSIONS = 10;

// Top-level plugin tabs cached inside one tab-layout connection workspace.
export const KEEP_ALIVE_TOP_LEVEL_PANELS_MAX = 10;

// Sidebar-tree workbench tabs kept open and cached for one connection.
export const KEEP_ALIVE_WORKBENCH_TABS_MAX = 12;

// Resource detail sub-panels cached while switching detail tabs.
export const KEEP_ALIVE_DETAIL_PANELS_MAX = 8;

// Docked action panels cached while switching dock tabs.
export const KEEP_ALIVE_DOCK_PANELS_MAX = 8;

// Per-stream frame backlog retained when a panel detaches and reattaches.
export const STREAM_CHANNEL_BUFFER_LIMIT = 2000;

// Frequency for refreshing live backend plugin sessions from the browser.
export const CONNECTION_SESSION_HEARTBEAT_MS = 30000;
