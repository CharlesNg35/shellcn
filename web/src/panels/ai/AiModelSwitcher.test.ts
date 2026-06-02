import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import AiModelSwitcher from "./AiModelSwitcher.vue";
import type { AiProviderSummary } from "../../api/ai";

function provider(over: Partial<AiProviderSummary> = {}): AiProviderSummary {
  return {
    id: "p1",
    kind: "openai",
    name: "My OpenAI",
    models: ["gpt-4o", "gpt-4o-mini"],
    model: "gpt-4o",
    hasKey: true,
    createdAt: "",
    updatedAt: "",
    ...over,
  };
}

describe("AiModelSwitcher", () => {
  it("locks to a read-only indicator when only the shared config exists", () => {
    const wrapper = mount(AiModelSwitcher, {
      props: {
        providers: [],
        global: { configured: true, provider: "Shared", model: "gpt-4o" },
        providerId: "",
      },
    });
    // No provider <select>; a Tag indicator instead.
    expect(wrapper.findAllComponents({ name: "Select" })).toHaveLength(0);
    expect(wrapper.text()).toContain("Shared");
    expect(wrapper.text()).not.toContain("gpt-4o");
  });

  it("shows the only personal provider without requiring a provider select", () => {
    const wrapper = mount(AiModelSwitcher, {
      props: {
        providers: [
          provider({
            name: "OpenRouter",
            model: "openai/gpt-4o",
            models: ["openai/gpt-4o"],
          }),
        ],
        global: { configured: false },
        providerId: "p1",
      },
    });
    expect(wrapper.findAllComponents({ name: "Select" })).toHaveLength(0);
    expect(wrapper.text()).toContain("OpenRouter");
    expect(wrapper.text()).not.toContain("openai/gpt-4o");
  });

  it("offers provider selection only", () => {
    const wrapper = mount(AiModelSwitcher, {
      props: {
        providers: [provider()],
        global: { configured: true, provider: "Shared", model: "gpt-4o" },
        providerId: "p1",
      },
    });
    expect(wrapper.findAllComponents({ name: "Select" })).toHaveLength(1);
  });
});
