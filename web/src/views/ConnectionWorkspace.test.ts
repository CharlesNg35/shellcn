import { defineComponent } from "vue";
import { mount, flushPromises } from "@vue/test-utils";
import { createMemoryHistory, createRouter } from "vue-router";
import { createPinia, setActivePinia } from "pinia";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { installFetch } from "../test/fetchMock";
import { useWorkspaceStore } from "../stores/workspace";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import type { PluginProjection } from "../types/projection";
import { Layout } from "../types/projection";
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
  layout: Layout.SidebarTree,
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
      return {
        body: {
          state: "connected",
          channels: 0,
          streams: 0,
          lastSeen: "2026-05-28T00:00:00Z",
        },
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
  it("centers the workspace loading state", async () => {
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
      if (url.endsWith("/api/plugins/docker")) return { body: projection };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });

    const wrapper = mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: {
        plugins: [router()],
        stubs: {
          AppIcon: true,
          PanelHost: true,
        },
      },
    });

    const loading = wrapper.get('[role="status"]');
    expect(loading.text()).toBe("Loading workspace…");
    expect(loading.classes()).toEqual(
      expect.arrayContaining([
        "flex",
        "h-full",
        "items-center",
        "justify-center",
      ]),
    );
  });

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

  it("renders manifest header actions in the workspace header when connected", async () => {
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);

    const withHeader: PluginProjection = {
      ...projection,
      actions: [
        {
          id: "k.shell",
          label: "Cluster Shell",
          icon: { type: "lucide", value: "terminal" },
          routeId: "k.shell",
          method: "GET",
          risk: "privileged",
          requiresConfirm: false,
          open: "dock",
          panel: "terminal",
          iconOnly: true,
        },
      ],
      headerActions: ["k.shell"],
    };
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections"))
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
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { state: "connected", channels: 0, streams: 0 } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: withHeader };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
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

    expect(
      wrapper
        .findAll("button")
        .some((b) => b.attributes("aria-label") === "Cluster Shell"),
    ).toBe(true);
  });

  it("opens the backend plugin session before entering the workspace", async () => {
    const ws = useWorkspaceStore();

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
      .find((candidate) => candidate.text().includes("Connect"));
    expect(button).toBeTruthy();
    await button!.trigger("click");
    await flushPromises();

    expect(ws.isConnected("c1")).toBe(true);
    expect(
      requests.some(
        (request) =>
          request.url.endsWith("/api/connections/c1/session") &&
          request.init?.method === "POST",
      ),
    ).toBe(true);
  });

  it("keeps the connect gate visible when the backend session reports an error", async () => {
    vi.unstubAllGlobals();
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
        return {
          body: {
            state: "error",
            reason: "docker ping failed",
            channels: 0,
            streams: 0,
          },
        };
      }
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: projection };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });

    const ws = useWorkspaceStore();
    const live = useConnectionStatusStore();
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
      .find((candidate) => candidate.text().includes("Connect"));
    expect(button).toBeTruthy();
    await button!.trigger("click");
    await flushPromises();

    expect(ws.isConnected("c1")).toBe(false);
    expect(live.get("c1")).toEqual({
      state: "error",
      reason: "docker ping failed",
    });
    expect(wrapper.find('[role="alert"]').text()).toContain(
      "docker ping failed",
    );
  });

  it("renders every panel as a card in the dashboard layout", async () => {
    const dashboard: PluginProjection = {
      ...projection,
      layout: Layout.Dashboard,
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
        return {
          body: {
            state: "connected",
            channels: 0,
            streams: 0,
            lastSeen: "2026-05-28T00:00:00Z",
          },
        };
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

  it("restores the active tab from ?v= and syncs tab switches back to the URL", async () => {
    const tabsProj: PluginProjection = {
      ...projection,
      layout: Layout.Tabs,
      tree: [],
      resources: [],
      tabs: [
        {
          key: "overview",
          label: "Overview",
          panel: "document",
          source: { routeId: "x.o" },
        },
        { key: "keys", label: "Keys", panel: "kv", source: { routeId: "x.k" } },
      ],
    };
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections"))
        return {
          body: [
            {
              id: "c1",
              name: "redis",
              protocol: "docker",
              transport: "direct",
            },
          ],
        };
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { state: "connected", channels: 0, streams: 0 } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: tabsProj };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);
    const r = router();
    await r.push("/?v=keys");

    mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: { plugins: [r], stubs: { AppIcon: true, PanelHost: true } },
    });
    await flushPromises();
    // Deep link restored the Keys tab, not the default first tab.
    expect(ws.view("c1").activeTab).toBe("keys");

    ws.setActiveTab("c1", "overview");
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("overview");

    // A tab switch uses replace, so Back does NOT step to the previous tab.
    await r.back();
    await flushPromises();
    expect(r.currentRoute.value.query.v).not.toBe("keys");
  });

  it("pushes ?v= per opened view and reconstructs it on Back (sidebar_tree)", async () => {
    let pluginFetches = 0;
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections"))
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
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { state: "connected", channels: 0, streams: 0 } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) {
        pluginFetches += 1;
        return { body: projection };
      }
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);
    const r = router();

    mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: { plugins: [r], stubs: { AppIcon: true, TreeWorkspace: true } },
    });
    await flushPromises();

    ws.openPreviewView("c1", {
      id: "group:containers",
      title: "Containers",
      kind: "list",
      groupKey: "containers",
    });
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("group:containers");

    ws.openPreviewView("c1", {
      id: "detail:abc",
      title: "web",
      kind: "detail",
      ref: { kind: "container", uid: "abc", name: "web" },
    });
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("detail:container:abc:n=web");

    await r.back();
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("group:containers");
    // The replaced preview is reconstructed from the URL and made active.
    expect(ws.activeView("c1")?.id).toBe("group:containers");
    // Navigation never re-fetched the projection (workspace was not remounted).
    expect(pluginFetches).toBe(1);
  });

  it("replaces (no new history) when switching between already-open workbench tabs", async () => {
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections"))
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
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { state: "connected", channels: 0, streams: 0 } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: projection };
      if (url.endsWith("/api/plugins")) return { body: [] };
      return { status: 404, body: { error: "not found" } };
    });
    const ws = useWorkspaceStore();
    ws.setConnected("c1", true);
    const r = router();
    mount(ConnectionWorkspace, {
      props: { id: "c1" },
      global: { plugins: [r], stubs: { AppIcon: true, TreeWorkspace: true } },
    });
    await flushPromises();

    // Two views open at once (pinned), each a new resource → each pushed.
    ws.openView("c1", {
      id: "group:containers",
      title: "Containers",
      kind: "list",
      groupKey: "containers",
    });
    await flushPromises();
    ws.openView("c1", {
      id: "detail:abc",
      title: "web",
      kind: "detail",
      ref: { kind: "container", uid: "abc", name: "web" },
    });
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("detail:container:abc:n=web");

    // Switching back to the already-open first tab replaces — it does NOT push.
    ws.activateView("c1", "group:containers");
    await flushPromises();
    expect(r.currentRoute.value.query.v).toBe("group:containers");

    // So Back lands on the entry *before* the detail (group), not the detail.
    await r.back();
    await flushPromises();
    expect(r.currentRoute.value.query.v).not.toBe("detail:container:abc:n=web");
  });

  it("renders a single full-bleed panel with no tab bar in the single layout", async () => {
    const single: PluginProjection = {
      ...projection,
      layout: Layout.Single,
      tree: [],
      resources: [],
      tabs: [
        {
          key: "desktop",
          label: "Desktop",
          panel: "remote_desktop",
          source: { routeId: "x.desktop" },
        },
      ],
    };
    vi.unstubAllGlobals();
    installFetch((url) => {
      if (url.endsWith("/api/connections"))
        return {
          body: [
            { id: "c1", name: "vnc", protocol: "docker", transport: "direct" },
          ],
        };
      if (url.endsWith("/api/connections/c1/session"))
        return { body: { state: "connected", channels: 0, streams: 0 } };
      if (url.endsWith("/api/connection-folders")) return { body: [] };
      if (url.endsWith("/api/plugins/docker")) return { body: single };
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

    expect(wrapper.find("panel-host-stub").exists()).toBe(true);
    // No tab bar chrome for a single screen.
    expect(wrapper.find('[role="tablist"]').exists()).toBe(false);
  });
});
