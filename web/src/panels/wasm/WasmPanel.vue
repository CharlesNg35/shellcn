<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { apiFetch } from "../../api/client";
import {
  callRoute,
  prepareStream,
  resolveParams,
  routeURL,
} from "../../api/dataSource";
import type {
  DataSource,
  Method,
  WasmAsset,
  WasmBridgeRoute,
  WasmBridgeStream,
  WasmPanelConfig,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import PanelLoader from "../../components/PanelLoader.vue";
import { useTheme } from "../../composables/useTheme";
import PanelError from "../shared/PanelError.vue";

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
    };

const props = defineProps<PanelProps>();

const cfg = computed(() => props.config as WasmPanelConfig | undefined);
const iframeEl = ref<HTMLIFrameElement | null>(null);
const srcdoc = ref("");
const error = ref<string | null>(null);
const loading = ref(true);
const { theme } = useTheme();
const activeStreams = new Map<string, WebSocket>();

const assetMap = computed(() => {
  const out = new Map<string, WasmAsset>();
  for (const asset of cfg.value?.assets ?? []) out.set(asset.path, asset);
  return out;
});
const routeMap = computed(() => {
  const out = new Map<string, WasmBridgeRoute>();
  for (const route of cfg.value?.bridge?.routes ?? [])
    out.set(route.routeId, route);
  return out;
});
const streamMap = computed(() => {
  const out = new Map<string, WasmBridgeStream>();
  for (const stream of cfg.value?.bridge?.streams ?? [])
    out.set(stream.routeId, stream);
  return out;
});
const viewportClass = computed(() => {
  switch (cfg.value?.scaleMode) {
    case "scroll":
      return "overflow-auto overscroll-contain";
    case "fit":
      return "grid place-items-center overflow-hidden";
    default:
      return "overflow-hidden";
  }
});
const iframeStyle = computed(() => {
  const width = cfg.value?.width;
  const height = cfg.value?.height;
  if (cfg.value?.scaleMode === "scroll" && width && height)
    return { width: `${width}px`, height: `${height}px` };
  if (cfg.value?.scaleMode === "fit" && width && height)
    return {
      width: `${width}px`,
      height: `${height}px`,
      maxWidth: "100%",
      maxHeight: "100%",
    };
  return { width: "100%", height: "100%" };
});
const sandbox = computed(() => {
  const tokens = ["allow-scripts"];
  if (cfg.value?.capabilities?.pointerLock) tokens.push("allow-pointer-lock");
  return tokens.join(" ");
});
const iframeAllow = computed(() => {
  const tokens: string[] = [];
  if (cfg.value?.capabilities?.fullscreen) tokens.push("fullscreen");
  if (cfg.value?.capabilities?.gamepad) tokens.push("gamepad");
  return tokens.join("; ");
});

watch(
  cfg,
  () => {
    void rebuild();
  },
  { immediate: true },
);
watch(theme, () => postTheme());

window.addEventListener("message", onMessage);
onUnmounted(() => {
  window.removeEventListener("message", onMessage);
  closeStreams();
});

async function rebuild(): Promise<void> {
  closeStreams();
  loading.value = true;
  error.value = null;
  try {
    const config = cfg.value;
    if (!config?.entry) throw new Error("WASM panel entry is required.");
    for (const path of [config.entry, ...(config.boot?.scripts ?? [])]) {
      if (!assetMap.value.has(path))
        throw new Error(`WASM asset "${path}" is not declared.`);
    }
    const scripts = await Promise.all(
      (config.boot?.scripts ?? []).map((path) => fetchAssetText(path)),
    );
    srcdoc.value = buildSrcdoc(config, scripts);
    await nextTick();
    postTheme();
  } catch (err) {
    error.value = (err as Error).message;
    srcdoc.value = "";
  } finally {
    loading.value = false;
  }
}

function onMessage(event: MessageEvent): void {
  if (event.source !== iframeEl.value?.contentWindow) return;
  const msg = event.data as BridgeRequest;
  if (msg?.source !== WASM_BRIDGE_SOURCE) return;
  switch (msg.type) {
    case "route.request":
      void handleRoute(msg);
      break;
    case "stream.open":
      void handleStreamOpen(msg);
      break;
    case "stream.send":
      sendStream(msg.id, msg.data);
      break;
    case "stream.close":
      closeStream(msg.id);
      break;
    case "asset.request":
      void handleAsset(msg.id, msg.path);
      break;
  }
}

async function handleRoute(
  msg: Extract<BridgeRequest, { type: "route.request" }>,
): Promise<void> {
  const allowed = routeMap.value.get(msg.routeId);
  if (!allowed) {
    post({
      type: "route.response",
      id: msg.id,
      ok: false,
      error: "Route is not allowed for this WASM panel.",
    });
    return;
  }
  try {
    const data = await callRoute(
      props.connectionId,
      msg.routeId,
      { resource: props.resource },
      msg.body,
      { ...(allowed.params ?? {}), ...(msg.params ?? {}) },
      msg.method ?? allowed.method ?? "POST",
    );
    post({ type: "route.response", id: msg.id, ok: true, data });
  } catch (err) {
    post({
      type: "route.response",
      id: msg.id,
      ok: false,
      error: (err as Error).message,
    });
  }
}

async function handleStreamOpen(
  msg: Extract<BridgeRequest, { type: "stream.open" }>,
): Promise<void> {
  const allowed = streamMap.value.get(msg.routeId);
  if (!allowed) {
    post({
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
    const handle = await prepareStream(props.connectionId, ds, {
      resource: props.resource,
    });
    const socket = new WebSocket(handle.url);
    activeStreams.set(msg.id, socket);
    socket.addEventListener("open", () =>
      post({ type: "stream.opened", id: msg.id }),
    );
    socket.addEventListener("message", (ev) =>
      post({ type: "stream.message", id: msg.id, data: ev.data }),
    );
    socket.addEventListener("error", () =>
      post({
        type: "stream.error",
        id: msg.id,
        error: "Stream connection failed.",
      }),
    );
    socket.addEventListener("close", () => {
      activeStreams.delete(msg.id);
      post({ type: "stream.close", id: msg.id });
    });
  } catch (err) {
    post({ type: "stream.error", id: msg.id, error: (err as Error).message });
  }
}

function sendStream(id: string, data: unknown): void {
  const socket = activeStreams.get(id);
  if (!socket || socket.readyState !== WebSocket.OPEN) return;
  socket.send(typeof data === "string" ? data : JSON.stringify(data ?? null));
}

function closeStream(id: string): void {
  activeStreams.get(id)?.close();
  activeStreams.delete(id);
}

function closeStreams(): void {
  for (const socket of activeStreams.values()) socket.close();
  activeStreams.clear();
}

function postTheme(): void {
  post({ type: "theme", theme: theme.value });
}

async function handleAsset(id: string, path: string): Promise<void> {
  try {
    const buffer = await fetchAssetBuffer(path);
    post({ type: "asset.response", id, ok: true, data: buffer }, [buffer]);
  } catch (err) {
    post({
      type: "asset.response",
      id,
      ok: false,
      error: (err as Error).message,
    });
  }
}

function post(
  message: Record<string, unknown>,
  transfer?: Transferable[],
): void {
  iframeEl.value?.contentWindow?.postMessage(
    { source: HOST_BRIDGE_SOURCE, ...message },
    "*",
    transfer ?? [],
  );
}

async function fetchAssetText(path: string): Promise<string> {
  const asset = assetMap.value.get(path);
  if (!asset) throw new Error(`WASM asset "${path}" is not declared.`);
  const res = await apiFetch(assetURL(asset));
  return res.text();
}

async function fetchAssetBuffer(path: string): Promise<ArrayBuffer> {
  const asset = assetMap.value.get(path);
  if (!asset) throw new Error(`WASM asset "${path}" is not declared.`);
  const res = await apiFetch(assetURL(asset));
  return res.arrayBuffer();
}

function assetURL(asset: WasmAsset): string {
  return routeURL(
    props.connectionId,
    asset.source.routeId,
    { resource: props.resource },
    resolveParams(asset.source.params, { resource: props.resource }),
  );
}

function buildSrcdoc(config: WasmPanelConfig, scripts: string[]): string {
  const scriptOpen = "<" + "script>";
  const scriptClose = "<" + "/" + "script>";
  const bodyOverflow = config.scaleMode === "scroll" ? "auto" : "hidden";
  const boot = scripts
    .map((script) => `${scriptOpen}${escapeScript(script)}${scriptClose}`)
    .join("\n");
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'unsafe-inline' 'wasm-unsafe-eval'; style-src 'unsafe-inline'; img-src blob: data:; media-src blob: data:;">
<style>
html,body{margin:0;width:100%;min-height:100%;height:100%;overflow:${bodyOverflow};background:#020617;color:#e2e8f0;font-family:Inter,system-ui,sans-serif}
#shellcn-wasm-status{position:fixed;inset:0;display:grid;place-items:center;padding:24px;text-align:center;color:#94a3b8}
</style>
</head>
<body>
<div id="shellcn-wasm-status">Loading</div>
${scriptOpen}${escapeScript(bridgeScript(config))}${scriptClose}
${boot}
${scriptOpen}${escapeScript(startScript(config))}${scriptClose}
</body>
</html>`;
}

function bridgeScript(config: WasmPanelConfig): string {
  const wasmSource = JSON.stringify(WASM_BRIDGE_SOURCE);
  const hostSource = JSON.stringify(HOST_BRIDGE_SOURCE);
  return `
(() => {
  const pending = new Map();
  const streams = new Map();
  const themeListeners = new Set();
  function request(type, payload) {
    const id = crypto.randomUUID();
    parent.postMessage({ source: ${wasmSource}, type, id, ...payload }, "*");
    return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
  }
  window.shellcn = {
    capabilities: ${JSON.stringify(config.capabilities ?? {})},
    theme: ${JSON.stringify(theme.value)},
    onTheme(fn) {
      if (typeof fn !== "function") return () => {};
      themeListeners.add(fn);
      return () => themeListeners.delete(fn);
    },
    route(routeId, body = {}, options = {}) {
      return request("route.request", { routeId, body, params: options.params || {}, method: options.method });
    },
    asset(path) {
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
  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || msg.source !== ${hostSource}) return;
    if (msg.type === "theme") {
      window.shellcn.theme = msg.theme;
      for (const fn of themeListeners) fn(msg.theme);
      return;
    }
    if (msg.type === "route.response" || msg.type === "asset.response") {
      const req = pending.get(msg.id);
      if (!req) return;
      pending.delete(msg.id);
      msg.ok ? req.resolve(msg.data) : req.reject(new Error(msg.error || "Bridge request failed"));
      return;
    }
    if (msg.type === "stream.message") streams.get(msg.id)?._emit(msg.data);
    if (msg.type === "stream.error") streams.get(msg.id)?._emit({ error: msg.error });
    if (msg.type === "stream.close") streams.delete(msg.id);
  });
})();`;
}

function startScript(config: WasmPanelConfig): string {
  const runtime = config.runtime ?? "generic";
  const entry = JSON.stringify(config.entry);
  return `
(async () => {
  const status = document.getElementById("shellcn-wasm-status");
  try {
    if (typeof WebAssembly !== "object") throw new Error("WebAssembly is not supported in this browser.");
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
    if (status) status.textContent = err instanceof Error ? err.message : String(err);
  }
})();`;
}

function escapeScript(value: string): string {
  return value.replaceAll("<" + "/script", "<\\/" + "script");
}
</script>

<template>
  <PanelError v-if="error" :message="error" />
  <div
    v-else
    class="relative h-full min-h-0 bg-surface-950"
    :class="viewportClass"
  >
    <PanelLoader v-if="loading" class="absolute inset-0 bg-surface-950" />
    <iframe
      v-if="srcdoc"
      ref="iframeEl"
      title="WASM panel"
      :aria-label="cfg?.ariaLabel || 'WebAssembly panel'"
      :sandbox="sandbox"
      :srcdoc="srcdoc"
      :style="iframeStyle"
      :allow="iframeAllow || undefined"
      class="block border-0 bg-surface-950"
      @load="postTheme"
    />
    <p class="sr-only">
      {{
        cfg?.instructions ||
        "This panel runs a sandboxed WebAssembly app declared by the active plugin."
      }}
    </p>
  </div>
</template>
