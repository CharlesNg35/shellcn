<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { fetchDoc } from "../api/dataSource";
import type { PanelProps } from "./types";

const props = defineProps<PanelProps>();

const doc = ref<unknown>(null);
const loading = ref(true);
const error = ref<string | null>(null);

const pretty = computed(() =>
  doc.value === null ? "" : JSON.stringify(doc.value, null, 2),
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

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});
</script>

<template>
  <div class="h-full overflow-auto p-4">
    <p v-if="loading" class="text-sm text-surface-400">Loading…</p>
    <p v-else-if="error" class="text-sm text-red-500">{{ error }}</p>
    <pre
      v-else
      class="m-0 font-mono text-xs leading-relaxed text-surface-700 dark:text-surface-200"
      >{{ pretty }}</pre
    >
  </div>
</template>
