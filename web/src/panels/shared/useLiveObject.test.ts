import { describe, expect, it, vi } from "vitest";
import { effectScope, ref } from "vue";

const hoisted = vi.hoisted(() => ({
  captured: null as null | ((ev: unknown) => void),
  stop: vi.fn(),
}));

vi.mock("@/api/dataSource", () => ({
  watch: (
    _conn: string,
    _ds: unknown,
    _ctx: unknown,
    onEvent: (ev: unknown) => void,
  ) => {
    hoisted.captured = onEvent;
    return hoisted.stop;
  },
}));

import { useLiveObject } from "./useLiveObject";

const ds = { routeId: "x.watch", method: "WS" } as never;
const fire = (ev: unknown): void => hoisted.captured?.(ev);

function inScope<T>(fn: () => T): { value: T; stop: () => void } {
  const scope = effectScope();
  const value = scope.run(fn) as T;
  return { value, stop: () => scope.stop() };
}

describe("useLiveObject", () => {
  it("live-updates when the accept guard allows (clean)", async () => {
    hoisted.captured = null;
    const { value: lo } = inScope(() =>
      useLiveObject<string>("c1", ds, {}, async () => "initial", {
        initialValue: () => "",
      }),
    );
    await lo.load();
    expect(lo.data.value).toBe("initial");

    fire({ type: "modified", resource: "v2" });
    expect(lo.data.value).toBe("v2");
    expect(lo.externalChanged.value).toBe(false);
  });

  it("stashes updates when dirty, then applies on demand", async () => {
    hoisted.captured = null;
    const dirty = ref(false);
    const { value: lo } = inScope(() =>
      useLiveObject<string>("c1", ds, {}, async () => "initial", {
        initialValue: () => "",
        accept: () => !dirty.value,
      }),
    );
    await lo.load();

    dirty.value = true;
    fire({ type: "modified", resource: "server-v3" });
    expect(lo.data.value).toBe("initial"); // not clobbered
    expect(lo.externalChanged.value).toBe(true);

    lo.applyPending();
    expect(lo.data.value).toBe("server-v3");
    expect(lo.externalChanged.value).toBe(false);
  });

  it("flags deletion", async () => {
    hoisted.captured = null;
    const { value: lo } = inScope(() =>
      useLiveObject<string>("c1", ds, {}, async () => "initial", {
        initialValue: () => "",
      }),
    );
    await lo.load();
    fire({ type: "deleted", resource: undefined });
    expect(lo.deleted.value).toBe(true);
  });

  it("unsubscribes when the scope is disposed", () => {
    hoisted.stop.mockClear();
    const { stop } = inScope(() =>
      useLiveObject<string>("c1", ds, {}, async () => "", {
        initialValue: () => "",
      }),
    );
    stop();
    expect(hoisted.stop).toHaveBeenCalled();
  });

  it("does not subscribe without a watch source", () => {
    hoisted.captured = null;
    inScope(() =>
      useLiveObject<string>("c1", undefined, {}, async () => "", {
        initialValue: () => "",
      }),
    );
    expect(hoisted.captured).toBeNull();
  });
});
