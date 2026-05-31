import { describe, it, expect, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import type { SocketLike } from "../stores/streamChannels";
import {
  fetchPage,
  interpolate,
  queryParams,
  resolveParams,
  runFormAction,
  uploadFiles,
  routePath,
  watch,
} from "./dataSource";
import { setCsrfToken } from "./client";

class FakeSocket implements SocketLike {
  closed = false;
  readonly url: string;
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};
  constructor(url: string) {
    this.url = url;
  }
  readyState: number = 1; // OPEN
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
  vi.useRealTimers();
  vi.unstubAllGlobals();
  setCsrfToken("");
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

  it("errors on a blank inside a composed string (no silent corruption)", () => {
    expect(() => interpolate("${resource.uid}", {})).toThrow(/Cannot resolve/);
    // A token embedded in surrounding text must resolve — a blank would corrupt it.
    expect(() => resolveParams({ p: "x-${resource.uid}" }, {})).toThrow();
  });

  it("omits a single-token param whose value is absent (no blank request)", () => {
    // A namespaced ref keeps the param; a ref without that field drops it — the
    // resolver special-cases no field name, only the single-token structure.
    expect(
      resolveParams(
        { namespace: "${resource.namespace}", name: "${resource.name}" },
        {
          resource: { kind: "table", namespace: "public", name: "t", uid: "1" },
        },
      ),
    ).toEqual({ namespace: "public", name: "t" });
    expect(
      resolveParams(
        { namespace: "${resource.namespace}", name: "${resource.name}" },
        { resource: { kind: "ns", name: "default", uid: "1" } },
      ),
    ).toEqual({ name: "default" });
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

  it("posts file uploads as multipart without forcing a JSON content type", async () => {
    const fetchMock = installFetch(() => ({ body: { ok: true } }));
    setCsrfToken("tok-uploads");
    const file = new File(["hello"], "hello.txt", { type: "text/plain" });

    await uploadFiles(
      "conn",
      "ssh.sftp.upload",
      {},
      [file],
      { path: "/" },
      "files",
    );

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain("/api/connections/conn/x/ssh.sftp.upload?p.path=%2F");
    expect(init?.method).toBe("POST");
    const headers = new Headers(init?.headers);
    expect(headers.get("Content-Type")).toBeNull();
    expect(headers.get("X-CSRF-Token")).toBe("tok-uploads");
    expect(init?.body).toBeInstanceOf(FormData);
    expect((init?.body as FormData).getAll("files")).toEqual([file]);
  });

  it("reports custom upload progress and keeps CSRF headers", async () => {
    class FakeXHR {
      static latest: FakeXHR | null = null;
      readonly upload: {
        onprogress?: (event: ProgressEvent) => void;
      } = {};
      method = "";
      url = "";
      body: BodyInit | null = null;
      status = 200;
      statusText = "OK";
      responseText = '{"ok":true}';
      onload?: () => void;
      onerror?: () => void;
      headers: Record<string, string> = {};

      constructor() {
        FakeXHR.latest = this;
      }

      open(method: string, url: string): void {
        this.method = method;
        this.url = url;
      }

      setRequestHeader(name: string, value: string): void {
        this.headers[name] = value;
      }

      getResponseHeader(): string | null {
        return null;
      }

      send(body: BodyInit): void {
        this.body = body;
        this.upload.onprogress?.({
          loaded: 5,
          total: 10,
          lengthComputable: true,
        } as ProgressEvent);
        this.onload?.();
      }
    }

    vi.stubGlobal("XMLHttpRequest", FakeXHR);
    setCsrfToken("tok-progress");
    const progress: number[] = [];
    const file = new File(["hello"], "hello.txt", { type: "text/plain" });

    const result = await uploadFiles(
      "conn",
      "ssh.sftp.upload",
      {},
      [file],
      { path: "/" },
      "files",
      { onProgress: (p) => progress.push(p.percent) },
    );

    expect(result).toEqual({ ok: true });
    expect(progress).toEqual([50, 100]);
    expect(FakeXHR.latest?.method).toBe("POST");
    expect(FakeXHR.latest?.url).toContain(
      "/api/connections/conn/x/ssh.sftp.upload?p.path=%2F",
    );
    expect(FakeXHR.latest?.headers["X-CSRF-Token"]).toBe("tok-progress");
    expect(FakeXHR.latest?.body).toBeInstanceOf(FormData);
  });

  it("promotes action bodies with files to multipart form data", async () => {
    const fetchMock = installFetch(() => ({ body: { ok: true } }));
    setCsrfToken("tok-form");
    const file = new File(["cert"], "client.pem");

    await runFormAction(
      "conn",
      "cert.upload",
      {},
      { name: "client", cert: [file], metadata: { rotate: true } },
      { id: "c1" },
      "PATCH",
    );

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toContain("/api/connections/conn/x/cert.upload?p.id=c1");
    expect(init?.method).toBe("PATCH");
    const headers = new Headers(init?.headers);
    expect(headers.get("Content-Type")).toBeNull();
    expect(headers.get("X-CSRF-Token")).toBe("tok-form");
    const body = init?.body as FormData;
    expect(body.get("name")).toBe("client");
    expect(body.getAll("cert")).toEqual([file]);
    expect(body.get("metadata")).toBe('{"rotate":true}');
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

  it("deduplicates reconnect timers for repeated close/error events", async () => {
    vi.useFakeTimers();
    installFetch(() => ({ body: { ticket: "t1" } }));
    const sockets: FakeSocket[] = [];
    const stop = watch(
      "conn",
      { routeId: "docker.container.watch" },
      {},
      () => {},
      {
        socketFactory: (url) => {
          const s = new FakeSocket(url);
          sockets.push(s);
          return s;
        },
        reconnectMs: 10,
      },
    );

    await vi.waitFor(() => expect(sockets).toHaveLength(1));
    sockets[0].emit("close");
    sockets[0].emit("error");
    await vi.advanceTimersByTimeAsync(10);
    await vi.waitFor(() => expect(sockets).toHaveLength(2));

    stop();
    vi.useRealTimers();
  });
});
