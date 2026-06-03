import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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
    model: "gpt-4o",
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
    previewModels: vi.fn(),
    testProviderDraft: vi.fn(),
    testProvider: vi.fn(),
  },
}));

vi.mock("vue-router", () => ({
  RouterLink: { template: "<a><slot /></a>" },
}));

import AiSettingsView from "./AiSettingsView.vue";

// Unmount after each test so PrimeVue Tabs' deferred ink-bar timer can't fire
// against a torn-down jsdom ("HTMLElement is not defined").
const mounted: ReturnType<typeof mount>[] = [];
afterEach(() => {
  mounted.splice(0).forEach((w) => w.unmount());
});
function mountView() {
  const w = mount(AiSettingsView);
  mounted.push(w);
  return w;
}

beforeEach(() => {
  setActivePinia(createPinia());
  providers.length = 0;
  global.configured = true;
  global.provider = "Shared OpenAI";
  global.kind = "openai";
  global.model = "gpt-4o";
  create.mockClear();
});

describe("AiSettingsView", () => {
  it("shows the shared-AI indicator and an empty state", async () => {
    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("Shared AI");
    expect(wrapper.text()).toContain("Configured");
    expect(wrapper.text()).toContain("Shared OpenAI");
    expect(wrapper.text()).toContain("No personal providers yet");
  });

  it("keeps the shared tab disabled when shared AI is unavailable", async () => {
    global.configured = false;
    global.provider = undefined;
    global.model = undefined;

    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("Not configured");
    expect(wrapper.text()).toContain("No personal providers yet");
  });

  it("lists configured providers", async () => {
    providers.push({
      id: "aip-9",
      kind: "anthropic",
      name: "My Claude",
      models: ["claude-sonnet-4-5"],
      model: "claude-sonnet-4-5",
      hasKey: true,
      createdAt: "now",
      updatedAt: "now",
    });
    const wrapper = mountView();
    await flushPromises();
    expect(wrapper.text()).toContain("My Claude");
    expect(wrapper.text()).toContain("claude-sonnet-4-5");
  });
});
