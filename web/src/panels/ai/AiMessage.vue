<script setup lang="ts">
import { ref } from "vue";
import AiMarkdown from "./AiMarkdown.vue";
import AiToolBadges from "./AiToolBadges.vue";
import AiReasoning from "./AiReasoning.vue";
import AppIcon from "../../components/AppIcon.vue";
import Button from "primevue/button";
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
    class="group flex gap-2"
    :class="isUser() ? 'justify-end' : 'justify-start'"
    :data-role="message.role"
  >
    <span
      v-if="!isUser()"
      class="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-primary-200 bg-primary-50 text-primary-600 dark:border-primary-900/70 dark:bg-primary-500/10 dark:text-primary-300"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'sparkles' }" :size="13" />
    </span>
    <div
      class="min-w-0 px-3 py-2 text-sm shadow-sm"
      :class="
        isUser()
          ? 'max-w-[82%] rounded-2xl rounded-br-md bg-primary-600 text-white'
          : 'w-[88%] max-w-[88%] rounded-2xl rounded-tl-md border border-surface-200 bg-surface-0 text-surface-800 dark:border-surface-800 dark:bg-surface-900 dark:text-surface-100'
      "
    >
      <p v-if="isUser()" class="wrap-break-word whitespace-pre-wrap">
        {{ message.content }}
      </p>

      <template v-else>
        <AiReasoning v-if="message.reasoning" :reasoning="message.reasoning" />
        <AiToolBadges :calls="message.toolCalls" />
        <p
          v-if="streaming && message.content"
          class="wrap-break-word whitespace-pre-wrap"
        >
          {{ message.content }}
        </p>
        <AiMarkdown v-else-if="message.content" :source="message.content" />
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
        <Button
          v-if="message.content && !streaming"
          type="button"
          text
          severity="secondary"
          size="small"
          class="mt-1 flex items-center gap-1 text-xs text-surface-400 opacity-0 transition-opacity group-hover:opacity-100 hover:text-surface-700 focus-visible:opacity-100 dark:hover:text-surface-100"
          :aria-label="copied ? 'Copied' : 'Copy message'"
          @click="copy"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: copied ? 'check' : 'copy' }"
            :size="12"
          />
          {{ copied ? "Copied" : "Copy" }}
        </Button>
      </template>
    </div>
  </div>
</template>
