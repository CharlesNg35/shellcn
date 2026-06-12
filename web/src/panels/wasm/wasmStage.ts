import { computed, markRaw, reactive } from "vue";
import { apiFetch } from "@/api/client";
import {
  callRoute,
  prepareStream,
  resolveParams,
  routeURL,
} from "@/api/dataSource";
import { useTheme } from "@/composables/useTheme";
import { KEEP_ALIVE_WASM_PANELS_MAX } from "@/stores/sessionLimits";
import type {
  DataSource,
  Method,
  ResourceRef,
  WasmAsset,
  WasmBridgeRoute,
  WasmBridgeStream,
  WasmPanelConfig,
} from "@/types/projection";

const WASM_BRIDGE_SOURCE = "shellcn.wasm" as const;
const HOST_BRIDGE_SOURCE = "shellcn.host" as const;

type BridgeRequest =
  | {
      source: typeof WASM_BRIDGE_SOURCE;
      type: "route.request";
      id: string;
      routeId: string;
      method?: Method;
      params?: Record<string, string>;
      body?: unknown;
    }
  | {
      source: typeof WASM_BRIDGE_SOURCE;
      type: "stream.open";
      id: string;
      routeId: string;
      params?: Record<string, string>;
    }
  | {
      source: typeof WASM_BRIDGE_SOURCE;
      type: "stream.send";
      id: string;
      data?: unknown;
    }
  | { source: typeof WASM_BRIDGE_SOURCE; type: "stream.close"; id: string }
  | {
      source: typeof WASM_BRIDGE_SOURCE;
      type: "asset.request";
      id: string;
      path: string;
    }
  | {
      source: typeof WASM_BRIDGE_SOURCE;
      type: "runtime.error";
      error: string;
    };

export interface WasmPanelRect {
  top: number;
  left: number;
  width: number;
  height: number;
}

export interface WasmPanelHandle {
  key: string;
  connectionId: string;
  config: WasmPanelConfig;
  resource?: ResourceRef | null;
}

export interface WasmStageEntry extends WasmPanelHandle {
  active: boolean;
  rect: WasmPanelRect | null;
  srcdoc: string;
  loading: boolean;
  error: string | null;
  lastUsed: number;
  signature: string;
  activeStreams: Map<string, WebSocket>;
}

const entries = reactive(new Map<string, WasmStageEntry>());
const iframeByKey = new Map<string, HTMLIFrameElement>();
let clock = 0;

export const wasmStageEntries = computed(() => Array.from(entries.values()));

export function registerWasmPanel(handle: WasmPanelHandle): void {
  const signature = panelSignature(handle);
  const existing = entries.get(handle.key);
  if (existing) {
    existing.connectionId = handle.connectionId;
    existing.config = handle.config;
    existing.resource = handle.resource;
    existing.active = true;
    existing.lastUsed = nextStamp();
    if (existing.signature !== signature) {
      existing.signature = signature;
      void rebuild(existing);
    }
    pruneWasmPanels();
    return;
  }

  const entry: WasmStageEntry = reactive({
    ...handle,
    active: true,
    rect: null,
    srcdoc: "",
    loading: true,
    error: null,
    lastUsed: nextStamp(),
    signature,
    activeStreams: markRaw(new Map<string, WebSocket>()),
  });
  entries.set(handle.key, entry);
  void rebuild(entry);
  pruneWasmPanels();
}

export function activateWasmPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = true;
  entry.lastUsed = nextStamp();
  postTheme(entry);
}

export function deactivateWasmPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = false;
  entry.lastUsed = nextStamp();
  pruneWasmPanels();
}

export function unregisterWasmPanel(key: string): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.active = false;
  entry.rect = null;
  entry.lastUsed = nextStamp();
  pruneWasmPanels();
}

export function updateWasmPanelRect(
  key: string,
  rect: WasmPanelRect | null,
): void {
  const entry = entries.get(key);
  if (!entry) return;
  entry.rect = rect;
}

export function setWasmPanelIframe(
  key: string,
  iframe: HTMLIFrameElement | null,
): void {
  const entry = entries.get(key);
  if (!entry) return;
  if (iframe) {
    if (iframeByKey.get(key) === iframe) return;
    iframeByKey.set(key, iframe);
    postTheme(entry);
    return;
  }
  iframeByKey.delete(key);
}

export function onWasmStageMessage(event: MessageEvent): void {
  const entry = Array.from(entries.values()).find(
    (item) => iframeByKey.get(item.key)?.contentWindow === event.source,
  );
  if (!entry) return;
  const msg = event.data as BridgeRequest;
  if (msg?.source !== WASM_BRIDGE_SOURCE) return;
  switch (msg.type) {
    case "route.request":
      void handleRoute(entry, msg);
      break;
    case "stream.open":
      void handleStreamOpen(entry, msg);
      break;
    case "stream.send":
      sendStream(entry, msg.id, msg.data);
      break;
    case "stream.close":
      closeStream(entry, msg.id);
      break;
    case "asset.request":
      void handleAsset(entry, msg.id, msg.path);
      break;
    case "runtime.error":
      entry.error = msg.error || "WebAssembly panel reported an error.";
      break;
  }
}

export function refreshWasmStageTheme(): void {
  for (const entry of entries.values()) postTheme(entry);
}

export function disposeWasmStage(): void {
  for (const entry of entries.values()) destroyEntry(entry);
  entries.clear();
}

export function disposeWasmConnection(connectionId: string): void {
  for (const entry of Array.from(entries.values())) {
    if (entry.connectionId !== connectionId) continue;
    destroyEntry(entry);
    entries.delete(entry.key);
  }
}

export function wasmStageEntryStyle(
  entry: WasmStageEntry,
): Record<string, string> {
  const rect = entry.rect;
  const visible = entry.active && rect && entry.srcdoc && !entry.error;
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

export function wasmStageFrameBoxStyle(
  entry: WasmStageEntry,
): Record<string, string> {
  const { config, rect } = entry;
  const width = config.width;
  const height = config.height;
  if (config.scaleMode === "fit" && width && height && rect) {
    const scale = Math.min(rect.width / width, rect.height / height);
    return {
      position: "relative",
      width: `${Math.max(0, width * scale)}px`,
      height: `${Math.max(0, height * scale)}px`,
      flex: "0 0 auto",
    };
  }
  if (config.scaleMode === "scroll" && width && height)
    return { width: `${width}px`, height: `${height}px` };

  return { width: "100%", height: "100%" };
}

export function wasmStageFrameStyle(
  entry: WasmStageEntry,
): Record<string, string> {
  const { config, rect } = entry;
  const width = config.width;
  const height = config.height;
  if (config.scaleMode === "fit" && width && height && rect) {
    const scale = Math.min(rect.width / width, rect.height / height);
    return {
      width: `${width}px`,
      height: `${height}px`,
      transform: `scale(${scale})`,
      transformOrigin: "top left",
    };
  }
  if (config.scaleMode === "scroll" && width && height)
    return { width: "100%", height: "100%" };

  return { width: "100%", height: "100%" };
}

export function wasmStageViewportClass(config: WasmPanelConfig): string {
  switch (config.scaleMode) {
    case "scroll":
      return "overflow-auto overscroll-contain";
    case "fit":
      return "grid place-items-center overflow-hidden";
    default:
      return "overflow-hidden";
  }
}

export function wasmStageSandbox(config: WasmPanelConfig): string {
  const tokens = ["allow-scripts"];
  if (config.capabilities?.pointerLock) tokens.push("allow-pointer-lock");
  return tokens.join(" ");
}

export function wasmStageAllow(config: WasmPanelConfig): string | undefined {
  const tokens: string[] = [];
  if (config.capabilities?.fullscreen) tokens.push("fullscreen");
  if (config.capabilities?.gamepad) tokens.push("gamepad");
  return tokens.length ? tokens.join("; ") : undefined;
}

async function rebuild(entry: WasmStageEntry): Promise<void> {
  closeStreams(entry);
  entry.loading = true;
  entry.error = null;
  entry.srcdoc = "";
  try {
    validateConfig(entry.config);
    const assetMap = panelAssetMap(entry.config);
    for (const path of [
      entry.config.entry,
      ...(entry.config.boot?.scripts ?? []),
    ]) {
      if (!assetMap.has(path))
        throw new Error(`WASM asset "${path}" is not declared.`);
    }
    const scripts = await Promise.all(
      (entry.config.boot?.scripts ?? []).map((path) =>
        fetchAssetText(entry, path),
      ),
    );
    if (entries.get(entry.key)?.signature !== entry.signature) return;
    entry.srcdoc = buildSrcdoc(entry.config, scripts);
  } catch (err) {
    entry.error = (err as Error).message;
  } finally {
    entry.loading = false;
  }
}

function validateConfig(config: WasmPanelConfig): void {
  if (!config.entry) throw new Error("WASM panel entry is required.");
}

async function handleRoute(
  entry: WasmStageEntry,
  msg: Extract<BridgeRequest, { type: "route.request" }>,
): Promise<void> {
  const allowed = routeMap(entry.config).get(msg.routeId);
  if (!allowed) {
    post(entry, {
      type: "route.response",
      id: msg.id,
      ok: false,
      error: "Route is not allowed for this WASM panel.",
    });
    return;
  }
  try {
    const data = await callRoute(
      entry.connectionId,
      msg.routeId,
      { resource: entry.resource },
      msg.body,
      { ...(allowed.params ?? {}), ...(msg.params ?? {}) },
      msg.method ?? allowed.method ?? "POST",
    );
    post(entry, { type: "route.response", id: msg.id, ok: true, data });
  } catch (err) {
    post(entry, {
      type: "route.response",
      id: msg.id,
      ok: false,
      error: (err as Error).message,
    });
  }
}

async function handleStreamOpen(
  entry: WasmStageEntry,
  msg: Extract<BridgeRequest, { type: "stream.open" }>,
): Promise<void> {
  const allowed = streamMap(entry.config).get(msg.routeId);
  if (!allowed) {
    post(entry, {
      type: "stream.error",
      id: msg.id,
      error: "Stream is not allowed for this WASM panel.",
    });
    return;
  }
  try {
    const ds: DataSource = {
      routeId: msg.routeId,
      method: "WS",
      params: { ...(allowed.params ?? {}), ...(msg.params ?? {}) },
    };
    const handle = await prepareStream(entry.connectionId, ds, {
      resource: entry.resource,
    });
    const socket = new WebSocket(handle.url);
    entry.activeStreams.set(msg.id, socket);
    socket.addEventListener("open", () =>
      post(entry, { type: "stream.opened", id: msg.id }),
    );
    socket.addEventListener("message", (ev) =>
      post(entry, { type: "stream.message", id: msg.id, data: ev.data }),
    );
    socket.addEventListener("error", () =>
      post(entry, {
        type: "stream.error",
        id: msg.id,
        error: "Stream connection failed.",
      }),
    );
    socket.addEventListener("close", () => {
      entry.activeStreams.delete(msg.id);
      post(entry, { type: "stream.close", id: msg.id });
    });
  } catch (err) {
    post(entry, {
      type: "stream.error",
      id: msg.id,
      error: (err as Error).message,
    });
  }
}

function sendStream(entry: WasmStageEntry, id: string, data: unknown): void {
  const socket = entry.activeStreams.get(id);
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(typeof data === "string" ? data : JSON.stringify(data ?? null));
}

function closeStream(entry: WasmStageEntry, id: string): void {
  entry.activeStreams.get(id)?.close();
  entry.activeStreams.delete(id);
}

function closeStreams(entry: WasmStageEntry): void {
  for (const socket of entry.activeStreams.values()) socket.close();
  entry.activeStreams.clear();
}

async function handleAsset(
  entry: WasmStageEntry,
  id: string,
  path: string,
): Promise<void> {
  try {
    const buffer = await fetchAssetBuffer(entry, path);
    post(entry, { type: "asset.response", id, ok: true, data: buffer }, [
      buffer,
    ]);
  } catch (err) {
    post(entry, {
      type: "asset.response",
      id,
      ok: false,
      error: (err as Error).message,
    });
  }
}

function post(
  entry: WasmStageEntry,
  message: Record<string, unknown>,
  transfer?: Transferable[],
): void {
  iframeByKey
    .get(entry.key)
    ?.contentWindow?.postMessage(
      { source: HOST_BRIDGE_SOURCE, ...message },
      "*",
      transfer ?? [],
    );
}

function postTheme(entry: WasmStageEntry): void {
  const { theme } = useTheme();
  post(entry, { type: "theme", theme: theme.value, colors: themeColors() });
}

async function fetchAssetText(
  entry: WasmStageEntry,
  path: string,
): Promise<string> {
  const bytes = await fetchAssetBytes(entry, path);
  return new TextDecoder().decode(bytes);
}

async function fetchAssetBuffer(
  entry: WasmStageEntry,
  path: string,
): Promise<ArrayBuffer> {
  const bytes = await fetchAssetBytes(entry, path);
  const buffer = new ArrayBuffer(bytes.byteLength);
  new Uint8Array(buffer).set(bytes);
  return buffer;
}

async function fetchAssetBytes(
  entry: WasmStageEntry,
  path: string,
): Promise<Uint8Array> {
  const asset = panelAssetMap(entry.config).get(path);
  if (!asset) throw new Error(`WASM asset "${path}" is not declared.`);
  const url = assetURL(entry, asset);
  const res = await apiFetch(url);
  return new Uint8Array(await res.arrayBuffer());
}

function assetURL(entry: WasmStageEntry, asset: WasmAsset): string {
  return routeURL(
    entry.connectionId,
    asset.source.routeId,
    { resource: entry.resource },
    resolveParams(asset.source.params, { resource: entry.resource }),
  );
}

function panelAssetMap(config: WasmPanelConfig): Map<string, WasmAsset> {
  const out = new Map<string, WasmAsset>();
  for (const asset of config.assets ?? []) out.set(asset.path, asset);
  return out;
}

function routeMap(config: WasmPanelConfig): Map<string, WasmBridgeRoute> {
  const out = new Map<string, WasmBridgeRoute>();
  for (const route of config.bridge?.routes ?? [])
    out.set(route.routeId, route);
  return out;
}

function streamMap(config: WasmPanelConfig): Map<string, WasmBridgeStream> {
  const out = new Map<string, WasmBridgeStream>();
  for (const stream of config.bridge?.streams ?? [])
    out.set(stream.routeId, stream);
  return out;
}

function buildSrcdoc(config: WasmPanelConfig, scripts: string[]): string {
  const scriptOpen = "<" + "script>";
  const scriptClose = "<" + "/" + "script>";
  const bodyOverflow = config.scaleMode === "scroll" ? "auto" : "hidden";
  const { theme } = useTheme();
  const boot = scripts
    .map((script) => `${scriptOpen}${escapeScript(script)}${scriptClose}`)
    .join("\n");
  return `<!doctype html>
<html data-theme="${theme.value}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'wasm-unsafe-eval' blob:; worker-src blob:; connect-src blob:; style-src 'unsafe-inline'; img-src blob: data:; media-src blob: data:; font-src blob: data:;">
<style>
:root{color-scheme:dark;--shellcn-bg:#020617;--shellcn-text:#e2e8f0;--shellcn-muted:#94a3b8}
:root[data-theme=light]{color-scheme:light;--shellcn-bg:#f8fafc;--shellcn-text:#0f172a;--shellcn-muted:#64748b}
html,body{margin:0;width:100%;min-height:100%;height:100%;overflow:${bodyOverflow};background:var(--shellcn-bg);color:var(--shellcn-text);font-family:Inter,system-ui,sans-serif}
#shellcn-wasm-status{position:fixed;inset:0;display:grid;place-items:center;padding:24px;text-align:center;color:var(--shellcn-muted);background:var(--shellcn-bg)}
</style>
</head>
<body data-theme="${theme.value}">
<div id="shellcn-wasm-status">Loading...</div>
${scriptOpen}${escapeScript(bridgeScript(config))}${scriptClose}
${boot}
${scriptOpen}${escapeScript(startScript(config))}${scriptClose}
</body>
</html>`;
}

function bridgeScript(config: WasmPanelConfig): string {
  const wasmSource = JSON.stringify(WASM_BRIDGE_SOURCE);
  const hostSource = JSON.stringify(HOST_BRIDGE_SOURCE);
  const { theme } = useTheme();
  const autoHideAfterAssets =
    (config.runtime ?? "generic") === "generic" &&
    Boolean(config.boot?.scripts?.length);
  return `
(() => {
  const pending = new Map();
  const streams = new Map();
  const themeListeners = new Set();
  const autoHideAfterAssets = ${JSON.stringify(autoHideAfterAssets)};
  let pendingAssets = 0;
  let hideStatusTimer = 0;
  function statusEl() {
    return document.getElementById("shellcn-wasm-status");
  }
  function applyShellTheme(next, colors = {}) {
    const theme = next === "light" ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", theme);
    document.body?.setAttribute("data-theme", theme);
    const bg = theme === "light"
      ? colors.surface0 || colors.surface50 || "#f8fafc"
      : colors.surface950 || colors.surface900 || "#020617";
    const text = theme === "light"
      ? colors.surface950 || colors.surface900 || "#0f172a"
      : colors.surface100 || colors.surface50 || "#e2e8f0";
    const muted = theme === "light"
      ? colors.surface500 || colors.surface600 || "#64748b"
      : colors.surface400 || colors.surface300 || "#94a3b8";
    document.documentElement.style.setProperty("--shellcn-bg", bg);
    document.documentElement.style.setProperty("--shellcn-text", text);
    document.documentElement.style.setProperty("--shellcn-muted", muted);
  }
  function hideStatusSoon() {
    if (!autoHideAfterAssets) return;
    if (pendingAssets > 0 || hideStatusTimer) return;
    hideStatusTimer = setTimeout(() => {
      hideStatusTimer = 0;
      if (pendingAssets === 0) statusEl()?.remove();
    }, 0);
  }
  function request(type, payload) {
    const id = crypto.randomUUID();
    parent.postMessage({ source: ${wasmSource}, type, id, ...payload }, "*");
    return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
  }
  window.shellcn = {
    entry: ${JSON.stringify(config.entry)},
    capabilities: ${JSON.stringify(config.capabilities ?? {})},
    theme: ${JSON.stringify(theme.value)},
    colors: ${JSON.stringify(themeColors())},
    onTheme(fn) {
      if (typeof fn !== "function") return () => {};
      themeListeners.add(fn);
      return () => themeListeners.delete(fn);
    },
    reportError(error) {
      if (hideStatusTimer) {
        clearTimeout(hideStatusTimer);
        hideStatusTimer = 0;
      }
      const message = error instanceof Error ? error.message : String(error || "WebAssembly panel failed.");
      const status = statusEl();
      if (status) status.textContent = message;
      parent.postMessage({ source: ${wasmSource}, type: "runtime.error", error: message }, "*");
    },
    hideStatus() {
      statusEl()?.remove();
    },
    route(routeId, body = {}, options = {}) {
      return request("route.request", { routeId, body, params: options.params || {}, method: options.method });
    },
    asset(path) {
      pendingAssets += 1;
      return request("asset.request", { path });
    },
    async assetURL(path, mime = "application/octet-stream") {
      const data = await this.asset(path);
      return URL.createObjectURL(new Blob([data], { type: mime }));
    },
    stream(routeId, params = {}) {
      const id = crypto.randomUUID();
      const listeners = new Set();
      parent.postMessage({ source: ${wasmSource}, type: "stream.open", id, routeId, params }, "*");
      const handle = {
        id,
        onMessage(fn) { listeners.add(fn); return () => listeners.delete(fn); },
        send(data) { parent.postMessage({ source: ${wasmSource}, type: "stream.send", id, data }, "*"); },
        close() { parent.postMessage({ source: ${wasmSource}, type: "stream.close", id }, "*"); streams.delete(id); },
        _emit(data) { for (const fn of listeners) fn(data); }
      };
      streams.set(id, handle);
      return handle;
    }
  };
  applyShellTheme(window.shellcn.theme, window.shellcn.colors);
  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || msg.source !== ${hostSource}) return;
    if (msg.type === "theme") {
      window.shellcn.theme = msg.theme;
      window.shellcn.colors = msg.colors || {};
      applyShellTheme(window.shellcn.theme, window.shellcn.colors);
      for (const fn of themeListeners) fn(msg.theme, window.shellcn.colors);
      return;
    }
    if (msg.type === "route.response" || msg.type === "asset.response") {
      const req = pending.get(msg.id);
      if (!req) return;
      pending.delete(msg.id);
      if (msg.type === "asset.response") pendingAssets = Math.max(0, pendingAssets - 1);
      msg.ok ? req.resolve(msg.data) : req.reject(new Error(msg.error || "Bridge request failed"));
      if (msg.type === "asset.response" && msg.ok) hideStatusSoon();
      return;
    }
    if (msg.type === "stream.message") streams.get(msg.id)?._emit(msg.data);
    if (msg.type === "stream.error") streams.get(msg.id)?._emit({ error: msg.error });
    if (msg.type === "stream.close") streams.delete(msg.id);
  });
})();`;
}

function themeColors(): Record<string, string> {
  const style = getComputedStyle(document.documentElement);
  const read = (name: string) => style.getPropertyValue(name).trim();
  return {
    primary50: read("--p-primary-50"),
    primary100: read("--p-primary-100"),
    primary200: read("--p-primary-200"),
    primary300: read("--p-primary-300"),
    primary400: read("--p-primary-400"),
    primary500: read("--p-primary-500"),
    primary600: read("--p-primary-600"),
    primary700: read("--p-primary-700"),
    primary800: read("--p-primary-800"),
    primary900: read("--p-primary-900"),
    primary950: read("--p-primary-950"),
    surface0: read("--p-surface-0"),
    surface50: read("--p-surface-50"),
    surface100: read("--p-surface-100"),
    surface200: read("--p-surface-200"),
    surface300: read("--p-surface-300"),
    surface400: read("--p-surface-400"),
    surface500: read("--p-surface-500"),
    surface600: read("--p-surface-600"),
    surface700: read("--p-surface-700"),
    surface800: read("--p-surface-800"),
    surface900: read("--p-surface-900"),
    surface950: read("--p-surface-950"),
  };
}

function startScript(config: WasmPanelConfig): string {
  const runtime = config.runtime ?? "generic";
  const entry = JSON.stringify(config.entry);
  const bootOwnsGenericStartup =
    runtime === "generic" && Boolean(config.boot?.scripts?.length);
  return `
(async () => {
  const status = document.getElementById("shellcn-wasm-status");
  try {
    if (typeof WebAssembly !== "object") throw new Error("WebAssembly is not supported in this browser.");
    if (${JSON.stringify(bootOwnsGenericStartup)}) return;
    const bytes = await window.shellcn.asset(${entry});
    if (${JSON.stringify(runtime)} === "go") {
      if (typeof Go !== "function") throw new Error("Go WASM runtime was not loaded.");
      const go = new Go();
      const result = await WebAssembly.instantiate(bytes, go.importObject);
      status?.remove();
      await go.run(result.instance);
      return;
    }
    const result = await WebAssembly.instantiate(bytes, {});
    status?.remove();
    const start = result.instance.exports._start || result.instance.exports.main;
    if (typeof start === "function") start();
  } catch (err) {
    window.shellcn.reportError(err);
  }
})();`;
}

function escapeScript(value: string): string {
  return value.replaceAll("<" + "/script", "<\\/" + "script");
}

function panelSignature(handle: WasmPanelHandle): string {
  return JSON.stringify({
    connectionId: handle.connectionId,
    config: handle.config,
    resource: handle.resource ?? null,
  });
}

function pruneWasmPanels(): void {
  const inactive = Array.from(entries.values())
    .filter((entry) => !entry.active)
    .sort((a, b) => a.lastUsed - b.lastUsed);
  while (entries.size > KEEP_ALIVE_WASM_PANELS_MAX && inactive.length) {
    const entry = inactive.shift();
    if (!entry) return;
    destroyEntry(entry);
    entries.delete(entry.key);
  }
}

function destroyEntry(entry: WasmStageEntry): void {
  closeStreams(entry);
  entry.srcdoc = "";
  iframeByKey.delete(entry.key);
}

function nextStamp(): number {
  clock += 1;
  return clock;
}
