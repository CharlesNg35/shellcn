import { describe, expect, it } from "vitest";
import { mount } from "@vue/test-utils";
import SplitPanel from "./SplitPanel.vue";

describe("SplitPanel", () => {
  it("renders child panels and propagates selection", async () => {
    const wrapper = mount(SplitPanel, {
      props: {
        connectionId: "c1",
        config: {
          orientation: "horizontal",
          panels: [
            { key: "left", panel: "table", size: 35 },
            { key: "right", panel: "object_detail", size: 65 },
          ],
        },
      },
      global: {
        stubs: {
          PanelHost: {
            props: ["panel"],
            emits: ["select"],
            template:
              '<div data-test="child-panel" @click="$emit(\'select\', { uid: panel })">{{ panel }}</div>',
          },
        },
      },
    });

    const children = wrapper.findAll('[data-test="child-panel"]');
    expect(children).toHaveLength(2);
    expect(wrapper.text()).toContain("table");
    expect(wrapper.text()).toContain("object_detail");

    await children[1].trigger("click");
    expect(wrapper.emitted("select")?.[0]).toEqual([{ uid: "object_detail" }]);
  });
});
