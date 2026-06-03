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
    expect(wrapper.text()).toContain("Shared");
    expect(wrapper.text()).not.toContain("gpt-4o");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    expect(wrapper.findComponent({ name: "Tag" }).attributes("title")).toBe(
      "Shared - gpt-4o",
    );
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
    expect(wrapper.text()).toContain("OpenRouter");
    expect(wrapper.text()).not.toContain("openai/gpt-4o");
    expect(wrapper.findComponent({ name: "Select" }).exists()).toBe(false);
    expect(wrapper.findComponent({ name: "Tag" }).attributes("title")).toBe(
      "OpenRouter - openai/gpt-4o",
    );
  });

  it("offers provider selection only", () => {
    const wrapper = mount(AiModelSwitcher, {
      props: {
        providers: [provider()],
        global: { configured: true, provider: "Shared", model: "gpt-4o" },
        providerId: "p1",
      },
    });
    const select = wrapper.findComponent({ name: "Select" });
    expect(select.exists()).toBe(true);
    expect(select.attributes("title")).toBe("My OpenAI - gpt-4o");
    const pt = select.props("pt") as {
      option: (options: {
        context: { option: { label: string; value: string; model: string } };
      }) => { title?: string };
    };
    expect(
      pt.option({
        context: {
          option: {
            label: "Shared",
            value: "",
            model: "gpt-4o",
          },
        },
      }).title,
    ).toBe("Shared - gpt-4o");
  });

  it("offers provider selection when multiple personal providers exist", async () => {
    const wrapper = mount(AiModelSwitcher, {
      props: {
        providers: [
          provider({ id: "p1", name: "OpenAI" }),
          provider({ id: "p2", name: "OpenRouter" }),
        ],
        global: { configured: false },
        providerId: "p1",
      },
    });
    const select = wrapper.findComponent({ name: "Select" });
    expect(select.exists()).toBe(true);

    select.vm.$emit("update:modelValue", "p2");

    expect(wrapper.emitted("select")).toEqual([["p2"]]);
  });
});
