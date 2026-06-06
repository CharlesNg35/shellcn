import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AiActionConfirm from "./AiActionConfirm.vue";
import type { PendingConfirm } from "../../stores/aiChat";
import { RiskLevel } from "../../types/projection";

function pending(over: Partial<PendingConfirm> = {}): PendingConfirm {
  return {
    toolId: "t1",
    toolName: "demo_delete",
    routeId: "demo.delete",
    risk: RiskLevel.Destructive,
    destructive: true,
    params: { name: "prod-db" },
    body: { cascade: true },
    ...over,
  };
}

describe("AiActionConfirm", () => {
  it("shows the route, resolved params, and destructive warning", () => {
    const wrapper = mount(AiActionConfirm, { props: { pending: pending() } });
    expect(wrapper.text()).toContain("demo.delete");
    expect(wrapper.text()).toContain("prod-db");
    expect(wrapper.text()).toContain("cascade");
    expect(wrapper.text()).toContain("Confirm destructive action");
    expect(wrapper.text()).toContain("cannot be undone");
  });

  it("emits approve and reject", async () => {
    const wrapper = mount(AiActionConfirm, {
      props: {
        pending: pending({ destructive: false, risk: RiskLevel.Write }),
      },
    });
    const buttons = wrapper.findAll("button");
    await buttons[0].trigger("click"); // Reject
    await buttons[1].trigger("click"); // Approve
    expect(wrapper.emitted("reject")).toBeTruthy();
    expect(wrapper.emitted("approve")).toBeTruthy();
  });
});
