<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import { fetchDoc } from "@/api/dataSource";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import JsonNode from "./JsonNode.vue";
import AppIcon from "@/components/AppIcon.vue";
import { useRefreshableSource } from "../shared/useRefreshableSource";

const props = defineProps<PanelProps>();

// A document has no page size: past MAX the tree would mount tens of thousands
// of components, so the reader falls back to the raw editor. Between the two it
// still renders as a tree but mounts each subtree only once it is opened.
const MAX_TREE_BYTES = 1_000_000;
const LAZY_TREE_BYTES = 64_000;

const copied = ref(false);
const mode = ref<"tree" | "raw">("tree");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

async function loadDocument(): Promise<unknown> {
  if (!props.source) return null;
  return fetchDoc(props.connectionId, props.source, {
    resource: props.resource,
    record: props.record,
  });
}

const {
  data: doc,
  refreshing,
  error,
  showInitialLoader,
  blockingError,
  load,
  reset,
} = useRefreshableSource<unknown>(loadDocument, {
  initialValue: () => null,
  connectionId: () => props.connectionId,
});

function clearCopiedTimer(): void {
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = undefined;
}

const pretty = computed(() =>
  doc.value === null ? "" : JSON.stringify(doc.value, null, 2),
);
const treeAvailable = computed(() => pretty.value.length <= MAX_TREE_BYTES);
const lazyTree = computed(() => pretty.value.length > LAZY_TREE_BYTES);
const view = computed<"tree" | "raw">(() =>
  mode.value === "tree" && treeAvailable.value ? "tree" : "raw",
);

let downloadUrl = "";
function releaseDownload(): void {
  if (!downloadUrl) return;
  URL.revokeObjectURL(downloadUrl);
  downloadUrl = "";
}

// Built on click: a reactive data: URI would re-encode the whole document on
// every refresh just to sit in an href.
function download(): void {
  releaseDownload();
  const blob = new Blob([pretty.value], {
    type: "application/json;charset=utf-8",
  });
  downloadUrl = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = downloadUrl;
  link.download = "document.json";
  link.rel = "noopener";
  link.click();
  setTimeout(releaseDownload, 0);
}

async function copy(): Promise<void> {
  if (!navigator.clipboard) return;
  await navigator.clipboard.writeText(pretty.value);
  copied.value = true;
  clearCopiedTimer();
  copiedTimer = window.setTimeout(() => {
    copied.value = false;
  }, 1500);
}

watch(
  () => [
    props.connectionId,
    props.resource?.uid,
    props.source?.routeId,
    JSON.stringify(props.source?.params ?? {}),
    JSON.stringify(props.record ?? {}),
  ],
  () => {
    reset();
    void load();
  },
  {
    immediate: true,
  },
);

onUnmounted(() => {
  clearCopiedTimer();
  releaseDownload();
});
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex shrink-0 items-center justify-between gap-2 overflow-x-auto border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <div class="flex shrink-0 items-center gap-2">
        <Button
          type="button"
          severity="secondary"
          size="small"
          :label="view === 'tree' ? 'Raw' : 'Tree'"
          :disabled="!treeAvailable"
          :title="
            treeAvailable
              ? undefined
              : 'This document is too large for the tree view.'
          "
          @click="mode = mode === 'tree' ? 'raw' : 'tree'"
        />
        <Button
          type="button"
          severity="secondary"
          size="small"
          :disabled="refreshing"
          @click="load"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'refresh-cw' }"
            :size="14"
            :loading="refreshing"
          />
          Refresh
        </Button>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <Button
          type="button"
          severity="secondary"
          size="small"
          :label="copied ? 'Copied' : 'Copy'"
          @click="copy"
        />
        <Button
          type="button"
          severity="secondary"
          size="small"
          label="Download"
          @click="download"
        />
      </div>
    </div>

    <div class="min-h-0 flex-1">
      <SkeletonList v-if="showInitialLoader" />
      <PanelError
        v-else-if="blockingError"
        :message="error ?? ''"
        retryable
        @retry="load"
      />
      <div v-else-if="view === 'tree'" class="h-full overflow-auto p-4">
        <PanelError
          v-if="error"
          class="mb-4"
          :message="error"
          retryable
          @retry="load"
        />
        <JsonNode :value="doc" :depth="0" :lazy="lazyTree" />
      </div>
      <CodeTextEditor
        v-else
        :value="pretty"
        language="json"
        readonly
        aria-label="Raw JSON document"
      />
    </div>
  </div>
</template>
