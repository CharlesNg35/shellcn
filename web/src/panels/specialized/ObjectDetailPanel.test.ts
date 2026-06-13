import { beforeEach, describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ObjectDetailPanel from "./ObjectDetailPanel.vue";

const fetchDoc = vi.hoisted(() => vi.fn());

vi.mock("../../api/dataSource", () => ({
  fetchDoc,
}));

describe("ObjectDetailPanel", () => {
  beforeEach(() => {
    fetchDoc.mockReset();
  });

  it("renders configured fields and redacts sensitive values", async () => {
    fetchDoc.mockResolvedValue({
      name: "api",
      status: "Running",
      token: "secret",
    });
    const wrapper = mount(ObjectDetailPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.object" },
        config: {
          rawToggle: true,
          sections: [
            {
              title: "Summary",
              fields: [
                { key: "name", label: "Name", copy: true },
                { key: "status", label: "Status", type: "badge" },
                { key: "token", label: "Token", redacted: true },
              ],
            },
          ],
        },
      },
      global: {
        stubs: {
          CodeTextEditor: true,
          AppIcon: true,
        },
      },
    });
    await flushPromises();

    expect(fetchDoc).toHaveBeenCalledWith(
      "c1",
      { routeId: "x.object" },
      { resource: undefined },
    );
    expect(wrapper.text()).toContain("Summary");
    expect(wrapper.text()).toContain("api");
    expect(wrapper.text()).toContain("Running");
    expect(wrapper.text()).toContain("********");
    expect(wrapper.text()).not.toContain("secret");
  });

  it("renders generic usage fields with used and total values", async () => {
    fetchDoc.mockResolvedValue({
      memPct: 14.72,
      mem: 2534030705,
      maxmem: 17179869184,
    });
    const wrapper = mount(ObjectDetailPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.object" },
        config: {
          sections: [
            {
              title: "Runtime",
              fields: [
                {
                  key: "memPct",
                  label: "Memory usage",
                  type: "percent",
                  usage: {
                    percentKey: "memPct",
                    usedKey: "mem",
                    totalKey: "maxmem",
                    usedType: "bytes",
                    totalType: "bytes",
                  },
                },
              ],
            },
          ],
        },
      },
      global: {
        stubs: {
          CodeTextEditor: true,
          AppIcon: true,
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Memory usage");
    expect(wrapper.text()).toContain("14.7%");
    expect(wrapper.text()).toContain("2.4 GiB");
    expect(wrapper.text()).toContain("16.0 GiB");
    const progress = wrapper.findComponent({ name: "ProgressBar" });
    expect(progress.exists()).toBe(true);
    expect(progress.props("pt")).toMatchObject({
      value: expect.stringContaining("h-full"),
    });
  });

  it("does not render zero totals as capacity", async () => {
    fetchDoc.mockResolvedValue({
      memPct: 0,
      mem: 53896806,
      maxmem: 0,
    });
    const wrapper = mount(ObjectDetailPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.object" },
        config: {
          sections: [
            {
              title: "Runtime",
              fields: [
                {
                  key: "memPct",
                  label: "Memory usage",
                  type: "percent",
                  usage: {
                    percentKey: "memPct",
                    usedKey: "mem",
                    totalKey: "maxmem",
                    usedType: "bytes",
                    totalType: "bytes",
                  },
                },
              ],
            },
          ],
        },
      },
      global: {
        stubs: {
          CodeTextEditor: true,
          AppIcon: true,
        },
      },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("51.4 MiB");
    expect(wrapper.text()).not.toContain("of 0 B");
    expect(wrapper.text()).not.toContain("0.0%");
  });

  it("keeps existing fields visible during refresh", async () => {
    let resolveRefresh:
      | ((value: { name: string; status: string }) => void)
      | undefined;
    fetchDoc
      .mockResolvedValueOnce({ name: "api", status: "Running" })
      .mockReturnValueOnce(
        new Promise((resolve) => {
          resolveRefresh = resolve;
        }),
      );

    const wrapper = mount(ObjectDetailPanel, {
      props: {
        connectionId: "c1",
        source: { routeId: "x.object" },
        config: {
          sections: [
            {
              fields: [
                { key: "name", label: "Name" },
                { key: "status", label: "Status" },
              ],
            },
          ],
        },
      },
      global: { stubs: { AppIcon: true } },
    });
    await flushPromises();

    await wrapper
      .findAll("button")
      .find((button) => button.text().includes("Refresh"))!
      .trigger("click");
    await flushPromises();

    expect(wrapper.find('[data-test="skeleton-list"]').exists()).toBe(false);
    expect(wrapper.text()).toContain("api");
    expect(wrapper.text()).toContain("Running");

    resolveRefresh?.({ name: "api-v2", status: "Ready" });
    await flushPromises();

    expect(wrapper.text()).toContain("api-v2");
    expect(wrapper.text()).toContain("Ready");
  });
});
