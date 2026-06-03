import { describe, it, expect, beforeEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import Drawer from "primevue/drawer";
import AiChatLauncher from "./AiChatLauncher.vue";
import { useAiChatStore } from "../stores/aiChat";

vi.mock("../api/ai", () => ({
  aiApi: {
    global: vi.fn(async () => ({
      configured: true,
      provider: "Shared AI",
      model: "gpt-4o",
    })),
    list: vi.fn(async () => []),
    turnControl: vi.fn(),
  },
  streamAiTurn: vi.fn(),
  isAbort: vi.fn(() => false),
}));

const CONN = "conn-1";

beforeEach(() => {
  localStorage.clear();
  setActivePinia(createPinia());
});

describe("AiChatLauncher", () => {
  it("allows outside close only when the chat is idle without pending confirmation", async () => {
    const wrapper = mount(AiChatLauncher, {
      props: {
        connectionId: CONN,
        connected: true,
      },
    });
    await flushPromises();

    const chat = useAiChatStore();
    const st = chat.state(CONN);
    expect(wrapper.findComponent(Drawer).props("dismissable")).toBe(true);

    st.runState = "streaming";
    await nextTick();
    expect(wrapper.findComponent(Drawer).props("dismissable")).toBe(false);

    st.runState = "idle";
    st.pendingConfirm = {
      toolId: "t1",
      toolName: "delete_file",
      routeId: "files.delete",
      risk: "destructive",
      destructive: true,
      params: {},
      body: {},
    };
    await nextTick();

    expect(wrapper.findComponent(Drawer).props("dismissable")).toBe(false);
  });
});
