import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import TimelinePanel from "./TimelinePanel.vue";

const fetchPage = vi.hoisted(() => vi.fn());

vi.mock("../../api/dataSource", () => ({
  fetchPage,
}));

describe("TimelinePanel", () => {
  beforeEach(() => {
    fetchPage.mockReset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

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

  it("keeps the existing timeline visible during background refresh", async () => {
    vi.useFakeTimers();
    let resolveRefresh:
      | ((value: { items: Array<Record<string, string>> }) => void)
      | undefined;
    fetchPage
      .mockResolvedValueOnce({
        items: [
          {
            at: "2026-06-05T10:00:00Z",
            event: "Started",
            message: "Initial event",
          },
        ],
      })
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveRefresh = resolve;
        }),
      );

    const wrapper = mount(TimelinePanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.events" },
        config: {
          timestampField: "at",
          titleField: "event",
          bodyField: "message",
          refreshIntervalMs: 1000,
        },
      },
      global: { stubs: { AppIcon: true } },
    });
    await flushPromises();

    expect(wrapper.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(wrapper.text()).toContain("Initial event");

    await vi.advanceTimersByTimeAsync(1000);
    await flushPromises();

    expect(fetchPage).toHaveBeenCalledTimes(2);
    expect(wrapper.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(wrapper.text()).toContain("Initial event");

    resolveRefresh?.({
      items: [
        {
          at: "2026-06-05T10:01:00Z",
          event: "Pulled",
          message: "Refreshed event",
        },
      ],
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Refreshed event");
    wrapper.unmount();
  });
});
