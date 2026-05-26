<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import Button from "primevue/button";
import { api, ApiError } from "../api/client";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useSessionsStore } from "../stores/sessions";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import PanelHost from "../panels/PanelHost.vue";
import EnrollPanel from "../panels/EnrollPanel.vue";
import ResourceTree from "../panels/tree/ResourceTree.vue";
import TablePanel from "../panels/TablePanel.vue";
import DetailView from "../panels/DetailView.vue";
import ConnectionFormDialog from "../components/ConnectionFormDialog.vue";
import ShareDialog from "../components/ShareDialog.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { recordingForStream } from "../composables/useRecordingControl";
import type {
  Action,
  PluginProjection,
  ResourceType,
  Row,
  Tab as TabDef,
} from "../types/projection";

const props = defineProps<{ id: string }>();
const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const sessions = useSessionsStore();
const router = useRouter();
const notify = useNotify();

const showEdit = ref(false);
const showShare = ref(false);
const { confirmDanger } = useConfirmAction();

const canManage = computed(() => connection.value?.canManage ?? false);

function askDelete(): void {
  confirmDanger({
    header: "Delete connection",
    message: `Delete “${connection.value?.name ?? props.id}”? This cannot be undone.`,
    accept: onDelete,
  });
}

async function onDelete(): Promise<void> {
  try {
    await api.del(`/connections/${props.id}`);
    await conns.refresh();
    notify.success("Connection deleted");
    await router.push({ name: "home" });
  } catch (e) {
    if (e instanceof ApiError && e.status === 409) {
      notify.error("Could not delete", e.message);
    }
  }
}

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

// A connection does not open on its own: the user connects explicitly, so a
// page refresh lands on the prompt rather than dialing the target again. The
// flag is per-connection (each connection owns its workspace instance) and is
// kept alive across in-app navigation by the parent <KeepAlive>.
const connected = ref(false);

const channelPrefix = computed(() => `${props.id}:`);
const hasLiveStream = computed(() =>
  Object.entries(sessions.statuses).some(
    ([key, status]) => key.startsWith(channelPrefix.value) && status === "open",
  ),
);

// Reflect a stream opening or its last one closing in the sidebar dot promptly,
// instead of waiting for the slow background poll.
watch(hasLiveStream, () => void conns.refresh().catch(() => undefined));

function connect(): void {
  connected.value = true;
}

function disconnect(): void {
  sessions.closeWhere((key) => key.startsWith(channelPrefix.value));
  connected.value = false;
  void conns.refresh().catch(() => undefined);
}

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

// Merge a recording descriptor into a stream tab's config so the panel can show
// recording state/controls — kept in the generic renderer, not per plugin.
function tabConfig(tab: TabDef): Record<string, unknown> {
  const base = tab.config ?? {};
  if (!projection.value || !tab.source) return base;
  const rec = recordingForStream(
    projection.value,
    connection.value,
    tab.source.routeId,
  );
  return rec ? { ...base, _recording: rec } : base;
}

function onSelectGroup(key: string): void {
  ws.selectGroup(props.id, key);
}
function onSelectNode(row: Row): void {
  ws.selectRow(props.id, row);
}
function onSelectRow(row: Row): void {
  ws.selectRow(props.id, row);
}
function onActionDone(action: Action): void {
  const tabKey = action.onSuccess?.selectTab;
  if (!tabKey || !projection.value?.tabs?.some((tab) => tab.key === tabKey)) {
    return;
  }
  ws.setActiveTab(props.id, tabKey);
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

      <div class="ml-auto flex items-center gap-1">
        <Button
          v-if="connected"
          severity="secondary"
          size="small"
          title="Close the live session"
          class="mr-1"
          @click="disconnect"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-emerald-400" />
          Disconnect
        </Button>
        <template v-if="canManage">
          <Button
            text
            rounded
            severity="secondary"
            title="Share"
            aria-label="Share connection"
            @click="showShare = true"
          >
            <AppIcon :icon="{ type: 'name', value: 'users' }" :size="17" />
          </Button>
          <Button
            text
            rounded
            severity="secondary"
            title="Edit"
            aria-label="Edit connection"
            @click="showEdit = true"
          >
            <AppIcon :icon="{ type: 'name', value: 'pencil' }" :size="17" />
          </Button>
          <Button
            text
            rounded
            severity="danger"
            title="Delete"
            aria-label="Delete connection"
            @click="askDelete()"
          >
            <AppIcon :icon="{ type: 'name', value: 'trash' }" :size="17" />
          </Button>
        </template>
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

      <div
        v-else-if="!connected"
        class="flex h-full flex-col items-center justify-center gap-5 p-8 text-center"
      >
        <span
          class="flex h-16 w-16 items-center justify-center rounded-2xl bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-400"
        >
          <AppIcon :icon="connection?.icon ?? projection?.icon" :size="28" />
        </span>
        <div class="space-y-1">
          <h2
            class="text-lg font-semibold text-surface-900 dark:text-surface-0"
          >
            Not connected
          </h2>
          <p class="text-sm text-surface-500 dark:text-surface-400">
            {{ connection?.name ?? id }} ·
            {{ projection?.title ?? connection?.protocol }}
          </p>
        </div>
        <Button @click="connect">
          <AppIcon :icon="{ type: 'name', value: 'play' }" :size="16" />
          Connect
        </Button>
      </div>

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
                :config="tabConfig(activeTab)"
                :actions="projection.actions ?? []"
                @action-done="onActionDone"
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
              @action-done="onActionDone"
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
              :actions="projection.actions ?? []"
              @select="onSelectRow"
              @action-done="onActionDone"
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

    <ConnectionFormDialog v-model:visible="showEdit" :connection-id="id" />
    <ShareDialog
      v-model:visible="showShare"
      resource="connections"
      :resource-id="id"
      :resource-name="connection?.name ?? id"
      allow-manage
    />
  </div>
</template>
