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
import { useConnectionSessionsStore } from "../stores/connectionSessions";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import { KEEP_ALIVE_TOP_LEVEL_PANELS_MAX } from "../stores/sessionLimits";
import { useNotify } from "../composables/useNotify";
import AppIcon from "../components/AppIcon.vue";
import PanelHost from "../panels/core/PanelHost.vue";
import { provideNavigableKinds } from "../panels/core/navigable";
import EnrollPanel from "../panels/enroll/EnrollPanel.vue";
import ConnectPanel from "../panels/connect/ConnectPanel.vue";
import PanelError from "../panels/shared/PanelError.vue";
import Dialog from "primevue/dialog";
import TreeWorkspace from "../panels/tree/TreeWorkspace.vue";
import DashboardWorkspace from "../panels/dashboard/DashboardWorkspace.vue";
import DockPanel from "../panels/dock/DockPanel.vue";
import { useDockStore } from "../stores/dock";
import ConnectionFormDialog from "../components/ConnectionFormDialog.vue";
import ShareDialog from "../components/ShareDialog.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { recordingForStream } from "../composables/useRecordingControl";
import type {
  Action,
  PluginProjection,
  Tab as TabDef,
} from "../types/projection";
import { dialogRoot } from "../primevue/preset";

const props = defineProps<{ id: string }>();
const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const dock = useDockStore();
const dockState = computed(() => dock.state(props.id));
const connectionSessions = useConnectionSessionsStore();
const liveStatus = useConnectionStatusStore();
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

// Resource kinds the connection can open a detail view for. The generic table
// uses this to decide row-click = navigate (resource) vs select (everything
// else), so no table has to declare it.
provideNavigableKinds(
  computed(
    () => new Set((projection.value?.resources ?? []).map((r) => r.kind)),
  ),
);
const loading = ref(true);
const error = ref<string | null>(null);
const sessionConnecting = ref(false);
// The connect screen can hand off to the agent enrollment screen and back.
const showEnroll = ref(false);

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

watch(
  () => props.id,
  () => {
    showEnroll.value = false;
    load();
  },
  { immediate: true },
);

// A connection does not open on its own: the user connects explicitly, so a
// page refresh lands on the prompt rather than dialing the target again. The
// connected set lives in the store so the sidebar dot can reflect it (and it
// survives in-app navigation, resetting only on a full reload).
const connected = computed(() => ws.isConnected(props.id));
const connectError = computed(() => {
  const status = liveStatus.get(props.id);
  return status?.state === "error"
    ? (status.reason ?? "Connection failed.")
    : "";
});

async function connect(): Promise<void> {
  showEnroll.value = false;
  sessionConnecting.value = true;
  try {
    await connectionSessions.connect(props.id, true);
  } finally {
    sessionConnecting.value = false;
  }
}

async function disconnect(): Promise<void> {
  try {
    await connectionSessions.disconnect(props.id);
  } catch (e) {
    notify.error("Could not close session", (e as Error).message);
  }
}

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

function onActionDone(action: Action): void {
  const tabKey = action.onSuccess?.selectTab;
  if (!tabKey || !projection.value?.tabs?.some((tab) => tab.key === tabKey)) {
    return;
  }
  ws.setActiveTab(props.id, tabKey);
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
            <AppIcon :icon="{ type: 'lucide', value: 'users' }" :size="17" />
          </Button>
          <Button
            text
            rounded
            severity="secondary"
            title="Edit"
            aria-label="Edit connection"
            @click="showEdit = true"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'pencil' }" :size="17" />
          </Button>
          <Button
            text
            rounded
            severity="danger"
            title="Delete"
            aria-label="Delete connection"
            @click="askDelete()"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'trash' }" :size="17" />
          </Button>
        </template>
      </div>
    </header>

    <div class="min-h-0 flex-1">
      <div
        v-if="loading"
        class="flex h-full items-center justify-center p-6 text-sm text-surface-400"
        role="status"
      >
        Loading workspace…
      </div>
      <PanelError v-else-if="error" :message="error" retryable @retry="load" />

      <EnrollPanel
        v-else-if="!connected && showEnroll"
        :connection-id="id"
        @online="showEnroll = false"
      />

      <ConnectPanel
        v-else-if="!connected"
        :connection-id="id"
        :connection="connection"
        :connecting="sessionConnecting"
        :error-message="connectError"
        @connect="connect"
        @enroll="showEnroll = true"
      />

      <div v-else-if="projection" class="flex h-full min-h-0 flex-col">
        <div class="min-h-0 flex-1 overflow-hidden">
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
              <KeepAlive :max="KEEP_ALIVE_TOP_LEVEL_PANELS_MAX">
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

          <!-- Dashboard layout: every panel rendered at once in a grid. -->
          <DashboardWorkspace
            v-else-if="projection.layout === 'dashboard'"
            :connection-id="id"
            :tabs="projection.tabs ?? []"
            :actions="projection.actions ?? []"
            :resolve-config="tabConfig"
            @action-done="onActionDone"
          />

          <!-- Hierarchical sidebar-tree layout (tree + workbench tabs). -->
          <TreeWorkspace
            v-else
            :connection-id="id"
            :tree="projection.tree ?? []"
            :resources="projection.resources ?? []"
            :actions="projection.actions ?? []"
          />
        </div>

        <DockPanel v-if="dockState.items.length" :connection-id="id" />

        <Dialog
          :visible="!!dockState.dialog"
          modal
          :header="dockState.dialog?.title"
          :dismissable-mask="true"
          :pt="{
            root: dialogRoot('max-w-4xl'),
            content: 'min-h-0 overflow-hidden p-0',
          }"
          @update:visible="(v) => !v && dock.closeDialog(id)"
        >
          <div class="h-[60vh]">
            <PanelHost
              v-if="dockState.dialog"
              :panel="dockState.dialog.panel"
              :connection-id="id"
              :source="dockState.dialog.source"
              :config="dockState.dialog.config"
              :resource="dockState.dialog.resource"
            />
          </div>
        </Dialog>
      </div>
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
