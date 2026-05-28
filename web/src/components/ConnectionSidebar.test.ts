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
    emits: ["update:modelValue", "choose", "start", "update", "end"],
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

const icon = { type: "lucide" as const, value: "server" };
const connections: ConnectionSummary[] = [
  {
    id: "c-root",
    name: "Bastion",
    protocol: "ssh",
    icon,
    transport: "direct",
    sortOrder: 1,
  },
  {
    id: "c-prod",
    name: "Prod DB",
    protocol: "postgres",
    icon,
    transport: "direct",
    folderId: "f2",
    sortOrder: 0,
  },
];

beforeEach(() => {
  setActivePinia(createPinia());
  localStorage.clear();
});
afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("ConnectionSidebar", () => {
  it("renders folders and expands the active connection folder", async () => {
    installFetch(() => ({ body: { ok: true } }));
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.folders = [
      { id: "f1", name: "Production", color: "blue", sortOrder: 0 },
      {
        id: "f2",
        parentId: "f1",
        name: "Databases",
        color: "teal",
        sortOrder: 0,
      },
    ];
    conns.connections = connections;

    const wrapper = mount(ConnectionSidebar, {
      props: { activeId: "c-prod", query: "" },
      global: { plugins: [router()] },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Production");
    expect(wrapper.text()).toContain("Databases");
    expect(wrapper.text()).toContain("Prod DB");

    const connectionButton = wrapper.get(
      '[data-connection-id="c-prod"] button',
    );
    expect(connectionButton.attributes("title")).toBe("Prod DB");
    expect(connectionButton.attributes("aria-label")).toBe("Open Prod DB");
    expect(
      wrapper.get('[data-connection-id="c-prod"] button span').classes(),
    ).toEqual(expect.arrayContaining(["block", "max-w-full", "truncate"]));
    expect(wrapper.get('[data-connection-id="c-prod"]').classes()).toEqual(
      expect.arrayContaining([
        "connection-sidebar-drag-item",
        "mx-1",
        "w-[calc(100%-0.5rem)]",
      ]),
    );
    expect(
      wrapper.get('[data-connection-id="c-prod"] > span').classes(),
    ).not.toContain("cursor-grab");

    const folderButton = wrapper.get('[data-folder-id="f1"] > div button');
    expect(folderButton.attributes("title")).toBe("Production");
    expect(folderButton.attributes("aria-label")).toBe("Collapse Production");
    expect(wrapper.get('[data-folder-id="f1"] > div').classes()).toContain(
      "connection-sidebar-drag-item",
    );
    expect(
      wrapper.get('[data-folder-id="f1"] > div > span').classes(),
    ).not.toContain("cursor-grab");
    expect(
      JSON.parse(
        localStorage.getItem("shellcn:connection-folders:expanded") ?? "{}",
      ),
    ).toMatchObject({
      f1: true,
      f2: true,
    });
  });

  it("reveals a top shadow after the connection list scrolls", async () => {
    installFetch(() => ({ body: { ok: true } }));
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.connections = connections;

    const wrapper = mount(ConnectionSidebar, {
      props: { activeId: null, query: "" },
      global: { plugins: [router()] },
    });
    await flushPromises();

    const shadow = wrapper.get("[data-sidebar-scroll-shadow]");
    const scroller = wrapper.get("[data-sidebar-scroll-region]");
    expect(shadow.classes()).toContain("opacity-0");

    Object.defineProperty(scroller.element, "scrollTop", {
      value: 24,
      configurable: true,
    });
    await scroller.trigger("scroll");

    expect(shadow.classes()).toContain("opacity-100");
  });

  it("suppresses sidebar row hover backgrounds while dragging", async () => {
    vi.useFakeTimers();
    installFetch((url) => {
      if (url.endsWith("/api/connections")) return { body: connections };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      return { body: { ok: true } };
    });
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.connections = connections;

    const wrapper = mount(ConnectionSidebar, {
      props: { activeId: null, query: "" },
      global: { plugins: [router()] },
    });
    await flushPromises();

    const draggable = wrapper.findComponent({ name: "VueDraggable" });
    const item = () => wrapper.get('[data-connection-id="c-root"]');
    expect(item().classes()).toContain("hover:bg-surface-100");

    draggable.vm.$emit("choose");
    await flushPromises();
    expect(item().classes()).not.toContain("hover:bg-surface-100");
    expect(wrapper.get("[data-sidebar-scroll-region]").classes()).toContain(
      "connection-sidebar-list--dragging",
    );

    draggable.vm.$emit("end");
    await vi.advanceTimersByTimeAsync(0);
    await flushPromises();
    expect(item().classes()).toContain("hover:bg-surface-100");
  });

  it("persists connection order and folder placement", async () => {
    let saved: Record<string, unknown> | null = null;
    installFetch((url, init) => {
      if (url.endsWith("/api/connections/layout") && init?.method === "PUT") {
        saved = JSON.parse(String(init.body));
      }
      if (url.endsWith("/api/connections")) return { body: connections };
      if (url.endsWith("/api/connection-folders")) {
        return {
          body: [
            { id: "f1", name: "Production", color: "blue", sortOrder: 0 },
            {
              id: "f2",
              parentId: "f1",
              name: "Databases",
              color: "teal",
              sortOrder: 0,
            },
          ],
        };
      }
      return { body: { ok: true } };
    });
    const conns = useConnectionsStore();
    conns.loaded = true;
    conns.folders = [
      { id: "f1", name: "Production", color: "blue", sortOrder: 0 },
      {
        id: "f2",
        parentId: "f1",
        name: "Databases",
        color: "teal",
        sortOrder: 0,
      },
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
      folders: expect.arrayContaining([
        { folderId: "f1", sortOrder: 0 },
        { folderId: "f2", parentId: "f1", sortOrder: 0 },
      ]),
      items: expect.arrayContaining([
        { connectionId: "c-root", sortOrder: 1 },
        { connectionId: "c-prod", folderId: "f2", sortOrder: 0 },
      ]),
    });
  });
});
