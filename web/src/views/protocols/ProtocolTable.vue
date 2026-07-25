<script setup lang="ts">
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Select from "primevue/select";
import Tag from "primevue/tag";
import AppIcon from "@/components/AppIcon.vue";
import type {
  ProtocolAdminItem,
  ProtocolAvailability,
} from "@/types/projection";

const props = defineProps<{
  protocols: ProtocolAdminItem[];
  loading: boolean;
  saving: Record<string, boolean>;
  // showStatus adds the subprocess health column for installed plugins.
  showStatus?: boolean;
  emptyText: string;
}>();

const emit = defineEmits<{
  (
    e: "set-availability",
    item: ProtocolAdminItem,
    next: ProtocolAvailability,
  ): void;
}>();

const availabilityChoices: { label: string; value: ProtocolAvailability }[] = [
  { label: "Enabled", value: "enabled" },
  { label: "Admins only", value: "admin_only" },
  { label: "Disabled", value: "disabled" },
];

function transportLabel(p: ProtocolAdminItem): string {
  if (!p.transports?.length) return "—";
  return p.transports
    .map((t) => (t === "agent" ? "Agent" : "Direct"))
    .join(", ");
}

function labelize(value: string): string {
  return value.replace(/[_-]+/g, " ");
}

const maxCapabilities = 3;
function capabilitySummary(p: ProtocolAdminItem): {
  shown: string[];
  hidden: string[];
} {
  const caps = p.capabilities ?? [];
  return {
    shown: caps.slice(0, maxCapabilities),
    hidden: caps.slice(maxCapabilities),
  };
}

type RiskSeverity = "success" | "info" | "warn" | "danger";
const riskOrder = ["safe", "write", "destructive", "privileged"] as const;
const riskMeta: Record<string, { label: string; severity: RiskSeverity }> = {
  safe: { label: "Safe", severity: "success" },
  write: { label: "Write", severity: "info" },
  destructive: { label: "Destructive", severity: "warn" },
  privileged: { label: "Privileged", severity: "danger" },
};

function riskBadge(
  p: ProtocolAdminItem,
): { label: string; severity: RiskSeverity; title: string } | null {
  const risks = p.risks ?? [];
  const present = riskOrder.filter((level) => risks.includes(level));
  const top = present.at(-1);
  if (!top) return null;
  return {
    ...riskMeta[top],
    title: `Highest risk of any action. This protocol has actions rated: ${present
      .map((level) => riskMeta[level].label)
      .join(", ")}.`,
  };
}
</script>

<template>
  <DataTable
    :value="props.protocols"
    :loading="props.loading"
    scrollable
    scroll-height="flex"
  >
    <Column header="Protocol">
      <template #body="{ data }">
        <span class="flex items-center gap-2">
          <AppIcon :icon="(data as ProtocolAdminItem).icon" :size="18" />
          <span class="min-w-0">
            <span
              class="block font-medium text-surface-800 dark:text-surface-100"
              >{{ (data as ProtocolAdminItem).title }}</span
            >
            <span class="block text-xs text-surface-400">{{
              (data as ProtocolAdminItem).name
            }}</span>
          </span>
        </span>
      </template>
    </Column>
    <Column v-if="props.showStatus" field="version" header="Version">
      <template #body="{ data }">
        <span class="text-sm text-surface-500">{{
          (data as ProtocolAdminItem).version || "—"
        }}</span>
      </template>
    </Column>
    <Column v-if="props.showStatus" header="Status">
      <template #body="{ data }">
        <span
          class="inline-flex items-center gap-1.5 text-sm"
          :class="
            (data as ProtocolAdminItem).healthy
              ? 'text-emerald-600'
              : 'text-rose-600'
          "
        >
          <span
            class="h-2 w-2 rounded-full"
            :class="
              (data as ProtocolAdminItem).healthy
                ? 'bg-emerald-500'
                : 'bg-rose-500'
            "
          />
          {{ (data as ProtocolAdminItem).healthy ? "Running" : "Offline" }}
        </span>
      </template>
    </Column>
    <Column v-if="!props.showStatus" header="Transports">
      <template #body="{ data }">
        <span class="text-sm text-surface-500">{{
          transportLabel(data as ProtocolAdminItem)
        }}</span>
      </template>
    </Column>
    <Column header="Capabilities">
      <template #body="{ data }">
        <div class="flex flex-wrap items-center gap-1">
          <span
            v-for="cap in capabilitySummary(data as ProtocolAdminItem).shown"
            :key="cap"
            class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-600 capitalize dark:bg-surface-800 dark:text-surface-300"
            >{{ labelize(cap) }}</span
          >
          <span
            v-if="capabilitySummary(data as ProtocolAdminItem).hidden.length"
            class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-500 dark:bg-surface-800 dark:text-surface-400"
            :title="
              capabilitySummary(data as ProtocolAdminItem)
                .hidden.map(labelize)
                .join(', ')
            "
            >+{{ capabilitySummary(data as ProtocolAdminItem).hidden.length }}</span
          >
          <span
            v-if="(data as ProtocolAdminItem).recording?.length"
            class="inline-flex items-center gap-1 rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-500 dark:bg-surface-800 dark:text-surface-400"
            :title="`Sessions can be recorded (${(data as ProtocolAdminItem).recording!.map(labelize).join(', ')})`"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'video' }" :size="12" />
            Recording
          </span>
          <span
            v-if="
              !(data as ProtocolAdminItem).capabilities?.length &&
              !(data as ProtocolAdminItem).recording?.length
            "
            class="text-sm text-surface-400"
            >—</span
          >
        </div>
      </template>
    </Column>
    <Column header="Risk">
      <template #body="{ data }">
        <Tag
          v-if="riskBadge(data as ProtocolAdminItem)"
          :value="riskBadge(data as ProtocolAdminItem)!.label"
          :severity="riskBadge(data as ProtocolAdminItem)!.severity"
          :title="riskBadge(data as ProtocolAdminItem)!.title"
        />
        <span v-else class="text-sm text-surface-400">—</span>
      </template>
    </Column>
    <Column header="Availability" :pt="{ bodyCell: 'w-44' }">
      <template #body="{ data }">
        <Select
          :model-value="(data as ProtocolAdminItem).availability"
          :options="availabilityChoices"
          option-label="label"
          option-value="value"
          :disabled="props.saving[(data as ProtocolAdminItem).name]"
          :aria-label="`Availability for ${(data as ProtocolAdminItem).title}`"
          fluid
          @update:model-value="
            emit('set-availability', data as ProtocolAdminItem, $event)
          "
        />
      </template>
    </Column>
    <template #empty>{{ props.emptyText }}</template>
  </DataTable>
</template>
