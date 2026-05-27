import { beforeEach, describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import { createPinia, setActivePinia } from "pinia";
import DockPanel from "./DockPanel.vue";
import { useDockStore } from "../../stores/dock";

describe("DockPanel", () => {
  beforeEach(() => {
    setActivePinia(createPinia());
  });

  it("bounds long dock tab labels with a full title", () => {
    const dock = useDockStore();
    const title = "Very long log stream title that should not expand the dock";
    dock.open("c1", {
      id: "logs",
      title,
      panel: "log_stream",
      source: { routeId: "x.logs", method: "WS" },
    });

    const wrapper = mount(DockPanel, {
      props: { connectionId: "c1" },
      global: {
        stubs: {
          AppIcon: true,
          Button: {
            template: "<button><slot /></button>",
          },
          PanelHost: true,
        },
      },
    });

    const tab = wrapper.get(`button[title="${title}"]`);
    expect(tab.classes()).toContain("max-w-48");
    expect(tab.classes()).toContain("overflow-hidden");
    expect(tab.get("span").classes()).toEqual(
      expect.arrayContaining(["min-w-0", "flex-1", "truncate"]),
    );
  });
});
