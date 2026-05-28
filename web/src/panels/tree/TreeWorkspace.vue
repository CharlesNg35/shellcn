<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import Button from "primevue/button";
import { VueDraggable } from "vue-draggable-plus";
import {
  useWorkspaceStore,
  MAX_WORKBENCH_TABS,
  type OpenView,
} from "../../stores/workspace";
import ResourceTree from "./ResourceTree.vue";
import DetailView from "../detail/DetailView.vue";
import TablePanel from "../table/TablePanel.vue";
import AppIcon from "../../components/AppIcon.vue";
import type {
  Action,
  ResourceRef,
  ResourceType,
  Row,
  TreeGroup,
} from "../../types/projection";

// Fallback qualifier (ref identity) for items opened outside the tree; the tree
// ancestor path is preferred. See openDetail.
function refSubtitle(ref: ResourceRef): string {
  return [ref.scope, ref.namespace].filter(Boolean).join(" / ");
}

// The sidebar-tree layout: a resource tree on the left and a closable workbench
// tab strip on the right, where each open view is a resource detail or a kind
// list. Extracted from ConnectionWorkspace so the orchestrator stays lean.
const props = defineProps<{
  connectionId: string;
  tree: TreeGroup[];
  resources: ResourceType[];
  actions: Action[];
}>();

const ws = useWorkspaceStore();
const view = computed(() => ws.view(props.connectionId));
const activeView = computed(() => ws.activeView(props.connectionId));
const tabStrip = ref<HTMLElement | null>(null);

const resourceByKind = computed(() => {
  const map = new Map<string, ResourceType>();
  for (const r of props.resources) map.set(r.kind, r);
  return map;
});

function resolveGroupResource(key: string): ResourceType | undefined {
  const group = props.tree.find((g) => g.key === key);
  if (!group) return undefined;
  if (group.resourceKind) return resourceByKind.value.get(group.resourceKind);
  return props.resources.find((r) => r.list.routeId === group.source?.routeId);
}

const activeDetailResource = computed(() => {
  const v = activeView.value;
  return v?.kind === "detail" && v.ref
    ? resourceByKind.value.get(v.ref.kind)
    : undefined;
});

const activeListResource = computed(() => {
  const v = activeView.value;
  if (v?.kind !== "list") return undefined;
  if (v.resourceKind) return resourceByKind.value.get(v.resourceKind);
  return v.groupKey ? resolveGroupResource(v.groupKey) : undefined;
});

// The opened kind list's source, scoped by any params the nav node carried.
const activeListSource = computed(() => {
  const res = activeListResource.value;
  if (!res) return undefined;
  const params = activeView.value?.params;
  return params
    ? { ...res.list, params: { ...res.list.params, ...params } }
    : res.list;
});

// A runtime columns source, scoped by the same nav params as the list (so a CRD
// list fetches the columns for its specific kind).
const activeColumnsSource = computed(() => {
  const res = activeListResource.value;
  if (!res?.columnsSource) return undefined;
  const params = activeView.value?.params;
  return params
    ? {
        ...res.columnsSource,
        params: { ...res.columnsSource.params, ...params },
      }
    : res.columnsSource;
});

const treeSelectedGroup = computed(() =>
  activeView.value?.kind === "list" ? activeView.value.groupKey : undefined,
);
const treeSelectedUid = computed(() =>
  activeView.value?.kind === "detail" ? activeView.value.ref?.uid : undefined,
);

function workbenchTabTitle(v: OpenView): string {
  return v.subtitle ? `${v.subtitle} / ${v.title}` : v.title;
}

function openDetail(row: Row, qualifier?: string): void {
  if (!row.ref || !resourceByKind.value.has(row.ref.kind)) return;
  ws.openPreviewView(props.connectionId, {
    id: "detail:" + row.ref.uid,
    title: row.ref.name,
    subtitle: qualifier || refSubtitle(row.ref),
    kind: "detail",
    ref: row.ref,
    row,
  });
}

function onSelectGroup(key: string): void {
  const group = props.tree.find((g) => g.key === key);
  if (!group) return;
  // A container group (no resolvable resource) only expands — no view/tab.
  if (!resolveGroupResource(key)) return;
  ws.openPreviewView(props.connectionId, {
    id: "group:" + key,
    title: group.label,
    icon: group.icon,
    kind: "list",
    groupKey: key,
  });
}

// Drag-to-reorder the workbench tabs, reusing the same vue-draggable-plus
// mechanism as the connection sidebar. Order is session-only (workspace store).
const tabs = computed<OpenView[]>({
  get: () => view.value.views,
  set: (next) => ws.setViews(props.connectionId, next),
});

async function scrollActiveTabIntoView(): Promise<void> {
  await nextTick();
  const activeTab = tabStrip.value?.querySelector<HTMLElement>(
    "[data-active-tab='true']",
  );
  if (!activeTab || typeof activeTab.scrollIntoView !== "function") return;
  activeTab.scrollIntoView({
    block: "nearest",
    inline: "nearest",
    behavior: "smooth",
  });
}

watch(
  () => [view.value.activeViewId, view.value.views.length] as const,
  ([activeId]) => {
    if (activeId) void scrollActiveTabIntoView();
  },
);

function onSelectList(kind: string, params?: Record<string, string>): void {
  const res = resourceByKind.value.get(kind);
  if (!res) return;
  const suffix = params
    ? ":" +
      Object.entries(params)
        .map(([k, v]) => `${k}=${v}`)
        .join(",")
    : "";
  ws.openPreviewView(props.connectionId, {
    id: "list:" + kind + suffix,
    title: res.title,
    subtitle: params ? Object.values(params).join(" / ") : undefined,
    kind: "list",
    resourceKind: kind,
    params,
  });
}
</script>

<template>
  <div class="flex h-full">
    <div
      class="w-64 shrink-0 border-r border-surface-200 dark:border-surface-800"
    >
      <ResourceTree
        :connection-id="connectionId"
        :groups="tree"
        :selected-group="treeSelectedGroup"
        :selected-uid="treeSelectedUid"
        @select-group="onSelectGroup"
        @select-node="openDetail"
        @select-list="onSelectList"
      />
    </div>
    <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
      <div
        v-if="view.views.length"
        ref="tabStrip"
        class="shrink-0 overflow-x-auto border-b border-surface-200 bg-surface-50 dark:border-surface-800 dark:bg-surface-900"
      >
        <VueDraggable
          v-model="tabs"
          :animation="150"
          ghost-class="opacity-40"
          class="flex w-max min-w-full items-center gap-1 px-2 py-1"
        >
          <button
            v-for="v in tabs"
            :key="v.id"
            type="button"
            :title="workbenchTabTitle(v)"
            :aria-label="workbenchTabTitle(v)"
            :data-active-tab="v.id === view.activeViewId ? 'true' : undefined"
            :data-preview-tab="v.preview ? 'true' : undefined"
            class="group flex max-w-60 shrink-0 cursor-pointer items-center gap-1.5 overflow-hidden rounded px-2 py-1 text-xs transition-colors active:cursor-pointer"
            :class="
              v.id === view.activeViewId
                ? 'bg-surface-0 text-surface-900 shadow-sm dark:bg-surface-800 dark:text-surface-0'
                : 'text-surface-500 hover:text-surface-800 dark:hover:text-surface-200'
            "
            @click="ws.activateView(connectionId, v.id)"
            @dblclick="ws.pinView(connectionId, v.id)"
          >
            <AppIcon v-if="v.icon" :icon="v.icon" :size="13" />
            <span class="flex min-w-0 flex-1 items-baseline gap-1">
              <span
                class="truncate font-medium"
                :class="{ italic: v.preview }"
                >{{ v.title }}</span
              >
              <span
                v-if="v.subtitle"
                class="truncate text-[10px] text-surface-400"
              >
                {{ v.subtitle }}
              </span>
            </span>
            <Button
              type="button"
              text
              rounded
              severity="secondary"
              size="small"
              :aria-label="`Close ${v.title}`"
              :pt="{ root: 'h-4 w-4 p-0 opacity-60 hover:opacity-100' }"
              @click.stop="ws.closeView(connectionId, v.id)"
              @dblclick.stop
            >
              <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="12" />
            </Button>
          </button>
        </VueDraggable>
      </div>

      <div class="min-h-0 flex-1 overflow-hidden">
        <KeepAlive :max="MAX_WORKBENCH_TABS">
          <DetailView
            v-if="
              activeView?.kind === 'detail' &&
              activeDetailResource &&
              activeView.row
            "
            :key="activeView.id"
            :connection-id="connectionId"
            :detail="activeDetailResource.detail"
            :row="activeView.row"
            :actions="actions"
            @select="openDetail"
          />
          <TablePanel
            v-else-if="activeListResource && activeListSource"
            :key="activeView!.id"
            :connection-id="connectionId"
            :source="activeListSource"
            :config="{
              columns: activeListResource.columns,
              columnsSource: activeColumnsSource,
              watch: activeListResource.watch,
              actionIds: activeListResource.listActionIds ?? [],
              rowActionIds:
                activeListResource.rowActionIds ?? activeListResource.actionIds,
            }"
            :actions="actions"
            @select="openDetail"
          />
        </KeepAlive>
        <div
          v-if="!activeView"
          class="flex h-full items-center justify-center text-sm text-surface-400"
        >
          Select an item from the tree.
        </div>
      </div>
    </div>
  </div>
</template>
