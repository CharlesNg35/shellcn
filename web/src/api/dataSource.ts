import { getActivePinia } from "pinia";
import {
  API_BASE,
  apiErrorFromResponse,
  apiFetch,
  ApiError,
  getCsrfToken,
  reportApiError,
} from "./client";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import type { SocketLike } from "../stores/streamChannels";
import type {
  DataSource,
  Page,
  PageRequest,
  ResourceEvent,
  ResourceRef,
} from "../types/projection";

// Reflect a request's outcome in the connection's live health: a success proves
// the upstream is reachable; a 5xx/network failure is a connection-level fault
// (a 4xx is an operation-level error — a missing file — and must NOT redden the
// connection). Guarded so callers without an active Pinia (unit tests) are noops.
async function track<T>(connectionId: string, run: Promise<T>): Promise<T> {
  if (!getActivePinia()) return run;
  const live = useConnectionStatusStore();
  try {
    const result = await run;
    live.connected(connectionId);
    return result;
  } catch (e) {
    if (e instanceof ApiError && (e.status === 0 || e.status >= 500)) {
      live.failed(connectionId, e.message);
    }
    throw e;
  }
}

export interface ResolveContext {
  resource?: ResourceRef | null;
}

// Tiny, typed interpolation — NOT a scripting language. Supports only
// `${resource.<field>}`; anything else (or an unresolved field) errors loudly
// so a missing param never silently produces a blank request.
const TOKEN = /\$\{([^}]+)\}/g;

export function interpolate(template: string, ctx: ResolveContext): string {
  return template.replace(TOKEN, (_, raw: string) => {
    const expr = raw.trim();
    const value = lookup(expr, ctx);
    if (value === undefined || value === "") {
      throw new Error(`Cannot resolve "\${${expr}}": no value in context`);
    }
    return value;
  });
}

function lookup(expr: string, ctx: ResolveContext): string | undefined {
  if (expr.startsWith("resource.")) {
    const key = expr.slice("resource.".length) as keyof ResourceRef;
    const v = ctx.resource?.[key];
    return typeof v === "string" ? v : undefined;
  }
  return undefined;
}

const LONE_TOKEN = /^\$\{([^}]+)\}$/;

export function resolveParams(
  params: Record<string, string> | undefined,
  ctx: ResolveContext,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, template] of Object.entries(params ?? {})) {
    // A param that is a single token is sourced entirely from one value; if that
    // value is absent the param is simply omitted, and the route handler applies
    // its own default/validation. A token embedded in a larger string must
    // resolve — a blank there would corrupt the value — so interpolate throws.
    const lone = template.match(LONE_TOKEN);
    if (lone) {
      const value = lookup(lone[1].trim(), ctx);
      if (value) out[key] = value;
      continue;
    }
    out[key] = TOKEN.test(template) ? interpolate(template, ctx) : template;
    TOKEN.lastIndex = 0;
  }
  return out;
}

export function routePath(connectionId: string, routeId: string): string {
  return `${API_BASE}/connections/${connectionId}/x/${routeId}`;
}

// Route params travel under the reserved `p.` prefix; list controls use the
// reserved top-level keys. The prefix keeps them from ever colliding.
export function queryParams(
  params: Record<string, string>,
  page?: PageRequest,
): URLSearchParams {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) sp.set(`p.${k}`, v);
  if (page) {
    if (page.cursor) sp.set("cursor", page.cursor);
    if (page.limit != null) sp.set("limit", String(page.limit));
    for (const [k, v] of Object.entries(page.filter ?? {})) {
      sp.set(k === "q" ? "filter" : `filter.${k}`, v);
    }
    const sort = page.sort?.[0];
    if (sort) sp.set("sort", `${sort.desc ? "-" : ""}${sort.field}`);
  }
  return sp;
}

function withQuery(base: string, sp: URLSearchParams): string {
  const qs = sp.toString();
  return qs ? `${base}?${qs}` : base;
}

export function routeURL(
  connectionId: string,
  routeId: string,
  ctx: ResolveContext = {},
  params: Record<string, string> = {},
): string {
  return withQuery(
    routePath(connectionId, routeId),
    queryParams(resolveParams(params, ctx)),
  );
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await apiFetch(url);
  return (await res.json()) as T;
}

export async function fetchPage<T = unknown>(
  connectionId: string,
  ds: DataSource,
  ctx: ResolveContext = {},
  page?: PageRequest,
): Promise<Page<T>> {
  const params = resolveParams(ds.params, ctx);
  const url = withQuery(
    routePath(connectionId, ds.routeId),
    queryParams(params, page),
  );
  const result = await track(connectionId, getJSON<Page<T>>(url));
  // A nil Go slice marshals to null; guarantee items is always an array so every
  // consumer can map/forEach without a guard.
  return { ...result, items: result.items ?? [] };
}

export async function fetchDoc<T = unknown>(
  connectionId: string,
  ds: DataSource,
  ctx: ResolveContext = {},
): Promise<T> {
  const params = resolveParams(ds.params, ctx);
  const url = withQuery(
    routePath(connectionId, ds.routeId),
    queryParams(params),
  );
  return track(connectionId, getJSON<T>(url));
}

export interface ActionResult {
  ok: boolean;
  [key: string]: unknown;
}

export interface UploadProgress {
  loaded: number;
  total: number;
  percent: number;
  indeterminate: boolean;
}

export interface UploadFilesOptions {
  onProgress?: (progress: UploadProgress) => void;
}

export async function runAction(
  connectionId: string,
  routeId: string,
  ctx: ResolveContext = {},
  body?: unknown,
  params: Record<string, string> = {},
  method = "POST",
): Promise<ActionResult> {
  const url = withQuery(
    routePath(connectionId, routeId),
    queryParams(resolveParams(params, ctx)),
  );
  return track(
    connectionId,
    apiFetch(url, {
      method,
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body ?? {}),
    }).then((res) => res.json() as Promise<ActionResult>),
  );
}

function isFile(value: unknown): value is File {
  return typeof File !== "undefined" && value instanceof File;
}

function bodyHasFile(body: unknown): boolean {
  if (isFile(body)) return true;
  if (Array.isArray(body)) return body.some(bodyHasFile);
  if (!body || typeof body !== "object") return false;
  return Object.values(body).some(bodyHasFile);
}

function appendFormValue(form: FormData, key: string, value: unknown): void {
  if (value === undefined || value === null) return;
  if (isFile(value)) {
    form.append(key, value, value.name);
    return;
  }
  if (Array.isArray(value)) {
    for (const item of value) appendFormValue(form, key, item);
    return;
  }
  if (typeof value === "object") {
    form.append(key, JSON.stringify(value));
    return;
  }
  form.append(key, String(value));
}

export async function runFormAction(
  connectionId: string,
  routeId: string,
  ctx: ResolveContext = {},
  body: Record<string, unknown> = {},
  params: Record<string, string> = {},
  method = "POST",
): Promise<ActionResult> {
  if (!bodyHasFile(body)) {
    return runAction(connectionId, routeId, ctx, body, params, method);
  }
  const form = new FormData();
  for (const [key, value] of Object.entries(body)) {
    appendFormValue(form, key, value);
  }
  const res = await apiFetch(routeURL(connectionId, routeId, ctx, params), {
    method,
    body: form,
  });
  return (await res.json()) as ActionResult;
}

export async function uploadFiles(
  connectionId: string,
  routeId: string,
  ctx: ResolveContext = {},
  files: File[],
  params: Record<string, string> = {},
  fieldName = "files",
  options: UploadFilesOptions = {},
): Promise<ActionResult> {
  const body = new FormData();
  for (const file of files) body.append(fieldName, file, file.name);
  const url = routeURL(connectionId, routeId, ctx, params);
  if (options.onProgress) {
    return track(connectionId, uploadForm(url, body, options.onProgress));
  }
  const res = await apiFetch(url, { method: "POST", body });
  return (await res.json()) as ActionResult;
}

function progress(
  loaded: number,
  total: number,
  indeterminate = total <= 0,
): UploadProgress {
  return {
    loaded,
    total,
    indeterminate,
    percent: indeterminate
      ? 0
      : Math.min(100, Math.round((loaded / total) * 100)),
  };
}

function parseActionResult(body: string): ActionResult {
  if (!body) return { ok: true };
  return JSON.parse(body) as ActionResult;
}

function uploadForm(
  url: string,
  body: FormData,
  onProgress: (progress: UploadProgress) => void,
): Promise<ActionResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    let lastTotal = 0;
    xhr.open("POST", url, true);
    const csrf = getCsrfToken();
    if (csrf) xhr.setRequestHeader("X-CSRF-Token", csrf);
    xhr.upload.onprogress = (event) => {
      lastTotal = event.lengthComputable ? event.total : 0;
      onProgress(
        progress(
          event.loaded,
          event.lengthComputable ? event.total : 0,
          !event.lengthComputable,
        ),
      );
    };
    xhr.onload = () => {
      const authRequired =
        xhr.getResponseHeader("X-ShellCN-Auth") === "required";
      if (xhr.status < 200 || xhr.status >= 300) {
        const err = apiErrorFromResponse(
          xhr.status,
          xhr.statusText,
          xhr.responseText,
          authRequired,
        );
        reportApiError(err);
        reject(err);
        return;
      }
      onProgress(
        lastTotal > 0 ? progress(lastTotal, lastTotal) : progress(1, 1),
      );
      try {
        resolve(parseActionResult(xhr.responseText));
      } catch {
        reject(new ApiError(xhr.status, "Invalid upload response"));
      }
    };
    xhr.onerror = () => {
      const err = new ApiError(0, "Network error — is the gateway reachable?");
      reportApiError(err);
      reject(err);
    };
    xhr.send(body);
  });
}

async function requestTicket(
  connectionId: string,
  routeId: string,
  params: Record<string, string>,
): Promise<string> {
  const res = await apiFetch(
    `${API_BASE}/connections/${connectionId}/tickets`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ routeId, params }),
    },
  );
  const { ticket } = (await res.json()) as { ticket: string };
  return ticket;
}

function streamUrl(
  connectionId: string,
  routeId: string,
  params: Record<string, string>,
  ticket: string,
): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const sp = queryParams(params);
  sp.set("ticket", ticket);
  return `${proto}//${location.host}${routePath(connectionId, routeId)}?${sp.toString()}`;
}

export interface StreamHandle {
  key: string;
  url: string;
}

// Stable channel identity for a stream — computed WITHOUT minting a ticket, so a
// caller can check for an already-open channel before requesting a new one.
export function channelKey(
  connectionId: string,
  ds: DataSource,
  ctx: ResolveContext = {},
): string {
  const params = resolveParams(ds.params, ctx);
  return `${connectionId}:${ds.routeId}:${new URLSearchParams(params).toString()}`;
}

// Resolves params, mints a single-use ticket, and returns the wss URL + a stable
// channel key. The caller hands the URL to the sessions store, which owns the
// socket lifecycle (so streams survive component remounts).
export async function prepareStream(
  connectionId: string,
  ds: DataSource,
  ctx: ResolveContext = {},
): Promise<StreamHandle> {
  const params = resolveParams(ds.params, ctx);
  const ticket = await requestTicket(connectionId, ds.routeId, params);
  const key = channelKey(connectionId, ds, ctx);
  return { key, url: streamUrl(connectionId, ds.routeId, params, ticket) };
}

export interface WatchOptions {
  socketFactory?: (url: string) => SocketLike;
  reconnectMs?: number;
}

// Background list sync: opens a watch socket and re-emits ResourceEvents, with
// automatic reconnect. Returns a stop function.
export function watch(
  connectionId: string,
  ds: DataSource,
  ctx: ResolveContext,
  onEvent: (ev: ResourceEvent) => void,
  opts: WatchOptions = {},
): () => void {
  const factory =
    opts.socketFactory ?? ((url: string) => new WebSocket(url) as SocketLike);
  const reconnectMs = opts.reconnectMs ?? 2000;
  let stopped = false;
  let socket: SocketLike | null = null;
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function connect(): Promise<void> {
    if (stopped) return;
    timer = undefined;
    try {
      const { url } = await prepareStream(connectionId, ds, ctx);
      if (stopped) return;
      socket = factory(url);
      socket.addEventListener("message", (ev) => {
        try {
          onEvent(JSON.parse((ev as { data: string }).data) as ResourceEvent);
        } catch {
          // ignore malformed frame
        }
      });
      socket.addEventListener("close", scheduleReconnect);
      socket.addEventListener("error", scheduleReconnect);
    } catch {
      scheduleReconnect();
    }
  }

  function scheduleReconnect(): void {
    if (stopped || timer) return;
    timer = setTimeout(connect, reconnectMs);
  }

  void connect();

  return () => {
    stopped = true;
    if (timer) clearTimeout(timer);
    socket?.close();
  };
}
