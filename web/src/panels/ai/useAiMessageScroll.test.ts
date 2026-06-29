import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { nextTick, ref, type Ref } from "vue";
import { useAiMessageScroll } from "./useAiMessageScroll";
import type { AiMessage } from "@/stores/aiChat";

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
    stick.scrollRef.value = null;
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("does not auto-scroll for assistant messages after the user scrolled up", async () => {
    const { messages } = setup();
    stick.escapedFromLock.value = true;

    messages.value = [message({ id: "a1", role: "assistant", content: "hi" })];
    await flushScrollWatchers();

    expect(stick.scrollToBottom).not.toHaveBeenCalled();
  });

  it("always scrolls to the bottom on send, even after the user scrolled up", async () => {
    const { messages } = setup();
    stick.escapedFromLock.value = true;

    messages.value = [message({ id: "u1", role: "user", content: "hello" })];
    await flushScrollWatchers();

    expect(stick.scrollToBottom).toHaveBeenCalledWith({
      animation: "smooth",
      wait: true,
    });
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

  it("re-pins on a viewport resize while following, but not after escape", async () => {
    const callbacks: Array<() => void> = [];
    class FakeResizeObserver {
      constructor(cb: () => void) {
        callbacks.push(cb);
      }
      observe(): void {}
      disconnect(): void {}
    }
    vi.stubGlobal("ResizeObserver", FakeResizeObserver);

    const scrollRef = ref<HTMLElement | null>(null);
    stick.scrollRef = scrollRef as unknown as typeof stick.scrollRef;
    setup();
    scrollRef.value = document.createElement("div");
    await flushScrollWatchers();

    const fire = callbacks[0];
    expect(fire).toBeTruthy();

    fire(); // initial observe is ignored
    expect(stick.scrollToBottom).not.toHaveBeenCalled();

    fire(); // viewport resize while still following → re-pin
    expect(stick.scrollToBottom).toHaveBeenCalledWith({
      animation: "instant",
      ignoreEscapes: true,
    });

    stick.scrollToBottom.mockClear();
    stick.escapedFromLock.value = true;
    fire(); // user has scrolled away → leave it alone
    expect(stick.scrollToBottom).not.toHaveBeenCalled();
  });
});
