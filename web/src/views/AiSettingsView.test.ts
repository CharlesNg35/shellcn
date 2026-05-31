import { describe, it, expect, vi, beforeEach } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { setActivePinia, createPinia } from "pinia";
import type { AiGlobalStatus, AiProviderSummary } from "../api/ai";

const providers: AiProviderSummary[] = [];
const global: AiGlobalStatus = {
  configured: true,
  provider: "Shared OpenAI",
  kind: "openai",
  model: "gpt-4o",
};

const create = vi.fn<(...a: unknown[]) => Promise<AiProviderSummary>>(
  async (input) => ({
    id: "aip-1",
    kind: "openai",
    name: (input as { name: string }).name,
    models: ["gpt-4o"],
    defaultModel: "gpt-4o",
    hasKey: true,
    createdAt: "now",
    updatedAt: "now",
  }),
);

vi.mock("../api/ai", () => ({
  aiApi: {
    global: () => Promise.resolve(global),
    list: () => Promise.resolve(providers.slice()),
    create: (...a: unknown[]) => create(...a),
    update: vi.fn(),
    remove: vi.fn(),
    models: vi.fn(),
  },
}));

vi.mock("vue-router", () => ({
  RouterLink: { template: "<a><slot /></a>" },
}));

import AiSettingsView from "./AiSettingsView.vue";

beforeEach(() => {
  setActivePinia(createPinia());
  providers.length = 0;
  create.mockClear();
});

describe("AiSettingsView", () => {
  it("shows the shared-AI indicator and an empty state", async () => {
    const wrapper = mount(AiSettingsView);
    await flushPromises();
    expect(wrapper.text()).toContain("Shared AI");
    expect(wrapper.text()).toContain("Shared OpenAI");
    expect(wrapper.text()).toContain("No AI providers yet");
  });

  it("lists configured providers", async () => {
    providers.push({
      id: "aip-9",
      kind: "anthropic",
      name: "My Claude",
      models: ["claude-sonnet-4-5"],
      defaultModel: "claude-sonnet-4-5",
      hasKey: true,
      createdAt: "now",
      updatedAt: "now",
    });
    const wrapper = mount(AiSettingsView);
    await flushPromises();
    expect(wrapper.text()).toContain("My Claude");
    expect(wrapper.text()).toContain("claude-sonnet-4-5");
  });
});
