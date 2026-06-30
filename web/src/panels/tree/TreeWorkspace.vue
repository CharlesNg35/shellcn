<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import Button from "primevue/button";
import { VueDraggable } from "vue-draggable-plus";
import { KEEP_ALIVE_WORKBENCH_TABS_MAX } from "@/stores/sessionLimits";
import {
  MAX_TREE_SIDEBAR_WIDTH,
  MIN_TREE_SIDEBAR_WIDTH,
  useWorkspaceStore,
  type OpenView,
} from "@/stores/workspace";
import { useScopeStore } from "@/stores/scope";
import ResourceTree from "./ResourceTree.vue";
import DetailView from "../detail/DetailView.vue";
import PanelHost from "../core/PanelHost.vue";
import AppIcon from "@/components/AppIcon.vue";
import { useConnectionInvalidationRefresh } from "../shared/useConnectionInvalidationRefresh";
import type {
  Action,
  ResourceIdentity,
  ResourceType,
  Row,
  TreeGroup,
} from "@/types/projection";

function refSubtitle(ref: ResourceIdentity): string {
  const location = [ref.scope, ref.namespace].filter(Boolean).join(" / ");
  return [ref.kind, location].filter(Boolean).join(" · ");
}

const props = defineProps<{
  connectionId: string;
  tree: TreeGroup[];
  resources: ResourceType[];
  actions: Action[];
}>();

const ws = useWorkspaceStore();
const scope = useScopeStore();
const view = computed(() => ws.view(props.connectionId));
const activeView = computed(() => ws.activeView(props.connectionId));
const layout = computed(() => ws.layout(props.connectionId));
const tabStrip = ref<HTMLElement | null>(null);
const scopeKey = computed(() => scope.key(props.connectionId));
const treeRefreshNonce = ref(0);
const isSidebarResizing = ref(false);
const treeRefreshKey = computed(() => {
  if (treeRefreshNonce.value === 0) {
    return scopeKey.value;
  }
  if (!scopeKey.value) {
    return String(treeRefreshNonce.value);
  }
  return `${scopeKey.value}:${treeRefreshNonce.value}`;
});

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

const activeListSource = computed(() => {
  const res = activeListResource.value;
  if (!res) return undefined;
  const params = activeView.value?.params;
  return params
    ? { ...res.list, params: { ...res.list.params, ...params } }
    : res.list;
});
const activeListKey = computed(() =>
  activeView.value
    ? `${props.connectionId}:${activeView.value.id}:${scopeKey.value}`
    : `${props.connectionId}:list:${scopeKey.value}`,
);

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
  return v.subtitle ? `${v.title} - ${v.subtitle} ` : v.title;
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
  if (!resolveGroupResource(key)) return;
  ws.openPreviewView(props.connectionId, {
    id: "group:" + key,
    title: group.label,
    icon: group.icon,
    kind: "list",
    groupKey: key,
  });
}

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

function refreshTree(): void {
  treeRefreshNonce.value += 1;
}

useConnectionInvalidationRefresh({
  connectionId: () => props.connectionId,
  refresh: refreshTree,
});

function onListActionDone(): void {
  refreshTree();
}

function onDetailActionDone(action: Action): void {
  refreshTree();
  if (action.onSuccess?.navigate !== "list") return;
  const v = activeView.value;
  if (v?.kind !== "detail" || !v.ref) return;
  const kind = v.ref.kind;
  ws.closeView(props.connectionId, v.id);
  if (resourceByKind.value.has(kind)) onSelectList(kind);
}

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

let sidebarStartX = 0;
let sidebarStartWidth = 0;
let resizeCursorBefore = "";
let resizeUserSelectBefore = "";

function stopSidebarResize(): void {
  isSidebarResizing.value = false;
  document.documentElement.style.cursor = resizeCursorBefore;
  document.body.style.userSelect = resizeUserSelectBefore;
  window.removeEventListener("pointermove", onSidebarResizeMove);
  window.removeEventListener("pointerup", stopSidebarResize);
}

function onSidebarResizeMove(event: PointerEvent): void {
  ws.setTreeSidebarWidth(
    props.connectionId,
    sidebarStartWidth + event.clientX - sidebarStartX,
  );
}

function startSidebarResize(event: PointerEvent): void {
  const target = event.currentTarget;
  if (
    target instanceof HTMLElement &&
    typeof target.setPointerCapture === "function"
  ) {
    target.setPointerCapture(event.pointerId);
  }
  isSidebarResizing.value = true;
  resizeCursorBefore = document.documentElement.style.cursor;
  resizeUserSelectBefore = document.body.style.userSelect;
  document.documentElement.style.cursor = "col-resize";
  document.body.style.userSelect = "none";
  sidebarStartX = event.clientX;
  sidebarStartWidth = layout.value.treeSidebarWidth;
  window.addEventListener("pointermove", onSidebarResizeMove);
  window.addEventListener("pointerup", stopSidebarResize);
}

function onSidebarResizeKeydown(event: KeyboardEvent): void {
  if (event.key === "ArrowLeft") {
    event.preventDefault();
    ws.setTreeSidebarWidth(
      props.connectionId,
      layout.value.treeSidebarWidth - 24,
    );
    return;
  }
  if (event.key === "ArrowRight") {
    event.preventDefault();
    if (layout.value.treeSidebarWidth === 0) {
      ws.setTreeSidebarWidth(props.connectionId, MIN_TREE_SIDEBAR_WIDTH);
    } else {
      ws.setTreeSidebarWidth(
        props.connectionId,
        layout.value.treeSidebarWidth + 24,
      );
    }
    return;
  }
  if (event.key === "Home") {
    event.preventDefault();
    ws.setTreeSidebarWidth(props.connectionId, 0);
    return;
  }
  if (event.key === "End") {
    event.preventDefault();
    ws.setTreeSidebarWidth(props.connectionId, MAX_TREE_SIDEBAR_WIDTH);
  }
}

onBeforeUnmount(stopSidebarResize);
</script>

<template>
  <div class="flex h-full min-h-0">
    <div
      data-test="resource-sidebar-shell"
      class="relative h-full min-h-0 shrink-0"
      :style="{ width: `${layout.treeSidebarWidth}px` }"
    >
      <div
        data-test="resource-sidebar"
        class="h-full min-h-0 overflow-hidden border-r border-surface-200 dark:border-surface-800"
        :class="{ 'border-r-0': layout.treeSidebarWidth === 0 }"
      >
        <ResourceTree
          :refresh-key="treeRefreshKey"
          :connection-id="connectionId"
          :groups="tree"
          :selected-group="treeSelectedGroup"
          :selected-uid="treeSelectedUid"
          @select-group="onSelectGroup"
          @select-node="openDetail"
          @select-list="onSelectList"
        />
      </div>
      <div
        data-test="resource-sidebar-resizer"
        role="separator"
        tabindex="0"
        aria-label="Resize resource sidebar"
        aria-orientation="vertical"
        :aria-valuemin="0"
        :aria-valuemax="MAX_TREE_SIDEBAR_WIDTH"
        :aria-valuenow="layout.treeSidebarWidth"
        class="group absolute top-0 -right-1.5 z-10 h-full w-3 cursor-col-resize focus-visible:outline-none"
        @pointerdown="startSidebarResize"
        @keydown="onSidebarResizeKeydown"
      >
        <span
          class="absolute top-0 left-1/2 h-full w-0.5 -translate-x-1/2 bg-surface-300/80 transition-[width,background-color] group-hover:w-1 group-hover:bg-primary-500/50 group-focus-visible:w-1 group-focus-visible:bg-primary-500/60 dark:bg-surface-700/80"
          :class="{ 'w-1 bg-primary-500/70': isSidebarResizing }"
        />
      </div>
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
          <div
            v-for="v in tabs"
            :key="v.id"
            class="group flex shrink-0 cursor-pointer items-center overflow-hidden rounded text-xs transition-colors active:cursor-pointer"
            :class="
              v.id === view.activeViewId
                ? 'bg-surface-0 text-surface-900 shadow-sm dark:bg-surface-800 dark:text-surface-0'
                : 'text-surface-500 hover:text-surface-800 dark:hover:text-surface-200'
            "
          >
            <button
              type="button"
              :title="workbenchTabTitle(v)"
              :aria-label="workbenchTabTitle(v)"
              :data-active-tab="v.id === view.activeViewId ? 'true' : undefined"
              :data-preview-tab="v.preview ? 'true' : undefined"
              class="flex max-w-60 min-w-0 flex-1 items-center gap-1.5 overflow-hidden px-2 py-1 text-left focus-visible:ring-2 focus-visible:ring-primary-500/35 focus-visible:outline-none focus-visible:ring-inset"
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
            </button>
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
          </div>
        </VueDraggable>
      </div>

      <div class="min-h-0 flex-1 overflow-hidden">
        <KeepAlive :max="KEEP_ALIVE_WORKBENCH_TABS_MAX">
          <DetailView
            v-if="
              activeView?.kind === 'detail' &&
              activeDetailResource &&
              activeView.row
            "
            :key="`${connectionId}:${activeView.id}`"
            :connection-id="connectionId"
            :detail="activeDetailResource.detail"
            :detail-action-ids="activeDetailResource.actions?.detail ?? []"
            :row="activeView.row"
            :actions="actions"
            @select="openDetail"
            @action-done="onDetailActionDone"
          />
          <PanelHost
            v-else-if="activeListResource && activeListSource"
            :key="activeListKey"
            panel="table"
            :connection-id="connectionId"
            :source="activeListSource"
            :config="{
              columns: activeListResource.columns,
              columnsSource: activeColumnsSource,
              watch: activeListResource.watch,
              actionIds: activeListResource.actions?.toolbar ?? [],
              rowActionIds: activeListResource.actions?.row ?? [],
              selectable: activeListResource.actions?.selectable,
            }"
            :actions="actions"
            @select="openDetail"
            @action-done="onListActionDone"
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
