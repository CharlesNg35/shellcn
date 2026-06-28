import { describe, it, expect, vi } from "vitest";
import { registerSessionCleanup, resetSession } from "./session";

describe("session reset registry", () => {
  it("runs every registered cleanup on reset", () => {
    const a = vi.fn();
    const b = vi.fn();
    registerSessionCleanup("a", a);
    registerSessionCleanup("b", b);

    resetSession();

    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenCalledTimes(1);
  });

  it("replaces a cleanup registered under the same id", () => {
    const stale = vi.fn();
    const fresh = vi.fn();
    registerSessionCleanup("dup", stale);
    registerSessionCleanup("dup", fresh);

    resetSession();

    expect(stale).not.toHaveBeenCalled();
    expect(fresh).toHaveBeenCalledTimes(1);
  });

  it("keeps wiping after a cleanup throws", () => {
    const boom = vi.fn(() => {
      throw new Error("boom");
    });
    const after = vi.fn();
    registerSessionCleanup("boom", boom);
    registerSessionCleanup("after", after);

    expect(() => resetSession()).not.toThrow();
    expect(after).toHaveBeenCalledTimes(1);
  });
});
