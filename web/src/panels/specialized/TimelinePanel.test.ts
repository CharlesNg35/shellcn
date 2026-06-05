import { describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import TimelinePanel from "./TimelinePanel.vue";

const fetchPage = vi.hoisted(() => vi.fn());

vi.mock("../../api/dataSource", () => ({
  fetchPage,
}));

describe("TimelinePanel", () => {
  it("renders events from configured timeline fields", async () => {
    fetchPage.mockResolvedValue({
      items: [
        {
          at: "2026-06-05T10:00:00Z",
          event: "Started",
          message: "Task entered running state",
          level: "info",
        },
      ],
    });
    const wrapper = mount(TimelinePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.events" },
        config: {
          timestampField: "at",
          titleField: "event",
          bodyField: "message",
          severityField: "level",
        },
      },
      global: { stubs: { AppIcon: true } },
    });
    await flushPromises();

    expect(fetchPage).toHaveBeenCalledWith(
      "c1",
      { routeId: "x.events" },
      { resource: undefined },
      { limit: 100 },
    );
    expect(wrapper.text()).toContain("Started");
    expect(wrapper.text()).toContain("Task entered running state");
    expect(wrapper.text()).toContain("info");
  });
});
