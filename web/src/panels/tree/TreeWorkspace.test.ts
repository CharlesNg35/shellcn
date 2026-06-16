import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { nextTick } from "vue";
import TreeWorkspace from "./TreeWorkspace.vue";
import { useWorkspaceStore } from "@/stores/workspace";
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
    const sidebar = root.get(".h-full.min-h-0.w-64");

    expect(sidebar.classes()).toContain("overflow-hidden");
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
