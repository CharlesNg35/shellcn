<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import { fetchDoc, runAction } from "../../api/dataSource";
import type { CodeEditorConfig } from "../../types/projection";
import type { PanelProps } from "../core/types";
import { useTheme } from "../../composables/useTheme";
import {
  currentMonacoTheme,
  loadMonaco,
  syncMonacoTheme,
  type MonacoModule,
} from "../../monaco";

const props = defineProps<PanelProps>();

const text = ref("");
const loading = ref(true);
const error = ref<string | null>(null);
const container = ref<HTMLElement | null>(null);
const useFallback = ref(false);
const saving = ref(false);
const saveError = ref<string | null>(null);
const saved = ref(false);
let editor: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;
let monacoModule: MonacoModule | null = null;
const editorConfig = computed(
  () => props.config as CodeEditorConfig | undefined,
);
const { isDark } = useTheme();

const language = computed(() => editorConfig.value?.language ?? "plaintext");
const saveRouteId = computed(() => editorConfig.value?.saveRouteId);
const editable = computed(() => Boolean(saveRouteId.value));

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
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
  if (!error.value) {
    await nextTick();
    await mountEditor();
  }
}

async function mountEditor(): Promise<void> {
  if (!container.value) {
    useFallback.value = true;
    return;
  }
  try {
    const monaco = await loadMonaco();
    monacoModule = monaco;
    editor?.dispose();
    const ed = monaco.editor.create(container.value, {
      value: text.value,
      language: language.value,
      readOnly: !editable.value,
      theme: currentMonacoTheme(),
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
    });
    editor = ed;
    ed.onDidChangeModelContent(() => {
      text.value = ed.getValue();
      saved.value = false;
    });
  } catch {
    useFallback.value = true;
  }
}

async function save(): Promise<void> {
  const routeId = saveRouteId.value;
  if (!routeId) return;
  if (editor) text.value = editor.getValue();
  saving.value = true;
  saveError.value = null;
  try {
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource },
      { content: text.value },
      editorConfig.value?.saveParams ?? props.source?.params ?? {},
      editorConfig.value?.saveMethod ?? "PUT",
    );
    saved.value = true;
  } catch (e) {
    saveError.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

onMounted(load);
watch(() => [props.connectionId, props.resource?.uid], load);
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
    <div
      v-if="editable"
      class="flex items-center justify-between border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <span class="text-xs text-surface-400">{{ language }}</span>
      <div class="flex items-center gap-2">
        <span v-if="saveError" class="text-xs text-red-500">{{
          saveError
        }}</span>
        <span v-else-if="saved" class="text-xs text-emerald-500">Saved</span>
        <Button type="button" label="Save" :disabled="saving" @click="save" />
      </div>
    </div>
    <p v-if="loading" class="p-4 text-sm text-surface-400">Loading…</p>
    <p v-else-if="error" class="p-4 text-sm text-red-500">{{ error }}</p>
    <textarea
      v-else-if="useFallback && editable"
      v-model="text"
      class="min-h-0 flex-1 resize-none bg-surface-0 p-4 font-mono text-xs leading-relaxed outline-none dark:bg-surface-950"
    />
    <pre
      v-else-if="useFallback"
      class="m-0 min-h-0 flex-1 overflow-auto p-4 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
      >{{ text }}</pre
    >
    <div
      v-show="!loading && !error && !useFallback"
      ref="container"
      class="shellcn-monaco-host min-h-0 flex-1"
    />
  </div>
</template>
