<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import { interpolate } from "../../api/dataSource";
import { useStream } from "../../composables/useStream";
import type { PanelProps } from "../types";
import StubBanner from "./StubBanner.vue";

const props = defineProps<PanelProps>();

interface Results {
  columns: string[];
  rows: unknown[][];
  rowCount?: number;
  elapsedMs?: number;
}

function initialQuery(): string {
  const raw = (props.config?.initialQuery as string) ?? "";
  try {
    return interpolate(raw, { resource: props.resource });
  } catch {
    return raw;
  }
}

const query = ref(initialQuery());
const results = ref<Results | null>(null);
const container = ref<HTMLElement | null>(null);
const useFallback = ref(false);
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Monaco editor loaded lazily
let editor: any = null;

function onFrame(frame: string): void {
  try {
    results.value = JSON.parse(frame) as Results;
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
  send(JSON.stringify({ query: query.value }));
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
      <button
        type="button"
        class="rounded-md bg-primary-500 px-3 py-1 text-xs font-medium text-white"
        @click="run"
      >
        Run
      </button>
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
