import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { createRouter, createMemoryHistory } from "vue-router";
import { installFetch } from "../test/fetchMock";
import { useConnectionsStore } from "../stores/connections";
import ConnectionSidebar from "./ConnectionSidebar.vue";
import type { ConnectionSummary } from "../types/projection";

vi.mock("vue-draggable-plus", () => ({
  VueDraggable: {
    name: "VueDraggable",
    props: ["modelValue"],
    emits: ["update:modelValue", "end"],
    template: '<div data-draggable="true"><slot /></div>',
  },
}));

const routes = [
  { path: "/", name: "home", component: { template: "<div />" } },
  {
    path: "/connections/:id",
    name: "connection",
    component: { template: "<div />" },
  },
];

function router() {
  return createRouter({ history: createMemoryHistory(), routes });
}

const icon = { type: "name" as const, value: "server" };
const connections: ConnectionSummary[] = [
  {
    id: "c-root",
    name: "Bastion",
    protocol: "ssh",
    icon,
    transport: "direct",
    sortOrder: 0,
  },
  {
    id: "c-prod",
    name: "Prod DB",
    protocol: "postgres",
    icon,
    transport: "direct",
    folderId: "f1",
    sortOrder: 0,
  },
];

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});
afterEach(() => vi.unstubAllGlobals());

describe("ConnectionSidebar", () => {
  it("renders folders and expands the active connection folder", async () => {
    installFetch(() => ({ body: { ok: true } }));
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.folders = [
      { id: "f1", name: "Production", color: "blue", sortOrder: 0 },
    ];
    conns.connections = connections;

    const wrapper = mount(ConnectionSidebar, {
      props: { activeId: "c-prod", query: "" },
      global: { plugins: [router()] },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Production");
    expect(wrapper.text()).toContain("Prod DB");
    expect(
      JSON.parse(
        localStorage.getItem("shellcn:connection-folders:expanded") ?? "{}",
      ),
    ).toMatchObject({
      f1: true,
    });
  });

  it("persists connection order and folder placement", async () => {
    let saved: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.endsWith("/api/connections/layout") && init?.method === "PUT") {
        saved = JSON.parse(String(init.body));
      }
      return { body: { ok: true } };
    });
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.folders = [
      { id: "f1", name: "Production", color: "blue", sortOrder: 0 },
    ];
    conns.connections = connections;

    const wrapper = mount(ConnectionSidebar, {
      props: { activeId: null, query: "" },
      global: { plugins: [router()] },
    });
    await flushPromises();
    wrapper.findComponent({ name: "VueDraggable" }).vm.$emit("end");
    await flushPromises();

    expect(saved).toMatchObject({
      items: expect.arrayContaining([
        { connectionId: "c-root", sortOrder: 0 },
        { connectionId: "c-prod", folderId: "f1", sortOrder: 0 },
      ]),
    });
  });
});
