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
const downloadHref = computed(
  () =>
    `data:application/json;charset=utf-8,${encodeURIComponent(pretty.value)}`,
);
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

onUnmounted(clearCopiedTimer);
</script>

<template>
  <div class="flex h-full flex-col">
    <div
      class="flex items-center justify-between gap-2 border-b border-surface-200 px-3 py-2 dark:border-surface-800"
    >
      <div class="flex items-center gap-2">
        <Button
          type="button"
          severity="secondary"
          :label="mode === 'tree' ? 'Raw' : 'Tree'"
          @click="mode = mode === 'tree' ? 'raw' : 'tree'"
        />
        <Button
          type="button"
          severity="secondary"
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
      <div class="flex items-center gap-2">
        <Button
          type="button"
          severity="secondary"
          :label="copied ? 'Copied' : 'Copy'"
          @click="copy"
        />
        <Button
          as="a"
          severity="secondary"
          :href="downloadHref"
          download="document.json"
          label="Download"
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
      <div v-else-if="mode === 'tree'" class="h-full overflow-auto p-4">
        <PanelError
          v-if="error"
          class="mb-4"
          :message="error"
          retryable
          @retry="load"
        />
        <JsonNode :value="doc" :depth="0" />
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
