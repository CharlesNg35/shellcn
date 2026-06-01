<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<{ reasoning: string; lines?: number }>();
const expanded = ref(false);

// Collapsed reasoning shows only the last N lines to avoid a wall of text.
const preview = computed(() => {
  const max = props.lines ?? 50;
  const all = props.reasoning.split("\n");
  return all.length > max ? all.slice(-max).join("\n") : props.reasoning;
});
</script>

<template>
  <div class="mb-1">
    <Button
      type="button"
      text
      severity="secondary"
      size="small"
      class="flex items-center gap-1 text-xs text-surface-400 hover:text-surface-600 dark:hover:text-surface-200"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <AppIcon
        :icon="{
          type: 'lucide',
          value: expanded ? 'chevron-down' : 'chevron-right',
        }"
        :size="12"
      />
      Reasoning
    </Button>
    <pre
      v-if="expanded"
      class="mt-1 max-h-48 overflow-auto rounded-md bg-surface-100 p-2 text-xs whitespace-pre-wrap text-surface-600 dark:bg-surface-800 dark:text-surface-300"
      >{{ preview }}</pre
    >
  </div>
</template>
