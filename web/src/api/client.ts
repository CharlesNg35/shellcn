export const API_BASE = "/api";

export class ApiError extends Error {
  readonly status: number;
  readonly authRequired: boolean;

  constructor(status: number, message: string, authRequired = false) {
    super(message);
    this.status = status;
    this.authRequired = authRequired;
    this.name = "ApiError";
  }
}

let csrfToken = "";
export function setCsrfToken(token: string): void {
  csrfToken = token;
}
export function getCsrfToken(): string {
  return csrfToken;
}

export type ApiErrorHandler = (err: ApiError) => void;
let errorHandler: ApiErrorHandler | null = null;
export function setApiErrorHandler(fn: ApiErrorHandler | null): void {
  errorHandler = fn;
}

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

export function reportApiError(err: ApiError): void {
  errorHandler?.(err);
}

export function apiErrorFromResponse(
  status: number,
  statusText: string,
  body: string,
  authRequired = false,
): ApiError {
  let message = statusText;
  try {
    const parsed = JSON.parse(body) as { error?: string };
    if (parsed.error) message = parsed.error;
  } catch {
    if (body) message = body;
  }
  return new ApiError(status, message, authRequired);
}

async function responseError(res: Response): Promise<ApiError> {
  return apiErrorFromResponse(
    res.status,
    res.statusText,
    await res.text(),
    res.headers.get("X-ShellCN-Auth") === "required",
  );
}

export async function apiFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers = new Headers(init.headers);
  if (MUTATING.has(method) && csrfToken && !headers.has("X-CSRF-Token")) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  // fetch rejects a GET/HEAD request that carries a body; strip it so callers
  // that pass one (e.g. an action invoked over GET) don't fail as a network error.
  const body = method === "GET" || method === "HEAD" ? undefined : init.body;

  let res: Response;
  try {
    res = await fetch(input, { ...init, method, headers, body });
  } catch {
    const err = new ApiError(0, "Network error. Is the gateway reachable?");
    reportApiError(err);
    throw err;
  }

  if (!res.ok) {
    const err = await responseError(res);
    reportApiError(err);
    throw err;
  }
  return res;
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (MUTATING.has(method) && csrfToken) headers["X-CSRF-Token"] = csrfToken;

  const res = await apiFetch(API_BASE + path, {
    method,
    headers: Object.keys(headers).length ? headers : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  del: <T>(path: string, body?: unknown) => request<T>("DELETE", path, body),
};
