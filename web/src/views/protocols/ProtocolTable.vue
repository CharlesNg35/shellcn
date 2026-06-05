<script setup lang="ts">
import DataTable from "primevue/datatable";
import Column from "primevue/column";
import Select from "primevue/select";
import AppIcon from "../../components/AppIcon.vue";
import type {
  ProtocolAdminItem,
  ProtocolAvailability,
} from "../../types/projection";

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
            v-for="risk in (data as ProtocolAdminItem).risks"
            :key="risk"
            class="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-600 capitalize dark:bg-surface-800 dark:text-surface-300"
            >{{ risk }}</span
          >
          <span
            v-if="(data as ProtocolAdminItem).recording?.length"
            class="inline-flex items-center gap-1 text-xs text-surface-400"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'video' }" :size="12" />
            {{ (data as ProtocolAdminItem).recording!.join(", ") }}
          </span>
          <span
            v-if="
              !(data as ProtocolAdminItem).risks?.length &&
              !(data as ProtocolAdminItem).recording?.length
            "
            class="text-sm text-surface-400"
            >—</span
          >
        </div>
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
