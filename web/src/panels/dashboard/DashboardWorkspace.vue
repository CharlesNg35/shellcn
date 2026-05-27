<script setup lang="ts">
import AppIcon from "../../components/AppIcon.vue";
import PanelHost from "../core/PanelHost.vue";
import type { Action, Tab as TabDef } from "../../types/projection";

const props = defineProps<{
  connectionId: string;
  tabs: TabDef[];
  actions: Action[];
  // Resolves the (recording-aware) config for a tab; owned by the parent so the
  // dashboard stays a pure layout shell.
  resolveConfig: (tab: TabDef) => Record<string, unknown>;
}>();

const emit = defineEmits<{ actionDone: [action: Action] }>();

// A span of 2 makes a card fill the row in the two-column grid; the grid
// collapses to a single column on narrow viewports.
function spanClass(tab: TabDef): string {
  return (tab.span ?? 1) >= 2 ? "lg:col-span-2" : "";
}

function onCardAction(action: Action): void {
  emit("actionDone", action);
}
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <div v-if="props.tabs.length" class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <section
        v-for="tab in props.tabs"
        :key="tab.key"
        :class="spanClass(tab)"
        class="flex flex-col overflow-hidden rounded-xl border border-surface-200 bg-surface-0 dark:border-surface-800 dark:bg-surface-900"
      >
        <header
          class="flex items-center gap-2 border-b border-surface-200 px-3 py-2 text-sm font-medium text-surface-700 dark:border-surface-800 dark:text-surface-200"
        >
          <AppIcon v-if="tab.icon" :icon="tab.icon" :size="15" />
          <span>{{ tab.label }}</span>
        </header>
        <div class="min-h-0 flex-1" style="min-height: 20rem">
          <PanelHost
            :panel="tab.panel"
            :connection-id="props.connectionId"
            :source="tab.source"
            :config="props.resolveConfig(tab)"
            :actions="props.actions"
            @action-done="onCardAction"
          />
        </div>
      </section>
    </div>
    <div
      v-else
      class="flex h-full items-center justify-center text-sm text-surface-400"
    >
      This dashboard has no panels.
    </div>
  </div>
</template>
