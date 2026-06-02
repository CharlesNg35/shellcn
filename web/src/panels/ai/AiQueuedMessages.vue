<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";

defineProps<{ messages: string[] }>();
const emit = defineEmits<{ remove: [index: number] }>();
</script>

<template>
  <ul v-if="messages.length" class="flex flex-col gap-1 px-3 pt-2">
    <li
      v-for="(message, index) in messages"
      :key="`${index}:${message}`"
      class="flex items-center gap-2 rounded-md bg-surface-100 px-2 py-1 text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'clock' }"
        :size="12"
        class="shrink-0 text-surface-400"
      />
      <span class="min-w-0 flex-1 truncate">{{ message }}</span>
      <Button
        type="button"
        text
        rounded
        severity="secondary"
        size="small"
        class="text-surface-400 hover:text-surface-700 dark:hover:text-surface-100"
        aria-label="Remove queued message"
        @click="emit('remove', index)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="12" />
      </Button>
    </li>
  </ul>
</template>
