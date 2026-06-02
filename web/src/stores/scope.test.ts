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
  it("closes connection stream channels when a declared scope changes", () => {
    const scope = useScopeStore();
    const streams = useStreamChannelsStore();
    const db0 = new FakeSocket();
    const allScope = new FakeSocket();
    const other = new FakeSocket();

    streams.ensure("conn:redis.terminal:database=0", () => db0);
    streams.ensure("conn:kubernetes.cluster.shell:", () => allScope);
    streams.ensure("other:redis.terminal:database=0", () => other);

    scope.configure("conn", [{ param: "database" }]);
    scope.set("conn", "database", "0");

    expect(db0.closed).toBe(true);
    expect(allScope.closed).toBe(true);
    expect(other.closed).toBe(false);
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
