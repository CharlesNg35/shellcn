<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import { fetchDoc, runAction, watch as watchResource } from "@/api/dataSource";
import type { CodeEditorConfig, ResourceEvent } from "@/types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import { useTheme } from "@/composables/useTheme";
import type { CodeMirrorEditor } from "@/codemirror";
import AppIcon from "@/components/AppIcon.vue";
import CodeDiffView from "../shared/CodeDiffView.vue";
import { dialogRoot } from "@/primevue/preset";
import { useDirtyGuard } from "../shared/useDirtyGuard";

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
const externalChanged = ref(false);
const deletedOnServer = ref(false);
const serverContent = ref<string | null>(null);
let stopWatch: (() => void) | null = null;
let editor: CodeMirrorEditor | null = null;
let codeMirror: typeof import("@/codemirror") | null = null;
let loadRequest = 0;
const editorConfig = computed(
  () => props.config as CodeEditorConfig | undefined,
);
const { isDark } = useTheme();

const language = computed(() => editorConfig.value?.language ?? "plaintext");
const saveRouteId = computed(() => editorConfig.value?.saveRouteId);
const editable = computed(() => Boolean(saveRouteId.value));
const changed = computed(() => text.value !== originalText.value);
const { confirmBeforeDiscard } = useDirtyGuard({
  isDirty: () => editable.value && changed.value,
  header: "Discard unsaved editor changes?",
  message: "This editor has unsaved changes. Discard them and reload?",
});
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
  const request = ++loadRequest;
  loading.value = true;
  const initial = editorConfig.value?.initialContent;
  if (initial !== undefined) {
    text.value = initial;
    originalText.value = initial;
    error.value = null;
    await mountEditor(request);
    return;
  }
  if (!props.source) {
    error.value = null;
    await mountEditor(request);
    return;
  }
  error.value = null;
  try {
    const doc = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
      record: props.record,
    });
    if (request !== loadRequest) return;
    text.value = typeof doc === "string" ? doc : JSON.stringify(doc, null, 2);
    originalText.value = text.value;
  } catch (e) {
    if (request !== loadRequest) return;
    error.value = (e as Error).message;
    loading.value = false;
    return;
  }
  await mountEditor(request);
}

async function guardedLoad(): Promise<void> {
  await confirmBeforeDiscard(load);
}

// onServerPush updates the editor in place when clean, and stashes the change
// behind a notice when the user has unsaved edits so work is never clobbered.
function onServerPush(ev: ResourceEvent): void {
  if (ev.type === "deleted") {
    deletedOnServer.value = true;
    return;
  }
  deletedOnServer.value = false;
  const next = ev.resource;
  if (typeof next !== "string") return;
  syncTextFromEditor();
  if (!changed.value) {
    originalText.value = next;
    text.value = next;
    externalChanged.value = false;
    serverContent.value = null;
    return;
  }
  serverContent.value = next;
  externalChanged.value = true;
}

function reloadFromServer(): void {
  if (serverContent.value !== null) {
    originalText.value = serverContent.value;
    text.value = serverContent.value;
    serverContent.value = null;
  }
  externalChanged.value = false;
  saved.value = false;
}

function startWatch(): void {
  const source = editorConfig.value?.watch;
  if (stopWatch || !source) return;
  stopWatch = watchResource(
    props.connectionId,
    source,
    { resource: props.resource, record: props.record },
    onServerPush,
  );
}

function stopWatching(): void {
  stopWatch?.();
  stopWatch = null;
}

async function mountEditor(request = loadRequest): Promise<void> {
  await nextTick();
  if (request !== loadRequest) return;
  if (!container.value) {
    useFallback.value = true;
    loading.value = false;
    return;
  }
  try {
    const helpers = await import("@/codemirror");
    if (request !== loadRequest) return;
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
    if (request !== loadRequest) return;
    useFallback.value = true;
  } finally {
    if (request === loadRequest) loading.value = false;
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
    const result = await runAction(
      props.connectionId,
      routeId,
      { resource: props.resource, record: props.record },
      body,
      editorConfig.value?.saveParams ?? props.source?.params ?? {},
      editorConfig.value?.saveMethod ?? "PUT",
    );
    // Reset the baseline to the server's canonical content so the next save and
    // the live watch reconcile against exactly what was persisted.
    const refreshField = editorConfig.value?.refreshField;
    const fresh = refreshField ? result[refreshField] : undefined;
    if (typeof fresh === "string") {
      text.value = fresh;
      originalText.value = fresh;
    } else {
      originalText.value = text.value;
    }
    saved.value = true;
    externalChanged.value = false;
    serverContent.value = null;
    showDiff.value = false;
  } catch (e) {
    saveError.value = (e as Error).message;
  } finally {
    saving.value = false;
  }
}

onMounted(async () => {
  await load();
  startWatch();
});
watch(
  () => [
    props.connectionId,
    props.resource?.uid,
    props.source?.routeId,
    JSON.stringify(props.source?.params ?? {}),
    JSON.stringify(props.record ?? {}),
    editorConfig.value?.initialContent,
  ],
  guardedLoad,
);
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
  stopWatching();
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
    <div
      v-if="externalChanged"
      class="flex items-center justify-between gap-2 border-b border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-200"
    >
      <span
        >This object changed on the server while you have unsaved edits.</span
      >
      <div class="flex items-center gap-2">
        <Button
          type="button"
          severity="secondary"
          size="small"
          label="Reload"
          @click="reloadFromServer"
        />
        <Button
          type="button"
          severity="secondary"
          variant="text"
          size="small"
          label="Keep editing"
          @click="externalChanged = false"
        />
      </div>
    </div>
    <div
      v-if="deletedOnServer"
      class="border-b border-red-300 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-800 dark:bg-red-950 dark:text-red-300"
    >
      This object no longer exists on the server.
    </div>
    <SkeletonList v-if="loading" />
    <PanelError
      v-else-if="error"
      :message="error"
      retryable
      @retry="guardedLoad"
    />
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
