<script setup lang="ts">
import AppIcon from "../../components/AppIcon.vue";
import PanelHost from "../core/PanelHost.vue";
import type { Action, DashboardCell, Row } from "../../types/projection";

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
    emptyText?: string;
  }>(),
  {
    actions: () => [],
    resolveConfig: undefined,
    emptyText: "No panels.",
  },
);

const emit = defineEmits<{
  actionDone: [action: Action];
  select: [row: Row];
}>();

function spanClass(cell: DashboardCell): string {
  return (cell.span ?? 1) >= 2 ? "lg:col-span-2" : "";
}

function cellConfig(cell: DashboardCell): Record<string, unknown> {
  return props.resolveConfig ? props.resolveConfig(cell) : (cell.config ?? {});
}

function onCardAction(action: Action): void {
  emit("actionDone", action);
}
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <div
      v-if="props.cells.length"
      class="grid grid-cols-1 gap-4 lg:grid-cols-2"
    >
      <section
        v-for="cell in props.cells"
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
            :panel="cell.panel"
            :connection-id="props.connectionId"
            :source="cell.source"
            :config="cellConfig(cell)"
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
