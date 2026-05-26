<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import { fetchDoc, interpolate, runAction } from "../../api/dataSource";
import type { QueryEditorConfig } from "../../types/projection";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useTheme } from "../../composables/useTheme";
import type { CodeMirrorCompletion, CodeMirrorEditor } from "../../codemirror";

const props = defineProps<PanelProps>();
const queryConfig = props.config as QueryEditorConfig | undefined;

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
  const raw = queryConfig?.initialQuery ?? "";
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
const useFallback = ref(false);
const reconnecting = ref(false);
const pendingConfirmation = ref(false);
const confirmationMessage = ref("");
const completionItems = ref<CodeMirrorCompletion[]>([]);
let editor: CodeMirrorEditor | null = null;
let codeMirror: typeof import("../../codemirror") | null = null;
const { isDark } = useTheme();
const editorLanguage = queryConfig?.language ?? "plaintext";
const editorLabel = queryConfig?.label ?? "Editor";
const executeLabel = queryConfig?.executeLabel ?? "Execute";
const cancelLabel = queryConfig?.cancelLabel ?? "Cancel";
const runningLabel = queryConfig?.runningLabel ?? "Executing…";
const emptyText = queryConfig?.emptyText ?? "Execute to see results.";

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
      results.value = payload;
      pendingConfirmation.value = false;
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
  const routeId = queryConfig?.cancelRouteId;
  running.value = false;
  if (!routeId) return;
  try {
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource },
      {},
      queryConfig?.cancelParams ?? props.source?.params ?? {},
      "POST",
    );
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function loadCompletions(): Promise<CodeMirrorCompletion[]> {
  const routeId = queryConfig?.completionRouteId;
  if (!routeId) return [];
  try {
    const items = await fetchDoc<CodeMirrorCompletion[]>(
      props.connectionId,
      {
        routeId,
        params: queryConfig?.completionParams ?? props.source?.params,
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
    return;
  }
  try {
    const helpers = await import("../../codemirror");
    codeMirror = helpers;
    completionItems.value = await loadCompletions();
    editor = helpers.createCodeMirrorEditor(container.value, {
      value: query.value,
      language: editorLanguage,
      ariaLabel: `${editorLabel} editor`,
      completions: completionItems.value,
      onChange(value) {
        query.value = value;
      },
    });
  } catch {
    useFallback.value = true;
  }
});

watch(isDark, () => {
  codeMirror?.syncCodeMirrorTheme(editor);
});

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
        <Button type="button" size="small" :disabled="running" @click="run()">
          {{ running ? runningLabel : executeLabel }}
        </Button>
      </div>
    </div>

    <div
      class="h-40 shrink-0 border-b border-surface-200 dark:border-surface-800"
    >
      <textarea
        v-if="useFallback"
        v-model="query"
        class="h-full w-full resize-none bg-surface-0 p-3 font-mono text-xs outline-none dark:bg-surface-950"
      />
      <div
        v-show="!useFallback"
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
          class="max-w-72"
          @click="recall(item)"
        >
          {{ item }}
        </Button>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <div
        v-if="results"
        class="border-b border-surface-200 px-3 py-2 text-xs text-surface-500 dark:border-surface-800"
      >
        {{
          results.commandTag ||
          `${results.rowCount ?? results.rows.length} rows`
        }}
        <span v-if="results.elapsedMs != null">
          · {{ results.elapsedMs }} ms</span
        >
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
