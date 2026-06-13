<script setup lang="ts">
import { computed } from "vue";
import { formatBytes } from "../../specialized/objectDetailFormat";

const props = defineProps<{
  label: string;
  value: number | null;
  unit?: string;
}>();

function formatNumber(value: number): string {
  if (Number.isInteger(value)) return String(value);
  if (Math.abs(value) < 1)
    return value.toFixed(3).replace(/0+$/, "").replace(/\.$/, "");
  return value.toFixed(1).replace(/\.0$/, "");
}

const display = computed(() => {
  if (props.value === null) return { value: "—", unit: "" };
  if (props.unit === "bytes")
    return { value: formatBytes(props.value), unit: "" };
  if (props.unit === "bytes/s") {
    return { value: `${formatBytes(props.value)}/s`, unit: "" };
  }
  return { value: formatNumber(props.value), unit: props.unit ?? "" };
});
</script>

<template>
  <div
    class="rounded-xl border border-surface-200 bg-surface-0 p-4 dark:border-surface-800 dark:bg-surface-900"
  >
    <p class="text-xs tracking-wide text-surface-400 uppercase">{{ label }}</p>
    <p class="mt-1 text-2xl font-semibold text-surface-900 dark:text-surface-0">
      <template v-if="display.value === '—'">—</template>
      <template v-else>
        {{ display.value }}
        <span
          v-if="display.unit"
          class="text-base font-normal text-surface-400"
          >{{ display.unit }}</span
        >
      </template>
    </p>
  </div>
</template>
