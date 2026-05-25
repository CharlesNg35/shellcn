<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import AppIcon from "../components/AppIcon.vue";
import PanelHost from "../panels/PanelHost.vue";
import EnrollPanel from "../panels/EnrollPanel.vue";
import ResourceTree from "../panels/tree/ResourceTree.vue";
import TablePanel from "../panels/TablePanel.vue";
import DetailView from "../panels/DetailView.vue";
import type {
  PluginProjection,
  ResourceRef,
  ResourceType,
  Row,
} from "../types/projection";

const props = defineProps<{ id: string }>();
const conns = useConnectionsStore();
const ws = useWorkspaceStore();

const projection = ref<PluginProjection | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const online = ref(true);

const connection = computed(() => conns.byId(props.id));
const view = computed(() => ws.view(props.id));

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  projection.value = null;
  try {
    if (!conns.loaded) await conns.load();
    const c = conns.byId(props.id);
    if (!c) throw new Error(`Unknown connection "${props.id}".`);
    ws.open(props.id);
    online.value = c.online !== false;
    const proj = await conns.projection(c.protocol);
    projection.value = proj;
    if (!ws.view(props.id).activeTab && proj.tabs?.length) {
      ws.setActiveTab(props.id, proj.tabs[0].key);
    }
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

watch(() => props.id, load, { immediate: true });

const needsEnroll = computed(
  () => connection.value?.transport === "agent" && !online.value,
);

const resourceByKind = computed(() => {
  const map = new Map<string, ResourceType>();
  for (const r of projection.value?.resources ?? []) map.set(r.kind, r);
  return map;
});

// In a sidebar tree, the selected group maps to a resource list table by shared
// route id (group label/key need not equal the resource kind); the selected
// node maps to that resource's detail view by kind.
const groupResource = computed(() => {
  const key = view.value.selectedGroup;
  if (!key) return undefined;
  const group = projection.value?.tree?.find((g) => g.key === key);
  if (!group) return undefined;
  return (projection.value?.resources ?? []).find(
    (r) => r.list.routeId === group.source.routeId,
  );
});

const detailResource = computed(() => {
  const ref = view.value.selectedRef;
  return ref ? resourceByKind.value.get(ref.kind) : undefined;
});

const activeTab = computed(() =>
  projection.value?.tabs?.find((t) => t.key === view.value.activeTab),
);

function onSelectGroup(key: string): void {
  ws.selectGroup(props.id, key);
}
function onSelectNode(ref: ResourceRef): void {
  ws.selectRef(props.id, ref);
}
function onSelectRow(row: Row): void {
  ws.selectRow(props.id, row);
}
function onEnrolled(): void {
  online.value = true;
}
</script>

<template>
  <div class="flex h-full flex-col">
    <header
      class="flex items-center gap-3 border-b border-surface-200 px-5 py-3 dark:border-surface-800"
    >
      <AppIcon
        :icon="connection?.icon ?? projection?.icon"
        :size="20"
        class="text-surface-500"
      />
      <div class="min-w-0">
        <h1 class="truncate font-semibold text-surface-900 dark:text-surface-0">
          {{ connection?.name ?? id }}
        </h1>
        <p class="truncate text-xs text-surface-400">
          {{ projection?.title ?? connection?.protocol }} ·
          {{ connection?.transport }}
        </p>
      </div>
    </header>

    <div class="min-h-0 flex-1">
      <p v-if="loading" class="p-6 text-surface-400">Loading workspace…</p>
      <p v-else-if="error" class="p-6 text-red-500">{{ error }}</p>

      <EnrollPanel
        v-else-if="needsEnroll"
        :connection-id="id"
        @online="onEnrolled"
      />

      <template v-else-if="projection">
        <!-- Flat tab layout. The tab bar is PrimeVue; content is rendered through
             KeepAlive (not PrimeVue's lazy TabPanels) so switching tabs HIDES a
             panel instead of destroying it — terminals/streams stay alive. -->
        <div v-if="projection.layout === 'tabs'" class="flex h-full flex-col">
          <Tabs
            :value="view.activeTab ?? ''"
            @update:value="ws.setActiveTab(id, String($event))"
          >
            <TabList>
              <Tab v-for="t in projection.tabs" :key="t.key" :value="t.key">
                <AppIcon :icon="t.icon" :size="15" />
                {{ t.label }}
              </Tab>
            </TabList>
          </Tabs>
          <div class="min-h-0 flex-1 overflow-hidden">
            <KeepAlive :max="10">
              <PanelHost
                v-if="activeTab"
                :key="`${id}:${activeTab.key}`"
                :panel="activeTab.panel"
                :connection-id="id"
                :source="activeTab.source"
                :config="activeTab.config"
              />
            </KeepAlive>
          </div>
        </div>

        <!-- Hierarchical sidebar-tree layout -->
        <div v-else class="flex h-full">
          <div
            class="w-64 shrink-0 border-r border-surface-200 dark:border-surface-800"
          >
            <ResourceTree
              :connection-id="id"
              :groups="projection.tree ?? []"
              :selected-group="view.selectedGroup"
              :selected-uid="view.selectedRef?.uid"
              @select-group="onSelectGroup"
              @select-node="onSelectNode"
            />
          </div>
          <div class="min-w-0 flex-1 overflow-hidden">
            <DetailView
              v-if="view.selectedRow && detailResource"
              :connection-id="id"
              :detail="detailResource.detail"
              :row="view.selectedRow"
              :actions="projection.actions ?? []"
            />
            <TablePanel
              v-else-if="groupResource"
              :key="groupResource.kind"
              :connection-id="id"
              :source="groupResource.list"
              :config="{
                columns: groupResource.columns,
                watch: groupResource.watch,
              }"
              @select="onSelectRow"
            />
            <div
              v-else
              class="flex h-full items-center justify-center text-sm text-surface-400"
            >
              Select an item from the tree.
            </div>
          </div>
        </div>
      </template>
    </div>
  </div>
</template>
