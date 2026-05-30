<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    protocols: string[];
    labels?: Record<string, string>;
    emptyLabel?: string;
    // Cap the number of badges shown; the rest collapse into a "+N" chip. 0 = all.
    max?: number;
  }>(),
  { max: 0, labels: undefined, emptyLabel: undefined },
);

function label(protocol: string): string {
  return props.labels?.[protocol] ?? protocol;
}

const visible = computed(() =>
  props.max > 0 ? props.protocols.slice(0, props.max) : props.protocols,
);
const overflow = computed(() =>
  props.max > 0 ? props.protocols.slice(props.max) : [],
);
const overflowTitle = computed(() => overflow.value.map(label).join(", "));
</script>

<template>
  <div
    v-if="protocols.length"
    class="flex max-w-full items-center gap-1.5 overflow-x-auto pb-1"
  >
    <span
      v-for="protocol in visible"
      :key="protocol"
      class="inline-flex shrink-0 items-center rounded-full border border-primary-200 bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700 dark:border-primary-800 dark:bg-primary-950 dark:text-primary-200"
    >
      {{ label(protocol) }}
    </span>
    <span
      v-if="overflow.length"
      class="inline-flex shrink-0 cursor-default items-center rounded-full border border-surface-200 bg-surface-100 px-2 py-0.5 text-xs font-medium text-surface-500 dark:border-surface-700 dark:bg-surface-800 dark:text-surface-400"
      :title="overflowTitle"
    >
      +{{ overflow.length }}
    </span>
  </div>
  <span v-else class="text-sm text-surface-400">
    {{ emptyLabel ?? "No compatible protocols" }}
  </span>
</template>
