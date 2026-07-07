<script setup lang="ts">
import Column from "primevue/column";
import DataTable from "primevue/datatable";
import AuditFilterBar from "./AuditFilterBar.vue";
import type { AuditEntry, AuditFilters } from "@/types/projection";

withDefaults(
  defineProps<{
    items: AuditEntry[];
    total: number;
    rows: number;
    first: number;
    filters?: AuditFilters;
    loading?: boolean;
  }>(),
  { filters: () => ({}), loading: false },
);
const emit = defineEmits<{
  page: [event: { first: number; rows: number }];
  filter: [filters: AuditFilters];
}>();

const resultClass: Record<string, string> = {
  allowed: "text-emerald-600 dark:text-emerald-400",
  denied: "text-amber-600 dark:text-amber-400",
  error: "text-rose-600 dark:text-rose-300",
};

function onPage(e: { first: number; rows: number }): void {
  emit("page", { first: e.first, rows: e.rows });
}

function onFilter(filters: AuditFilters): void {
  emit("filter", filters);
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString();
}
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col gap-3">
    <AuditFilterBar :filters="filters" @apply="onFilter" />

    <DataTable
      :value="items"
      lazy
      paginator
      :rows="rows"
      :first="first"
      :total-records="total"
      :loading="loading"
      scrollable
      scroll-height="flex"
      class="min-h-0"
      @page="onPage"
    >
      <Column header="Time">
        <template #body="{ data }">
          <span
            class="text-sm whitespace-nowrap text-surface-600 dark:text-surface-300"
          >
            {{ formatTime((data as AuditEntry).time) }}
          </span>
        </template>
      </Column>
      <Column field="event" header="Event" />
      <Column header="Result">
        <template #body="{ data }">
          <span
            class="text-xs font-medium capitalize"
            :class="resultClass[(data as AuditEntry).result]"
          >
            {{ (data as AuditEntry).result }}
          </span>
        </template>
      </Column>
      <Column header="Risk">
        <template #body="{ data }">
          <span class="text-surface-500 capitalize">
            {{ (data as AuditEntry).risk || "—" }}
          </span>
        </template>
      </Column>
      <Column header="From">
        <template #body="{ data }">
          <span class="text-surface-500 tabular-nums">
            {{ (data as AuditEntry).remoteAddr || "—" }}
          </span>
        </template>
      </Column>
      <template #empty>No activity.</template>
    </DataTable>
  </div>
</template>
