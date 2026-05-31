<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import AiMessageItem from "./AiMessage.vue";
import AppIcon from "../../components/AppIcon.vue";
import type { AiMessage } from "../../stores/aiChat";

const props = defineProps<{
  messages: AiMessage[];
  currentId: string | null;
  streaming: boolean;
}>();
const emit = defineEmits<{ quickStart: [prompt: string] }>();

const scroller = ref<HTMLElement | null>(null);

const quickStarts = [
  "What resources are available on this connection?",
  "Summarize the current state.",
  "List recent items.",
];

function scrollToBottom(): void {
  const el = scroller.value;
  if (el) el.scrollTop = el.scrollHeight;
}

watch(
  () => [props.messages.length, props.messages.at(-1)?.content],
  async () => {
    await nextTick();
    scrollToBottom();
  },
);
</script>

<template>
  <div
    ref="scroller"
    class="flex-1 overflow-y-auto px-4 py-3"
    role="log"
    aria-live="polite"
    aria-label="Conversation"
  >
    <div
      v-if="messages.length === 0"
      class="flex h-full flex-col items-center justify-center gap-4 text-center"
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
        <button
          v-for="q in quickStarts"
          :key="q"
          type="button"
          class="rounded-lg border border-surface-200 px-3 py-2 text-left text-xs text-surface-600 transition-colors hover:bg-surface-100 dark:border-surface-700 dark:text-surface-300 dark:hover:bg-surface-800"
          @click="emit('quickStart', q)"
        >
          {{ q }}
        </button>
      </div>
    </div>

    <div v-else class="flex flex-col gap-3">
      <AiMessageItem
        v-for="m in messages"
        :key="m.id"
        :message="m"
        :streaming="streaming && m.id === currentId"
      />
    </div>
  </div>
</template>
