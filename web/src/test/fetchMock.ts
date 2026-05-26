import { vi } from "vitest";

export interface MockResult {
  status?: number;
  headers?: HeadersInit;
  body: unknown;
}

export type MockHandler = (url: string, init?: RequestInit) => MockResult;

export function installFetch(handler: MockHandler) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const { status = 200, headers, body } = handler(url, init);
    return new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json", ...headers },
    });
  });
  vi.stubGlobal("fetch", fn);
  return fn;
}
