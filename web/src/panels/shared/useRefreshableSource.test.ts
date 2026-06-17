import { describe, expect, it, vi } from "vitest";
import { useRefreshableSource } from "./useRefreshableSource";

describe("useRefreshableSource", () => {
  it("ignores stale results after reset and a newer load", async () => {
    let resolveFirst: ((value: string) => void) | undefined;
    const loader = vi
      .fn<() => Promise<string>>()
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveFirst = resolve;
        }),
      )
      .mockResolvedValueOnce("fresh");
    const source = useRefreshableSource(loader, {
      initialValue: () => "",
    });

    const first = source.load();
    source.reset();
    await source.load();

    expect(source.data.value).toBe("fresh");

    resolveFirst?.("stale");
    await first;

    expect(source.data.value).toBe("fresh");
    expect(source.refreshing.value).toBe(false);
    expect(source.error.value).toBeNull();
  });

  it("keeps existing data visible when refresh fails", async () => {
    const loader = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("loaded")
      .mockRejectedValueOnce(new Error("refresh failed"));
    const source = useRefreshableSource(loader, {
      initialValue: () => "",
    });

    await source.load();
    await source.load();

    expect(source.data.value).toBe("loaded");
    expect(source.loadedOnce.value).toBe(true);
    expect(source.error.value).toBe("refresh failed");
    expect(source.blockingError.value).toBeNull();
  });
});
