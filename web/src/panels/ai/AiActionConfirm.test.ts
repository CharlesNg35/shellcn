import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AiActionConfirm from "./AiActionConfirm.vue";
import type { PendingConfirm } from "@/stores/aiChat";
import { RiskLevel } from "@/types/projection";

const global = {
  stubs: {
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

function findButton(wrapper: ReturnType<typeof mount>, label: string) {
  return wrapper.findAll("button").find((b) => b.text().includes(label));
}

describe("AiActionConfirm", () => {
  it("renders inline (not a modal dialog) with route and destructive labels", () => {
    const wrapper = mount(AiActionConfirm, {
      props: { pending: pending() },
      global,
    });
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
    expect(wrapper.find('[role="alertdialog"]').exists()).toBe(true);
    expect(wrapper.text()).toContain("demo.delete");
    expect(wrapper.text()).toContain("Allow destructive action?");
    expect(findButton(wrapper, "Run anyway")).toBeTruthy();
    expect(wrapper.find('[data-testid="remember"]').exists()).toBe(false);
  });

  it("reveals resolved params only when details are expanded", async () => {
    const wrapper = mount(AiActionConfirm, {
      props: { pending: pending() },
      global,
    });
    expect(wrapper.text()).not.toContain("prod-db");
    await findButton(wrapper, "Details")!.trigger("click");
    expect(wrapper.text()).toContain("prod-db");
    expect(wrapper.text()).toContain("cascade");
  });

  it("emits approve with remember preference and reject", async () => {
    const wrapper = mount(AiActionConfirm, {
      props: {
        pending: pending({ destructive: false, risk: RiskLevel.Write }),
      },
      global,
    });
    await wrapper.find('[data-testid="remember"]').setValue(true);
    await findButton(wrapper, "Reject")!.trigger("click");
    await findButton(wrapper, "Approve")!.trigger("click");
    expect(wrapper.emitted("reject")).toBeTruthy();
    expect(wrapper.emitted("approve")).toEqual([[true]]);
  });
});
