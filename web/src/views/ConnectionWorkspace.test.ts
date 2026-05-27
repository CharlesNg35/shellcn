import { defineComponent } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { createPinia, setActivePinia } from "pinia";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import { useWorkspaceStore } from "../stores/workspace";
import type { PluginProjection } from "../types/projection";
import ConnectionWorkspace from "./ConnectionWorkspace.vue";

const projection: PluginProjection = {
  apiVersion: 2,
  name: "docker",
  version: "0.1.0",
  title: "Docker",
  description: "Docker test projection",
  icon: { type: "lucide", value: "box" },
  category: {
    key: "containers",
    label: "Containers",
    icon: { type: "lucide", value: "boxes" },
    order: 30,
  },
  config: { groups: [] },
  capabilities: [],
  supportedTransports: ["direct"],
  layout: "sidebar_tree",
  tree: [
    {
      key: "containers",
      label: "Containers",
      source: { routeId: "docker.container.tree" },
      resourceKind: "container",
    },
  ],
  resources: [
    {
      kind: "container",
      title: "Containers",
      list: { routeId: "docker.container.list" },
      columns: [],
      actionIds: [],
      detail: { header: {}, tabs: [] },
    },
  ],
  actions: [],
};

const TablePanelStub = defineComponent({
  emits: ["select"],
  template: `
    <div>
      <button data-test="unknown" @click="$emit('select', { name: 'snapshot', ref: { kind: 'snapshot', name: '100', uid: 'snap1' } })">unknown</button>
      <button data-test="known" @click="$emit('select', { name: 'web', ref: { kind: 'container', name: 'web', uid: 'abc' } })">known</button>
    </div>
  `,
});

let requests: Array<{ url: string; init?: RequestInit }>;

function router() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/", component: { template: "<div />" } }],
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
  requests = [];
  installFetch((url, init) => {
    requests.push({ url, init });
    if (url.endsWith("/api/connections")) {
      return {
        body: [
          {
            id: "c1",
            name: "docker",
            protocol: "docker",
            transport: "direct",
          },
        ],
      };
    }
    if (url.endsWith("/api/connections/c1/session")) {
      return { body: { ok: true } };
    }
    if (url.endsWith("/api/connection-folders")) return { body: [] };
    if (url.endsWith("/api/plugins/docker")) return { body: projection };
    if (url.endsWith("/api/plugins")) return { body: [] };
    return { status: 404, body: { error: "not found" } };
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("ConnectionWorkspace", () => {
  it("ignores selectable table refs that are not declared resources", async () => {
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);
    ws.openView("c1", {
      id: "group:containers",
      title: "Containers",
      kind: "list",
      groupKey: "containers",
    });

    const wrapper = mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: {
        plugins: [router()],
        stubs: {
          AppIcon: true,
          DetailView: true,
          ResourceTree: true,
          TablePanel: TablePanelStub,
        },
      },
    });
    await flushPromises();

    await wrapper.get('[data-test="unknown"]').trigger("click");
    expect(ws.view("c1").views.some((v) => v.kind === "detail")).toBe(false);

    await wrapper.get('[data-test="known"]').trigger("click");
    const opened = ws.view("c1").views.find((v) => v.kind === "detail");
    expect(opened?.ref?.kind).toBe("container");
  });

  it("closes the backend plugin session when disconnecting", async () => {
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);

    const wrapper = mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: {
        plugins: [router()],
        stubs: {
          AppIcon: true,
          DetailView: true,
          ResourceTree: true,
          TablePanel: TablePanelStub,
        },
      },
    });
    await flushPromises();

    const button = wrapper
      .findAll("button")
      .find((candidate) => candidate.text().includes("Disconnect"));
    expect(button).toBeTruthy();
    await button!.trigger("click");
    await flushPromises();

    expect(ws.isConnected("c1")).toBe(false);
    expect(
      requests.some(
        (request) =>
          request.url.endsWith("/api/connections/c1/session") &&
          request.init?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("renders every panel as a card in the dashboard layout", async () => {
    const dashboard: PluginProjection = {
      ...projection,
      layout: "dashboard",
      tree: [],
      resources: [],
      tabs: [
        {
          key: "overview",
          label: "Overview",
          panel: "document",
          source: { routeId: "x.overview" },
        },
        {
          key: "logs",
          label: "Logs",
          panel: "log_stream",
          source: { routeId: "x.logs" },
          span: 2,
        },
      ],
    };
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections")) {
        return {
          body: [
            {
              id: "c1",
              name: "docker",
              protocol: "docker",
              transport: "direct",
            },
          ],
        };
      }
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { ok: true } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: dashboard };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });

    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);

    const wrapper = mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: {
        plugins: [router()],
        stubs: { AppIcon: true, PanelHost: true },
      },
    });
    await flushPromises();

    const cards = wrapper.findAll("section");
    expect(cards).toHaveLength(2);
    expect(wrapper.text()).toContain("Overview");
    expect(wrapper.text()).toContain("Logs");
    // The span=2 panel fills the row.
    expect(cards[1].classes()).toContain("lg:col-span-2");
  });

  it("asks the browser to confirm reload while connected", async () => {
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);

    mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: {
        plugins: [router()],
        stubs: {
          AppIcon: true,
          DetailView: true,
          ResourceTree: true,
          TablePanel: TablePanelStub,
        },
      },
    });
    await flushPromises();

    const event = new Event("beforeunload", {
      cancelable: true,
    }) as BeforeUnloadEvent;
    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
  });
});
