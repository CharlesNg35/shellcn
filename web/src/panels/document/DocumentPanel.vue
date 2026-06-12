<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import Button from "primevue/button";
import { fetchDoc } from "../../api/dataSource";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "../../components/SkeletonList.vue";
import CodeTextEditor from "../shared/CodeTextEditor.vue";
import JsonNode from "./JsonNode.vue";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<PanelProps>();

const doc = ref<unknown>(null);
const loadedOnce = ref(false);
const refreshing = ref(false);
const error = ref<string | null>(null);
const copied = ref(false);
const mode = ref<"tree" | "raw">("tree");
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

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
const showInitialLoader = computed(() => refreshing.value && !loadedOnce.value);
const blockingError = computed(() => error.value && !loadedOnce.value);

async function load(): Promise<void> {
  if (!props.source) {
    loadedOnce.value = true;
    return;
  }
  if (refreshing.value) return;
  refreshing.value = true;
  error.value = null;
  try {
    doc.value = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
    loadedOnce.value = true;
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    refreshing.value = false;
  }
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
  () => [props.connectionId, props.resource?.uid],
  () => {
    doc.value = null;
    loadedOnce.value = false;
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
