<script setup lang="ts">
import { ref } from "vue";
import AiMarkdown from "./AiMarkdown.vue";
import AiToolBadges from "./AiToolBadges.vue";
import AiReasoning from "./AiReasoning.vue";
import AppIcon from "../../components/AppIcon.vue";
import Message from "primevue/message";
import type { AiMessage } from "../../stores/aiChat";

const props = defineProps<{ message: AiMessage; streaming: boolean }>();
const isUser = () => props.message.role === "user";

const copied = ref(false);
async function copy(): Promise<void> {
  try {
    await navigator.clipboard.writeText(props.message.content);
    copied.value = true;
    setTimeout(() => (copied.value = false), 1500);
  } catch {
    // clipboard unavailable; ignore
  }
}
</script>

<template>
  <div
    class="group flex"
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
      <p v-if="isUser()" class="text-sm wrap-break-word whitespace-pre-wrap">
        {{ message.content }}
      </p>

      <template v-else>
        <AiReasoning v-if="message.reasoning" :reasoning="message.reasoning" />
        <AiToolBadges :calls="message.toolCalls" />
        <AiMarkdown v-if="message.content" :source="message.content" />
        <span
          v-if="streaming && !message.content"
          class="inline-flex items-center gap-1 text-xs text-surface-400"
          aria-live="polite"
        >
          <span
            class="h-1.5 w-1.5 animate-pulse rounded-full bg-surface-400 motion-reduce:animate-none"
          />
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
        <button
          v-if="message.content && !streaming"
          type="button"
          class="mt-1 flex items-center gap-1 text-xs text-surface-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-surface-700 focus-visible:opacity-100 dark:hover:text-surface-100"
          :aria-label="copied ? 'Copied' : 'Copy message'"
          @click="copy"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: copied ? 'check' : 'copy' }"
            :size="12"
          />
          {{ copied ? "Copied" : "Copy" }}
        </button>
      </template>
    </div>
  </div>
</template>
