import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AiActionConfirm from "./AiActionConfirm.vue";
import type { PendingConfirm } from "@/stores/aiChat";
import { RiskLevel } from "@/types/projection";

const global = {
  stubs: {
    Dialog: {
      template:
        '<section><slot name="header" /><slot /><slot name="footer" /></section>',
    },
    Checkbox: {
      props: ["modelValue"],
      emits: ["update:modelValue"],
      template:
        '<input data-testid="remember" type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />',
    },
  },
};

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
    const wrapper = mount(AiActionConfirm, {
      props: { pending: pending() },
      global,
    });
    expect(wrapper.text()).toContain("demo.delete");
    expect(wrapper.text()).toContain("prod-db");
    expect(wrapper.text()).toContain("cascade");
    expect(wrapper.text()).toContain("Approve destructive action");
    expect(wrapper.text()).toContain("may not be reversible");
    expect(wrapper.find('[data-testid="remember"]').exists()).toBe(false);
  });

  it("emits approve with remember preference and reject", async () => {
    const wrapper = mount(AiActionConfirm, {
      props: {
        pending: pending({ destructive: false, risk: RiskLevel.Write }),
      },
      global,
    });
    await wrapper.find('[data-testid="remember"]').setValue(true);
    const buttons = wrapper.findAll("button");
    await buttons[0].trigger("click"); // Reject
    await buttons[1].trigger("click"); // Approve
    expect(wrapper.emitted("reject")).toBeTruthy();
    expect(wrapper.emitted("approve")).toEqual([[true]]);
  });
});
