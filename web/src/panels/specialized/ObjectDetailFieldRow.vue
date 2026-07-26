<script setup lang="ts">
import Button from "primevue/button";
import ProgressBar from "primevue/progressbar";
import AppIcon from "@/components/AppIcon.vue";
import type { ObjectDetailField, Row } from "@/types/projection";
import { badgeClassFor } from "../shared/severity";
import {
  opsDetailLabel,
  opsDetailRow,
  opsDetailValue,
  opsUsageCaption,
  opsUsageRow,
  opsUsageValue,
} from "@/primevue/preset";
import {
  formatValue,
  humanize,
  usageCaption,
  usageMainText,
  usagePercent,
  usageToneClass,
  valueFor,
} from "./objectDetailFormat";

defineProps<{
  field: ObjectDetailField;
  record: Row;
  copied?: boolean;
}>();

const emit = defineEmits<{
  copy: [field: ObjectDetailField];
}>();
</script>

<template>
  <div :class="field.usage ? opsUsageRow : opsDetailRow">
    <dt :class="[opsDetailLabel, 'col-start-1 @md:col-auto']">
      {{ field.label ?? humanize(field.key) }}
    </dt>

    <dd v-if="field.usage" class="col-start-1 min-w-0 @md:col-auto">
      <div
        class="flex min-w-0 flex-col gap-2 @md:grid @md:grid-cols-[minmax(0,1fr)_minmax(0,1.5fr)] @md:items-center"
      >
        <span
          class="min-w-0 truncate font-medium text-surface-900 dark:text-surface-100"
        >
          {{ usageMainText(record, field) }}
        </span>
        <ProgressBar
          :value="usagePercent(record, field) ?? 0"
          :show-value="false"
          :aria-label="field.label ?? humanize(field.key)"
          :pt="{ value: usageToneClass(record, field) }"
          class="h-2 w-full"
        />
      </div>
      <div v-if="usageCaption(record, field)" :class="opsUsageCaption">
        <span>{{ usageCaption(record, field) }}</span>
        <span :class="opsUsageValue"
          >{{ usagePercent(record, field)?.toFixed(1) ?? "—" }}%</span
        >
      </div>
    </dd>

    <dd v-else :class="[opsDetailValue, 'col-start-1 @md:col-auto']">
      <span v-if="field.redacted" class="font-mono text-surface-400"
        >********</span
      >
      <span
        v-else-if="field.type === 'badge'"
        class="inline-block max-w-full truncate rounded-full px-2 py-0.5 align-bottom text-xs"
        :class="badgeClassFor(field.severities, valueFor(record, field))"
        :title="formatValue(valueFor(record, field), field.type)"
        >{{ formatValue(valueFor(record, field), field.type) }}</span
      >
      <span v-else>{{ formatValue(valueFor(record, field), field.type) }}</span>
    </dd>

    <Button
      v-if="field.copy && !field.redacted && !field.usage"
      class="col-start-2 row-start-1 @md:col-auto @md:row-auto"
      type="button"
      text
      rounded
      severity="secondary"
      size="small"
      :aria-label="`Copy ${field.label ?? humanize(field.key)}`"
      @click="emit('copy', field)"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: copied ? 'check' : 'copy' }"
        :size="13"
      />
    </Button>
  </div>
</template>
