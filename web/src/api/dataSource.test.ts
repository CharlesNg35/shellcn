import { describe, it, expect, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import type { SocketLike } from "../stores/sessions";
import {
  fetchPage,
  interpolate,
  queryParams,
  resolveParams,
  routePath,
  watch,
} from "./dataSource";

class FakeSocket implements SocketLike {
  closed = false;
  readonly url: string;
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};
  constructor(url: string) {
    this.url = url;
  }
  send(): void {}
  close(): void {
    this.closed = true;
  }
  addEventListener(type: string, fn: (ev: unknown) => void): void {
    (this.handlers[type] ??= []).push(fn);
  }
  emit(type: string, ev?: unknown): void {
    for (const fn of this.handlers[type] ?? []) fn(ev);
  }
}

function waitFor(cond: () => boolean, timeout = 500): Promise<void> {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const tick = () => {
      if (cond()) return resolve();
      if (Date.now() - start > timeout)
        return reject(new Error("waitFor timed out"));
      setTimeout(tick, 5);
    };
    tick();
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("dataSource resolver", () => {
  it("interpolates ${resource.*} and passes statics through", () => {
    const ctx = {
      resource: { kind: "vm", namespace: "pve1", name: "web", uid: "101" },
    };
    expect(interpolate("${resource.uid}", ctx)).toBe("101");
    const params = resolveParams(
      {
        vmid: "${resource.uid}",
        node: "${resource.namespace}",
        view: "summary",
      },
      ctx,
    );
    expect(params).toEqual({ vmid: "101", node: "pve1", view: "summary" });
  });

  it("errors cleanly on an unresolved param (no silent blank)", () => {
    expect(() => interpolate("${resource.uid}", {})).toThrow(/Cannot resolve/);
    expect(() => resolveParams({ id: "${resource.uid}" }, {})).toThrow();
  });

  it("builds the p.-prefixed scheme without colliding with list controls", () => {
    const sp = queryParams(
      { vmid: "101", cursor: "shadow" },
      {
        cursor: "abc",
        limit: 50,
        filter: { q: "web" },
        sort: [{ field: "name", desc: true }],
      },
    );
    expect(sp.get("p.vmid")).toBe("101");
    expect(sp.get("p.cursor")).toBe("shadow"); // a param named cursor stays under p.
    expect(sp.get("cursor")).toBe("abc"); // the list control is separate
    expect(sp.get("limit")).toBe("50");
    expect(sp.get("filter")).toBe("web");
    expect(sp.get("sort")).toBe("-name");
  });

  it("resolves a RouteID to the connection-scoped path", () => {
    expect(routePath("conn-1", "docker.container.list")).toBe(
      "/api/connections/conn-1/x/docker.container.list",
    );
  });

  it("follows NextCursor across pages", async () => {
    const requested: string[] = [];
    installFetch((url) => {
      requested.push(url);
      const cursor = new URL(url, "http://h").searchParams.get("cursor");
      if (!cursor)
        return { body: { items: [1, 2], nextCursor: "c2", total: 4 } };
      return { body: { items: [3, 4], nextCursor: "", total: 4 } };
    });

    const all: number[] = [];
    let cursor = "";
    do {
      const page = await fetchPage<number>(
        "conn",
        { routeId: "docker.container.list" },
        {},
        { cursor, limit: 2 },
      );
      all.push(...page.items);
      cursor = page.nextCursor;
    } while (cursor);

    expect(all).toEqual([1, 2, 3, 4]);
    expect(requested).toHaveLength(2);
  });

  it("watch reconnects after the socket closes", async () => {
    installFetch(() => ({ body: { ticket: "t1" } }));
    const sockets: FakeSocket[] = [];
    const events: unknown[] = [];
    const stop = watch(
      "conn",
      { routeId: "docker.container.watch" },
      {},
      (ev) => events.push(ev),
      {
        socketFactory: (url) => {
          const s = new FakeSocket(url);
          sockets.push(s);
          return s;
        },
        reconnectMs: 10,
      },
    );

    await waitFor(() => sockets.length === 1);
    expect(sockets[0].url).toContain("ticket=t1");

    sockets[0].emit("message", {
      data: JSON.stringify({
        type: "updated",
        ref: { kind: "c", name: "x", uid: "1" },
      }),
    });
    expect(events).toHaveLength(1);

    sockets[0].emit("close");
    await waitFor(() => sockets.length === 2);

    stop();
    expect(sockets[1].closed).toBe(true);
  });
});
