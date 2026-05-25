import { describe, it, expect, beforeEach } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { useSessionsStore, type SocketLike } from "./sessions";

class FakeSocket implements SocketLike {
  sent: string[] = [];
  closed = false;
  private handlers: Record<string, ((ev: unknown) => void)[]> = {};

  send(data: string): void {
    this.sent.push(data);
  }
  close(): void {
    this.closed = true;
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

describe("sessions store", () => {
  it("opens a channel once and reuses it", () => {
    const store = useSessionsStore();
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
    const store = useSessionsStore();
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
    const store = useSessionsStore();
    const socket = new FakeSocket();
    store.ensure("k", () => socket);

    socket.emit("error");
    expect(store.status("k")).toBe("error");

    store.send("k", "ping");
    expect(socket.sent).toEqual(["ping"]);

    store.close("k");
    expect(socket.closed).toBe(true);
    expect(store.status("k")).toBeUndefined();
  });

  it("closeWhere tears down matching channels only", () => {
    const store = useSessionsStore();
    const a = new FakeSocket();
    const b = new FakeSocket();
    store.ensure("conn1:logs", () => a);
    store.ensure("conn2:logs", () => b);
    store.closeWhere((key) => key.startsWith("conn1:"));
    expect(a.closed).toBe(true);
    expect(b.closed).toBe(false);
  });
});
