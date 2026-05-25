// Same-origin API base. The mock↔real swap happens in the Vite layer
// (mock plugin vs. proxy), so panels never branch on it.
export const API_BASE = "/api";

export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
    this.name = "ApiError";
  }
}

// Attached to every state-changing request; set/cleared by the auth store.
let csrfToken = "";
export function setCsrfToken(token: string): void {
  csrfToken = token;
}

// One app-installed hook that runs on every API error (401→re-login, etc.).
export type ApiErrorHandler = (err: ApiError) => void;
let errorHandler: ApiErrorHandler | null = null;
export function setApiErrorHandler(fn: ApiErrorHandler | null): void {
  errorHandler = fn;
}

const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (MUTATING.has(method) && csrfToken) headers["X-CSRF-Token"] = csrfToken;

  let res: Response;
  try {
    res = await fetch(API_BASE + path, {
      method,
      headers: Object.keys(headers).length ? headers : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    const err = new ApiError(0, "Network error — is the gateway reachable?");
    errorHandler?.(err);
    throw err;
  }

  if (!res.ok) {
    let message = res.statusText;
    try {
      const parsed = (await res.json()) as { error?: string };
      if (parsed.error) message = parsed.error;
    } catch {
      // body was not JSON; keep statusText
    }
    const err = new ApiError(res.status, message);
    errorHandler?.(err);
    throw err;
  }
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
