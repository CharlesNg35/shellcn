import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { nextTick } from "vue";
import TreeWorkspace from "./TreeWorkspace.vue";
import {
  DEFAULT_TREE_SIDEBAR_WIDTH,
  MAX_TREE_SIDEBAR_WIDTH,
  MIN_TREE_SIDEBAR_WIDTH,
  TREE_SIDEBAR_COLLAPSE_THRESHOLD,
  useWorkspaceStore,
} from "@/stores/workspace";
import { useScopeStore } from "@/stores/scope";

describe("TreeWorkspace", () => {
  let scrollIntoView: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    setActivePinia(createPinia());
    // jsdom doesn't implement scrollIntoView, so install a mock rather than spy.
    scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView =
      scrollIntoView as unknown as typeof Element.prototype.scrollIntoView;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mountWorkspace() {
    return mount(TreeWorkspace, {
      props: {
        connectionId: "c1",
        tree: [],
        resources: [],
        actions: [],
      },
      global: {
        stubs: {
          AppIcon: true,
          Button: {
            template: "<button><slot /></button>",
          },
          ResourceTree: true,
          VueDraggable: {
            template: '<div data-test="draggable"><slot /></div>',
          },
        },
      },
    });
  }

  it("scrolls the active workbench tab into view when a tab is appended", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();

    ws.openView("c1", {
      id: "list:first",
      title: "First",
      kind: "list",
      resourceKind: "missing",
    });
    await nextTick();
    await flushPromises();
    scrollIntoView.mockClear();

    ws.openView("c1", {
      id: "list:second",
      title: "Second",
      kind: "list",
      resourceKind: "missing",
    });
    await nextTick();
    await flushPromises();

    expect(wrapper.get("[data-active-tab='true']").text()).toContain("Second");
    expect(scrollIntoView).toHaveBeenCalledWith({
      block: "nearest",
      inline: "nearest",
      behavior: "smooth",
    });
  });

  it("bounds long workbench tab labels with a full title", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();
    const title = "public.github_app_installation_repositories";

    ws.openView("c1", {
      id: "list:long",
      title,
      subtitle: "production / analytics",
      kind: "list",
      resourceKind: "missing",
    });
    await nextTick();
    await flushPromises();

    const tab = wrapper.get(
      "button[title='public.github_app_installation_repositories - production / analytics ']",
    );
    expect(tab.classes()).toContain("max-w-60");
    expect(tab.classes()).toContain("overflow-hidden");
    expect(tab.get("span").classes()).toContain("min-w-0");
  });

  it("lets the resource tree fill the available sidebar height", () => {
    const wrapper = mountWorkspace();
    const root = wrapper.get(".flex.h-full.min-h-0");
    const shell = root.get("[data-test='resource-sidebar-shell']");
    const sidebar = root.get("[data-test='resource-sidebar']");

    expect(sidebar.classes()).toContain("overflow-hidden");
    expect(shell.attributes("style")).toContain(
      `width: ${DEFAULT_TREE_SIDEBAR_WIDTH}px`,
    );
  });

  it("resizes the resource sidebar in per-connection workspace state", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();
    const handle = wrapper.get("[data-test='resource-sidebar-resizer']");

    handle.element.dispatchEvent(
      new MouseEvent("pointerdown", { clientX: 100, bubbles: true }),
    );
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 180 }));
    await nextTick();

    expect(ws.layout("c1").treeSidebarWidth).toBe(
      DEFAULT_TREE_SIDEBAR_WIDTH + 80,
    );
    expect(
      wrapper.get("[data-test='resource-sidebar-shell']").attributes("style"),
    ).toContain(`width: ${DEFAULT_TREE_SIDEBAR_WIDTH + 80}px`);

    window.dispatchEvent(new MouseEvent("pointerup"));
  });

  it("collapses and expands the sidebar using a midpoint threshold", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();
    const handle = wrapper.get("[data-test='resource-sidebar-resizer']");

    handle.element.dispatchEvent(
      new MouseEvent("pointerdown", { clientX: 300, bubbles: true }),
    );
    window.dispatchEvent(new MouseEvent("pointermove", { clientX: 100 }));
    await nextTick();

    expect(ws.layout("c1").treeSidebarWidth).toBe(0);
    expect(
      wrapper.get("[data-test='resource-sidebar-shell']").attributes("style"),
    ).toContain("width: 0px");

    window.dispatchEvent(new MouseEvent("pointerup"));

    handle.element.dispatchEvent(
      new MouseEvent("pointerdown", { clientX: 100, bubbles: true }),
    );
    window.dispatchEvent(
      new MouseEvent("pointermove", {
        clientX: 100 + TREE_SIDEBAR_COLLAPSE_THRESHOLD,
      }),
    );
    await nextTick();

    expect(ws.layout("c1").treeSidebarWidth).toBe(0);

    window.dispatchEvent(
      new MouseEvent("pointermove", {
        clientX: 101 + TREE_SIDEBAR_COLLAPSE_THRESHOLD,
      }),
    );
    await nextTick();

    expect(ws.layout("c1").treeSidebarWidth).toBe(MIN_TREE_SIDEBAR_WIDTH);

    window.dispatchEvent(new MouseEvent("pointerup"));
  });

  it("supports keyboard resizing, collapse, and clamping", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();
    const handle = wrapper.get("[data-test='resource-sidebar-resizer']");

    await handle.trigger("keydown", { key: "ArrowRight" });
    expect(ws.layout("c1").treeSidebarWidth).toBe(
      DEFAULT_TREE_SIDEBAR_WIDTH + 24,
    );

    await handle.trigger("keydown", { key: "End" });
    expect(ws.layout("c1").treeSidebarWidth).toBe(MAX_TREE_SIDEBAR_WIDTH);

    await handle.trigger("keydown", { key: "Home" });
    expect(ws.layout("c1").treeSidebarWidth).toBe(0);

    await handle.trigger("keydown", { key: "ArrowRight" });
    expect(ws.layout("c1").treeSidebarWidth).toBe(MIN_TREE_SIDEBAR_WIDTH);
  });

  it("keeps a resource's actionIds out of row actions (rows are selectable only when RowActionIDs is declared)", async () => {
    let captured: Record<string, unknown> | null = null;
    const wrapper = mount(TreeWorkspace, {
      props: {
        connectionId: "c1",
        tree: [],
        resources: [
          {
            kind: "container",
            title: "Containers",
            list: { routeId: "docker.containers.list" },
            columns: [],
            actions: {
              toolbar: ["docker.container.create"],
              detail: ["docker.container.start", "docker.container.stop"],
              selectable: true,
            },
            detail: { header: { title: "x" }, tabs: [] },
          },
        ] as never,
        actions: [],
      },
      global: {
        stubs: {
          AppIcon: true,
          Button: { template: "<button><slot /></button>" },
          ResourceTree: true,
          VueDraggable: { template: "<div><slot /></div>" },
          PanelHost: {
            props: ["config", "source", "connectionId", "actions"],
            template: "<div data-test='table' />",
            created() {
              captured = (this as { config: Record<string, unknown> }).config;
            },
          },
        },
      },
    });
    const ws = useWorkspaceStore();
    ws.openView("c1", {
      id: "list:container",
      title: "Containers",
      kind: "list",
      resourceKind: "container",
    });
    await nextTick();
    await flushPromises();

    expect(captured).toBeTruthy();
    // The table toolbar comes from actions.toolbar; row actions only from
    // actions.row (here empty, so detail actions never leak onto rows).
    expect(captured!.actionIds).toEqual(["docker.container.create"]);
    expect(captured!.rowActionIds).toEqual([]);
    // Selectable still makes the rows selectable (checkboxes) without row actions.
    expect(captured!.selectable).toBe(true);
    wrapper.unmount();
  });

  it("keeps tree navigation expanded and refreshes the active list when connection scope changes", async () => {
    const scope = useScopeStore();
    scope.configure("c1", [{ param: "namespace" }]);
    let treeMounts = 0;
    let treeUnmounts = 0;
    let treeRefreshKey = "";
    let panelMounts = 0;

    mount(TreeWorkspace, {
      props: {
        connectionId: "c1",
        tree: [
          {
            key: "workloads",
            label: "Workloads",
            source: { routeId: "kubernetes.tree.workloads" },
          },
        ],
        resources: [
          {
            kind: "pod",
            title: "Pods",
            list: {
              routeId: "kubernetes.resource.list",
              params: { kind: "pod" },
            },
            columns: [],
            detail: { header: { title: "Pod" }, tabs: [] },
          },
        ] as never,
        actions: [],
      },
      global: {
        stubs: {
          AppIcon: true,
          Button: { template: "<button><slot /></button>" },
          ResourceTree: {
            name: "ResourceTree",
            props: [
              "connectionId",
              "groups",
              "selectedGroup",
              "selectedUid",
              "refreshKey",
            ],
            mounted() {
              treeMounts += 1;
            },
            updated() {
              treeRefreshKey = String(
                (this as { refreshKey?: string }).refreshKey ?? "",
              );
            },
            unmounted() {
              treeUnmounts += 1;
            },
            template: "<div data-test='tree' />",
          },
          VueDraggable: { template: "<div><slot /></div>" },
          PanelHost: {
            name: "PanelHost",
            props: ["panel", "connectionId", "source", "config", "actions"],
            mounted() {
              panelMounts += 1;
            },
            template: "<div data-test='panel-host' />",
          },
        },
      },
    });

    const ws = useWorkspaceStore();
    ws.openView("c1", {
      id: "list:pod",
      title: "Pods",
      kind: "list",
      resourceKind: "pod",
    });
    await nextTick();
    await flushPromises();

    expect(treeMounts).toBe(1);
    expect(panelMounts).toBe(1);

    scope.set("c1", "namespace", "prod");
    await nextTick();
    await flushPromises();

    expect(treeUnmounts).toBe(0);
    expect(treeMounts).toBe(1);
    expect(treeRefreshKey).toBe("namespace=prod");
    expect(panelMounts).toBe(2);
  });

  it("refreshes the resource tree after a list action completes", async () => {
    let treeRefreshKey = "";
    const wrapper = mount(TreeWorkspace, {
      props: {
        connectionId: "c1",
        tree: [],
        resources: [
          {
            kind: "database",
            title: "Databases",
            list: { routeId: "mysql.databases.list" },
            columns: [],
            actions: { toolbar: ["mysql.database.drop"] },
            detail: { header: { title: "Database" }, tabs: [] },
          },
        ] as never,
        actions: [],
      },
      global: {
        stubs: {
          AppIcon: true,
          Button: { template: "<button><slot /></button>" },
          ResourceTree: {
            name: "ResourceTree",
            props: ["refreshKey"],
            updated() {
              treeRefreshKey = String(
                (this as { refreshKey?: string }).refreshKey ?? "",
              );
            },
            template: "<div data-test='tree' />",
          },
          VueDraggable: { template: "<div><slot /></div>" },
          PanelHost: {
            emits: ["actionDone"],
            template:
              "<button data-test='done' @click=\"$emit('actionDone', { id: 'mysql.database.drop', label: 'Drop' })\">done</button>",
          },
        },
      },
    });
    const ws = useWorkspaceStore();
    ws.openView("c1", {
      id: "list:database",
      title: "Databases",
      kind: "list",
      resourceKind: "database",
    });
    await nextTick();
    await flushPromises();

    await wrapper.get("[data-test='done']").trigger("click");
    await nextTick();

    expect(treeRefreshKey).toBe("1");
    wrapper.unmount();
  });

  it("pins a preview tab on double-click", async () => {
    const wrapper = mountWorkspace();
    const ws = useWorkspaceStore();

    ws.openPreviewView("c1", {
      id: "list:containers",
      title: "Containers",
      kind: "list",
      resourceKind: "missing",
    });
    await nextTick();
    await flushPromises();

    const tab = wrapper.get("[data-active-tab='true']");
    expect(ws.activeView("c1")?.preview).toBe(true);
    expect(tab.attributes("data-preview-tab")).toBe("true");
    expect(tab.attributes("title")).toBe("Containers");
    expect(tab.get(".font-medium").classes()).toContain("italic");

    await tab.trigger("dblclick");

    expect(ws.activeView("c1")?.preview).toBe(false);
    await nextTick();
    const pinnedTab = wrapper.get("[data-active-tab='true']");
    expect(pinnedTab.attributes("data-preview-tab")).toBeUndefined();
    expect(pinnedTab.attributes("title")).toBe("Containers");
    expect(pinnedTab.get(".font-medium").classes()).not.toContain("italic");
  });
});
