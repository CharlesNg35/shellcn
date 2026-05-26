import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { installFetch } from "../../test/fetchMock";
import TablePanel from "./TablePanel.vue";
import type { Action, Column } from "../../types/projection";

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

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

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

  it("renders declarative table and row actions", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      calls.push(url);
      if (init?.method === "POST")
        return { body: { ok: true, output: "ran command" } };
      return {
        body: {
          items: [row("s1", "disk usage")],
          nextCursor: "",
          total: 1,
        },
      };
    });
    const create: Action = {
      id: "snippet.create",
      label: "New snippet",
      routeId: "ssh.snippet.create",
      method: "POST",
      risk: "write",
      requiresConfirm: false,
    };
    const run: Action = {
      id: "snippet.run",
      label: "Run",
      routeId: "ssh.snippet.run",
      method: "POST",
      params: { id: "${resource.uid}" },
      risk: "privileged",
      requiresConfirm: true,
      confirmText: "Run it?",
    };
    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "ssh.snippet.list" },
        config: {
          columns,
          actionIds: ["snippet.create"],
          rowActionIds: ["snippet.run"],
        },
        actions: [create, run],
      },
    });
    await flushPromises();
    expect(w.text()).toContain("New snippet");
    expect(w.text()).not.toContain("Run");

    await w.find("tbody tr").trigger("click");
    await flushPromises();
    expect(w.text()).toContain("Run");
    await w
      .findAll("button")
      .find((b) => b.text() === "Run")!
      .trigger("click");
    await flushPromises();
    bodyButton("Confirm")!.click();
    await flushPromises();

    expect(calls.some((url) => url.includes("p.id=s1"))).toBe(true);
    expect(document.body.textContent).toContain("ran command");
    expect(w.emitted("actionDone")?.[0]).toEqual([
      run,
      { ok: true, output: "ran command" },
    ]);
    w.unmount();
  });
});
