<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import Menu from "primevue/menu";
import { fetchDoc, interpolate, runAction } from "../../api/dataSource";
import { exportMatrix, type ExportFormat } from "../shared/exportData";
import type { QueryEditorConfig } from "../../types/projection";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../core/types";
import SkeletonList from "../../components/SkeletonList.vue";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useTheme } from "../../composables/useTheme";
import type { CodeMirrorCompletion, CodeMirrorEditor } from "../../codemirror";

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
    return interpolate(raw, { resource: props.resource });
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
let codeMirror: typeof import("../../codemirror") | null = null;
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
  { resource: props.resource },
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
  if (editor) query.value = codeMirror?.editorValue(editor) ?? query.value;
  const text = query.value.trim();
  if (!text) return;
  history.value = [text, ...history.value.filter((q) => q !== text)].slice(
    0,
    8,
  );
  running.value = true;
  error.value = null;
  pendingConfirmation.value = false;
  send(JSON.stringify({ query: query.value, confirm }));
}

async function cancel(): Promise<void> {
  const routeId = queryConfig.value?.cancelRouteId;
  running.value = false;
  if (!routeId) return;
  try {
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource },
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
      { resource: props.resource },
    );
    return Array.isArray(items) ? items : [];
  } catch {
    return [];
  }
}

function recall(text: string): void {
  query.value = text;
  codeMirror?.setEditorValue(editor, text);
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
    const helpers = await import("../../codemirror");
    codeMirror = helpers;
    completionItems.value = await loadCompletions();
    editor = helpers.createCodeMirrorEditor(container.value, {
      value: query.value,
      language: editorLanguage.value,
      ariaLabel: `${editorLabel.value} editor`,
      completions: completionItems.value,
      onChange(value) {
        query.value = value;
      },
    });
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
    }),
  async () => {
    query.value = initialQuery();
    results.value = null;
    running.value = false;
    error.value = null;
    pendingConfirmation.value = false;
    confirmationMessage.value = "";
    codeMirror?.setEditorValue(editor, query.value);
    completionItems.value = await loadCompletions();
  },
);

onUnmounted(() => {
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
      class="flex items-center justify-between border-b border-surface-200 px-3 py-1.5 dark:border-surface-800"
    >
      <span class="text-xs text-surface-400">{{ editorLabel }}</span>
      <div class="flex items-center gap-2">
        <span v-if="error" class="text-xs text-red-500">{{ error }}</span>
        <Button
          v-if="running"
          type="button"
          size="small"
          severity="secondary"
          outlined
          @click="cancel"
        >
          {{ cancelLabel }}
        </Button>
        <Button
          type="button"
          size="small"
          :label="executeLabel"
          :loading="running"
          :disabled="running"
          @click="run()"
        />
      </div>
    </div>

    <div
      class="h-40 shrink-0 border-b border-surface-200 dark:border-surface-800"
    >
      <SkeletonList v-if="editorLoading" :rows="4" />
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
      v-if="history.length"
      class="border-b border-surface-200 px-3 py-2 dark:border-surface-800"
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

    <div class="min-h-0 flex-1 overflow-auto">
      <div
        v-if="results"
        data-test="query-result-toolbar"
        class="flex items-center gap-2 border-b border-surface-200 px-3 py-2 text-xs text-surface-500 dark:border-surface-800"
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
      </div>
      <table v-if="results" class="w-full border-collapse text-xs">
        <thead class="sticky top-0 bg-surface-50 dark:bg-surface-900">
          <tr>
            <th
              v-for="c in results.columns"
              :key="c"
              class="border-b border-surface-200 px-3 py-1.5 text-left font-medium text-surface-500 dark:border-surface-800"
            >
              {{ c }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, i) in results.rows"
            :key="i"
            class="border-b border-surface-100 dark:border-surface-800/60"
          >
            <td
              v-for="(cell, j) in row"
              :key="j"
              class="px-3 py-1 text-surface-700 dark:text-surface-200"
            >
              {{ cell === null || cell === undefined ? "NULL" : cell }}
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="p-4 text-sm text-surface-400">{{ emptyText }}</p>
    </div>

    <Dialog
      :visible="pendingConfirmation"
      modal
      header="Confirm execution"
      :dismissable-mask="true"
      @update:visible="(v) => !v && (pendingConfirmation = false)"
    >
      <p class="mb-4 text-sm text-surface-500">
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
