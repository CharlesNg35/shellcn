<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { useStorage } from "@vueuse/core";
import Button from "primevue/button";
import Column from "primevue/column";
import DataTable from "primevue/datatable";
import Dialog from "primevue/dialog";
import Menu from "primevue/menu";
import { fetchDoc, interpolate, runAction } from "@/api/dataSource";
import { exportMatrix, type ExportFormat } from "../shared/exportData";
import type { QueryEditorConfig } from "@/types/projection";
import { useStream } from "@/composables/useStream";
import type { PanelProps } from "../core/types";
import SkeletonList from "@/components/SkeletonList.vue";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useTheme } from "@/composables/useTheme";
import type { CodeMirrorCompletion, CodeMirrorEditor } from "@/codemirror";
import { useDirtyGuard } from "../shared/useDirtyGuard";

const props = defineProps<PanelProps>();
const queryConfig = computed(
  () => props.config as QueryEditorConfig | undefined,
);

interface Results {
  columns: string[];
  rows: unknown[][];
  rowCount?: number;
  elapsedMs?: number;
  commandTag?: string;
  error?: string;
  requiresConfirmation?: boolean;
  confirmMessage?: string;
}

function initialQuery(): string {
  const raw = queryConfig.value?.initialQuery ?? "";
  try {
    return interpolate(raw, { resource: props.resource, record: props.record });
  } catch {
    return raw;
  }
}

const query = ref(initialQuery());
const results = ref<Results | null>(null);
const history = ref<string[]>([]);
const running = ref(false);
const error = ref<string | null>(null);
const container = ref<HTMLElement | null>(null);
const editorLoading = ref(true);
const useFallback = ref(false);
const reconnecting = ref(false);
const pendingConfirmation = ref(false);
const confirmationMessage = ref("");
const completionItems = ref<CodeMirrorCompletion[]>([]);
let editor: CodeMirrorEditor | null = null;
let codeMirror: typeof import("@/codemirror") | null = null;
let completionRequest = 0;
const { isDark } = useTheme();
const editorLanguage = computed(
  () => queryConfig.value?.language ?? "plaintext",
);
const editorLabel = computed(() => queryConfig.value?.label ?? "Editor");
const executeLabel = computed(
  () => queryConfig.value?.executeLabel ?? "Execute",
);
const cancelLabel = computed(() => queryConfig.value?.cancelLabel ?? "Cancel");
const emptyText = computed(
  () => queryConfig.value?.emptyText ?? "Execute to see results.",
);
const canExport = computed(() => queryConfig.value?.exportable === true);
const baselineQuery = ref(query.value);

// A plugin that forgets its own row cap must not be able to take the grid down,
// so the panel renders at most this many rows and says so; export still writes
// the full result set.
const MAX_RESULT_ROWS = 5000;
const VIRTUAL_THRESHOLD = 100;
const ROW_HEIGHT = 30;

const resultRows = computed(() => results.value?.rows ?? []);
const truncated = computed(() => resultRows.value.length > MAX_RESULT_ROWS);
const displayRows = computed(() =>
  truncated.value
    ? resultRows.value.slice(0, MAX_RESULT_ROWS)
    : resultRows.value,
);
const resultVirtualScroller = computed(() =>
  displayRows.value.length > VIRTUAL_THRESHOLD
    ? { itemSize: ROW_HEIGHT }
    : undefined,
);

// The editor keeps a compact default (~5 lines) so results own most of the
// panel; the drag handle below it stores the preferred height per browser.
const MIN_EDITOR_HEIGHT = 72;
const MAX_EDITOR_HEIGHT = 640;
const DEFAULT_EDITOR_HEIGHT = 120;
const editorHeight = useStorage(
  "shellcn:query-editor:height",
  DEFAULT_EDITOR_HEIGHT,
);
const resizing = ref(false);

function setEditorHeight(value: number): void {
  const next = Number.isFinite(value)
    ? Math.round(value)
    : DEFAULT_EDITOR_HEIGHT;
  editorHeight.value = Math.min(
    MAX_EDITOR_HEIGHT,
    Math.max(MIN_EDITOR_HEIGHT, next),
  );
}

// Sanitize whatever was persisted before the first render.
setEditorHeight(editorHeight.value);

let resizeStartY = 0;
let resizeStartHeight = 0;
let resizeCapture: { el: HTMLElement; pointerId: number } | null = null;
let cursorBefore = "";
let userSelectBefore = "";

function onResizeMove(event: PointerEvent): void {
  setEditorHeight(resizeStartHeight + event.clientY - resizeStartY);
}

function stopResize(): void {
  if (resizeCapture?.el.hasPointerCapture?.(resizeCapture.pointerId)) {
    resizeCapture.el.releasePointerCapture(resizeCapture.pointerId);
  }
  resizeCapture = null;
  if (resizing.value) {
    document.documentElement.style.cursor = cursorBefore;
    document.body.style.userSelect = userSelectBefore;
  }
  resizing.value = false;
  window.removeEventListener("pointermove", onResizeMove);
  window.removeEventListener("pointerup", stopResize);
}

function startResize(event: PointerEvent): void {
  const el = event.currentTarget;
  if (el instanceof HTMLElement && el.setPointerCapture) {
    el.setPointerCapture(event.pointerId);
    resizeCapture = { el, pointerId: event.pointerId };
  }
  resizeStartY = event.clientY;
  resizeStartHeight = editorHeight.value;
  cursorBefore = document.documentElement.style.cursor;
  userSelectBefore = document.body.style.userSelect;
  document.documentElement.style.cursor = "row-resize";
  document.body.style.userSelect = "none";
  resizing.value = true;
  window.addEventListener("pointermove", onResizeMove);
  window.addEventListener("pointerup", stopResize);
}

function onResizeKeydown(event: KeyboardEvent): void {
  if (event.key === "ArrowUp") {
    event.preventDefault();
    setEditorHeight(editorHeight.value - 16);
  } else if (event.key === "ArrowDown") {
    event.preventDefault();
    setEditorHeight(editorHeight.value + 16);
  } else if (event.key === "Home") {
    event.preventDefault();
    setEditorHeight(DEFAULT_EDITOR_HEIGHT);
  }
}

function resetEditorHeight(): void {
  setEditorHeight(DEFAULT_EDITOR_HEIGHT);
}

// CodeMirror caches viewport/gutter geometry, so nudge it after every resize.
watch(editorHeight, async () => {
  await nextTick();
  editor?.view.requestMeasure();
});

function syncQueryFromEditor(): void {
  if (editor) query.value = codeMirror?.editorValue(editor) ?? query.value;
}

function queryDirty(): boolean {
  return query.value !== baselineQuery.value;
}

const { confirmBeforeDiscard } = useDirtyGuard({
  isDirty: queryDirty,
  header: "Discard unsaved query changes?",
  message: "The current query has unsaved changes. Discard them and continue?",
});

// Export the current result set — only when the plugin opts in via the manifest.
const exportMenu = ref<{ toggle: (event: Event) => void } | null>(null);
function runExport(format: ExportFormat): void {
  if (!results.value) return;
  exportMatrix(
    props.source?.routeId ?? "query",
    results.value.columns,
    results.value.rows,
    format,
  );
}
const exportItems = [
  { label: "Export CSV", command: () => runExport("csv") },
  { label: "Export JSON", command: () => runExport("json") },
];

function onFrame(frame: string): void {
  try {
    const payload = JSON.parse(frame) as Results;
    if (payload.error) {
      error.value = payload.error;
      pendingConfirmation.value = payload.requiresConfirmation === true;
      confirmationMessage.value =
        payload.confirmMessage ??
        "This operation requires confirmation before it can run.";
    } else {
      results.value = { ...payload, rows: payload.rows ?? [] };
      error.value = null;
      pendingConfirmation.value = false;
      confirmationMessage.value = "";
    }
    running.value = false;
  } catch {
    /* ignore */
  }
}

const {
  status,
  error: streamError,
  send,
  reconnect,
} = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource, record: props.record },
  onFrame,
);

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await reconnect();
  } finally {
    reconnecting.value = false;
  }
}

function run(confirm = false): void {
  if (status.value !== "open") {
    error.value = "The query stream is not connected yet.";
    return;
  }
  syncQueryFromEditor();
  const text = query.value.trim();
  if (!text) return;
  baselineQuery.value = query.value;
  history.value = [text, ...history.value.filter((q) => q !== text)].slice(
    0,
    8,
  );
  running.value = true;
  error.value = null;
  pendingConfirmation.value = false;
  if (!send(JSON.stringify({ query: query.value, confirm }))) {
    running.value = false;
    error.value = "The query stream is not ready. Reconnect and try again.";
  }
}

async function cancel(): Promise<void> {
  const routeId = queryConfig.value?.cancelRouteId;
  running.value = false;
  if (!routeId) return;
  try {
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource, record: props.record },
      {},
      queryConfig.value?.cancelParams ?? props.source?.params ?? {},
      "POST",
    );
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function loadCompletions(): Promise<CodeMirrorCompletion[]> {
  const routeId = queryConfig.value?.completionRouteId;
  if (!routeId) return [];
  try {
    const items = await fetchDoc<CodeMirrorCompletion[]>(
      props.connectionId,
      {
        routeId,
        params: queryConfig.value?.completionParams ?? props.source?.params,
      },
      { resource: props.resource, record: props.record },
    );
    return Array.isArray(items) ? items : [];
  } catch {
    return [];
  }
}

async function refreshCompletions(): Promise<void> {
  const request = ++completionRequest;
  const items = await loadCompletions();
  if (request !== completionRequest) return;
  completionItems.value = items;
  codeMirror?.setEditorCompletions(editor, items, editorLanguage.value);
}

function applyQuery(text: string): void {
  query.value = text;
  baselineQuery.value = text;
  codeMirror?.setEditorValue(editor, text);
}

async function recall(text: string): Promise<void> {
  await confirmBeforeDiscard(() => applyQuery(text));
}

function confirmExecution(): void {
  pendingConfirmation.value = false;
  run(true);
}

onMounted(async () => {
  await nextTick();
  if (!container.value) {
    useFallback.value = true;
    editorLoading.value = false;
    return;
  }
  editorLoading.value = true;
  try {
    const helpers = await import("@/codemirror");
    codeMirror = helpers;
    await refreshCompletions();
    editor = helpers.createCodeMirrorEditor(container.value, {
      value: query.value,
      language: editorLanguage.value,
      ariaLabel: `${editorLabel.value} editor`,
      completions: completionItems.value,
      onChange(value) {
        query.value = value;
      },
    });
    editor.view.requestMeasure();
  } catch {
    useFallback.value = true;
  } finally {
    editorLoading.value = false;
  }
});

watch(isDark, () => {
  codeMirror?.syncCodeMirrorTheme(editor);
});

watch(
  () =>
    JSON.stringify({
      connectionId: props.connectionId,
      routeId: props.source?.routeId,
      params: props.source?.params,
      resource: props.resource?.uid,
      initialQuery: queryConfig.value?.initialQuery,
      language: queryConfig.value?.language,
      completionRouteId: queryConfig.value?.completionRouteId,
      completionParams: queryConfig.value?.completionParams,
    }),
  async () => {
    await confirmBeforeDiscard(async () => {
      applyQuery(initialQuery());
      codeMirror?.setEditorLanguage(editor, editorLanguage.value);
      results.value = null;
      running.value = false;
      error.value = null;
      pendingConfirmation.value = false;
      confirmationMessage.value = "";
      await refreshCompletions();
    });
  },
);

onUnmounted(() => {
  stopResize();
  try {
    editor?.view.destroy();
  } catch {
    /* already disposed */
  }
});
</script>

<template>
  <div class="flex h-full flex-col">
    <StreamStatusBar
      :status="status"
      :error="streamError"
      :reconnecting="reconnecting"
      can-reconnect
      @reconnect="onReconnect"
    />
    <div
      class="flex items-center justify-between gap-2 border-b border-surface-200 px-3 py-1.5 dark:border-surface-800"
    >
      <span class="min-w-0 truncate text-xs text-surface-400">{{
        editorLabel
      }}</span>
      <div class="flex min-w-0 flex-1 items-center justify-end gap-2">
        <span
          v-if="error"
          role="alert"
          aria-live="assertive"
          class="min-w-0 flex-1 truncate text-xs text-red-500"
          :title="error"
          >{{ error }}</span
        >
        <Button
          v-if="running"
          type="button"
          size="small"
          severity="secondary"
          outlined
          class="shrink-0"
          @click="cancel"
        >
          {{ cancelLabel }}
        </Button>
        <Button
          type="button"
          size="small"
          class="shrink-0"
          :label="executeLabel"
          :loading="running"
          :disabled="running || status !== 'open'"
          @click="run()"
        />
      </div>
    </div>

    <div
      data-test="query-editor-pane"
      class="max-h-[70%] shrink-0 overflow-hidden"
      :style="{ height: `${editorHeight}px` }"
    >
      <SkeletonList v-if="editorLoading" :rows="3" />
      <textarea
        v-else-if="useFallback"
        v-model="query"
        class="h-full w-full resize-none bg-surface-0 p-3 font-mono text-xs outline-none dark:bg-surface-950"
      />
      <div
        v-show="!editorLoading && !useFallback"
        ref="container"
        class="shellcn-codemirror-host h-full"
      />
    </div>

    <div
      data-test="query-editor-resizer"
      role="separator"
      aria-orientation="horizontal"
      aria-label="Resize query editor"
      title="Drag to resize the editor (double-click to reset)"
      tabindex="0"
      :aria-valuemin="MIN_EDITOR_HEIGHT"
      :aria-valuemax="MAX_EDITOR_HEIGHT"
      :aria-valuenow="editorHeight"
      class="group relative h-2.5 shrink-0 cursor-row-resize border-y border-surface-200 bg-surface-50 transition-colors hover:bg-primary-500/10 focus-visible:bg-primary-500/15 focus-visible:outline-none dark:border-surface-800 dark:bg-surface-900"
      :class="{ 'bg-primary-500/10 dark:bg-primary-500/10': resizing }"
      @pointerdown="startResize"
      @dblclick="resetEditorHeight"
      @keydown="onResizeKeydown"
    >
      <span
        class="pointer-events-none absolute top-1/2 left-1/2 h-0.5 w-8 -translate-x-1/2 -translate-y-1/2 rounded-full bg-surface-300 transition-colors group-hover:bg-primary-500/70 group-focus-visible:bg-primary-500/70 dark:bg-surface-700"
        :class="{ 'bg-primary-500/80 dark:bg-primary-500/80': resizing }"
      />
    </div>

    <div
      v-if="history.length"
      class="max-h-16 overflow-y-auto border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <div class="flex flex-wrap gap-2">
        <Button
          v-for="item in history"
          :key="item"
          type="button"
          size="small"
          severity="secondary"
          outlined
          :label="item"
          :title="item"
          class="max-w-72 overflow-hidden"
          @click="recall(item)"
        />
      </div>
    </div>

    <div
      v-if="results"
      data-test="query-result-toolbar"
      class="flex shrink-0 items-center gap-2 border-b border-surface-200 px-3 py-2 text-xs text-surface-500 dark:border-surface-800"
    >
      <template v-if="canExport && results.rows.length">
        <Button
          type="button"
          text
          size="small"
          label="Export"
          title="Export results"
          aria-haspopup="true"
          data-test="query-export-button"
          @click="exportMenu?.toggle($event)"
        />
        <Menu ref="exportMenu" :model="exportItems" popup />
      </template>
      <span>
        {{
          results.commandTag ||
          `${results.rowCount ?? results.rows.length} rows`
        }}
        <span v-if="results.elapsedMs != null">
          · {{ results.elapsedMs }} ms</span
        >
      </span>
      <span v-if="truncated" data-test="query-result-truncated" class="ml-auto">
        Showing the first {{ MAX_RESULT_ROWS }} of {{ resultRows.length }} rows
        — refine the query or export.
      </span>
    </div>

    <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
      <DataTable
        v-if="results"
        :value="displayRows"
        size="small"
        scrollable
        scroll-height="flex"
        :virtual-scroller-options="resultVirtualScroller"
        class="min-h-0 flex-1 text-xs"
      >
        <Column
          v-for="(c, j) in results.columns"
          :key="`${c}-${j}`"
          :header="c"
        >
          <template #body="{ data }">
            <span
              class="block max-w-96 truncate"
              :title="
                (data as unknown[])[j] == null
                  ? 'NULL'
                  : String((data as unknown[])[j])
              "
            >
              {{ (data as unknown[])[j] ?? "NULL" }}
            </span>
          </template>
        </Column>
        <template #empty>No rows.</template>
      </DataTable>
      <p v-else class="p-4 text-sm text-surface-400">{{ emptyText }}</p>
    </div>

    <Dialog
      :visible="pendingConfirmation"
      modal
      header="Confirm execution"
      :dismissable-mask="true"
      @update:visible="(v) => !v && (pendingConfirmation = false)"
    >
      <p class="mb-4 text-sm text-surface-500" role="alert">
        {{ confirmationMessage }}
      </p>
      <div class="flex justify-end gap-2">
        <Button
          type="button"
          severity="secondary"
          @click="pendingConfirmation = false"
        >
          Cancel
        </Button>
        <Button type="button" severity="danger" @click="confirmExecution">
          {{ executeLabel }}
        </Button>
      </div>
    </Dialog>
  </div>
</template>
