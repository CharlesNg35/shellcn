/* eslint-disable vue/one-component-per-file */
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { defineComponent, h, KeepAlive } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import ConfirmDialog from "primevue/confirmdialog";
import { installFetch } from "@/test/fetchMock";
import TablePanel from "./TablePanel.vue";
import { RiskLevel, type Action, type Column } from "@/types/projection";
import CodeTextEditor from "../shared/CodeTextEditor.vue";

const columns: Column[] = [
  { key: "name", label: "Name", sortable: true },
  { key: "state", label: "State" },
];

const editableColumns: Column[] = [
  { key: "name", label: "Name", sortable: true, readOnly: true },
  { key: "state", label: "State", editable: true, editor: "text" },
];

function row(id: string, name: string, state = "running") {
  return { ref: { kind: "container", name, uid: id }, name, state };
}

beforeEach(() => {
  setActivePinia(createPinia());
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
afterEach(() => {
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (b) => b.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

class FakeSocket {
  readyState = 1;
  closed = false;
  listeners = new Map<string, Array<(ev: unknown) => void>>();

  addEventListener(type: string, listener: (ev: unknown) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener]);
  }

  send(): void {}

  close(): void {
    this.closed = true;
  }

  emit(type: string, ev: unknown): void {
    for (const listener of this.listeners.get(type) ?? []) listener(ev);
  }
}

describe("TablePanel", () => {
  it("renders manifest columns and rows, paginates server-side", async () => {
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

    w.findComponent({ name: "DataTable" }).vm.$emit("page", {
      first: 2,
      rows: 50,
    });
    await flushPromises();
    expect(w.findAll("tbody tr")).toHaveLength(1);
    expect(w.text()).toContain("gamma");
  });

  it("uses the previous page nextCursor instead of treating cursor as an offset", async () => {
    const cursors: Array<string | null> = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      const u = new URL(url, "http://h");
      cursors.push(u.searchParams.get("cursor"));
      return u.searchParams.get("cursor") === "opaque-next"
        ? { body: { items: [row("b", "beta")], nextCursor: "", total: 2 } }
        : {
            body: {
              items: [row("a", "alpha")],
              nextCursor: "opaque-next",
              total: 2,
            },
          };
    });
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns },
      },
    });
    await flushPromises();

    w.findComponent({ name: "DataTable" }).vm.$emit("page", {
      first: 1,
      rows: 50,
    });
    await flushPromises();

    expect(cursors).toEqual([null, "opaque-next"]);
    expect(w.text()).toContain("beta");
  });

  it("bounds and titles long cell values", async () => {
    const longValue = "sha256:" + "a".repeat(96);
    vi.unstubAllGlobals();
    installFetch(() => ({
      body: {
        items: [row("long", longValue)],
        nextCursor: "",
        total: 1,
      },
    }));

    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: {
          columns: [{ key: "name", label: "Image", width: "12rem" }],
        },
      },
    });
    await flushPromises();

    const cell = w.get('[data-test="table-cell-value"]');
    expect(cell.classes()).toContain("truncate");
    expect(cell.attributes("title")).toBe(longValue);
    expect(cell.attributes("style")).toContain("max-width: 12rem");
    expect(w.get("thead th").attributes("style")).toContain("width: 12rem");
    expect(w.get("thead th").attributes("style")).toContain("min-width: 12rem");
  });

  it("renders icon columns with compact cells", async () => {
    vi.unstubAllGlobals();
    installFetch(() => ({
      body: {
        items: [{ name: "web", kindIcon: "monitor" }],
        nextCursor: "",
        total: 1,
      },
    }));

    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "proxmox.guest.list" },
        config: {
          columns: [
            { key: "kindIcon", label: "", type: "icon" },
            { key: "name", label: "Name" },
          ],
        },
      },
    });
    await flushPromises();

    expect(w.get("thead th").attributes("style")).toContain("width: 3rem");
    expect(w.find('[data-test="table-cell-value"] svg').exists()).toBe(true);
  });

  it("shows an edit affordance only for explicitly editable cells", async () => {
    vi.unstubAllGlobals();
    installFetch(() => ({
      body: {
        items: [row("a", "alpha")],
        nextCursor: "",
        total: 1,
      },
    }));

    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: {
          columns: editableColumns,
          editable: true,
          rowKey: ["name"],
          update: { routeId: "db.row.update", method: "POST" },
        },
      },
    });
    await flushPromises();

    const cells = w.findAll('[data-test="table-cell-value"]');
    expect(cells[0].find("svg").exists()).toBe(false);
    expect(cells[1].find("svg").exists()).toBe(true);
  });

  it("summarizes structured values and opens JSON editing explicitly", async () => {
    vi.unstubAllGlobals();
    installFetch((_url, init) => {
      if (init?.method === "POST") {
        return { body: { ok: true } };
      }
      return {
        body: {
          items: [
            {
              _key: { id: 1 },
              id: 1,
              metadata: { labels: { app: "web" }, replicas: 2 },
            },
          ],
          nextCursor: "",
          total: 1,
        },
      };
    });

    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: {
          columns: [
            { key: "id", label: "ID", type: "number", readOnly: true },
            {
              key: "metadata",
              label: "Metadata",
              type: "json",
              editable: true,
              editor: "json",
            },
          ],
          editable: true,
          update: { routeId: "db.row.update", method: "POST" },
        },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("{2 keys}");
    expect(w.text()).not.toContain("[object Object]");
    const edit = [...document.body.querySelectorAll("button")].find(
      (b) => b.getAttribute("aria-label") === "Edit JSON",
    ) as HTMLButtonElement;
    expect(edit).toBeTruthy();
    edit.click();
    await flushPromises();
    expect(w.findComponent(CodeTextEditor).props("value")).toContain(
      '"replicas": 2',
    );
    w.unmount();
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

  it("restores filter, sort, and page after the table is remounted", async () => {
    const urls: string[] = [];
    installFetch((url) => {
      urls.push(url);
      const u = new URL(url, "http://h");
      const filter = u.searchParams.get("filter");
      const sort = u.searchParams.get("sort");
      const cursor = u.searchParams.get("cursor");
      if (filter === "beta" && sort === "-name" && cursor === "2") {
        return {
          body: { items: [row("b2", "beta-2")], nextCursor: "", total: 2 },
        };
      }
      if (filter === "beta")
        return {
          body: { items: [row("b", "beta")], nextCursor: "", total: 1 },
        };
      return {
        body: {
          items: [row("a", "alpha"), row("b", "beta")],
          nextCursor: "",
          total: 2,
        },
      };
    });
    const props = {
      connectionId: "c1",
      source: { routeId: "docker.container.list" },
      config: { columns },
    };
    const first = mount(TablePanel, { props });
    await flushPromises();

    await first.find('input[type="search"]').setValue("beta");
    await new Promise((r) => setTimeout(r, 300));
    await flushPromises();
    first.findComponent({ name: "DataTable" }).vm.$emit("sort", {
      sortField: "name",
      sortOrder: -1,
    });
    await flushPromises();
    first.findComponent({ name: "DataTable" }).vm.$emit("page", {
      first: 2,
      rows: 100,
    });
    await flushPromises();
    first.unmount();

    const second = mount(TablePanel, { props });
    await flushPromises();

    expect(second.find('input[type="search"]').element).toHaveProperty(
      "value",
      "beta",
    );
    const dt = second.findComponent({ name: "DataTable" });
    expect(dt.props("sortField")).toBe("name");
    expect(dt.props("sortOrder")).toBe(-1);
    expect(dt.props("first")).toBe(2);
    expect(dt.props("rows")).toBe(100);
    expect(urls.at(-1)).toContain("filter=beta");
    expect(urls.at(-1)).toContain("sort=-name");
    expect(urls.at(-1)).toContain("cursor=2");
    expect(urls.at(-1)).toContain("limit=100");
    second.unmount();
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
      risk: RiskLevel.Write,
      requiresConfirm: false,
    };
    const run: Action = {
      id: "snippet.run",
      label: "Run",
      routeId: "ssh.snippet.run",
      method: "POST",
      params: { id: "${resource.uid}" },
      risk: RiskLevel.Privileged,
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

  it("loads editable columns from a source when the table is empty", async () => {
    const calls: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      calls.push(url);
      if (url.includes("db.table.columns")) {
        return {
          body: {
            items: [
              { name: "id", nullable: false, editable: true, editor: "text" },
              { name: "name", nullable: true, editable: true, editor: "text" },
            ],
            nextCursor: "",
            total: 2,
          },
        };
      }
      return { body: { items: [], nextCursor: "", total: 0 } };
    });

    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: {
          editable: true,
          insert: { routeId: "db.row.insert", method: "POST" },
          columnsSource: { routeId: "db.table.columns" },
        },
      },
    });
    await flushPromises();

    const add = bodyButton("Add row")!;
    expect(add.disabled).toBe(false);
    expect(calls.some((url) => url.includes("db.table.columns"))).toBe(true);

    add.click();
    await flushPromises();
    expect(document.body.textContent).toContain("id");
    expect(document.body.textContent).toContain("name");
    w.unmount();
  });

  it("derives add-row inputs from each column's data type", async () => {
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("db.table.columns")) {
        return {
          body: {
            items: [
              {
                name: "id",
                type: "bigint unsigned",
                columnType: "number",
                nullable: false,
                editable: true,
              },
              {
                name: "active",
                type: "tinyint(1)",
                columnType: "bool",
                nullable: true,
                editable: true,
              },
              {
                name: "label",
                type: "text",
                nullable: true,
                editable: true,
                editor: "text",
              },
            ],
            nextCursor: "",
            total: 3,
          },
        };
      }
      return { body: { items: [], nextCursor: "", total: 0 } };
    });

    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: {
          editable: true,
          insert: { routeId: "db.row.insert", method: "POST" },
          columnsSource: { routeId: "db.table.columns" },
        },
      },
    });
    await flushPromises();

    bodyButton("Add row")!.click();
    await flushPromises();
    expect(w.findComponent({ name: "ToggleSwitch" }).exists()).toBe(true);
    expect(w.findComponent({ name: "InputNumber" }).exists()).toBe(true);
    w.unmount();
  });

  it("confirms direct row deletes with the app dialog", async () => {
    const nativeConfirm = vi.fn();
    const calls: { url: string; method: string; body: unknown }[] = [];
    let deleted = false;
    vi.unstubAllGlobals();
    vi.stubGlobal("confirm", nativeConfirm);
    installFetch((url, init) => {
      if (init?.method === "DELETE") {
        calls.push({
          url,
          method: init.method,
          body: init.body ? JSON.parse(init.body as string) : undefined,
        });
        deleted = true;
        return { body: { ok: true } };
      }
      return {
        body: {
          items: deleted ? [row("b", "beta")] : [row("a", "alpha")],
          nextCursor: "",
          total: deleted ? 1 : 2,
        },
      };
    });

    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: {
          columns,
          editable: true,
          rowKey: ["name"],
          delete: { routeId: "db.row.delete", method: "DELETE" as const },
        },
      },
    });
    await flushPromises();

    const delBtn = [...document.body.querySelectorAll("button")].find((b) =>
      b.getAttribute("aria-label")?.includes("Delete"),
    ) as HTMLButtonElement;
    delBtn.click();
    await flushPromises();

    expect(nativeConfirm).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain("Delete this row?");
    expect(document.body.textContent).toContain(
      "This change is permanent and cannot be undone.",
    );

    bodyButton("Delete")!.click();
    await flushPromises();

    expect(calls).toEqual([
      {
        url: expect.stringContaining("db.row.delete"),
        method: "DELETE",
        body: { key: { name: "alpha" } },
      },
    ]);
    expect(w.text()).not.toContain("alpha");
    expect(w.text()).toContain("beta");
    w.unmount();
  });
});

describe("TablePanel staged edits", () => {
  type Call = { url: string; method: string; body: unknown };

  const stagedConfig = {
    columns: editableColumns,
    editable: true,
    stagedEdits: true,
    rowKey: ["name"],
    insert: { routeId: "db.row.insert", method: "POST" as const },
    update: { routeId: "db.row.update", method: "POST" as const },
    delete: { routeId: "db.row.delete", method: "POST" as const },
  };

  function mountStaged() {
    const calls: Call[] = [];
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      if (init?.method === "POST") {
        calls.push({
          url,
          method: "POST",
          body: init.body ? JSON.parse(init.body as string) : undefined,
        });
        return { body: { ok: true } };
      }
      return {
        body: {
          items: [row("a", "alpha"), row("b", "beta")],
          nextCursor: "",
          total: 2,
        },
      };
    });
    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "db.table.rows" },
        config: stagedConfig,
      },
    });
    return { w, calls };
  }

  function mountStagedWithConfirm() {
    const calls: Call[] = [];
    let listCalls = 0;
    vi.unstubAllGlobals();
    installFetch((url, init) => {
      if (init?.method === "POST") {
        calls.push({
          url,
          method: "POST",
          body: init.body ? JSON.parse(init.body as string) : undefined,
        });
        return { body: { ok: true } };
      }
      listCalls += 1;
      return {
        body: {
          items:
            listCalls === 1
              ? [row("a", "alpha"), row("b", "beta")]
              : [row("c", "gamma")],
          nextCursor: "",
          total: listCalls === 1 ? 2 : 1,
        },
      };
    });
    const host = document.createElement("div");
    document.body.appendChild(host);
    const w = mount(
      {
        render: () =>
          h("div", [
            h(TablePanel, {
              connectionId: "c1",
              source: { routeId: "db.table.rows" },
              config: stagedConfig,
            }),
            h(ConfirmDialog),
          ]),
      },
      { attachTo: host },
    );
    return { w, calls };
  }

  function editCell(
    w: ReturnType<typeof mount>,
    index: number,
    field: string,
    newValue: unknown,
  ) {
    const dt = w.findComponent({ name: "DataTable" });
    const data = (dt.props("value") as Record<string, unknown>[])[index];
    dt.vm.$emit("cell-edit-complete", {
      data,
      field,
      value: data[field],
      newValue,
    });
  }

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("buffers a cell edit and commits it through the update route", async () => {
    const { w, calls } = mountStaged();
    await flushPromises();

    editCell(w, 0, "state", "stopped");
    await flushPromises();
    expect(calls).toHaveLength(0); // nothing sent yet
    expect(w.text()).toContain("1 unsaved change");

    bodyButton("Commit")!.click();
    await flushPromises();

    const update = calls.find((c) => c.url.includes("db.row.update"));
    expect(update?.body).toEqual({
      key: { name: "alpha" },
      values: { state: "stopped" },
    });
    expect(w.text()).not.toContain("unsaved");
    w.unmount();
  });

  it("discards buffered edits and restores the original value", async () => {
    const { w, calls } = mountStaged();
    await flushPromises();

    editCell(w, 0, "state", "stopped");
    await flushPromises();
    expect(w.text()).toContain("stopped");

    bodyButton("Discard")!.click();
    await flushPromises();
    expect(calls).toHaveLength(0);
    expect(w.text()).not.toContain("unsaved");
    expect(w.text()).toContain("running");
    expect(w.text()).not.toContain("stopped");
    w.unmount();
  });

  it("commits a staged delete through the delete route", async () => {
    const { w, calls } = mountStaged();
    await flushPromises();

    const delBtn = [...document.body.querySelectorAll("button")].find((b) =>
      b.getAttribute("aria-label")?.includes("Delete"),
    ) as HTMLButtonElement;
    delBtn.click();
    await flushPromises();
    expect(calls).toHaveLength(0);
    expect(w.text()).toContain("1 unsaved change");

    bodyButton("Commit")!.click();
    await flushPromises();
    const del = calls.find((c) => c.url.includes("db.row.delete"));
    expect(del?.body).toEqual({ key: { name: "alpha" } });
    w.unmount();
  });

  it("keeps staged edits when pagination is canceled", async () => {
    const { w } = mountStagedWithConfirm();
    await flushPromises();

    editCell(w, 0, "state", "stopped");
    await flushPromises();
    w.findComponent({ name: "DataTable" }).vm.$emit("page", {
      first: 50,
      rows: 50,
    });
    await flushPromises();
    bodyButton("Keep editing")!.click();
    await flushPromises();

    expect(w.text()).toContain("1 unsaved change");
    expect(w.text()).toContain("alpha");
    expect(w.text()).not.toContain("gamma");
    w.unmount();
  });

  it("discards staged edits before pagination", async () => {
    const { w } = mountStagedWithConfirm();
    await flushPromises();

    editCell(w, 0, "state", "stopped");
    await flushPromises();
    w.findComponent({ name: "DataTable" }).vm.$emit("page", {
      first: 50,
      rows: 50,
    });
    await flushPromises();
    bodyButton("Discard changes")!.click();
    await flushPromises();

    expect(w.text()).not.toContain("unsaved");
    expect(w.text()).toContain("gamma");
    w.unmount();
  });

  it("requests the declared defaultSort on first load", async () => {
    const fetchFn = installFetch(() => ({
      body: { items: [row("a", "alpha")], nextCursor: "", total: 1 },
    }));
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: { columns, defaultSort: { field: "cpuPct", desc: true } },
      },
    });
    await flushPromises();
    const url = fetchFn.mock.calls[0]?.[0] as string;
    expect(url).toContain("sort=-cpuPct");
    w.unmount();
  });

  it("formats percent columns with fixed precision", async () => {
    installFetch(() => ({
      body: {
        items: [{ ref: { kind: "p", name: "x", uid: "x" }, cpuPct: 12.3456 }],
        nextCursor: "",
        total: 1,
      },
    }));
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: {
          columns: [
            { key: "cpuPct", label: "CPU", type: "percent", precision: 1 },
          ],
        },
      },
    });
    await flushPromises();
    expect(w.find('[data-test="table-cell-value"]').text()).toBe("12.3%");
    w.unmount();
  });

  it("updates relative-time columns without refetching rows", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-05T12:00:00Z"));
    let calls = 0;
    installFetch(() => {
      calls += 1;
      return {
        body: {
          items: [
            {
              _id: "pod-1",
              name: "web",
              age: "2026-06-05T11:59:30Z",
            },
          ],
          nextCursor: "",
          total: 1,
        },
      };
    });
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kubernetes.resource.list" },
        config: {
          columns: [
            { key: "name", label: "Name" },
            { key: "age", label: "Age", type: "relative_time" },
          ],
        },
      },
    });
    await flushPromises();
    expect(w.text()).toContain("30s");

    await vi.advanceTimersByTimeAsync(30_000);
    await flushPromises();
    expect(w.text()).toContain("1m");
    expect(calls).toBe(1);
    w.unmount();
  });

  it("polls the current page on refreshIntervalMs and replaces rows in place", async () => {
    vi.useFakeTimers();
    let calls = 0;
    installFetch(() => {
      calls += 1;
      return {
        body: {
          items: [row("a", calls === 1 ? "alpha" : "alpha-refreshed")],
          nextCursor: "",
          total: 1,
        },
      };
    });
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: { columns, refreshIntervalMs: 1000 },
      },
    });
    await flushPromises();
    expect(calls).toBe(1);
    expect(w.text()).toContain("alpha");

    await vi.advanceTimersByTimeAsync(1000);
    expect(calls).toBe(2);
    expect(w.text()).toContain("alpha-refreshed");
    w.unmount();
  });

  it("keeps existing rows visible when refresh fails", async () => {
    let calls = 0;
    installFetch(() => {
      calls += 1;
      if (calls === 1) {
        return {
          body: {
            items: [row("a", "alpha")],
            nextCursor: "",
            total: 1,
          },
        };
      }
      return {
        status: 500,
        body: { error: "refresh failed" },
      };
    });
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: { columns },
      },
    });
    await flushPromises();
    expect(w.text()).toContain("alpha");

    await w
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(w.text()).toContain("alpha");
    expect(w.text()).toContain("refresh failed");
    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
    w.unmount();
  });

  it("opens the detail dialog from the details icon when rowClick is detail", async () => {
    installFetch(() => ({
      body: {
        items: [{ _id: "p1", name: "nginx", cpuPct: 12.34 }],
        nextCursor: "",
        total: 1,
      },
    }));
    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: {
          columns: [
            { key: "name", label: "Name" },
            { key: "cpuPct", label: "CPU", type: "percent", precision: 1 },
          ],
          rowClick: "detail",
        },
      },
    });
    await flushPromises();
    const detailsBtn = [...document.body.querySelectorAll("button")].find(
      (b) => b.getAttribute("aria-label") === "View details",
    ) as HTMLButtonElement;
    expect(detailsBtn).toBeTruthy();
    detailsBtn.click();
    await flushPromises();
    expect(document.body.textContent).toContain("12.3%");
    w.unmount();
  });

  it("opens the dialog on row-body click when rowClick is detail", async () => {
    installFetch(() => ({
      body: {
        items: [{ _id: "p1", name: "nginx", cpuPct: 5 }],
        nextCursor: "",
        total: 1,
      },
    }));
    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: {
          columns: [{ key: "name", label: "Name" }],
          rowClick: "detail",
        },
      },
    });
    await flushPromises();
    await w.find("tbody tr").trigger("click");
    await flushPromises();
    expect(w.emitted("select")).toBeFalsy();
    expect(document.body.textContent).toContain("nginx");
    w.unmount();
  });

  it("opens the dialog via keyboard on a clickable row", async () => {
    installFetch(() => ({
      body: {
        items: [{ _id: "p1", name: "nginx", cpuPct: 5 }],
        nextCursor: "",
        total: 1,
      },
    }));
    const w = mount(TablePanel, {
      attachTo: document.body,
      props: {
        connectionId: "c1",
        source: { routeId: "server_monitor.processes" },
        config: {
          columns: [{ key: "name", label: "Name" }],
          rowClick: "detail",
        },
      },
    });
    await flushPromises();
    const tr = w.find("tbody tr");
    expect(tr.attributes("tabindex")).toBe("0");
    // PT merge must keep the preset row styling, not clobber it.
    expect(tr.classes()).toContain("cursor-pointer");
    await tr.trigger("keydown", { key: "Enter" });
    await flushPromises();
    expect(w.emitted("select")).toBeFalsy();
    expect(document.body.textContent).toContain("nginx");
    w.unmount();
  });

  it("pauses live polling while deactivated under KeepAlive", async () => {
    vi.useFakeTimers();
    let calls = 0;
    installFetch(() => {
      calls += 1;
      return {
        body: { items: [{ _id: "a", name: "x" }], nextCursor: "", total: 1 },
      };
    });
    const Parent = defineComponent({
      props: { show: { type: Boolean, default: true } },
      setup(p) {
        return () =>
          h(KeepAlive, () =>
            p.show
              ? h(TablePanel, {
                  connectionId: "c1",
                  source: { routeId: "server_monitor.processes" },
                  config: {
                    columns: [{ key: "name", label: "Name" }],
                    refreshIntervalMs: 1000,
                  },
                })
              : null,
          );
      },
    });
    const w = mount(Parent, { props: { show: true } });
    await flushPromises();
    expect(calls).toBe(1); // initial load
    await vi.advanceTimersByTimeAsync(1000);
    expect(calls).toBe(2); // polls while visible/active

    await w.setProps({ show: false }); // deactivate (kept alive, not unmounted)
    await flushPromises();
    await vi.advanceTimersByTimeAsync(3000);
    expect(calls).toBe(2); // paused — no background polling

    await w.setProps({ show: true }); // reactivate
    await flushPromises();
    expect(calls).toBe(3); // immediate catch-up refresh
    w.unmount();
  });

  it("pauses live watch sockets while deactivated under KeepAlive", async () => {
    const sockets: FakeSocket[] = [];
    vi.stubGlobal(
      "WebSocket",
      class extends FakeSocket {
        constructor() {
          super();
          sockets.push(this);
        }
      },
    );
    installFetch((url) => {
      if (url.endsWith("/tickets")) return { body: { ticket: "t1" } };
      return {
        body: { items: [{ _id: "a", name: "x" }], nextCursor: "", total: 1 },
      };
    });

    const Parent = defineComponent({
      props: { show: { type: Boolean, default: true } },
      setup(p) {
        return () =>
          h(KeepAlive, () =>
            p.show
              ? h(TablePanel, {
                  connectionId: "c1",
                  source: { routeId: "server_monitor.processes" },
                  config: {
                    columns: [{ key: "name", label: "Name" }],
                    watch: { routeId: "server_monitor.processes.watch" },
                  },
                })
              : null,
          );
      },
    });
    const w = mount(Parent, { props: { show: true } });
    await flushPromises();
    expect(sockets).toHaveLength(1);
    expect(sockets[0].closed).toBe(false);

    await w.setProps({ show: false });
    await flushPromises();
    expect(sockets[0].closed).toBe(true);

    await w.setProps({ show: true });
    await flushPromises();
    expect(sockets).toHaveLength(2);
    expect(sockets[1].closed).toBe(false);

    w.unmount();
  });

  it("refreshes from the server instead of applying watch rows outside the active filter", async () => {
    const sockets: FakeSocket[] = [];
    const urls: string[] = [];
    vi.stubGlobal(
      "WebSocket",
      class extends FakeSocket {
        constructor() {
          super();
          sockets.push(this);
        }
      },
    );
    installFetch((url) => {
      urls.push(url);
      if (url.endsWith("/tickets")) return { body: { ticket: "t1" } };
      const u = new URL(url, "http://h");
      if (u.searchParams.get("filter") === "beta") {
        return {
          body: { items: [row("b", "beta")], nextCursor: "", total: 1 },
        };
      }
      return {
        body: {
          items: [row("a", "alpha"), row("b", "beta")],
          nextCursor: "",
          total: 2,
        },
      };
    });

    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kubernetes.resource.list" },
        config: {
          columns,
          watch: { routeId: "kubernetes.resource.watch" },
        },
      },
    });
    await flushPromises();
    expect(sockets).toHaveLength(1);

    await w.find('input[type="search"]').setValue("beta");
    await new Promise((r) => setTimeout(r, 300));
    await flushPromises();
    expect(w.text()).toContain("beta");
    expect(w.text()).not.toContain("alpha");

    sockets[0].emit("message", {
      data: JSON.stringify({
        type: "added",
        ref: { kind: "pod", name: "alpha-live", uid: "a-live" },
        resource: row("a-live", "alpha-live"),
      }),
    });
    await new Promise((r) => setTimeout(r, 150));
    await flushPromises();

    expect(w.text()).toContain("beta");
    expect(w.text()).not.toContain("alpha-live");
    expect(
      urls.filter((url) => url.includes("filter=beta")).length,
    ).toBeGreaterThan(1);
    w.unmount();
  });

  it("toggles selection when the selection-column cell is clicked (not just the checkbox)", async () => {
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns, selectable: true },
      },
    });
    await flushPromises();
    const cell = w.find('td[data-p-selection-column="true"]');
    expect(cell.exists()).toBe(true);
    await cell.trigger("click");
    // The cell padding toggles the row instead of navigating away.
    expect(w.emitted("select")).toBeFalsy();
    expect(
      (w.findComponent({ name: "DataTable" }).props("selection") as unknown[])
        .length,
    ).toBe(1);
    w.unmount();
  });

  it("navigates on row-click when rowClick is navigate", async () => {
    const w = mount(TablePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "docker.container.list" },
        config: { columns, rowClick: "navigate" },
      },
    });
    await flushPromises();
    // Rows carry a ref → navigate; the detail dialog stays closed.
    await w.find("tbody tr").trigger("click");
    expect(w.emitted("select")).toBeTruthy();
    expect(document.body.textContent).not.toContain("Close");
    w.unmount();
  });
});
