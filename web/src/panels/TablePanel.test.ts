import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../test/fetchMock";
import TablePanel from "./TablePanel.vue";
import type { Column } from "../types/projection";

const columns: Column[] = [
  { key: "name", label: "Name", sortable: true },
  { key: "state", label: "State" },
];

function row(id: string, name: string, state = "running") {
  return { ref: { kind: "container", name, uid: id }, name, state };
}

beforeEach(() => {
  installFetch((url) => {
    const u = new URL(url, "http://h");
    const cursor = u.searchParams.get("cursor");
    const filter = u.searchParams.get("filter");
    if (filter === "beta")
      return { body: { items: [row("b", "beta")], nextCursor: "", total: 1 } };
    if (cursor === "c2")
      return { body: { items: [row("c", "gamma")], nextCursor: "", total: 3 } };
    return {
      body: {
        items: [row("a", "alpha"), row("b", "beta")],
        nextCursor: "c2",
        total: 3,
      },
    };
  });
});
afterEach(() => vi.unstubAllGlobals());

describe("TablePanel", () => {
  it("renders manifest columns and rows, paginates via cursor", async () => {
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns },
      },
    });
    await flushPromises();
    expect(w.findAll("thead th").map((t) => t.text())).toEqual([
      "Name",
      "State",
    ]);
    expect(w.findAll("tbody tr")).toHaveLength(2);

    await w.find("tbody").exists();
    const loadMore = w
      .findAll("button")
      .find((b) => b.text().includes("Load more"));
    expect(loadMore).toBeTruthy();
    await loadMore!.trigger("click");
    await flushPromises();
    expect(w.findAll("tbody tr")).toHaveLength(3);
  });

  it("filters server-side and resets the list", async () => {
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns },
      },
    });
    await flushPromises();
    await w.find('input[type="search"]').setValue("beta");
    await new Promise((r) => setTimeout(r, 300));
    await flushPromises();
    expect(w.findAll("tbody tr")).toHaveLength(1);
    expect(w.text()).toContain("beta");
  });

  it("emits the full row on click", async () => {
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns },
      },
    });
    await flushPromises();
    await w.find("tbody tr").trigger("click");
    const ev = w.emitted("select");
    expect(ev).toBeTruthy();
    expect((ev?.[0][0] as { ref: { uid: string } }).ref.uid).toBe("a");
  });
});
