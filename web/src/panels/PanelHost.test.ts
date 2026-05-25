import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import PanelHost from "./PanelHost.vue";

describe("PanelHost", () => {
  it("renders a graceful fallback for an unknown panel type", () => {
    const w = mount(PanelHost, {
      props: { panel: "graph", connectionId: "c1" },
    });
    expect(w.text()).toContain("No renderer for panel type");
    expect(w.text()).toContain("graph");
  });
});
