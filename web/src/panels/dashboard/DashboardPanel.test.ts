import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import DashboardPanel from "./DashboardPanel.vue";
import PanelHost from "../core/PanelHost.vue";
import type { DashboardPanelConfig } from "../../types/projection";

const config: DashboardPanelConfig = {
  cells: [
    {
      key: "server",
      label: "Server",
      panel: "document",
      source: { routeId: "x.overview" },
      span: 2,
    },
    {
      key: "clients",
      label: "Clients",
      panel: "table",
      source: { routeId: "x.clients" },
    },
  ],
};

describe("DashboardPanel", () => {
  it("renders each cell as a card and honors span hints", () => {
    const wrapper = mount(DashboardPanel, {
      props: {
        connectionId: "c1",
        config: config as unknown as Record<string, unknown>,
      },
      global: { stubs: { AppIcon: true, PanelHost: true } },
    });
    const cards = wrapper.findAll("section");
    expect(cards).toHaveLength(2);
    expect(wrapper.text()).toContain("Server");
    expect(wrapper.text()).toContain("Clients");
    expect(cards[0].classes()).toContain("lg:col-span-2");
    expect(cards[1].classes()).not.toContain("lg:col-span-2");
  });

  it("propagates a row select from a cell panel so the host can navigate", async () => {
    const wrapper = mount(DashboardPanel, {
      props: {
        connectionId: "c1",
        config: config as unknown as Record<string, unknown>,
      },
      global: { stubs: { AppIcon: true, PanelHost: true } },
    });
    const row = { ref: { kind: "node", name: "n1", uid: "u1" } };
    wrapper.findComponent(PanelHost).vm.$emit("select", row);
    await wrapper.vm.$nextTick();
    expect(wrapper.emitted("select")?.[0]).toEqual([row]);
  });

  it("shows an empty state when there are no cells", () => {
    const wrapper = mount(DashboardPanel, {
      props: { connectionId: "c1", config: { cells: [] } },
      global: { stubs: { AppIcon: true, PanelHost: true } },
    });
    expect(wrapper.findAll("section")).toHaveLength(0);
    expect(wrapper.text()).toContain("No panels.");
  });
});
