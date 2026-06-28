<script setup lang="ts">
import { onMounted, toRef, type VNodeRef } from "vue";
import Button from "primevue/button";
import AiMessageItem from "./AiMessage.vue";
import AiMessageSkeleton from "./AiMessageSkeleton.vue";
import AppIcon from "@/components/AppIcon.vue";
import { useAiMessageScroll } from "./useAiMessageScroll";
import type { AiMessage } from "@/stores/aiChat";

const props = defineProps<{
  messages: AiMessage[];
  currentId: string | null;
  streaming: boolean;
  hasMore: boolean;
  loadingOlder: boolean;
  loading?: boolean;
  disabled?: boolean;
}>();
const emit = defineEmits<{ quickStart: [prompt: string]; loadOlder: [] }>();

const { scrollRef, contentRef, showScrollToLatest, jumpToBottom } =
  useAiMessageScroll(toRef(props, "messages"), toRef(props, "streaming"));

const setScrollRef: VNodeRef = (el) => {
  scrollRef.value = el instanceof HTMLElement ? el : null;
};

const setContentRef: VNodeRef = (el) => {
  contentRef.value = el instanceof HTMLElement ? el : null;
};

onMounted(() => {
  // Pin to the bottom synchronously (before first paint) so a freshly loaded
  // long conversation never flashes from top to bottom before the scroll
  // engine's rAF settles.
  const el = scrollRef.value;
  if (el) el.scrollTop = el.scrollHeight;
});

const quickStarts = [
  "What resources are available on this connection?",
  "Summarize the current state.",
  "List recent items.",
];
</script>

<template>
  <div
    v-if="loading"
    class="min-h-0 flex-1 overflow-hidden"
    role="status"
    aria-busy="true"
    aria-label="Loading conversation"
  >
    <AiMessageSkeleton />
  </div>

  <div
    v-else-if="messages.length === 0"
    class="flex min-h-0 flex-1 flex-col items-center justify-center gap-4 px-4 text-center"
  >
    <AppIcon
      :icon="{ type: 'lucide', value: 'sparkles' }"
      :size="32"
      class="text-surface-300"
    />
    <p class="text-sm text-surface-500 dark:text-surface-400">
      Ask the assistant about this connection.
    </p>
    <div class="flex flex-col gap-2">
      <Button
        v-for="q in quickStarts"
        :key="q"
        type="button"
        severity="secondary"
        outlined
        size="small"
        class="rounded-lg border border-surface-200 px-3 py-2 text-left text-xs text-surface-600 transition-colors hover:bg-surface-100 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
        :disabled="disabled"
        @click="emit('quickStart', q)"
      >
        {{ q }}
      </Button>
    </div>
  </div>

  <div v-else class="relative min-h-0 flex-1 overflow-hidden">
    <div :ref="setScrollRef" class="h-full overflow-y-auto">
      <div
        :ref="setContentRef"
        class="flex flex-col gap-3 px-4 py-3"
        role="log"
        aria-live="polite"
        aria-label="Conversation"
      >
        <Button
          v-if="hasMore"
          type="button"
          text
          severity="secondary"
          size="small"
          class="mx-auto rounded-md px-3 py-1 text-xs text-surface-500 hover:bg-surface-100 disabled:opacity-50 dark:text-surface-400 dark:hover:bg-surface-800"
          :disabled="loadingOlder"
          @click="emit('loadOlder')"
        >
          {{ loadingOlder ? "Loading..." : "Load earlier messages" }}
        </Button>
        <TransitionGroup
          name="ai-chat-message"
          tag="div"
          class="flex flex-col gap-3"
        >
          <AiMessageItem
            v-for="m in messages"
            :key="m.id"
            :message="m"
            :streaming="streaming && m.id === currentId"
          />
        </TransitionGroup>
      </div>
    </div>

    <Button
      v-show="showScrollToLatest"
      type="button"
      rounded
      severity="secondary"
      outlined
      class="absolute bottom-4 left-1/2 z-10 -translate-x-1/2 rounded-full border border-surface-200 bg-surface-0 p-2 shadow-lg transition hover:bg-surface-100 dark:border-surface-700 dark:bg-surface-800 dark:hover:bg-surface-700"
      aria-label="Scroll to latest"
      @click="jumpToBottom"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'chevrons-down' }"
        :size="16"
        class="text-surface-500 dark:text-surface-300"
      />
    </Button>
  </div>
</template>

<style scoped>
.ai-chat-message-enter-active {
  transition:
    opacity 0.25s ease-out,
    transform 0.25s ease-out;
}

.ai-chat-message-enter-from {
  opacity: 0;
  transform: translateY(12px);
}

.ai-chat-message-enter-to {
  opacity: 1;
  transform: translateY(0);
}

@media (prefers-reduced-motion: reduce) {
  .ai-chat-message-enter-active {
    transition: none;
  }

  .ai-chat-message-enter-from {
    opacity: 1;
    transform: none;
  }
}
</style>
