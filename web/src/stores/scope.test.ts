import { beforeEach, describe, expect, it } from "vitest";
import { createPinia, setActivePinia } from "pinia";
import { useScopeStore } from "./scope";
import { useStreamChannelsStore, type SocketLike } from "./streamChannels";

class FakeSocket implements SocketLike {
  closed = false;
  readyState = 1;
  send(): void {}
  close(): void {
    this.closed = true;
  }
  addEventListener(): void {}
}

beforeEach(() => {
  setActivePinia(createPinia());
});

describe("scope store", () => {
  it("closes scoped stream channels when a declared scope changes", () => {
    const scope = useScopeStore();
    const streams = useStreamChannelsStore();
    const db0 = new FakeSocket();
    const db1 = new FakeSocket();

    streams.ensure("conn:redis.terminal:database=0", () => db0);
    streams.ensure("conn:redis.terminal:database=1", () => db1);

    scope.configure("conn", [{ param: "database" }]);
    scope.set("conn", "database", "0");

    expect(db0.closed).toBe(true);
    expect(db1.closed).toBe(true);
  });

  it("does not close streams for undeclared or unchanged scope values", () => {
    const scope = useScopeStore();
    const streams = useStreamChannelsStore();
    const socket = new FakeSocket();

    streams.ensure("conn:redis.terminal:database=0", () => socket);
    scope.set("conn", "database", "0");
    expect(socket.closed).toBe(false);

    scope.configure("conn", [{ param: "database" }]);
    scope.set("conn", "database", "0");
    socket.closed = false;
    scope.set("conn", "database", "0");

    expect(socket.closed).toBe(false);
  });
});
