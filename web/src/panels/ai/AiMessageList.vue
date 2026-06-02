<script setup lang="ts">
import { computed, nextTick, watch } from "vue";
import { useStickToBottom } from "vue-stick-to-bottom";
import Button from "primevue/button";
import AiMessageItem from "./AiMessage.vue";
import AppIcon from "../../components/AppIcon.vue";
import type { AiMessage } from "../../stores/aiChat";

const props = defineProps<{
  messages: AiMessage[];
  currentId: string | null;
  streaming: boolean;
  hasMore: boolean;
  loadingOlder: boolean;
}>();
const emit = defineEmits<{ quickStart: [prompt: string]; loadOlder: [] }>();

const { scrollRef, contentRef, isAtBottom, isNearBottom, scrollToBottom } =
  useStickToBottom({ initial: "instant", resize: "instant" });
const lastMessage = computed(() => props.messages.at(-1));

watch(
  () => props.messages.length,
  async (count, prev) => {
    if (count <= prev) return;
    const shouldFollow =
      lastMessage.value?.role === "user" ||
      isAtBottom.value ||
      isNearBottom.value;
    if (!shouldFollow) return;
    await nextTick();
    scrollToBottom({ animation: "smooth", ignoreEscapes: true, wait: true });
  },
  { flush: "post" },
);

watch(
  () => lastMessage.value?.content.length ?? 0,
  async (length, prev) => {
    if (length === prev || !props.streaming) return;
    if (!isAtBottom.value && !isNearBottom.value) return;
    await nextTick();
    scrollToBottom({ animation: "instant", preserveScrollPosition: true });
  },
  { flush: "post" },
);

const quickStarts = [
  "What resources are available on this connection?",
  "Summarize the current state.",
  "List recent items.",
];
</script>

<template>
  <div
    v-if="messages.length === 0"
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
        @click="emit('quickStart', q)"
      >
        {{ q }}
      </Button>
    </div>
  </div>

  <div v-else class="relative min-h-0 flex-1 overflow-hidden">
    <div ref="scrollRef" class="h-full overflow-y-auto scroll-smooth">
      <div
        ref="contentRef"
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
        <AiMessageItem
          v-for="m in messages"
          :key="m.id"
          :message="m"
          :streaming="streaming && m.id === currentId"
        />
      </div>
    </div>

    <Button
      v-if="!isAtBottom"
      type="button"
      rounded
      severity="secondary"
      outlined
      class="absolute bottom-3 left-1/2 -translate-x-1/2 rounded-full border border-surface-200 bg-surface-0 p-1.5 shadow-md hover:bg-surface-100 dark:border-surface-700 dark:bg-surface-800 dark:hover:bg-surface-700"
      aria-label="Scroll to latest"
      @click="scrollToBottom({ animation: 'smooth', ignoreEscapes: true })"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'chevrons-down' }"
        :size="16"
        class="text-surface-500 dark:text-surface-300"
      />
    </Button>
  </div>
</template>
