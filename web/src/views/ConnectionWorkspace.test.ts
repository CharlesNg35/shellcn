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

function router() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [{ path: "/", component: { template: "<div />" } }],
  });
}

beforeEach(() => {
  setActivePinia(createPinia());
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
    ws.selectGroup("c1", "containers");

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
    expect(ws.view("c1").selectedRef).toBeNull();

    await wrapper.get('[data-test="known"]').trigger("click");
    expect(ws.view("c1").selectedRef?.kind).toBe("container");
  });
});
