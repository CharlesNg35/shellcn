import { computed, reactive } from "vue";
import { KEEP_ALIVE_WEB_PROXY_PANELS_MAX } from "@/stores/sessionLimits";
import { registerSessionCleanup } from "@/stores/session";
import type {
  ResourceIdentity,
  Row,
  WebProxyCapability,
  WebProxyPanelConfig,
} from "@/types/projection";
import type { StageRect } from "../shared/usePersistentStagePanel";

export interface WebProxyPanelHandle {
  key: string;
  connectionId: string;
  config: WebProxyPanelConfig;
  resource?: ResourceIdentity | null;
  record?: Row | null;
}

export interface WebProxyStageEntry extends WebProxyPanelHandle {
  active: boolean;
  rect: StageRect | null;
  src: string;
  loaded: boolean;
  error: string | null;
  reloadToken: number;
  lastUsed: number;
  signature: string;
}

const entries = reactive(new Map<string, WebProxyStageEntry>());
let clock = 0;

export const webProxyStageEntries = computed(() =>
  Array.from(entries.values()),
);

export function registerWebProxyPanel(handle: WebProxyPanelHandle): void {
  const signature = panelSignature(handle);
  const existing = entries.get(handle.key);
  if (existing) {
    existing.connectionId = handle.connectionId;
    existing.config = handle.config;
    existing.resource = handle.resource;
    existing.record = handle.record;
    existing.active = true;
    existing.lastUsed = nextStamp();
    if (existing.signature !== signature) {
      existing.signature = signature;
      rebuild(existing);
    }
    pruneWebProxyPanels();
    return;
  }

  const entry = reactive<WebProxyStageEntry>({
    ...handle,
    active: true,
    rect: null,
    src: "",
    loaded: false,
    error: null,
    reloadToken: 0,
    lastUsed: nextStamp(),
    signature,
  });
  entries.set(handle.key, entry);
  rebuild(entry);
  pruneWebProxyPanels();
}

export function activateWebProxyPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = true;
  entry.lastUsed = nextStamp();
}

export function deactivateWebProxyPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = false;
  entry.lastUsed = nextStamp();
  pruneWebProxyPanels();
}

export function unregisterWebProxyPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = false;
  entry.rect = null;
  entry.lastUsed = nextStamp();
  pruneWebProxyPanels();
}

export function updateWebProxyPanelRect(
  key: string,
  rect: StageRect | null,
): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.rect = rect;
}

export function markWebProxyPanelLoaded(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.loaded = true;
}

export function reloadWebProxyPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry || entry.error) return;
  entry.loaded = false;
  entry.reloadToken += 1;
  entry.lastUsed = nextStamp();
}

export function disposeWebProxyStage(): void {
  entries.clear();
}

registerSessionCleanup("webProxyStage", disposeWebProxyStage);

export function disposeWebProxyConnection(connectionId: string): void {
  for (const entry of Array.from(entries.values())) {
    if (entry.connectionId !== connectionId) continue;
    entries.delete(entry.key);
  }
}

export function webProxyStageEntryStyle(
  entry: WebProxyStageEntry,
): Record<string, string> {
  const rect = entry.rect;
  const visible = entry.active && rect && entry.src && !entry.error;
  return {
    position: "fixed",
    top: `${rect?.top ?? 0}px`,
    left: `${rect?.left ?? 0}px`,
    width: `${rect?.width ?? 0}px`,
    height: `${rect?.height ?? 0}px`,
    visibility: visible ? "visible" : "hidden",
    pointerEvents: visible ? "auto" : "none",
    zIndex: "30",
  };
}

export function webProxySandboxPolicy(config: WebProxyPanelConfig): string {
  const tokens = ["allow-scripts", "allow-forms", "allow-modals"];
  if (hasCapability(config, "downloads")) tokens.push("allow-downloads");
  if (hasCapability(config, "popups"))
    tokens.push("allow-popups", "allow-popups-to-escape-sandbox");
  if (hasCapability(config, "same_origin")) tokens.push("allow-same-origin");
  return tokens.join(" ");
}

export function webProxyAllowPolicy(
  config: WebProxyPanelConfig,
): string | undefined {
  const policies: string[] = [];
  if (hasCapability(config, "clipboard"))
    policies.push("clipboard-read", "clipboard-write");
  if (hasCapability(config, "fullscreen")) policies.push("fullscreen");
  return policies.length ? policies.join("; ") : undefined;
}

export function webProxyAriaLabel(config: WebProxyPanelConfig): string {
  return config.ariaLabel?.trim() || "Proxied web surface";
}

export function webProxyFrameURL(
  connectionId: string,
  config: WebProxyPanelConfig,
): string | null {
  const path = normalizeProxyPath(config.path);
  if (!path) return null;
  return `/api/connections/${encodeURIComponent(connectionId)}/proxy${path}`;
}

export function normalizeProxyPath(path?: string): string | null {
  const raw = path?.trim() || "/";
  if (!raw.startsWith("/") || raw.startsWith("//") || raw.startsWith("/\\")) {
    return null;
  }
  try {
    const parsed = new URL(raw, "https://shellcn.local");
    if (parsed.origin !== "https://shellcn.local") return null;
    return `${parsed.pathname}${parsed.search}${parsed.hash}` || "/";
  } catch {
    return null;
  }
}

function rebuild(entry: WebProxyStageEntry): void {
  entry.loaded = false;
  entry.error = null;
  const src = webProxyFrameURL(entry.connectionId, entry.config);
  if (!src) {
    entry.src = "";
    entry.error = "This panel declares an invalid proxy path.";
    return;
  }
  entry.src = src;
}

function hasCapability(
  config: WebProxyPanelConfig,
  capability: WebProxyCapability,
): boolean {
  return new Set(config.capabilities ?? []).has(capability);
}

function panelSignature(handle: WebProxyPanelHandle): string {
  return JSON.stringify({
    connectionId: handle.connectionId,
    config: handle.config,
    resource: handle.resource,
    record: handle.record,
  });
}

function pruneWebProxyPanels(): void {
  const inactive = Array.from(entries.values())
    .filter((entry) => !entry.active)
    .sort((a, b) => a.lastUsed - b.lastUsed);
  while (
    entries.size > KEEP_ALIVE_WEB_PROXY_PANELS_MAX &&
    inactive.length > 0
  ) {
    const entry = inactive.shift();
    if (entry) entries.delete(entry.key);
  }
}

function nextStamp(): number {
  clock += 1;
  return clock;
}
