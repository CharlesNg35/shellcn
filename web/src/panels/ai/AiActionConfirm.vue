<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import Checkbox from "primevue/checkbox";
import Dialog from "primevue/dialog";
import AppIcon from "@/components/AppIcon.vue";
import { btnGhost, dialogRoot } from "@/primevue/preset";
import type { PendingConfirm } from "@/stores/aiChat";
import { RiskLevel } from "@/types/projection";

const props = defineProps<{ pending: PendingConfirm }>();
const emit = defineEmits<{ approve: [remember: boolean]; reject: [] }>();

const remember = ref(false);
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

const icon = computed(() => ({
  type: "lucide" as const,
  value: props.pending.destructive ? "triangle-alert" : "shield-question",
}));
const title = computed(() =>
  props.pending.destructive ? "Approve destructive action" : "Approve action",
);
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

function close(visible: boolean): void {
  if (!visible) emit("reject");
}
</script>

<template>
  <Dialog
    :visible="true"
    modal
    :closable="true"
    :pt="{
      root: dialogRoot('max-w-lg'),
      header: 'px-5 py-4',
      content: 'min-h-0 overflow-auto px-5 pb-5 pt-0',
      footer: 'border-t border-surface-200 px-5 py-3 dark:border-surface-800',
    }"
    @update:visible="close"
  >
    <template #header>
      <div class="flex min-w-0 items-center gap-3">
        <span
          class="flex size-9 shrink-0 items-center justify-center rounded-full"
          :class="
            pending.destructive
              ? 'bg-red-100 text-red-600 dark:bg-red-950/60 dark:text-red-300'
              : 'bg-amber-100 text-amber-600 dark:bg-amber-950/60 dark:text-amber-300'
          "
        >
          <AppIcon :icon="icon" :size="18" />
        </span>
        <div class="min-w-0">
          <h2
            class="truncate text-base font-semibold text-surface-900 dark:text-surface-50"
          >
            {{ title }}
          </h2>
          <p class="truncate text-xs text-surface-500 dark:text-surface-400">
            Assistant requested a tool action
          </p>
        </div>
      </div>
    </template>

    <div class="flex min-w-0 flex-col gap-4 text-sm">
      <div
        class="rounded-lg border border-surface-200 bg-surface-50 p-3 dark:border-surface-800 dark:bg-surface-950/60"
      >
        <div class="flex min-w-0 flex-wrap items-center gap-2">
          <code
            class="min-w-0 rounded bg-surface-0 px-1.5 py-0.5 text-xs break-all text-surface-900 dark:bg-surface-900 dark:text-surface-100"
          >
            {{ pending.routeId }}
          </code>
          <span
            class="rounded-full px-2 py-0.5 text-xs font-medium"
            :class="
              pending.destructive
                ? 'bg-red-100 text-red-700 dark:bg-red-950/70 dark:text-red-300'
                : 'bg-amber-100 text-amber-700 dark:bg-amber-950/70 dark:text-amber-300'
            "
          >
            {{ pending.risk }}
          </span>
        </div>
        <p class="mt-2 text-xs text-surface-600 dark:text-surface-300">
          Review the request before allowing the assistant to continue.
          <template v-if="pending.destructive">
            This action is marked destructive and may not be reversible.
          </template>
        </p>
      </div>

      <dl
        v-if="rows.length"
        class="grid max-h-52 min-w-0 grid-cols-[minmax(6rem,auto)_1fr] gap-x-3 gap-y-2 overflow-auto rounded-lg border border-surface-200 p-3 text-xs dark:border-surface-800"
      >
        <template v-for="row in rows" :key="row.key">
          <dt class="font-medium text-surface-500 dark:text-surface-400">
            {{ row.key }}
          </dt>
          <dd
            class="min-w-0 break-words text-surface-800 dark:text-surface-100"
            :title="row.value"
          >
            {{ row.value }}
          </dd>
        </template>
      </dl>

      <label
        v-if="canRemember"
        class="flex cursor-pointer items-start gap-3 rounded-lg border border-surface-200 p-3 transition-colors hover:bg-surface-50 dark:border-surface-800 dark:hover:bg-surface-900/60"
      >
        <Checkbox v-model="remember" binary input-id="ai-remember-confirm" />
        <span class="grid gap-1">
          <span
            class="text-sm font-medium text-surface-800 dark:text-surface-100"
          >
            Remember this write action for this connection
          </span>
          <span
            class="text-xs leading-5 text-surface-500 dark:text-surface-400"
          >
            Future requests to this route will be approved automatically.
            Destructive actions always require confirmation.
          </span>
        </span>
      </label>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
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
    </template>
  </Dialog>
</template>
