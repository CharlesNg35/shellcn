<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import { fetchDoc, runAction } from "../../api/dataSource";
import type { CodeEditorConfig } from "../../types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "../../components/SkeletonList.vue";
import { useTheme } from "../../composables/useTheme";
import type { CodeMirrorEditor } from "../../codemirror";
import AppIcon from "../../components/AppIcon.vue";
import CodeDiffView from "../shared/CodeDiffView.vue";
import { dialogRoot } from "../../primevue/preset";

const props = defineProps<PanelProps>();

const text = ref("");
const loading = ref(true);
const error = ref<string | null>(null);
const container = ref<HTMLElement | null>(null);
const useFallback = ref(false);
const saving = ref(false);
const saveError = ref<string | null>(null);
const saved = ref(false);
const originalText = ref("");
const showDiff = ref(false);
let editor: CodeMirrorEditor | null = null;
let codeMirror: typeof import("../../codemirror") | null = null;
const editorConfig = computed(
  () => props.config as CodeEditorConfig | undefined,
);
const { isDark } = useTheme();

const language = computed(() => editorConfig.value?.language ?? "plaintext");
const saveRouteId = computed(() => editorConfig.value?.saveRouteId);
const editable = computed(() => Boolean(saveRouteId.value));
const changed = computed(() => text.value !== originalText.value);
const diffDialogStyle = { width: "88vw" };
const diffDialogBreakpoints = { "1199px": "94vw", "575px": "100vw" };
const diffDialogPt = {
  root: dialogRoot("max-w-6xl"),
  content: "min-h-0 overflow-hidden p-0",
};
const diffDialogCloseButtonProps = {
  "aria-label": "Close diff review",
  title: "Close diff review",
};
const diffDialogMaximizeButtonProps = {
  "aria-label": "Maximize or restore diff review",
  title: "Maximize or restore diff review",
};

async function load(): Promise<void> {
  loading.value = true;
  const initial = editorConfig.value?.initialContent;
  if (initial !== undefined) {
    text.value = initial;
    originalText.value = initial;
    error.value = null;
    await mountEditor();
    return;
  }
  if (!props.source) {
    error.value = null;
    await mountEditor();
    return;
  }
  error.value = null;
  try {
    const doc = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
    text.value = typeof doc === "string" ? doc : JSON.stringify(doc, null, 2);
    originalText.value = text.value;
  } catch (e) {
    error.value = (e as Error).message;
    loading.value = false;
    return;
  }
  await mountEditor();
}

async function mountEditor(): Promise<void> {
  await nextTick();
  if (!container.value) {
    useFallback.value = true;
    loading.value = false;
    return;
  }
  try {
    const helpers = await import("../../codemirror");
    codeMirror = helpers;
    editor?.view.destroy();
    editor = helpers.createCodeMirrorEditor(container.value, {
      value: text.value,
      language: language.value,
      readOnly: !editable.value,
      ariaLabel: "Code editor",
      onChange(value) {
        text.value = value;
        saved.value = false;
      },
    });
  } catch {
    useFallback.value = true;
  } finally {
    loading.value = false;
  }
}

function syncTextFromEditor(): void {
  if (editor) text.value = codeMirror?.editorValue(editor) ?? text.value;
}

function openDiff(): void {
  syncTextFromEditor();
  showDiff.value = true;
}

async function save(): Promise<void> {
  const routeId = saveRouteId.value;
  if (!routeId) return;
  syncTextFromEditor();
  saving.value = true;
  saveError.value = null;
  try {
    const bodyKey = editorConfig.value?.saveBodyKey;
    const body = bodyKey
      ? {
          ...(editorConfig.value?.saveExtra ?? {}),
          [bodyKey]: JSON.parse(text.value),
        }
      : { content: text.value };
    await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource },
      body,
      editorConfig.value?.saveParams ?? props.source?.params ?? {},
      editorConfig.value?.saveMethod ?? "PUT",
    );
    saved.value = true;
    originalText.value = text.value;
    showDiff.value = false;
  } catch (e) {
    saveError.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

onMounted(load);
watch(() => [props.connectionId, props.resource?.uid], load);
watch(language, (next) => {
  codeMirror?.setEditorLanguage(editor, next);
});
watch(editable, (next) => {
  codeMirror?.setEditorReadOnly(editor, !next);
});
watch(text, (value) => {
  codeMirror?.setEditorValue(editor, value);
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
        <Button
          v-if="changed"
          type="button"
          severity="secondary"
          variant="outlined"
          size="small"
          aria-label="Show changes"
          @click="openDiff"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'git-compare' }"
            :size="14"
          />
          Diff
        </Button>
        <Button
          type="button"
          label="Save"
          :loading="saving"
          :disabled="saving"
          @click="save"
        />
      </div>
    </div>
    <SkeletonList v-if="loading" />
    <PanelError v-else-if="error" :message="error" retryable @retry="load" />
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
      class="shellcn-codemirror-host min-h-0 flex-1"
    />
    <Dialog
      v-model:visible="showDiff"
      modal
      maximizable
      header="Review changes"
      :style="diffDialogStyle"
      :breakpoints="diffDialogBreakpoints"
      :pt="diffDialogPt"
      :close-button-props="diffDialogCloseButtonProps"
      :maximize-button-props="diffDialogMaximizeButtonProps"
    >
      <div class="h-[min(76vh,56rem)] min-h-0">
        <CodeDiffView
          :original="originalText"
          :modified="text"
          :language="language"
          original-label="Loaded"
          modified-label="Edited"
          collapse-unchanged
        />
      </div>
    </Dialog>
  </div>
</template>
