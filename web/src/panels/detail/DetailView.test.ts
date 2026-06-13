import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { mount, type VueWrapper } from "@vue/test-utils";
import DetailView from "./DetailView.vue";

class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

describe("DetailView", () => {
  let wrapper: VueWrapper | undefined;

  beforeEach(() => {
    // PrimeVue's TabList schedules an unguarded setTimeout(updateInkBar, 150) on
    // mount; fake timers keep it from firing after this file's jsdom is torn
    // down (which would throw "HTMLElement is not defined").
    vi.useFakeTimers();
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);
  });

  afterEach(() => {
    wrapper?.unmount();
    wrapper = undefined;
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("opens the declared default tab first", () => {
    wrapper = mount(DetailView, {
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

    expect(wrapper.get('[data-test="panel"]').text()).toBe("code_editor");
  });

  it("hides tabs whose visibleWhen fails against the active row", () => {
    wrapper = mount(DetailView, {
      props: {
        connectionId: "c1",
        row: {
          name: "ubuntu-template",
          template: true,
          ref: { kind: "qemu", name: "ubuntu-template", uid: "900" },
        },
        actions: [],
        detail: {
          header: { title: "${resource.name}" },
          defaultTab: "metrics",
          tabs: [
            { key: "summary", label: "Summary", panel: "object_detail" },
            { key: "backups", label: "Backups", panel: "table" },
            {
              key: "metrics",
              label: "Metrics",
              panel: "metrics",
              visibleWhen: {
                allOf: [{ field: "template", op: "neq", value: true }],
              },
            },
            {
              key: "console",
              label: "Console",
              panel: "terminal",
              visibleWhen: {
                allOf: [{ field: "template", op: "neq", value: true }],
              },
            },
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

    expect(wrapper.get('[data-test="panel"]').text()).toBe("object_detail");
    expect(wrapper.text()).toContain("Summary");
    expect(wrapper.text()).not.toContain("Metrics");
    expect(wrapper.text()).not.toContain("Console");
  });
});
