import { describe, it, expect, beforeEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import AutoComplete from "primevue/autocomplete";
import InputText from "primevue/inputtext";
import Select from "primevue/select";
import type { AiProviderInput, AiProviderSummary } from "../../api/ai";
import AiProviderDialog from "./AiProviderDialog.vue";

const create = vi.fn();
const update = vi.fn();
const models = vi.fn();
const previewModels = vi.fn();
const testProviderDraft = vi.fn();
const testProvider = vi.fn();

vi.mock("../../api/ai", () => ({
  aiApi: {
    create: (...a: unknown[]) => create(...a),
    update: (...a: unknown[]) => update(...a),
    models: (...a: unknown[]) => models(...a),
    previewModels: (...a: unknown[]) => previewModels(...a),
    testProviderDraft: (...a: unknown[]) => testProviderDraft(...a),
    testProvider: (...a: unknown[]) => testProvider(...a),
  },
}));

function provider(over: Partial<AiProviderSummary> = {}): AiProviderSummary {
  return {
    id: "p1",
    kind: "openai",
    name: "OpenAI",
    models: ["gpt-4o"],
    model: "gpt-4o",
    hasKey: true,
    createdAt: "",
    updatedAt: "",
    ...over,
  };
}

function mountDialog(
  props: Partial<InstanceType<typeof AiProviderDialog>["$props"]> = {},
) {
  return mount(AiProviderDialog, {
    props: {
      visible: true,
      providers: [],
      ...props,
    },
    global: {
      stubs: {
        teleport: true,
      },
    },
  });
}

beforeEach(() => {
  create.mockReset();
  update.mockReset();
  models.mockReset();
  previewModels.mockReset();
  testProviderDraft.mockReset();
  testProvider.mockReset();
  testProviderDraft.mockResolvedValue({ ok: true });
  testProvider.mockResolvedValue({ ok: true });
});

describe("AiProviderDialog", () => {
  it("requires a base URL for OpenAI-compatible providers before saving", async () => {
    const wrapper = mountDialog();
    await flushPromises();
    wrapper
      .findComponent(Select)
      .vm.$emit("update:modelValue", "openai_compatible");
    wrapper.findComponent(AutoComplete).vm.$emit("update:modelValue", "llama3");
    await flushPromises();

    await wrapper.find("[data-testid='save-ai-provider']").trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("Base URL is required");
    expect(create).not.toHaveBeenCalled();
  });

  it("tests an unsaved provider draft with the current form values", async () => {
    const wrapper = mountDialog();
    await flushPromises();
    wrapper
      .findComponent(Select)
      .vm.$emit("update:modelValue", "openai_compatible");
    await flushPromises();
    wrapper
      .findAllComponents(InputText)[1]
      .vm.$emit("update:modelValue", "http://127.0.0.1:11434/v1");
    wrapper.findComponent(AutoComplete).vm.$emit("update:modelValue", "llama3");
    await flushPromises();

    await wrapper.find("[data-testid='test-ai-provider']").trigger("click");
    await flushPromises();

    expect(testProviderDraft).toHaveBeenCalledWith(
      expect.objectContaining<Partial<AiProviderInput>>({
        kind: "openai_compatible",
        name: "Custom provider",
        baseUrl: "http://127.0.0.1:11434/v1",
        model: "llama3",
      }),
    );
    expect(wrapper.text()).toContain("Connection OK");
  });

  it("uses the saved-provider test when editing unchanged connection settings", async () => {
    const wrapper = mountDialog({
      provider: provider({
        id: "p-saved",
        kind: "openai_compatible",
        name: "Local",
        baseUrl: "http://127.0.0.1:11434/v1",
        model: "llama3",
        models: ["llama3"],
      }),
    });
    await flushPromises();

    await wrapper.find("[data-testid='test-ai-provider']").trigger("click");
    await flushPromises();

    expect(testProvider).toHaveBeenCalledWith("p-saved");
    expect(testProviderDraft).not.toHaveBeenCalled();
  });

  it("uses the draft test when editing changes connection settings", async () => {
    const wrapper = mountDialog({
      provider: provider({
        id: "p-saved",
        kind: "openai_compatible",
        name: "Local",
        baseUrl: "http://127.0.0.1:11434/v1",
        model: "llama3",
        models: ["llama3"],
      }),
    });
    await flushPromises();
    wrapper
      .findAllComponents(InputText)[1]
      .vm.$emit("update:modelValue", "http://127.0.0.1:11435/v1");
    await flushPromises();

    await wrapper.find("[data-testid='test-ai-provider']").trigger("click");
    await flushPromises();

    expect(testProvider).not.toHaveBeenCalled();
    expect(testProviderDraft).toHaveBeenCalledWith(
      expect.objectContaining({ baseUrl: "http://127.0.0.1:11435/v1" }),
    );
  });
});
