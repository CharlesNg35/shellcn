import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount } from "@vue/test-utils";
import DetailView from "./DetailView.vue";

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

describe("DetailView", () => {
  beforeEach(() => {
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("opens the declared default tab first", () => {
    const w = mount(DetailView, {
      props: {
        connectionId: "c1",
        row: {
          name: "doc-1",
          ref: { kind: "document", name: "doc-1", uid: "doc-1" },
        },
        actions: [],
        detail: {
          header: { title: "${resource.name}" },
          defaultTab: "editor",
          tabs: [
            { key: "document", label: "Document", panel: "document" },
            { key: "editor", label: "Editor", panel: "code_editor" },
          ],
        },
      },
      global: {
        stubs: {
          PanelHost: {
            props: ["panel"],
            template: '<div data-test="panel">{{ panel }}</div>',
          },
          AppIcon: true,
        },
      },
    });

    expect(w.get('[data-test="panel"]').text()).toBe("code_editor");
  });
});
