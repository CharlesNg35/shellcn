import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import { installFetch } from "@/test/fetchMock";
import KVPanel from "./KVPanel.vue";

interface ListCall {
  cursor: string;
  filter: string;
  limit: string;
}

interface PageBody {
  items: { key: string; type?: string }[];
  nextCursor: string;
}

const listCalls: ListCall[] = [];

function installKV(page: (cursor: string, filter: string) => PageBody): void {
  installFetch((url) => {
    const params = new URL(url, "http://h").searchParams;
    if (url.includes("kv.list")) {
      const cursor = params.get("cursor") ?? "";
      const filter = params.get("filter") ?? "";
      listCalls.push({ cursor, filter, limit: params.get("limit") ?? "" });
      return { body: page(cursor, filter) };
    }
    if (url.includes("kv.read")) {
      return { body: { key: params.get("p.key"), type: "string", value: "v" } };
    }
    return { body: {} };
  });
}

function mountPanel(config: Record<string, unknown> = {}) {
  return mount(KVPanel, {
    props: {
      connectionId: "c1",
      source: { routeId: "kv.list" },
      config: { readRouteId: "kv.read", ...config },
    },
    global: { stubs: { CodeTextEditor: true, AppIcon: true } },
  });
}

function loadMore(w: ReturnType<typeof mountPanel>) {
  return w.findAll("button").find((button) => button.text().includes("Load"));
}

function rowLabels(w: ReturnType<typeof mountPanel>): string[] {
  return w.findAll('[data-test="kv-row"]').map((row) => row.text());
}

// jsdom has no layout, so PrimeVue's VirtualScroller would measure a zero-height
// viewport and render nothing; a fake box lets it compute a real window.
function withLayout(height: number): () => void {
  const proto = HTMLElement.prototype;
  const offsetHeight = Object.getOwnPropertyDescriptor(proto, "offsetHeight")!;
  const offsetParent = Object.getOwnPropertyDescriptor(proto, "offsetParent")!;
  Object.defineProperty(proto, "offsetHeight", {
    configurable: true,
    get: () => height,
  });
  Object.defineProperty(proto, "offsetParent", {
    configurable: true,
    get: () => document.body,
  });
  return () => {
    Object.defineProperty(proto, "offsetHeight", offsetHeight);
    Object.defineProperty(proto, "offsetParent", offsetParent);
  };
}

describe("KVPanel", () => {
  beforeEach(() => {
    listCalls.length = 0;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads only the first page and never follows the cursor on its own", async () => {
    installKV(() => ({
      items: [{ key: "a" }, { key: "b" }],
      nextCursor: "c1",
    }));
    const w = mountPanel();
    await flushPromises();
    await new Promise((resolve) => setTimeout(resolve, 20));
    await flushPromises();

    expect(listCalls).toEqual([{ cursor: "", filter: "", limit: "200" }]);
    expect(w.text()).toContain("Showing 2 keys (+more)");
    expect(loadMore(w)).toBeDefined();
    w.unmount();
  });

  it("appends exactly one page per load more click", async () => {
    const pages: Record<string, PageBody> = {
      "": { items: [{ key: "a" }, { key: "b" }], nextCursor: "c1" },
      c1: { items: [{ key: "c" }, { key: "d" }], nextCursor: "c2" },
      c2: { items: [{ key: "e" }], nextCursor: "" },
    };
    installKV((cursor) => pages[cursor]);
    const w = mountPanel();
    await flushPromises();

    await loadMore(w)!.trigger("click");
    await flushPromises();

    expect(listCalls.map((call) => call.cursor)).toEqual(["", "c1"]);
    expect(w.text()).toContain("Showing 4 keys (+more)");

    await loadMore(w)!.trigger("click");
    await flushPromises();

    expect(listCalls.map((call) => call.cursor)).toEqual(["", "c1", "c2"]);
    expect(w.text()).toContain("5 keys");
    expect(loadMore(w)).toBeUndefined();
    w.unmount();
  });

  it("merges an appended page into the existing namespace tree", async () => {
    const pages: Record<string, PageBody> = {
      "": { items: [{ key: "user:1" }, { key: "user:2" }], nextCursor: "c1" },
      c1: { items: [{ key: "user:3" }, { key: "job:queue" }], nextCursor: "" },
    };
    installKV((cursor) => pages[cursor]);
    const w = mountPanel({ delimiter: ":" });
    await flushPromises();

    expect(rowLabels(w)).toEqual(["user2"]);

    await loadMore(w)!.trigger("click");
    await flushPromises();

    expect(rowLabels(w)).toEqual(["job1", "user3"]);
    expect(w.text()).toContain("4 keys");
    w.unmount();
  });

  it("restarts paging from the first page when the filter changes", async () => {
    vi.useFakeTimers();
    try {
      installKV((cursor, filter) => {
        if (filter) return { items: [{ key: "user:1" }], nextCursor: "" };
        return cursor === ""
          ? { items: [{ key: "a" }, { key: "b" }], nextCursor: "c1" }
          : { items: [{ key: "c" }], nextCursor: "" };
      });
      const w = mountPanel();
      await flushPromises();
      await loadMore(w)!.trigger("click");
      await flushPromises();
      expect(w.text()).toContain("3 keys");

      await w.get('input[aria-label="Filter keys"]').setValue("user");
      await vi.advanceTimersByTimeAsync(250);
      await flushPromises();

      expect(listCalls.at(-1)).toEqual({
        cursor: "",
        filter: "user",
        limit: "200",
      });
      expect(w.text()).toContain("1 key");
      expect(w.text()).not.toContain("3 keys");
      w.unmount();
    } finally {
      vi.useRealTimers();
    }
  });

  it("virtualizes key rows past the render threshold", async () => {
    const items = Array.from({ length: 300 }, (_, i) => ({
      key: `k${String(i).padStart(3, "0")}`,
      type: "string",
    }));
    installKV(() => ({ items, nextCursor: "" }));
    const restoreLayout = withLayout(320);
    try {
      const w = mountPanel({ delimiter: ":" });
      await flushPromises();

      const scroller = w.findComponent({ name: "VirtualScroller" });
      expect(scroller.props("disabled")).toBe(false);
      expect((scroller.props("items") as unknown[]).length).toBe(300);
      const rendered = w.findAll('[data-test="kv-row"]').length;
      expect(rendered).toBeGreaterThan(0);
      expect(rendered).toBeLessThan(60);
      w.unmount();
    } finally {
      restoreLayout();
    }
  });

  it("virtualizes the flat list past the render threshold", async () => {
    const items = Array.from({ length: 300 }, (_, i) => ({
      key: `k${String(i).padStart(3, "0")}`,
      type: "string",
    }));
    installKV(() => ({ items, nextCursor: "" }));
    const restoreLayout = withLayout(320);
    try {
      const w = mountPanel();
      await flushPromises();

      expect(
        w.findComponent({ name: "VirtualScroller" }).props("disabled"),
      ).toBe(false);
      const rendered = w.findAll("tbody tr").length;
      expect(rendered).toBeGreaterThan(0);
      expect(rendered).toBeLessThan(60);
      w.unmount();
    } finally {
      restoreLayout();
    }
  });

  it("refuses to page while a reload is in flight", async () => {
    const gates: Record<string, () => void> = {};
    const json = (body: unknown) =>
      new Response(JSON.stringify(body), {
        headers: { "Content-Type": "application/json" },
      });
    const cursors: string[] = [];
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input);
        if (!url.includes("kv.list")) return json({ key: "a", value: "v" });
        const cursor =
          new URL(url, "http://h").searchParams.get("cursor") ?? "";
        cursors.push(cursor);
        if (cursors.length === 2) {
          await new Promise<void>((resolve) => (gates.reload = resolve));
          return json({ items: [{ key: "fresh" }], nextCursor: "fresh-next" });
        }
        if (cursor === "stale") {
          await new Promise<void>((resolve) => (gates.stale = resolve));
          return json({ items: [{ key: "stale" }], nextCursor: "stale-next" });
        }
        return json({ items: [{ key: "a" }], nextCursor: "stale" });
      }),
    );
    const w = mountPanel();
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();
    await loadMore(w)!.trigger("click");
    await flushPromises();

    // The pre-reload cursor belongs to a scan that is about to be discarded.
    expect(gates.stale).toBeUndefined();

    gates.reload();
    await flushPromises();

    expect(cursors).toEqual(["", ""]);
    expect(w.text()).toContain("fresh");
    expect(w.text()).not.toContain("stale");
    w.unmount();
  });

  it("renders a small keyspace in full and expands namespaces in place", async () => {
    installKV(() => ({
      items: [
        { key: "user:1", type: "string" },
        { key: "user:2", type: "hash" },
        { key: "job:queue", type: "list" },
        { key: "standalone", type: "string" },
      ],
      nextCursor: "",
    }));
    const w = mountPanel({ delimiter: ":" });
    await flushPromises();

    const scroller = w.findComponent({ name: "VirtualScroller" });
    expect(scroller.props("disabled")).toBe(true);
    expect(rowLabels(w)).toEqual(["job1", "user2", "standalonestring"]);
    expect(w.text()).toContain("4 keys");

    await w.findAll('[data-test="kv-row"]')[1].trigger("click");
    await flushPromises();

    expect(rowLabels(w)).toEqual([
      "job1",
      "user2",
      "1string",
      "2hash",
      "standalonestring",
    ]);
    w.unmount();
  });

  it("caps the in-memory keyspace and points at the filter instead of scanning on", async () => {
    const items = Array.from({ length: 5200 }, (_, i) => ({ key: `k${i}` }));
    installKV(() => ({ items, nextCursor: "c1" }));
    const w = mountPanel();
    await flushPromises();

    expect(listCalls).toHaveLength(1);
    expect(w.text()).toContain("5000+ keys");
    expect(w.text()).toContain("filter to narrow");
    expect(loadMore(w)).toBeUndefined();
    w.unmount();
  });
});
