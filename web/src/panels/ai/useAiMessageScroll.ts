import {
  computed,
  getCurrentScope,
  onScopeDispose,
  watch,
  type Ref,
} from "vue";
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

  // The library watches only content height; a viewport resize (confirm bar /
  // queue / composer toggling) clamps scrollTop and it misreads that as a
  // scroll-up. Re-pin across viewport resizes unless the user actually escaped.
  let viewportObserver: ResizeObserver | null = null;
  watch(
    () => scrollRef.value,
    (el) => {
      viewportObserver?.disconnect();
      viewportObserver = null;
      if (!el || typeof ResizeObserver === "undefined") return;
      let settled = false;
      viewportObserver = new ResizeObserver(() => {
        if (!settled) {
          settled = true;
          return;
        }
        if (escapedFromLock.value) return;
        void scrollToBottom({ animation: "instant", ignoreEscapes: true });
      });
      viewportObserver.observe(el);
    },
  );
  if (getCurrentScope()) {
    onScopeDispose(() => viewportObserver?.disconnect());
  }

  watch(
    () => messages.value.length,
    (count, prev) => {
      if (count <= prev) return;
      const added = messages.value.slice(prev);
      // Sending is deliberate intent to see the reply, so it always returns to
      // the bottom (and re-engages the lock); appends only follow when already
      // near the bottom and the user hasn't scrolled away.
      const sentByUser = added.some((m) => m.role === "user");
      if (sentByUser || (!escapedFromLock.value && isNearBottom.value)) {
        void scrollToBottom({ animation: "smooth", wait: true });
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
