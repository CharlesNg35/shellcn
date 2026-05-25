<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import { interpolate } from "../api/dataSource";
import type {
  Action,
  DetailView as DetailViewSpec,
  Row,
} from "../types/projection";
import AppIcon from "../components/AppIcon.vue";
import PanelHost from "./PanelHost.vue";
import ActionBar from "./ActionBar.vue";

const props = defineProps<{
  connectionId: string;
  detail: DetailViewSpec;
  row: Row;
  actions: Action[];
}>();

const resource = computed(() => props.row.ref ?? null);
const activeTab = ref(props.detail.tabs[0]?.key);

watch(
  () => props.row.ref?.uid,
  () => {
    activeTab.value = props.detail.tabs[0]?.key;
  },
);

const title = computed(() => {
  const t = props.detail.header.title;
  if (!t) return resource.value?.name ?? "";
  try {
    return interpolate(t, { resource: resource.value });
  } catch {
    return resource.value?.name ?? t;
  }
});

const status = computed(() => {
  const f = props.detail.header.statusField;
  return f ? props.row[f] : undefined;
});

const current = computed(() =>
  props.detail.tabs.find((t) => t.key === activeTab.value),
);

const headerActions = computed(() =>
  (props.detail.header.actionIds ?? [])
    .map((id) => props.actions.find((a) => a.id === id))
    .filter((a): a is Action => Boolean(a)),
);
</script>

<template>
  <div class="flex h-full flex-col">
    <header
      class="border-b border-surface-200 px-5 py-3 dark:border-surface-800"
    >
      <div class="flex items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <h2
            class="text-base font-semibold text-surface-900 dark:text-surface-0"
          >
            {{ title }}
          </h2>
          <span
            v-if="status !== undefined"
            class="rounded-full bg-surface-100 px-2 py-0.5 text-xs text-surface-500 dark:bg-surface-800"
            >{{ status }}</span
          >
        </div>
        <ActionBar
          v-if="headerActions.length"
          :connection-id="connectionId"
          :actions="headerActions"
          :resource="resource"
        />
      </div>
    </header>

    <Tabs :value="activeTab" @update:value="activeTab = String($event)">
      <TabList>
        <Tab v-for="tab in detail.tabs" :key="tab.key" :value="tab.key">
          <AppIcon :icon="tab.icon" :size="14" />
          {{ tab.label }}
        </Tab>
      </TabList>
    </Tabs>

    <!-- KeepAlive (not lazy TabPanels) so a resource's console/logs stay alive
         when switching between its detail tabs. -->
    <div class="min-h-0 flex-1 overflow-hidden">
      <KeepAlive :max="8">
        <PanelHost
          v-if="current"
          :key="`${row.ref?.uid}:${current.key}`"
          :panel="current.panel"
          :connection-id="connectionId"
          :source="current.source"
          :config="current.config"
          :resource="resource"
        />
      </KeepAlive>
    </div>
  </div>
</template>
