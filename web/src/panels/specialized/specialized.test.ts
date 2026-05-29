/* eslint-disable vue/one-component-per-file, vue/require-prop-types */
import { defineComponent } from "vue";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import Button from "primevue/button";
import { installFetch } from "../../test/fetchMock";
import GraphPanel from "./GraphPanel.vue";
import TracePanel from "./TracePanel.vue";
import KVPanel from "./KVPanel.vue";
import HTTPClientPanel from "./HTTPClientPanel.vue";

vi.mock("@vue-flow/core", () => ({
  VueFlow: defineComponent({
    props: ["nodes", "edges"],
    emits: ["node-click"],
    template:
      '<div data-test="graph"><button v-for="n in nodes" :key="n.id" type="button" @click="$emit(\'node-click\', { node: n })">{{ n.data.label }}</button><slot /></div>',
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
vi.mock("../../codemirror", () => ({
  createCodeMirrorEditor: () => ({ view: { destroy() {} } }),
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
    return { body: {} };
  });
});

afterEach(() => {
  document.body.innerHTML = "";
  vi.unstubAllGlobals();
});

describe("specialized panels", () => {
  it("renders graph nodes and node details", async () => {
    const w = mount(GraphPanel, {
      props: { connectionId: "c1", source: { routeId: "graph" } },
    });
    await flushPromises();
    await flushPromises();

    expect(w.text()).toContain("API");
    await w.get('[data-test="graph"] button').trigger("click");
    expect(w.text()).toContain("runtime");
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
