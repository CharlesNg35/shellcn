import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useStreamChannelsStore, type SocketLike } from "./streamChannels";
import { useConnectionStatusStore } from "./connectionStatus";

class FakeSocket implements SocketLike {
  sent: string[] = [];
  closed = false;
  readyState = 1; // OPEN
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};

  send(data: string): void {
    this.sent.push(data);
  }
  close(): void {
    this.closed = true;
    this.readyState = 3; // CLOSED
  }
  addEventListener(type: string, listener: (ev: unknown) => void): void {
    (this.handlers[type] ??= []).push(listener);
  }
  emit(type: string, ev?: unknown): void {
    for (const fn of this.handlers[type] ?? []) fn(ev);
  }
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe("stream channels store", () => {
  it("opens a channel once and reuses it", () => {
    const store = useStreamChannelsStore();
    let created = 0;
    const factory = () => {
      created++;
      return new FakeSocket();
    };
    store.ensure("k", factory);
    store.ensure("k", factory);
    expect(created).toBe(1);
  });

  it("delivers frames to subscribers and buffers them", () => {
    const store = useStreamChannelsStore();
    const socket = new FakeSocket();
    store.ensure("k", () => socket);
    const received: string[] = [];
    const unsub = store.subscribe("k", (d) => received.push(d));

    socket.emit("open");
    socket.emit("message", { data: "line-1" });
    socket.emit("message", { data: "line-2" });

    expect(store.status("k")).toBe("open");
    expect(received).toEqual(["line-1", "line-2"]);
    expect(store.buffer("k")).toEqual(["line-1", "line-2"]);

    unsub();
    socket.emit("message", { data: "line-3" });
    expect(received).toEqual(["line-1", "line-2"]); // detached
    expect(store.buffer("k")).toContain("line-3"); // channel still alive
  });

  it("reflects error/close status and closes cleanly", () => {
    const store = useStreamChannelsStore();
    const live = useConnectionStatusStore();
    const socket = new FakeSocket();
    live.connected("conn");
    store.ensure("conn:k", () => socket);

    socket.emit("error");
    expect(store.status("conn:k")).toBe("error");
    expect(live.get("conn")?.state).toBe("connected");

    store.send("conn:k", "ping");
    expect(socket.sent).toEqual(["ping"]);

    store.close("conn:k");
    expect(socket.closed).toBe(true);
    expect(store.status("conn:k")).toBeUndefined();
  });

  it("drops sends on a socket that is not open", () => {
    const store = useStreamChannelsStore();
    const socket = new FakeSocket();
    socket.readyState = 0; // CONNECTING
    store.ensure("conn:k", () => socket);
    store.send("conn:k", "early");
    expect(socket.sent).toEqual([]);

    socket.readyState = 1; // OPEN
    store.send("conn:k", "now");
    expect(socket.sent).toEqual(["now"]);
  });

  it("sends once a connecting socket opens", async () => {
    const store = useStreamChannelsStore();
    const socket = new FakeSocket();
    socket.readyState = 0; // CONNECTING
    store.ensure("conn:k", () => socket);

    const sent = store.sendWhenOpen("conn:k", "ready", {
      attempts: 5,
      intervalMs: 1,
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
    socket.readyState = 1;
    socket.emit("open");

    await expect(sent).resolves.toBe(true);
    expect(socket.sent).toEqual(["ready"]);
  });

  it("does not wait after a channel closes", async () => {
    const store = useStreamChannelsStore();
    const socket = new FakeSocket();
    socket.readyState = 0; // CONNECTING
    store.ensure("conn:k", () => socket);
    socket.emit("close");

    await expect(
      store.sendWhenOpen("conn:k", "late", { attempts: 5, intervalMs: 1 }),
    ).resolves.toBe(false);
    expect(socket.sent).toEqual([]);
  });

  it("closeWhere tears down matching channels only", () => {
    const store = useStreamChannelsStore();
    const a = new FakeSocket();
    const b = new FakeSocket();
    store.ensure("conn1:logs", () => a);
    store.ensure("conn2:logs", () => b);
    store.closeWhere((key) => key.startsWith("conn1:"));
    expect(a.closed).toBe(true);
    expect(b.closed).toBe(false);
  });

  it("closes all channels for a connection", () => {
    const store = useStreamChannelsStore();
    const redis0 = new FakeSocket();
    const clusterShell = new FakeSocket();
    const redisInfo = new FakeSocket();
    const other = new FakeSocket();

    store.ensure("conn:redis.terminal:database=0", () => redis0);
    store.ensure("conn:kubernetes.cluster.shell:", () => clusterShell);
    store.ensure("conn:redis.info:", () => redisInfo);
    store.ensure("other:redis.terminal:database=0", () => other);

    store.closeForConnection("conn");

    expect(redis0.closed).toBe(true);
    expect(clusterShell.closed).toBe(true);
    expect(redisInfo.closed).toBe(true);
    expect(other.closed).toBe(false);
  });
});
