<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import Checkbox from "primevue/checkbox";
import AppIcon from "@/components/AppIcon.vue";
import { btnGhost } from "@/primevue/preset";
import type { PendingConfirm } from "@/stores/aiChat";
import { RiskLevel } from "@/types/projection";

const props = defineProps<{ pending: PendingConfirm }>();
const emit = defineEmits<{ approve: [remember: boolean]; reject: [] }>();

const remember = ref(false);
const showDetails = ref(false);
const canRemember = computed(
  () => !props.pending.destructive && props.pending.risk === RiskLevel.Write,
);

const rows = computed(() => {
  const out: { key: string; value: string }[] = [];
  for (const [k, v] of Object.entries(props.pending.params ?? {})) {
    out.push({ key: k, value: formatValue(v) });
  }
  for (const [k, v] of Object.entries(props.pending.body ?? {})) {
    out.push({ key: k, value: formatValue(v) });
  }
  return out;
});

const approveLabel = computed(() =>
  props.pending.destructive ? "Run anyway" : "Approve",
);

function formatValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === null || value === undefined) return "";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

function approve(): void {
  emit("approve", canRemember.value && remember.value);
}
</script>

<template>
  <div
    class="shrink-0 px-3 pt-2"
    role="alertdialog"
    aria-label="Confirm assistant action"
  >
    <div
      class="overflow-hidden rounded-lg border"
      :class="
        pending.destructive
          ? 'border-rose-300 bg-rose-50/60 dark:border-rose-900/70 dark:bg-rose-950/30'
          : 'border-surface-200 bg-surface-50 dark:border-surface-700 dark:bg-surface-900/60'
      "
    >
      <div class="flex min-w-0 items-center gap-2 px-3 py-2">
        <AppIcon
          :icon="{
            type: 'lucide',
            value: pending.destructive ? 'triangle-alert' : 'shield-question',
          }"
          :size="15"
          class="shrink-0"
          :class="
            pending.destructive
              ? 'text-rose-600 dark:text-rose-400'
              : 'text-amber-600 dark:text-amber-400'
          "
        />
        <span
          class="shrink-0 text-sm font-medium text-surface-800 dark:text-surface-100"
        >
          {{
            pending.destructive ? "Allow destructive action?" : "Allow action?"
          }}
        </span>
        <code
          class="min-w-0 truncate rounded bg-surface-0 px-1.5 py-0.5 text-xs text-surface-600 dark:bg-surface-800 dark:text-surface-300"
          :title="pending.routeId"
        >
          {{ pending.routeId }}
        </code>
        <Button
          v-if="rows.length"
          type="button"
          text
          severity="secondary"
          size="small"
          class="ml-auto shrink-0 gap-1 px-1.5 py-0.5 text-xs text-surface-500 dark:text-surface-400"
          :aria-expanded="showDetails"
          @click="showDetails = !showDetails"
        >
          {{ showDetails ? "Hide" : "Details" }}
          <AppIcon
            :icon="{
              type: 'lucide',
              value: showDetails ? 'chevron-up' : 'chevron-down',
            }"
            :size="13"
          />
        </Button>
      </div>

      <dl
        v-if="rows.length && showDetails"
        class="grid max-h-40 grid-cols-[minmax(5rem,auto)_1fr] gap-x-3 gap-y-1.5 overflow-auto border-t px-3 py-2 text-xs"
        :class="
          pending.destructive
            ? 'border-rose-200/70 dark:border-rose-900/50'
            : 'border-surface-200 dark:border-surface-700'
        "
      >
        <template v-for="row in rows" :key="row.key">
          <dt class="font-medium text-surface-500 dark:text-surface-400">
            {{ row.key }}
          </dt>
          <dd
            class="min-w-0 wrap-break-word text-surface-700 dark:text-surface-200"
            :title="row.value"
          >
            {{ row.value }}
          </dd>
        </template>
      </dl>

      <div
        class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t px-3 py-2"
        :class="
          pending.destructive
            ? 'border-rose-200/70 dark:border-rose-900/50'
            : 'border-surface-200 dark:border-surface-700'
        "
      >
        <label
          v-if="canRemember"
          class="flex cursor-pointer items-center gap-2 text-xs text-surface-600 dark:text-surface-300"
        >
          <Checkbox v-model="remember" binary input-id="ai-remember-confirm" />
          Always allow for this connection
        </label>
        <div class="ml-auto flex items-center gap-2">
          <Button :pt="{ root: btnGhost }" size="small" @click="emit('reject')">
            Reject
          </Button>
          <Button
            size="small"
            :severity="pending.destructive ? 'danger' : 'primary'"
            autofocus
            @click="approve"
          >
            {{ approveLabel }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
