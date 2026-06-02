import { describe, it, expect, beforeEach, vi } from "vitest";
import { nextTick, ref, type Ref } from "vue";
import { useAiMessageScroll } from "./useAiMessageScroll";
import type { AiMessage } from "../../stores/aiChat";

const stick = vi.hoisted(() => ({
  scrollRef: { value: null },
  contentRef: { value: null },
  isAtBottom: { value: false },
  isNearBottom: { value: false },
  escapedFromLock: { value: false },
  scrollToBottom: vi.fn(),
  stopScroll: vi.fn(),
}));

vi.mock("vue-stick-to-bottom", () => ({
  useStickToBottom: () => stick,
}));

function message(over: Partial<AiMessage> = {}): AiMessage {
  return {
    id: "m1",
    role: "assistant",
    content: "",
    reasoning: "",
    toolCalls: [],
    ...over,
  };
}

async function flushScrollWatchers(): Promise<void> {
  await nextTick();
  await nextTick();
}

function setup(messages: Ref<AiMessage[]> = ref([]), streaming = ref(false)) {
  useAiMessageScroll(messages, streaming);
  return { messages, streaming };
}

describe("useAiMessageScroll", () => {
  beforeEach(() => {
    stick.isAtBottom.value = false;
    stick.isNearBottom.value = false;
    stick.escapedFromLock.value = false;
    stick.scrollToBottom.mockClear();
    stick.stopScroll.mockClear();
  });

  it("does not force-scroll when the user has escaped the bottom lock", async () => {
    const { messages } = setup();
    stick.escapedFromLock.value = true;

    messages.value = [message({ id: "u1", role: "user", content: "hello" })];
    await flushScrollWatchers();

    expect(stick.scrollToBottom).not.toHaveBeenCalled();
  });

  it("auto-follows new user messages without disabling user escape", async () => {
    const { messages } = setup();

    messages.value = [message({ id: "u1", role: "user", content: "hello" })];
    await flushScrollWatchers();

    expect(stick.scrollToBottom).toHaveBeenCalledWith({
      animation: "smooth",
      wait: true,
    });
  });

  it("does not follow streaming token updates after manual scroll-up", async () => {
    const messages = ref([
      message({ id: "a1", role: "assistant", content: "a" }),
    ]);
    const streaming = ref(true);
    setup(messages, streaming);
    await flushScrollWatchers();
    stick.scrollToBottom.mockClear();
    stick.isNearBottom.value = true;
    stick.escapedFromLock.value = true;

    messages.value = [
      message({ id: "a1", role: "assistant", content: "answer" }),
    ];
    await flushScrollWatchers();

    expect(stick.scrollToBottom).not.toHaveBeenCalled();
  });
});
