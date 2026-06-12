import { describe, it, expect, beforeEach, vi } from "vitest";
import { mount, flushPromises } from "@vue/test-utils";
import { nextTick } from "vue";
import { createPinia, setActivePinia } from "pinia";
import Drawer from "primevue/drawer";
import AiChatLauncher from "./AiChatLauncher.vue";
import { cleanupConnection } from "../stores/connectionCleanup";
import { useAiChatStore } from "../stores/aiChat";
import { RiskLevel } from "../types/projection";

const turnControl = vi.fn();

vi.mock("../api/ai", () => ({
  aiApi: {
    global: vi.fn(async () => ({
      configured: true,
      provider: "Shared AI",
      model: "gpt-4o",
    })),
    list: vi.fn(async () => []),
    turnControl: (...args: unknown[]) => turnControl(...args),
  },
  streamAiTurn: vi.fn(),
  isAbort: vi.fn(() => false),
}));

const CONN = "conn-1";

beforeEach(() => {
  localStorage.clear();
  turnControl.mockClear();
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
      risk: RiskLevel.Destructive,
      destructive: true,
      params: {},
      body: {},
    };
    await nextTick();

    expect(wrapper.findComponent(Drawer).props("dismissable")).toBe(false);
    wrapper.unmount();
  });

  it("stops active assistant work when the connection is cleaned up", async () => {
    const wrapper = mount(AiChatLauncher, {
      props: {
        connectionId: CONN,
        connected: true,
      },
    });
    await flushPromises();

    const chat = useAiChatStore();
    const st = chat.state(CONN);
    st.runState = "streaming";
    st.turnId = "turn-1";
    st.queue = ["next"];

    cleanupConnection(CONN);
    await nextTick();

    expect(st.runState).toBe("idle");
    expect(st.turnId).toBe("");
    expect(st.queue).toEqual([]);
    expect(turnControl).toHaveBeenCalledWith(CONN, "turn-1", {
      type: "stop",
    });
    wrapper.unmount();
  });
});
