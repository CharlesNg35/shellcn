/* eslint-disable vue/one-component-per-file, vue/require-prop-types */
import { defineComponent, h } from "vue";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import Button from "primevue/button";
import ConfirmDialog from "primevue/confirmdialog";
import ToastService from "primevue/toastservice";
import { installFetch } from "@/test/fetchMock";
import { toPng } from "html-to-image";
import GraphPanel from "./GraphPanel.vue";
import TracePanel from "./TracePanel.vue";
import KVPanel from "./KVPanel.vue";
import HTTPClientPanel from "./HTTPClientPanel.vue";
import DiffPanel from "./DiffPanel.vue";

vi.mock("html-to-image", () => ({
  toJpeg: vi.fn(() => Promise.resolve("data:image/jpeg;base64,graph")),
  toPng: vi.fn(() => Promise.resolve("data:image/png;base64,graph")),
  toSvg: vi.fn(() => Promise.resolve("data:image/svg+xml,graph")),
}));

vi.mock("@vue-flow/core", () => ({
  VueFlow: defineComponent({
    props: ["nodes", "edges"],
    emits: ["node-click"],
    template:
      '<div data-test="graph" class="vue-flow"><button v-for="n in nodes" :key="n.id" type="button" @click="$emit(\'node-click\', { node: n })">{{ n.data.label }}</button><slot /></div>',
  }),
  Handle: defineComponent({
    props: ["type", "position", "id"],
    template: "<div />",
  }),
  Position: { Left: "left", Right: "right", Top: "top", Bottom: "bottom" },
  MarkerType: { ArrowClosed: "arrowclosed" },
}));
vi.mock("@vue-flow/background", () => ({
  Background: defineComponent({ template: "<div />" }),
}));
vi.mock("@vue-flow/controls", () => ({
  Controls: defineComponent({ template: "<div />" }),
}));
vi.mock("@vue-flow/minimap", () => ({
  MiniMap: defineComponent({ template: "<div />" }),
}));
const mockCodeMirror = vi.hoisted(() => ({
  onChange: null as ((value: string) => void) | null,
}));
vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: (
    _parent: HTMLElement,
    options: { onChange?: (value: string) => void },
  ) => {
    mockCodeMirror.onChange = options.onChange ?? null;
    return { view: { destroy() {} } };
  },
  createCodeMirrorDiffView: () => ({ destroy() {}, syncTheme() {} }),
  editorValue: () => "",
  setEditorValue: () => {},
  setEditorLanguage: () => {},
  setEditorReadOnly: () => {},
  syncCodeMirrorTheme: () => {},
}));

beforeEach(() => {
  installFetch((url, init) => {
    if (url.includes("graph")) {
      return {
        body: {
          nodes: [
            {
              id: "api",
              label: "API",
              group: "service",
              properties: { runtime: "go" },
            },
            { id: "db", label: "Database", group: "database" },
          ],
          edges: [{ source: "api", target: "db", label: "queries" }],
        },
      };
    }
    if (url.includes("trace")) {
      return {
        body: {
          traceId: "t1",
          spans: [
            {
              id: "root",
              name: "GET /users",
              service: "api",
              startMs: 0,
              durationMs: 50,
            },
            {
              id: "db",
              parentId: "root",
              name: "select users",
              service: "postgres",
              startMs: 10,
              durationMs: 20,
              tags: { table: "users" },
            },
          ],
        },
      };
    }
    if (url.includes("kv.read")) {
      return {
        body: {
          key: "session:1",
          type: "json",
          value: { user: "ada" },
        },
      };
    }
    if (url.includes("kv.list")) {
      return {
        body: {
          items: [{ key: "session:1", type: "json" }],
          nextCursor: "",
        },
      };
    }
    if (url.includes("kv.write") && init?.method === "PUT") {
      return { body: { ok: true } };
    }
    if (url.includes("http.exec") && init?.method === "POST") {
      return {
        body: {
          status: 200,
          durationMs: 12.4,
          headers: { "content-type": "application/json" },
          body: { ok: true },
        },
      };
    }
    if (url.includes("diff")) {
      return {
        body: {
          before: "kind: Pod\n",
          after: "kind: Service\n",
        },
      };
    }
    return { body: {} };
  });
});

afterEach(() => {
  document.body.innerHTML = "";
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function bodyButton(text: string): HTMLButtonElement | undefined {
  return [...document.body.querySelectorAll("button")].find(
    (button) => button.textContent?.trim() === text,
  ) as HTMLButtonElement | undefined;
}

function mountKVWithConfirm(props: InstanceType<typeof KVPanel>["$props"]) {
  const host = document.createElement("div");
  document.body.appendChild(host);
  return mount(
    {
      render: () => h("div", [h(KVPanel, props), h(ConfirmDialog)]),
    },
    { attachTo: host },
  );
}

function selectedKVKey(w: ReturnType<typeof mount>): string | undefined {
  return (
    w.findComponent({ name: "DataTable" }).props("selection") as
      | { key?: string }
      | null
      | undefined
  )?.key;
}

async function selectKVRow(
  w: ReturnType<typeof mount>,
  entry: { key: string; type?: string },
): Promise<void> {
  const table = w.findComponent({ name: "DataTable" });
  table.vm.$emit("update:selection", entry);
  table.vm.$emit("row-select", { data: entry });
  await flushPromises();
}

describe("specialized panels", () => {
  it("renders a configured diff payload", async () => {
    const w = mount(DiffPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "diff" },
        config: {
          language: "yaml",
          originalField: "before",
          modifiedField: "after",
          originalLabel: "Current",
          modifiedLabel: "Proposed",
        },
      },
    });
    await flushPromises();
    await flushPromises();

    expect(w.text()).toContain("Current");
    expect(w.text()).toContain("Proposed");
    w.unmount();
  });

  it("renders graph nodes and node details", async () => {
    const w = mount(GraphPanel, {
      props: { connectionId: "c1", source: { routeId: "graph" } },
      global: { plugins: [ToastService] },
    });
    await flushPromises();
    await flushPromises();

    expect(w.text()).toContain("API");
    await w.get('[data-test="graph"] button').trigger("click");
    expect(w.text()).toContain("runtime");
  });

  it("keeps graph content visible during refresh", async () => {
    let calls = 0;
    let resolveRefresh: ((value: Response) => void) | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        calls += 1;
        if (calls === 1) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                nodes: [{ id: "api", label: "API", group: "service" }],
                edges: [],
              }),
              { headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        return new Promise((resolve) => {
          resolveRefresh = resolve;
        });
      }),
    );

    const w = mount(GraphPanel, {
      props: { connectionId: "c1", source: { routeId: "graph" } },
      global: { plugins: [ToastService] },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(w.find('[data-test="panel-loader"]').exists()).toBe(false);
    expect(w.text()).toContain("API");

    resolveRefresh?.(
      new Response(
        JSON.stringify({
          nodes: [{ id: "worker", label: "Worker", group: "service" }],
          edges: [],
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
    );
    await flushPromises();

    expect(w.text()).toContain("Worker");
  });

  it("exports the graph viewport as a PNG", async () => {
    const w = mount(GraphPanel, {
      props: { connectionId: "c1", source: { routeId: "graph" } },
      global: { plugins: [ToastService] },
    });
    await flushPromises();
    await flushPromises();

    await w.get('button[aria-label="Export graph"]').trigger("click");
    await flushPromises();
    const pngItem = [...document.body.querySelectorAll("span")].find(
      (el) => el.textContent === "PNG",
    );
    expect(pngItem).toBeTruthy();
    const click = vi.fn();
    const anchor = { click } as unknown as HTMLAnchorElement;
    const createElement = vi
      .spyOn(document, "createElement")
      .mockReturnValue(anchor);
    pngItem?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    await flushPromises();

    expect(toPng).toHaveBeenCalledWith(
      expect.objectContaining({
        className: expect.stringContaining("vue-flow"),
      }),
      expect.objectContaining({ cacheBust: true, pixelRatio: 2 }),
    );
    const options = vi.mocked(toPng).mock.calls[0][1];
    expect(
      options?.filter?.(
        document.createTextNode("edge label") as unknown as HTMLElement,
      ),
    ).toBe(true);
    expect(click).toHaveBeenCalled();
    createElement.mockRestore();
  });

  it("renders trace spans as a selectable waterfall", async () => {
    const w = mount(TracePanel, {
      props: { connectionId: "c1", source: { routeId: "trace" } },
    });
    await flushPromises();

    expect(w.text()).toContain("GET /users");
    expect(w.text()).toContain("select users");
    await w.findAll("tbody tr")[1].trigger("click");
    expect(w.text()).toContain("table");
  });

  it("keeps trace spans visible during refresh", async () => {
    let calls = 0;
    let resolveRefresh: ((value: Response) => void) | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn(() => {
        calls += 1;
        if (calls === 1) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                traceId: "t1",
                spans: [
                  {
                    id: "root",
                    name: "GET /users",
                    service: "api",
                    startMs: 0,
                    durationMs: 50,
                  },
                ],
              }),
              { headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        return new Promise((resolve) => {
          resolveRefresh = resolve;
        });
      }),
    );

    const w = mount(TracePanel, {
      props: { connectionId: "c1", source: { routeId: "trace" } },
    });
    await flushPromises();

    await w
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(w.text()).toContain("GET /users");

    resolveRefresh?.(
      new Response(
        JSON.stringify({
          traceId: "t1",
          spans: [
            {
              id: "root",
              name: "POST /orders",
              service: "api",
              startMs: 0,
              durationMs: 42,
            },
          ],
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
    );
    await flushPromises();

    expect(w.text()).toContain("POST /orders");
  });

  it("loads and edits a typed key value", async () => {
    const w = mount(KVPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kv.list" },
        config: { readRouteId: "kv.read", writable: true },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("session:1");
    expect(w.find(".shellcn-codemirror-host").exists()).toBe(true);
  });

  it("keeps editing when KV key selection is canceled with unsaved changes", async () => {
    const reads: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("kv.list")) {
        return {
          body: {
            items: [
              { key: "session:1", type: "json" },
              { key: "session:2", type: "json" },
            ],
            nextCursor: "",
          },
        };
      }
      if (url.includes("kv.read")) {
        reads.push(url);
        const key = new URL(url, "http://h").searchParams.get("p.key");
        return {
          body: {
            key,
            type: "json",
            value: { user: key === "session:2" ? "grace" : "ada" },
          },
        };
      }
      return { body: {} };
    });
    const w = mountKVWithConfirm({
      connectionId: "c1",
      source: { routeId: "kv.list" },
      config: { readRouteId: "kv.read", writable: true },
    });
    await flushPromises();

    expect(selectedKVKey(w)).toBe("session:1");
    mockCodeMirror.onChange?.('{"user":"edited"}');
    await flushPromises();
    expect(w.text()).toContain("Unsaved");
    await selectKVRow(w, { key: "session:2", type: "json" });
    bodyButton("Keep editing")!.click();
    await flushPromises();

    expect(selectedKVKey(w)).toBe("session:1");
    expect(w.text()).toContain("Unsaved");
    expect(reads.filter((url) => url.includes("session%3A2"))).toHaveLength(0);
    w.unmount();
  });

  it("discards unsaved KV edits before selecting another key", async () => {
    const reads: string[] = [];
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("kv.list")) {
        return {
          body: {
            items: [
              { key: "session:1", type: "json" },
              { key: "session:2", type: "json" },
            ],
            nextCursor: "",
          },
        };
      }
      if (url.includes("kv.read")) {
        reads.push(url);
        const key = new URL(url, "http://h").searchParams.get("p.key");
        return {
          body: {
            key,
            type: "json",
            value: { user: key === "session:2" ? "grace" : "ada" },
          },
        };
      }
      return { body: {} };
    });
    const w = mountKVWithConfirm({
      connectionId: "c1",
      source: { routeId: "kv.list" },
      config: { readRouteId: "kv.read", writable: true },
    });
    await flushPromises();

    expect(selectedKVKey(w)).toBe("session:1");
    mockCodeMirror.onChange?.('{"user":"edited"}');
    await flushPromises();
    expect(w.text()).toContain("Unsaved");
    await selectKVRow(w, { key: "session:2", type: "json" });
    bodyButton("Discard changes")!.click();
    await flushPromises();

    expect(selectedKVKey(w)).toBe("session:2");
    expect(w.text()).not.toContain("Unsaved");
    expect(reads.some((url) => url.includes("session%3A2"))).toBe(true);
    w.unmount();
  });

  it("preserves the active KV key when refreshing the list", async () => {
    let listCalls = 0;
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.includes("kv.list")) {
        listCalls += 1;
        return {
          body: {
            items:
              listCalls === 1
                ? [
                    { key: "session:1", type: "json" },
                    { key: "session:2", type: "json" },
                  ]
                : [
                    { key: "session:2", type: "json" },
                    { key: "session:3", type: "json" },
                  ],
            nextCursor: "",
          },
        };
      }
      if (url.includes("kv.read")) {
        const key = new URL(url, "http://h").searchParams.get("p.key");
        return {
          body: {
            key,
            type: "json",
            value: { user: key },
          },
        };
      }
      return { body: {} };
    });
    const w = mount(KVPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kv.list" },
        config: { readRouteId: "kv.read" },
      },
    });
    await flushPromises();

    await selectKVRow(w, { key: "session:2", type: "json" });
    expect(selectedKVKey(w)).toBe("session:2");
    await w
      .findAllComponents(Button)
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(selectedKVKey(w)).toBe("session:2");
    expect(w.text()).toContain("session:3");
    w.unmount();
  });

  it("keeps KV refresh loading state on the refresh button", async () => {
    let listCalls = 0;
    let resolveRefresh: (() => void) | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();
        if (url.includes("kv.read")) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                key: "session:1",
                type: "json",
                value: { user: "ada" },
              }),
              { headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        if (url.includes("kv.list")) {
          listCalls += 1;
          if (listCalls === 1) {
            return Promise.resolve(
              new Response(
                JSON.stringify({
                  items: [{ key: "session:1", type: "json" }],
                  nextCursor: "",
                }),
                { headers: { "Content-Type": "application/json" } },
              ),
            );
          }
          return new Promise((resolve) => {
            resolveRefresh = () =>
              resolve(
                new Response(
                  JSON.stringify({
                    items: [{ key: "session:1", type: "json" }],
                    nextCursor: "",
                  }),
                  { headers: { "Content-Type": "application/json" } },
                ),
              );
          });
        }
        return Promise.resolve(
          new Response(JSON.stringify({}), {
            headers: { "Content-Type": "application/json" },
          }),
        );
      }),
    );
    const w = mount(KVPanel, {
      props: { connectionId: "c1", source: { routeId: "kv.list" } },
    });
    await flushPromises();

    const refresh = () =>
      w
        .findAllComponents(Button)
        .find((button) => button.text().includes("Refresh"))!;
    await refresh().trigger("click");

    expect(refresh().find(".animate-spin").exists()).toBe(true);
    resolveRefresh?.();
    await flushPromises();
    expect(refresh().find(".animate-spin").exists()).toBe(false);
  });

  it("keeps KV rows visible when refresh fails", async () => {
    let listCalls = 0;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = typeof input === "string" ? input : input.toString();
        if (url.includes("kv.read")) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                key: "session:1",
                type: "json",
                value: { user: "ada" },
              }),
              { headers: { "Content-Type": "application/json" } },
            ),
          );
        }
        if (url.includes("kv.list")) {
          listCalls += 1;
          if (listCalls === 1) {
            return Promise.resolve(
              new Response(
                JSON.stringify({
                  items: [{ key: "session:1", type: "json" }],
                  nextCursor: "",
                }),
                { headers: { "Content-Type": "application/json" } },
              ),
            );
          }
          return Promise.resolve(
            new Response(JSON.stringify({ error: "refresh failed" }), {
              status: 500,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(JSON.stringify({}), {
            headers: { "Content-Type": "application/json" },
          }),
        );
      }),
    );
    const w = mount(KVPanel, {
      props: { connectionId: "c1", source: { routeId: "kv.list" } },
    });
    await flushPromises();
    expect(w.text()).toContain("session:1");

    await w
      .findAllComponents(Button)
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(w.text()).toContain("session:1");
    expect(w.text()).toContain("refresh failed");
    expect(w.find('[data-test="skeleton-list"]').exists()).toBe(false);
  });

  it("shows key creation only when the generic kv create route is declared", async () => {
    const readOnlyCreate = mount(KVPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kv.list" },
        config: {
          readRouteId: "kv.read",
          writeRouteId: "kv.write",
          keyParam: "key",
          writable: true,
        },
      },
    });
    await flushPromises();

    expect(readOnlyCreate.text()).not.toContain("New");

    const w = mount(KVPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "kv.list" },
        config: {
          createRouteId: "kv.write",
          readRouteId: "kv.read",
          writeRouteId: "kv.write",
          keyParam: "key",
          writable: true,
        },
      },
    });
    await flushPromises();

    expect(w.text()).toContain("New");
  });

  it("executes a declarative HTTP request", async () => {
    const w = mount(HTTPClientPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "http.exec" },
        config: { defaultUrl: "/health" },
      },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((button) => button.text() === "Send")!
      .trigger("click");
    await flushPromises();

    expect(w.text()).toContain("200");
    expect(w.findAll(".shellcn-codemirror-host")).toHaveLength(2);
  });

  it("renders HTTP request controls with visible labels and input styling", () => {
    const w = mount(HTTPClientPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "http.exec" },
      },
    });

    expect(w.text()).toContain("Method");
    expect(w.text()).toContain("Request URL");
    const urlInput = w.get('input[aria-label="Request URL"]');
    expect(urlInput.classes()).toContain("border");
    expect(urlInput.classes()).toContain("text-surface-800");
  });
});
