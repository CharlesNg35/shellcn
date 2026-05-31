<script setup lang="ts">
import { ref } from "vue";
import Textarea from "primevue/textarea";
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<{ busy: boolean; disabled: boolean }>();
const emit = defineEmits<{ send: [text: string]; stop: [] }>();

const text = ref("");

function submit(): void {
  if (props.busy || props.disabled) return;
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
    class="flex items-end gap-2 border-t border-surface-200 p-3 dark:border-surface-800"
  >
    <Textarea
      v-model="text"
      :disabled="disabled"
      auto-resize
      rows="1"
      placeholder="Ask about this connection…"
      class="max-h-40 min-h-0 flex-1 resize-none"
      aria-label="Message"
      @keydown="onKeydown"
    />
    <Button
      v-if="busy"
      severity="secondary"
      rounded
      aria-label="Stop"
      @click="emit('stop')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'square' }" :size="16" />
    </Button>
    <Button
      v-else
      rounded
      :disabled="disabled || !text.trim()"
      aria-label="Send"
      @click="submit"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'arrow-up' }" :size="16" />
    </Button>
  </div>
</template>
