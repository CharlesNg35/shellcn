import { describe, it, expect, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import { api, ApiError, setCsrfToken, setApiErrorHandler } from "./client";

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

  it("invokes the error interceptor and rejects with an ApiError", async () => {
    installFetch(() => ({ status: 403, body: { error: "forbidden" } }));
    const seen: number[] = [];
    setApiErrorHandler((err) => seen.push(err.status));

    await expect(api.del("/connections/1")).rejects.toBeInstanceOf(ApiError);
    expect(seen).toEqual([403]);
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
