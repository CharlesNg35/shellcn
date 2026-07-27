import { describe, it, expect, beforeEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import type { MarketEntry, MarketList } from "../types/projection/market";

const list = vi.fn<() => Promise<MarketList>>();
vi.mock("../api/admin", () => ({
  adminMarketApi: { list: () => list() },
}));
vi.mock("../composables/useNotify", () => ({
  useNotify: () => ({ success: vi.fn(), error: vi.fn() }),
}));

import { useMarketStore } from "./market";

function entry(over: Partial<MarketEntry>): MarketEntry {
  return {
    name: "x",
    displayName: "X",
    description: "",
    repo: "",
    license: "MIT",
    maintainers: [],
    compatible: true,
    managed: false,
    updateAvailable: false,
    ...over,
  };
}

beforeEach(() => {
  setActivePinia(createPinia());
  list.mockReset();
});

describe("market store", () => {
  it("reports an update only for a managed plugin that has one", async () => {
    list.mockResolvedValue({
      enabled: true,
      plugins: [
        entry({ name: "vault", managed: true, updateAvailable: true }),
        entry({ name: "consul", managed: true, updateAvailable: false }),
        entry({ name: "nomad", managed: false, updateAvailable: true }),
      ],
    });
    const store = useMarketStore();
    await store.ensureLoaded();

    expect(store.updateFor("vault")?.name).toBe("vault");
    expect(store.updateFor("consul")).toBeNull();
    expect(store.updateFor("nomad")).toBeNull();
    expect(store.updateFor("unknown")).toBeNull();
    expect(store.updateFor(undefined)).toBeNull();
  });

  it("fetches the catalog at most once via ensureLoaded", async () => {
    list.mockResolvedValue({ enabled: true, plugins: [] });
    const store = useMarketStore();
    await Promise.all([store.ensureLoaded(), store.ensureLoaded()]);
    await store.ensureLoaded();
    expect(list).toHaveBeenCalledTimes(1);
  });

  it("treats a failed market load as disabled with no updates", async () => {
    list.mockRejectedValueOnce(new Error("forbidden"));
    const store = useMarketStore();
    await store.load();
    expect(store.enabled).toBe(false);
    expect(store.updateFor("vault")).toBeNull();
  });
});
