<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "@/components/AppIcon.vue";
import PanelHost from "../core/PanelHost.vue";
import { resolvedPanelConfig, resolvedPanelType } from "../core/variants";
import { isVisible } from "../form/condition";
import type {
  Action,
  DashboardCell,
  PanelType,
  ResourceIdentity,
  Row,
} from "@/types/projection";

// Presentational, layout-only: a responsive grid of panel cards rendered at once.
// Shared by the connection-level `dashboard` layout and the `dashboard` panel
// type so the grid lives in exactly one place.
const props = withDefaults(
  defineProps<{
    connectionId: string;
    cells: DashboardCell[];
    actions?: Action[];
    // Optional config resolver for connection-level dashboard tabs; defaults to
    // the cell's own manifest config.
    resolveConfig?: (cell: DashboardCell) => Record<string, unknown>;
    resolvePanel?: (cell: DashboardCell) => PanelType;
    variantData?: Record<string, unknown> | null;
    resource?: ResourceIdentity | null;
    record?: Row | null;
    emptyText?: string;
  }>(),
  {
    actions: () => [],
    resolveConfig: undefined,
    resolvePanel: undefined,
    variantData: null,
    resource: null,
    record: null,
    emptyText: "No panels.",
  },
);

const emit = defineEmits<{
  actionDone: [action: Action, result?: Record<string, unknown>];
  select: [row: Row];
}>();

function spanClass(cell: DashboardCell): string {
  return (cell.span ?? 1) >= 2 ? "lg:col-span-2" : "";
}

function cellConfig(cell: DashboardCell): Record<string, unknown> {
  return props.resolveConfig
    ? props.resolveConfig(cell)
    : resolvedPanelConfig(cell, props.variantData ?? {});
}

function cellPanel(cell: DashboardCell): PanelType {
  return props.resolvePanel
    ? props.resolvePanel(cell)
    : resolvedPanelType(cell, props.variantData ?? {});
}

const visibleCells = computed(() =>
  props.cells.filter((cell) =>
    isVisible(cell.visibleWhen, props.variantData ?? {}),
  ),
);

function onCardAction(action: Action, result?: Record<string, unknown>): void {
  emit("actionDone", action, result);
}
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <div
      v-if="visibleCells.length"
      class="grid grid-cols-1 gap-4 lg:grid-cols-2"
    >
      <section
        v-for="cell in visibleCells"
        :key="cell.key"
        :class="spanClass(cell)"
        class="flex flex-col overflow-hidden rounded-xl border border-surface-200 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
      >
        <header
          class="flex items-center gap-2 border-b border-surface-200 px-3 py-2 text-sm font-medium text-surface-700 dark:border-surface-800 dark:text-surface-200"
        >
          <AppIcon v-if="cell.icon" :icon="cell.icon" :size="15" />
          <span>{{ cell.label }}</span>
        </header>
        <div class="min-h-0 flex-1" style="min-height: 20rem">
          <PanelHost
            :panel="cellPanel(cell)"
            :connection-id="props.connectionId"
            :source="cell.source"
            :config="cellConfig(cell)"
            :resource="props.resource"
            :record="props.record"
            :actions="props.actions"
            @action-done="onCardAction"
            @select="(row) => emit('select', row)"
          />
        </div>
      </section>
    </div>
    <div
      v-else
      class="flex h-full items-center justify-center text-sm text-surface-400"
    >
      {{ props.emptyText }}
    </div>
  </div>
</template>
