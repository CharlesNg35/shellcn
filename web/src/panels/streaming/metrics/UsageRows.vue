<script setup lang="ts">
import type { MetricUsage, Row } from "@/types/projection";
import ObjectDetailFieldRow from "../../specialized/ObjectDetailFieldRow.vue";

defineProps<{
  fields: MetricUsage[];
  values: Row;
}>();
</script>

<template>
  <div
    class="rounded-lg border border-surface-200 bg-surface-0 dark:border-surface-800 dark:bg-surface-950"
  >
    <dl class="divide-y divide-surface-100 dark:divide-surface-800">
      <ObjectDetailFieldRow
        v-for="field in fields"
        :key="field.key"
        :field="field"
        :record="values"
      />
    </dl>
  </div>
</template>
