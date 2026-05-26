<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import SkeletonList from "../../components/SkeletonList.vue";
import { useTheme } from "../../composables/useTheme";
import type { MonacoModule } from "../../monaco";

const props = withDefaults(
  defineProps<{
    value: string;
    language?: string;
    readonly?: boolean;
    disabled?: boolean;
    ariaLabel?: string;
  }>(),
  {
    language: "plaintext",
    readonly: false,
    disabled: false,
    ariaLabel: "Code editor",
  },
);

const emit = defineEmits<{
  "update:value": [value: string];
}>();

const container = ref<HTMLElement | null>(null);
const loading = ref(true);
const useFallback = ref(false);
const { isDark } = useTheme();
let editor: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;
let monacoModule: MonacoModule | null = null;
let monacoHelpers: typeof import("../../monaco") | null = null;

async function mountEditor(): Promise<void> {
  await nextTick();
  if (!container.value) {
    useFallback.value = true;
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const helpers = await import("../../monaco");
    const monaco = await helpers.loadMonaco();
    monacoHelpers = helpers;
    monacoModule = monaco;
    editor?.dispose();
    const ed = monaco.editor.create(container.value, {
      value: props.value,
      language: props.language,
      readOnly: props.readonly || props.disabled,
      theme: helpers.currentMonacoTheme(),
      minimap: { enabled: false },
      automaticLayout: true,
      scrollBeyondLastLine: false,
      wordWrap: "on",
    });
    editor = ed;
    ed.onDidChangeModelContent(() => {
      if (!props.readonly) emit("update:value", ed.getValue());
    });
  } catch {
    useFallback.value = true;
  } finally {
    loading.value = false;
  }
}

function syncEditorValue(value: string): void {
  if (editor && editor.getValue() !== value) {
    editor.setValue(value);
  }
}

onMounted(mountEditor);

watch(
  () => props.value,
  (value) => syncEditorValue(value),
);

watch(
  () => props.language,
  (next) => {
    if (monacoModule && editor?.getModel()) {
      monacoModule.editor.setModelLanguage(editor.getModel()!, next);
    }
  },
);

watch(
  () => [props.readonly, props.disabled] as const,
  () => {
    editor?.updateOptions({ readOnly: props.readonly || props.disabled });
  },
);

watch(isDark, () => {
  if (monacoModule && monacoHelpers) {
    monacoHelpers.syncMonacoTheme(monacoModule);
  }
});

onUnmounted(() => {
  editor?.dispose();
});
</script>

<template>
  <div class="h-full min-h-0">
    <SkeletonList v-if="loading" :rows="8" />
    <textarea
      v-else-if="useFallback && !readonly"
      :value="value"
      class="h-full min-h-0 w-full flex-1 resize-none rounded-none border-0 bg-surface-0 p-4 font-mono text-xs leading-relaxed outline-none dark:bg-surface-950"
      spellcheck="false"
      :aria-label="ariaLabel"
      :disabled="disabled"
      @input="
        emit('update:value', ($event.target as HTMLTextAreaElement).value)
      "
    />
    <pre
      v-else-if="useFallback"
      class="m-0 h-full overflow-auto p-4 font-mono text-xs leading-relaxed wrap-break-word whitespace-pre-wrap text-surface-700 dark:text-surface-200"
      >{{ value }}</pre
    >
    <div
      v-show="!useFallback"
      ref="container"
      class="shellcn-monaco-host h-full min-h-0"
      :aria-label="ariaLabel"
    />
  </div>
</template>
