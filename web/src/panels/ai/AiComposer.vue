<script setup lang="ts">
import { ref } from "vue";
import Textarea from "primevue/textarea";
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<{ busy: boolean; disabled: boolean }>();
const emit = defineEmits<{ send: [text: string]; stop: [] }>();

const text = ref("");

function submit(): void {
  if (props.disabled) return;
  const value = text.value.trim();
  if (!value) return;
  emit("send", value);
  text.value = "";
}

function onKeydown(e: KeyboardEvent): void {
  // Enter sends; Shift+Enter inserts a newline.
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    submit();
  }
}
</script>

<template>
  <div
    class="border-t border-surface-200 bg-surface-0/95 p-3 dark:border-surface-800 dark:bg-surface-950/95"
  >
    <div
      class="rounded-2xl border border-surface-200 bg-surface-0 p-2 shadow-sm transition-colors focus-within:border-primary-400 focus-within:ring-2 focus-within:ring-primary-500/20 dark:border-surface-800 dark:bg-surface-900"
    >
      <Textarea
        v-model="text"
        :disabled="disabled"
        auto-resize
        rows="1"
        placeholder="Ask about this connection..."
        aria-label="Message"
        :pt="{
          root: 'max-h-36 min-h-[2.5rem] w-full resize-none border-0 bg-transparent px-2 py-2 text-sm leading-5 text-surface-800 outline-none shadow-none placeholder:text-surface-400 focus:ring-0 disabled:opacity-60 dark:text-surface-100',
        }"
        @keydown="onKeydown"
      />
      <div class="flex min-w-0 items-center justify-between gap-2 px-1 pt-1">
        <span
          v-if="busy"
          class="inline-flex min-w-0 items-center gap-1.5 text-xs text-surface-500 dark:text-surface-400"
          aria-live="polite"
        >
          <span
            class="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-primary-500 motion-reduce:animate-none"
          />
          Streaming
        </span>
        <span v-else class="min-w-0" />
        <div class="flex shrink-0 items-center gap-1.5">
          <Button
            v-if="busy"
            severity="secondary"
            outlined
            rounded
            size="small"
            aria-label="Stop"
            @click="emit('stop')"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'square' }" :size="14" />
          </Button>
          <Button
            rounded
            size="small"
            :disabled="disabled || !text.trim()"
            :aria-label="busy ? 'Queue message' : 'Send'"
            @click="submit"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'arrow-up' }" :size="15" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
