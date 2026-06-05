import { describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import TaskProgressPanel from "./TaskProgressPanel.vue";

const mocks = vi.hoisted(() => ({
  runAction: vi.fn(),
  frame: undefined as ((data: string) => void) | undefined,
}));

vi.mock("../../api/dataSource", () => ({
  runAction: mocks.runAction,
}));

vi.mock("../../composables/useStream", () => ({
  useStream: (
    _connectionId: string,
    _source: unknown,
    _ctx: unknown,
    onFrame?: (data: string) => void,
  ) => {
    mocks.frame = onFrame;
    return {
      status: { value: "open" },
      error: { value: null },
      reconnect: vi.fn(),
    };
  },
}));

describe("TaskProgressPanel", () => {
  it("renders task frames and calls cancel/retry routes", async () => {
    mocks.runAction.mockResolvedValue({});
    const wrapper = mount(TaskProgressPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.task", method: "WS" },
        config: {
          title: "Backup",
          cancelRouteId: "x.task.cancel",
          retryRouteId: "x.task.retry",
        },
        resource: { kind: "job", name: "backup", uid: "job/backup" },
      },
      global: { stubs: { AppIcon: true, StreamStatusBar: true } },
    });

    mocks.frame?.(
      JSON.stringify({ status: "Running", percent: 42, line: "copying data" }),
    );
    mocks.frame?.("plain output");
    await flushPromises();

    expect(wrapper.text()).toContain("Backup");
    expect(wrapper.text()).toContain("Running");
    expect(wrapper.text()).toContain("copying data");
    expect(wrapper.text()).toContain("plain output");

    await wrapper.get("button").trigger("click");
    await wrapper.findAll("button")[1].trigger("click");
    expect(mocks.runAction).toHaveBeenCalledWith(
      "c1",
      "x.task.retry",
      { resource: { kind: "job", name: "backup", uid: "job/backup" } },
      {},
      {},
      "POST",
    );
    expect(mocks.runAction).toHaveBeenCalledWith(
      "c1",
      "x.task.cancel",
      { resource: { kind: "job", name: "backup", uid: "job/backup" } },
      {},
      {},
      "POST",
    );
  });
});
