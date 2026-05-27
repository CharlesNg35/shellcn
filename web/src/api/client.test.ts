import { describe, it, expect, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import {
  api,
  apiFetch,
  ApiError,
  setCsrfToken,
  setApiErrorHandler,
} from "./client";

function headers(init?: RequestInit): Headers {
  return new Headers(init?.headers);
}

afterEach(() => {
  vi.unstubAllGlobals();
  setCsrfToken("");
  setApiErrorHandler(null);
});

describe("api client", () => {
  it("attaches the CSRF token to mutations but not to GETs", async () => {
    setCsrfToken("tok-123");
    const fetchFn = installFetch(() => ({ body: { ok: true } }));

    await api.get("/connections");
    await api.post("/connections", { name: "x" });

    const [, getInit] = fetchFn.mock.calls[0];
    const [, postInit] = fetchFn.mock.calls[1];
    expect(headers(getInit).get("X-CSRF-Token")).toBeNull();
    expect(headers(postInit).get("X-CSRF-Token")).toBe("tok-123");
  });

  it("strips the body from GET/HEAD so fetch doesn't reject the request", async () => {
    const fetchFn = installFetch(() => ({ body: { ok: true } }));

    await apiFetch("/x", { method: "GET", body: JSON.stringify({ a: 1 }) });

    const [, init] = fetchFn.mock.calls[0];
    expect(init?.body).toBeUndefined();
  });

  it("invokes the error interceptor and rejects with an ApiError", async () => {
    installFetch(() => ({ status: 403, body: { error: "forbidden" } }));
    const seen: number[] = [];
    setApiErrorHandler((err) => seen.push(err.status));

    await expect(api.del("/connections/1")).rejects.toBeInstanceOf(ApiError);
    expect(seen).toEqual([403]);
  });

  it("marks only platform-auth 401 responses as auth-required", async () => {
    installFetch(() => ({
      status: 401,
      headers: { "X-ShellCN-Auth": "required" },
      body: { error: "unauthorized" },
    }));

    await expect(api.get("/auth/me")).rejects.toMatchObject({
      status: 401,
      authRequired: true,
    });
  });

  it("does not mark plugin route 401 responses as auth-required", async () => {
    installFetch(() => ({
      status: 401,
      body: { error: "unauthorized: ssh handshake failed" },
    }));

    await expect(
      api.get("/connections/c/x/ssh.sftp.list"),
    ).rejects.toMatchObject({
      status: 401,
      authRequired: false,
    });
  });

  it("surfaces a network failure as status 0", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => {
        throw new TypeError("offline");
      }),
    );
    const seen: number[] = [];
    setApiErrorHandler((err) => seen.push(err.status));
    await expect(api.get("/connections")).rejects.toMatchObject({ status: 0 });
    expect(seen).toEqual([0]);
  });
});
