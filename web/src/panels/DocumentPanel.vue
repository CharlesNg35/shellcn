<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import { fetchDoc } from "../api/dataSource";
import type { PanelProps } from "./types";
import JsonNode from "./document/JsonNode.vue";

const props = defineProps<PanelProps>();

const doc = ref<unknown>(null);
const loading = ref(true);
const error = ref<string | null>(null);
const copied = ref(false);
const mode = ref<"tree" | "raw">("tree");

const pretty = computed(() =>
  doc.value === null ? "" : JSON.stringify(doc.value, null, 2),
);
const downloadHref = computed(
  () =>
    `data:application/json;charset=utf-8,${encodeURIComponent(pretty.value)}`,
);

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    doc.value = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function copy(): Promise<void> {
  if (!navigator.clipboard) return;
  await navigator.clipboard.writeText(pretty.value);
  copied.value = true;
  window.setTimeout(() => {
    copied.value = false;
  }, 1500);
}

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});
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
          label="Refresh"
          :disabled="loading"
          @click="load"
        />
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

    <div class="min-h-0 flex-1 overflow-auto p-4">
      <p v-if="loading" class="text-sm text-surface-400">Loading…</p>
      <p v-else-if="error" class="text-sm text-red-500">{{ error }}</p>
      <JsonNode v-else-if="mode === 'tree'" :value="doc" :depth="0" />
      <pre
        v-else
        class="m-0 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
        >{{ pretty }}</pre
      >
    </div>
  </div>
</template>
