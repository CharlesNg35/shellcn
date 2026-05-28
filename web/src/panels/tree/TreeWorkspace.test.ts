import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import { nextTick } from "vue";
import TreeWorkspace from "./TreeWorkspace.vue";
import { useWorkspaceStore } from "../../stores/workspace";

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
