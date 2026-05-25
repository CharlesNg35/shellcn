<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { interpolate, runAction } from "../../api/dataSource";
import type { QueryEditorConfig } from "../../types/projection";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();
const queryConfig = props.config as QueryEditorConfig | undefined;

interface Results {
  columns: string[];
  rows: unknown[][];
  rowCount?: number;
  elapsedMs?: number;
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
let editor: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;

function onFrame(frame: string): void {
  try {
    results.value = JSON.parse(frame) as Results;
    running.value = false;
  } catch {
    /* ignore */
  }
}

const { status, send } = useStream(
  props.connectionId,
  props.source,
  { resource: props.resource },
  onFrame,
);

function run(): void {
  if (editor) query.value = editor.getValue();
  const text = query.value.trim();
  if (!text) return;
  history.value = [text, ...history.value.filter((q) => q !== text)].slice(
    0,
    8,
  );
  running.value = true;
  error.value = null;
  send(JSON.stringify({ query: query.value }));
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

function recall(text: string): void {
  query.value = text;
  editor?.setValue?.(text);
}

onMounted(async () => {
  if (!container.value) {
    useFallback.value = true;
    return;
  }
  try {
    const monaco = await import("monaco-editor");
    editor = monaco.editor.create(container.value, {
      value: query.value,
      language: "sql",
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
    });
  } catch {
    useFallback.value = true;
  }
});

onUnmounted(() => {
  try {
    editor?.dispose();
  } catch {
    /* already disposed */
  }
});
</script>

<template>
  <div class="flex h-full flex-col">
    <StubBanner :status="status" />
    <div
      class="flex items-center justify-between border-b border-surface-200 px-3 py-1.5 dark:border-surface-800"
    >
      <span class="text-xs text-surface-400">SQL</span>
      <div class="flex items-center gap-2">
        <span v-if="error" class="text-xs text-red-500">{{ error }}</span>
        <button
          v-if="running"
          type="button"
          class="rounded-md border border-surface-300 px-3 py-1 text-xs font-medium dark:border-surface-700"
          @click="cancel"
        >
          Cancel
        </button>
        <button
          type="button"
          class="rounded-md bg-primary-500 px-3 py-1 text-xs font-medium text-white disabled:opacity-50"
          :disabled="running"
          @click="run"
        >
          {{ running ? "Running…" : "Run" }}
        </button>
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
      <div v-show="!useFallback" ref="container" class="h-full" />
    </div>

    <div
      v-if="history.length"
      class="border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <div class="flex flex-wrap gap-2">
        <button
          v-for="item in history"
          :key="item"
          type="button"
          class="max-w-72 truncate rounded border border-surface-300 px-2 py-1 text-xs text-surface-500 hover:bg-surface-100 dark:border-surface-700 dark:hover:bg-surface-800"
          @click="recall(item)"
        >
          {{ item }}
        </button>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
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
              {{ cell }}
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="p-4 text-sm text-surface-400">
        Run a query to see results.
      </p>
    </div>
  </div>
</template>
