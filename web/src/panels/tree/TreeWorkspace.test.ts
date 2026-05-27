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
    scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
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
});
