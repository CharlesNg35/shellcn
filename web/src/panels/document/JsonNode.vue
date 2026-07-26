<script setup lang="ts">
import { computed, ref } from "vue";

const props = defineProps<{
  name?: string;
  value: unknown;
  depth?: number;
  // Mount children only once opened; eager elsewhere so the whole document is
  // searchable from the first paint.
  lazy?: boolean;
}>();

const CHILD_PAGE = 100;

const kind = computed(() => {
  if (props.value === null) return "null";
  if (Array.isArray(props.value)) return "array";
  return typeof props.value;
});

const entries = computed(() => {
  if (!props.value || typeof props.value !== "object") return [];
  return Object.entries(props.value as Record<string, unknown>);
});

const expanded = ref(props.depth === 0);
const shown = ref(CHILD_PAGE);
const visibleEntries = computed(() => entries.value.slice(0, shown.value));
const hiddenCount = computed(
  () => entries.value.length - visibleEntries.value.length,
);
const renderChildren = computed(() => !props.lazy || expanded.value);

function onToggle(event: Event): void {
  expanded.value = (event.target as HTMLDetailsElement).open;
}

function showMore(): void {
  shown.value += CHILD_PAGE;
}

const preview = computed(() => {
  if (kind.value === "string") return JSON.stringify(props.value);
  if (kind.value === "array") return `Array(${entries.value.length})`;
  if (kind.value === "object") return `{${entries.value.length}}`;
  return String(props.value);
});
</script>

<template>
  <div class="font-mono text-xs leading-relaxed">
    <details
      v-if="kind === 'object' || kind === 'array'"
      :open="expanded"
      @toggle="onToggle"
    >
      <summary
        class="cursor-pointer text-surface-700 select-none dark:text-surface-200"
      >
        <span
          v-if="name !== undefined"
          class="text-primary-600 dark:text-primary-300"
        >
          {{ name }}:
        </span>
        <span class="break-words text-surface-400">{{ preview }}</span>
      </summary>
      <div
        v-if="renderChildren"
        class="ml-4 border-l border-surface-200 pl-3 dark:border-surface-800"
      >
        <JsonNode
          v-for="[key, child] in visibleEntries"
          :key="key"
          :name="key"
          :value="child"
          :depth="(depth ?? 0) + 1"
          :lazy="lazy"
        />
        <button
          v-if="hiddenCount > 0"
          type="button"
          class="cursor-pointer text-primary-600 hover:underline dark:text-primary-300"
          @click="showMore"
        >
          Show {{ hiddenCount }} more
        </button>
      </div>
    </details>
    <div v-else class="text-surface-700 dark:text-surface-200">
      <span
        v-if="name !== undefined"
        class="text-primary-600 dark:text-primary-300"
      >
        {{ name }}:
      </span>
      <span class="break-words">{{ preview }}</span>
    </div>
  </div>
</template>
