<script setup lang="ts">
import AiMarkdown from "./AiMarkdown.vue";
import AiToolBadges from "./AiToolBadges.vue";
import Message from "primevue/message";
import type { AiMessage } from "../../stores/aiChat";

const props = defineProps<{ message: AiMessage; streaming: boolean }>();
const isUser = () => props.message.role === "user";
</script>

<template>
  <div
    class="flex"
    :class="isUser() ? 'justify-end' : 'justify-start'"
    :data-role="message.role"
  >
    <div
      class="max-w-[85%] min-w-0 rounded-xl px-3 py-2"
      :class="
        isUser()
          ? 'bg-primary-500 text-white'
          : 'bg-surface-100 text-surface-800 dark:bg-surface-800 dark:text-surface-100'
      "
    >
      <p v-if="isUser()" class="text-sm break-words whitespace-pre-wrap">
        {{ message.content }}
      </p>

      <template v-else>
        <AiToolBadges :calls="message.toolCalls" />
        <AiMarkdown v-if="message.content" :source="message.content" />
        <span
          v-if="streaming && !message.content"
          class="inline-flex items-center gap-1 text-xs text-surface-400"
          aria-live="polite"
        >
          <span class="h-1.5 w-1.5 animate-pulse rounded-full bg-surface-400" />
          Thinking…
        </span>
        <p
          v-if="message.truncated"
          class="mt-1 text-xs text-amber-600 dark:text-amber-400"
        >
          Response was capped at the output limit.
        </p>
        <Message
          v-if="message.error"
          severity="error"
          variant="simple"
          size="small"
          class="mt-1"
        >
          {{ message.error }}
        </Message>
      </template>
    </div>
  </div>
</template>
