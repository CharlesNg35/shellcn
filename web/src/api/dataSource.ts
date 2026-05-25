import { API_BASE, ApiError } from "./client";
import type { SocketLike } from "../stores/sessions";
import type {
  DataSource,
  Page,
  PageRequest,
  ResourceEvent,
  ResourceRef,
} from "../types/projection";

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

export function resolveParams(
  params: Record<string, string> | undefined,
  ctx: ResolveContext,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, template] of Object.entries(params ?? {})) {
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

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new ApiError(res.status, res.statusText);
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
  return getJSON<Page<T>>(url);
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
  return getJSON<T>(url);
}

export interface ActionResult {
  ok: boolean;
  [key: string]: unknown;
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
  const res = await fetch(url, {
    method,
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body ?? {}),
  });
  if (!res.ok) throw new ApiError(res.status, res.statusText);
  return (await res.json()) as ActionResult;
}

async function requestTicket(
  connectionId: string,
  routeId: string,
  params: Record<string, string>,
): Promise<string> {
  const res = await fetch(`${API_BASE}/connections/${connectionId}/tickets`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ routeId, params }),
  });
  if (!res.ok) throw new ApiError(res.status, res.statusText);
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
    if (stopped) return;
    timer = setTimeout(connect, reconnectMs);
  }

  void connect();

  return () => {
    stopped = true;
    if (timer) clearTimeout(timer);
    socket?.close();
  };
}
