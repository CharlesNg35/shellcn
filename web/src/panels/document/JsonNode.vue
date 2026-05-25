<script setup lang="ts">
import { computed } from "vue";

const props = defineProps<{
  name?: string;
  value: unknown;
  depth?: number;
}>();

const kind = computed(() => {
  if (props.value === null) return "null";
  if (Array.isArray(props.value)) return "array";
  return typeof props.value;
});

const entries = computed(() => {
  if (!props.value || typeof props.value !== "object") return [];
  return Object.entries(props.value as Record<string, unknown>);
});

const preview = computed(() => {
  if (kind.value === "string") return JSON.stringify(props.value);
  if (kind.value === "array") return `Array(${entries.value.length})`;
  if (kind.value === "object") return `{${entries.value.length}}`;
  return String(props.value);
});
</script>

<template>
  <div class="font-mono text-xs leading-relaxed">
    <details v-if="kind === 'object' || kind === 'array'" :open="depth === 0">
      <summary
        class="cursor-pointer select-none text-surface-700 dark:text-surface-200"
      >
        <span
          v-if="name !== undefined"
          class="text-primary-600 dark:text-primary-300"
        >
          {{ name }}:
        </span>
        <span class="text-surface-400">{{ preview }}</span>
      </summary>
      <div
        class="ml-4 border-l border-surface-200 pl-3 dark:border-surface-800"
      >
        <JsonNode
          v-for="[key, child] in entries"
          :key="key"
          :name="key"
          :value="child"
          :depth="(depth ?? 0) + 1"
        />
      </div>
    </details>
    <div v-else class="text-surface-700 dark:text-surface-200">
      <span
        v-if="name !== undefined"
        class="text-primary-600 dark:text-primary-300"
      >
        {{ name }}:
      </span>
      <span>{{ preview }}</span>
    </div>
  </div>
</template>
