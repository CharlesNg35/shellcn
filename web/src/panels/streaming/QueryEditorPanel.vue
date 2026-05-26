<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import { interpolate, runAction } from "../../api/dataSource";
import type { QueryEditorConfig } from "../../types/projection";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StreamStatusBar from "./StreamStatusBar.vue";
import { useTheme } from "../../composables/useTheme";
import { loadMonaco, syncMonacoTheme, type MonacoModule } from "../../monaco";

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
const reconnecting = ref(false);
let editor: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;
let monacoModule: MonacoModule | null = null;
const { isDark } = useTheme();

function onFrame(frame: string): void {
  try {
    results.value = JSON.parse(frame) as Results;
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
  await nextTick();
  if (!container.value) {
    useFallback.value = true;
    return;
  }
  try {
    const monaco = await loadMonaco();
    monacoModule = monaco;
    editor = monaco.editor.create(container.value, {
      value: query.value,
      language: "sql",
      theme: document.documentElement.classList.contains("dark")
        ? "vs-dark"
        : "vs",
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
    });
  } catch {
    useFallback.value = true;
  }
});

watch(isDark, () => {
  if (monacoModule) syncMonacoTheme(monacoModule);
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
      <span class="text-xs text-surface-400">SQL</span>
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
          Cancel
        </Button>
        <Button type="button" size="small" :disabled="running" @click="run">
          {{ running ? "Running…" : "Run" }}
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
      <div v-show="!useFallback" ref="container" class="h-full" />
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
