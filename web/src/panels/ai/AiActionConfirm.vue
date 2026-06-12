<script setup lang="ts">
import { computed } from "vue";
import Button from "primevue/button";
import AppIcon from "@/components/AppIcon.vue";
import { btnGhost } from "@/primevue/preset";
import type { PendingConfirm } from "@/stores/aiChat";

const props = defineProps<{ pending: PendingConfirm }>();
const emit = defineEmits<{ approve: []; reject: [] }>();

const rows = computed(() => {
  const out: { key: string; value: string }[] = [];
  for (const [k, v] of Object.entries(props.pending.params ?? {})) {
    out.push({ key: k, value: String(v) });
  }
  for (const [k, v] of Object.entries(props.pending.body ?? {})) {
    out.push({ key: k, value: typeof v === "string" ? v : JSON.stringify(v) });
  }
  return out;
});
</script>

<template>
  <div
    role="alertdialog"
    aria-label="Confirm action"
    class="rounded-lg border p-3 text-sm"
    :class="
      pending.destructive
        ? 'border-red-300 bg-red-50 dark:border-red-900/60 dark:bg-red-950/30'
        : 'border-amber-300 bg-amber-50 dark:border-amber-900/60 dark:bg-amber-950/30'
    "
  >
    <div class="flex items-center gap-2">
      <AppIcon
        :icon="{
          type: 'lucide',
          value: pending.destructive ? 'triangle-alert' : 'shield-question',
        }"
        :size="16"
        :class="
          pending.destructive
            ? 'text-red-600 dark:text-red-400'
            : 'text-amber-600 dark:text-amber-400'
        "
      />
      <span class="font-medium text-surface-800 dark:text-surface-100">
        {{
          pending.destructive ? "Confirm destructive action" : "Confirm action"
        }}
      </span>
    </div>

    <p class="mt-1 text-xs text-surface-600 dark:text-surface-300">
      The assistant wants to run
      <code class="text-surface-800 dark:text-surface-100">{{
        pending.routeId
      }}</code>
      ({{ pending.risk }}).
      <template v-if="pending.destructive">This cannot be undone.</template>
    </p>

    <dl
      v-if="rows.length"
      class="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-0.5 rounded-md bg-surface-0/60 p-2 text-xs dark:bg-surface-900/40"
    >
      <template v-for="row in rows" :key="row.key">
        <dt class="font-medium text-surface-500 dark:text-surface-400">
          {{ row.key }}
        </dt>
        <dd
          class="truncate text-surface-800 dark:text-surface-100"
          :title="row.value"
        >
          {{ row.value }}
        </dd>
      </template>
    </dl>

    <div class="mt-3 flex justify-end gap-2">
      <Button :pt="{ root: btnGhost }" size="small" @click="emit('reject')">
        Reject
      </Button>
      <Button
        size="small"
        :severity="pending.destructive ? 'danger' : 'primary'"
        autofocus
        @click="emit('approve')"
      >
        {{ pending.destructive ? "Run anyway" : "Approve" }}
      </Button>
    </div>
  </div>
</template>
