import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { installFetch } from "../test/fetchMock";
import { useConnectionSessionsStore } from "./connectionSessions";
import {
  CONNECTION_SESSION_HEARTBEAT_MS,
  MAX_LIVE_CONNECTION_SESSIONS,
} from "./sessionLimits";
import { useWorkspaceStore } from "./workspace";

let requests: Array<{ url: string; init?: RequestInit }>;

beforeEach(() => {
  vi.useFakeTimers();
  setActivePinia(createPinia());
  requests = [];
  installFetch((url, init) => {
    requests.push({ url, init });
    return {
      body: {
        state: "connected",
        channels: 0,
        streams: 0,
        lastSeen: "2026-05-28T00:00:00Z",
      },
    };
  });
});

afterEach(() => {
  useConnectionSessionsStore().stop();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("connection sessions store", () => {
  it("keeps all connected backend sessions alive from one lifecycle owner", async () => {
    const store = useConnectionSessionsStore();
    const ws = useWorkspaceStore();
    store.start();

    expect(await store.connect("c1")).toBe(true);
    expect(await store.connect("c2")).toBe(true);
    expect(ws.isConnected("c1")).toBe(true);
    expect(ws.isConnected("c2")).toBe(true);

    await vi.advanceTimersByTimeAsync(CONNECTION_SESSION_HEARTBEAT_MS);

    const posts = requests.filter((request) => request.init?.method === "POST");
    expect(posts.map((request) => request.url)).toEqual([
      "/api/connections/c1/session",
      "/api/connections/c2/session",
      "/api/connections/c1/session",
      "/api/connections/c2/session",
    ]);
  });

  it("disconnects one connection without clearing the others", async () => {
    const store = useConnectionSessionsStore();
    const ws = useWorkspaceStore();
    store.start();

    await store.connect("c1");
    await store.connect("c2");
    await store.disconnect("c1");

    expect(ws.isConnected("c1")).toBe(false);
    expect(ws.isConnected("c2")).toBe(true);
    expect(
      requests.some(
        (request) =>
          request.url === "/api/connections/c1/session" &&
          request.init?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("caps frontend live connection sessions and closes the oldest extras", async () => {
    const store = useConnectionSessionsStore();
    const ws = useWorkspaceStore();
    store.start();

    for (let i = 0; i < MAX_LIVE_CONNECTION_SESSIONS + 2; i += 1) {
      expect(await store.connect(`c${i}`)).toBe(true);
    }

    expect(store.connectedIds()).toHaveLength(MAX_LIVE_CONNECTION_SESSIONS);
    expect(ws.isConnected("c0")).toBe(false);
    expect(ws.isConnected("c1")).toBe(false);
    expect(ws.isConnected(`c${MAX_LIVE_CONNECTION_SESSIONS + 1}`)).toBe(true);
    expect(
      requests
        .filter((request) => request.init?.method === "DELETE")
        .map((request) => request.url),
    ).toEqual(["/api/connections/c0/session", "/api/connections/c1/session"]);
  });

  it("asks before unloading and best-effort closes live sessions on pagehide", async () => {
    const store = useConnectionSessionsStore();
    store.start();
    await store.connect("c1");
    await store.connect("c2");

    const beforeUnload = new Event("beforeunload", {
      cancelable: true,
    }) as BeforeUnloadEvent;
    window.dispatchEvent(beforeUnload);
    expect(beforeUnload.defaultPrevented).toBe(true);

    window.dispatchEvent(new Event("pagehide"));
    await Promise.resolve();

    const deletes = requests.filter(
      (request) => request.init?.method === "DELETE",
    );
    expect(deletes.map((request) => request.url)).toEqual([
      "/api/connections/c1/session",
      "/api/connections/c2/session",
    ]);
    expect(deletes.every((request) => request.init?.keepalive)).toBe(true);
  });
});
