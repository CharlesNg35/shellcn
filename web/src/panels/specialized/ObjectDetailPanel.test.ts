import { describe, expect, it, vi } from "vitest";
import { flushPromises, mount } from "@vue/test-utils";
import ObjectDetailPanel from "./ObjectDetailPanel.vue";

const fetchDoc = vi.hoisted(() => vi.fn());

vi.mock("../../api/dataSource", () => ({
  fetchDoc,
}));

describe("ObjectDetailPanel", () => {
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
});
