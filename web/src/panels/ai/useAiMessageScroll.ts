import { computed, watch, type Ref } from "vue";
import { useStickToBottom } from "vue-stick-to-bottom";
import type { AiMessage } from "@/stores/aiChat";

export function useAiMessageScroll(
  messages: Ref<AiMessage[]>,
  streaming: Ref<boolean>,
) {
  const {
    scrollRef,
    contentRef,
    isAtBottom,
    isNearBottom,
    escapedFromLock,
    scrollToBottom,
    stopScroll,
  } = useStickToBottom({
    initial: "instant",
    resize: { damping: 0.7, stiffness: 0.05, mass: 1.25 },
  });

  const lastMessage = computed(() => messages.value.at(-1));
  const showScrollToLatest = computed(
    () => messages.value.length > 0 && !isAtBottom.value,
  );

  function jumpToBottom(): void {
    void scrollToBottom({ animation: "smooth", ignoreEscapes: true });
  }

  watch(
    () => messages.value.length,
    (count, prev) => {
      if (count <= prev) return;
      const added = messages.value.slice(prev);
      if (
        !escapedFromLock.value &&
        (added.some((m) => m.role === "user") || isNearBottom.value)
      ) {
        void scrollToBottom({
          animation: "smooth",
          wait: true,
        });
      }
    },
    { flush: "post" },
  );

  watch(
    () => lastMessage.value?.content.length ?? 0,
    (length, prev) => {
      if (
        !streaming.value ||
        length === prev ||
        escapedFromLock.value ||
        !isNearBottom.value
      ) {
        return;
      }
      void scrollToBottom({
        preserveScrollPosition: true,
        duration: 80,
      });
    },
    { flush: "post" },
  );

  watch(
    () => lastMessage.value?.id ?? null,
    () => {
      if (messages.value.length === 0) {
        stopScroll();
        return;
      }
      if (!escapedFromLock.value && isNearBottom.value) {
        void scrollToBottom("instant");
      }
    },
  );

  return {
    scrollRef,
    contentRef,
    showScrollToLatest,
    jumpToBottom,
  };
}
