<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRouter } from "vue-router";
import Tabs from "primevue/tabs";
import TabList from "primevue/tablist";
import Tab from "primevue/tab";
import Button from "primevue/button";
import { ApiError } from "../api/client";
import { connectionsApi } from "../api/connections";
import { useConnectionsStore } from "../stores/connections";
import { useWorkspaceStore } from "../stores/workspace";
import { useConnectionSessionsStore } from "../stores/connectionSessions";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import { useScopeStore } from "../stores/scope";
import { KEEP_ALIVE_TOP_LEVEL_PANELS_MAX } from "../stores/sessionLimits";
import { useNotify } from "../composables/useNotify";
import { useWorkspaceUrlSync } from "../composables/useWorkspaceUrlSync";
import AppIcon from "../components/AppIcon.vue";
import PanelHost from "../panels/core/PanelHost.vue";
import ActionBar from "../panels/shared/ActionBar.vue";
import ScopeBar from "../panels/shared/ScopeBar.vue";
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
import AiChatLauncher from "../components/AiChatLauncher.vue";
import { useConfirmAction } from "../composables/useConfirmAction";
import { recordingForStream } from "../composables/useRecordingControl";
import { useActionSuccess } from "../panels/core/actionSuccess";
import { providePanelConfigSchemas } from "../panels/core/config";
import { providePanelRecordingResolver } from "../panels/core/recording";
import {
  resolvedPanelConfig,
  resolvedPanelType,
} from "../panels/core/variants";
import { isVisible } from "../panels/form/condition";
import { Layout } from "../types/projection";
import type {
  Action,
  PluginProjection,
  Row,
  Tab as TabDef,
} from "../types/projection";
import { dialogRoot } from "../primevue/preset";

const props = defineProps<{ id: string }>();
const conns = useConnectionsStore();
const ws = useWorkspaceStore();
const dock = useDockStore();
const scope = useScopeStore();
const dockState = computed(() => dock.state(props.id));
const connectionSessions = useConnectionSessionsStore();
const liveStatus = useConnectionStatusStore();
const router = useRouter();
const notify = useNotify();

const showEdit = ref(false);
const showShare = ref(false);
const { confirmDanger } = useConfirmAction();

const canManage = computed(() => connection.value?.canManage ?? false);
const canShare = computed(() => connection.value?.canShare ?? false);

function askDelete(): void {
  confirmDanger({
    header: "Delete connection",
    message: `Delete “${connection.value?.name ?? props.id}”? This cannot be undone.`,
    accept: onDelete,
  });
}

async function onDelete(): Promise<void> {
  try {
    await connectionsApi.remove(props.id);
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
const connection = computed(() => conns.byId(props.id));
const connectionConfig = computed<Record<string, unknown>>(
  () => connection.value?.config ?? {},
);
providePanelConfigSchemas(
  computed(() => projection.value?.panelConfigSchemas ?? {}),
);
providePanelRecordingResolver((source) => {
  if (!projection.value || !source) return null;
  return recordingForStream(projection.value, connection.value, source.routeId);
});

provideNavigableKinds(
  computed(
    () => new Set((projection.value?.resources ?? []).map((r) => r.kind)),
  ),
);
const loading = ref(true);
const error = ref<string | null>(null);
const sessionConnecting = ref(false);
const showEnroll = ref(false);

const view = computed(() => ws.view(props.id));
const workspaceUrl = useWorkspaceUrlSync({
  connectionId: () => props.id,
  projection,
});

async function load(): Promise<void> {
  loading.value = true;
  error.value = null;
  projection.value = null;
  scope.configure(props.id, []);
  try {
    if (!conns.loaded) await conns.load();
    const c = conns.byId(props.id);
    if (!c) throw new Error(`Unknown connection "${props.id}".`);
    ws.open(props.id);
    const proj = await conns.projection(c.protocol);
    projection.value = proj;
    scope.configure(props.id, proj.scope ?? []);
    workspaceUrl.restoreFromUrl();
    const firstVisibleTab = proj.tabs?.find((tab) =>
      isVisible(tab.visibleWhen, c.config ?? {}),
    );
    const currentTab = ws.view(props.id).activeTab;
    const currentVisible = proj.tabs?.some(
      (tab) =>
        tab.key === currentTab && isVisible(tab.visibleWhen, c.config ?? {}),
    );
    if ((!currentTab || !currentVisible) && firstVisibleTab) {
      ws.setActiveTab(props.id, firstVisibleTab.key);
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

const visibleTabs = computed(() =>
  (projection.value?.tabs ?? []).filter((tab) =>
    isVisible(tab.visibleWhen, connectionConfig.value),
  ),
);

const activeTab = computed(() =>
  visibleTabs.value.find((t) => t.key === view.value.activeTab),
);

const resourceKinds = computed(
  () => new Set((projection.value?.resources ?? []).map((r) => r.kind)),
);

function tabConfig(tab: TabDef): Record<string, unknown> {
  return resolvedPanelConfig(tab, connectionConfig.value);
}

function tabPanel(tab: TabDef) {
  return resolvedPanelType(tab, connectionConfig.value);
}

function refSubtitle(ref: Row["ref"]): string {
  return ref?.namespace ? `${ref.kind} / ${ref.namespace}` : (ref?.kind ?? "");
}

function openDetail(row: Row): void {
  if (!row.ref || !resourceKinds.value.has(row.ref.kind)) return;
  ws.openPreviewView(props.id, {
    id: "detail:" + row.ref.uid,
    title: row.ref.name,
    subtitle: refSubtitle(row.ref),
    kind: "detail",
    ref: row.ref,
    row,
  });
}

const actionSuccess = useActionSuccess({
  connectionId: () => props.id,
  tabs: visibleTabs,
  resolvePanel: tabPanel,
  selectTab: (key) => ws.setActiveTab(props.id, key),
});

const scopeFilters = computed(() => projection.value?.scope ?? []);

const headerActions = computed<Action[]>(() => {
  const ids = projection.value?.headerActions ?? [];
  const byId = new Map((projection.value?.actions ?? []).map((a) => [a.id, a]));
  return ids.map((id) => byId.get(id)).filter((a): a is Action => Boolean(a));
});

watch(
  visibleTabs,
  (tabs) => {
    if (!tabs.length) return;
    if (!tabs.some((tab) => tab.key === view.value.activeTab)) {
      ws.setActiveTab(props.id, tabs[0].key);
    }
  },
  { flush: "post" },
);

async function onActionDone(
  action: Action,
  result?: Record<string, unknown>,
): Promise<void> {
  await actionSuccess.run(action, result);
}
</script>

<template>
  <div class="flex h-full flex-col">
    <header
      class="flex items-center gap-3 border-b border-surface-200 px-5 py-3 dark:border-surface-800"
    >
      <div class="flex min-w-0 flex-1 items-center gap-3">
        <AppIcon
          :icon="connection?.icon ?? projection?.icon"
          :size="20"
          class="text-surface-500"
        />
        <div class="min-w-0">
          <h1
            class="truncate font-semibold text-surface-900 dark:text-surface-0"
          >
            {{ connection?.name ?? id }}
          </h1>
          <p class="truncate text-xs text-surface-400">
            {{ projection?.title ?? connection?.protocol }} ·
            {{ connection?.transport }}
          </p>
        </div>
      </div>

      <div
        v-if="connected && (scopeFilters.length || headerActions.length)"
        class="flex shrink-0 items-center gap-3 rounded-lg border border-surface-200 bg-surface-50 px-2 py-1 shadow-sm dark:border-surface-700 dark:bg-surface-800/60"
      >
        <ScopeBar
          v-if="scopeFilters.length"
          :connection-id="id"
          :scope="scopeFilters"
        />
        <span
          v-if="scopeFilters.length && headerActions.length"
          class="h-5 w-px bg-surface-200 dark:bg-surface-700"
        />
        <ActionBar
          v-if="headerActions.length"
          :connection-id="id"
          :actions="headerActions"
          @done="onActionDone"
        />
      </div>

      <div class="flex flex-1 items-center justify-end gap-1">
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
        <AiChatLauncher
          :connection-id="id"
          :connected="connected"
          :ai-mode="connection?.aiMode"
        />
        <Button
          v-if="canShare"
          text
          rounded
          severity="secondary"
          title="Share"
          aria-label="Share connection"
          @click="showShare = true"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'users' }" :size="17" />
        </Button>
        <template v-if="canManage">
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
          <div
            v-if="projection.layout === Layout.Tabs"
            class="flex h-full min-w-0 flex-col"
          >
            <Tabs
              :value="view.activeTab ?? ''"
              scrollable
              @update:value="ws.setActiveTab(id, String($event))"
            >
              <TabList>
                <Tab v-for="t in visibleTabs" :key="t.key" :value="t.key">
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
                  :panel="tabPanel(activeTab)"
                  :connection-id="id"
                  :source="activeTab.source"
                  :config="tabConfig(activeTab)"
                  :actions="projection.actions ?? []"
                  @action-done="onActionDone"
                />
              </KeepAlive>
            </div>
          </div>

          <div
            v-else-if="projection.layout === Layout.Single && activeTab"
            class="h-full min-h-0 overflow-hidden"
          >
            <PanelHost
              :key="`${id}:${activeTab.key}`"
              :panel="tabPanel(activeTab)"
              :connection-id="id"
              :source="activeTab.source"
              :config="tabConfig(activeTab)"
              :actions="projection.actions ?? []"
              @action-done="onActionDone"
            />
          </div>

          <DashboardWorkspace
            v-else-if="projection.layout === Layout.Dashboard"
            :connection-id="id"
            :tabs="visibleTabs"
            :actions="projection.actions ?? []"
            :resolve-config="tabConfig"
            :resolve-panel="tabPanel"
            @action-done="onActionDone"
            @select="openDetail"
          />

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
