<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { fetchDoc } from "../../api/dataSource";
import type { PanelProps } from "../types";

const props = defineProps<PanelProps>();

const text = ref("");
const loading = ref(true);
const error = ref<string | null>(null);
const container = ref<HTMLElement | null>(null);
const useFallback = ref(false);
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- Monaco editor loaded lazily
let editor: any = null;

const language = computed(
  () => (props.config?.language as string) ?? "plaintext",
);

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const doc = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
    text.value = typeof doc === "string" ? doc : JSON.stringify(doc, null, 2);
    await mountEditor();
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function mountEditor(): Promise<void> {
  if (!container.value) {
    useFallback.value = true;
    return;
  }
  try {
    const monaco = await import("monaco-editor");
    editor?.dispose();
    editor = monaco.editor.create(container.value, {
      value: text.value,
      language: language.value,
      readOnly: true,
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
    });
  } catch {
    useFallback.value = true;
  }
}

onMounted(load);
watch(() => [props.connectionId, props.resource?.uid], load);
onUnmounted(() => {
  try {
    editor?.dispose();
  } catch {
    /* already disposed */
  }
});
</script>

<template>
  <div class="h-full">
    <p v-if="loading" class="p-4 text-sm text-surface-400">Loading…</p>
    <p v-else-if="error" class="p-4 text-sm text-red-500">{{ error }}</p>
    <pre
      v-else-if="useFallback"
      class="m-0 h-full overflow-auto p-4 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
      >{{ text }}</pre
    >
    <div
      v-show="!loading && !error && !useFallback"
      ref="container"
      class="h-full"
    />
  </div>
</template>
