import { mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import AuditFilterBar from "./AuditFilterBar.vue";

describe("AuditFilterBar", () => {
  it("renders labeled actions with AppIcon instead of Prime icon-only spans", () => {
    const wrapper = mount(AuditFilterBar, {
      props: { filters: {} },
      global: {
        stubs: {
          Teleport: true,
        },
      },
    });

    expect(wrapper.text()).toContain("More");
    expect(
      wrapper.find('input[aria-label="Search audit events"]').exists(),
    ).toBe(true);
    expect(wrapper.html()).not.toContain("pi pi-search");
    expect(wrapper.html()).not.toContain("Apply event search");
  });

  it("renders clear filters as an accessible icon button", () => {
    const wrapper = mount(AuditFilterBar, {
      props: { filters: { event: "login" } },
      global: {
        stubs: {
          Teleport: true,
        },
      },
    });

    const clearButton = wrapper.get('button[aria-label="Clear filters"]');

    expect(clearButton.text()).toBe("");
    expect(clearButton.findComponent({ name: "AppIcon" }).exists()).toBe(true);
  });
});
