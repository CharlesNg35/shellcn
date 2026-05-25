<script setup lang="ts">
import { ref, watch } from "vue";
import { fetchDoc } from "../api/dataSource";
import type { Schema } from "../types/projection";
import type { PanelProps } from "./types";
import SchemaForm from "./form/SchemaForm.vue";

const props = defineProps<PanelProps>();

const schema = ref<Schema | null>(null);
const loading = ref(true);
const error = ref<string | null>(null);

async function load(): Promise<void> {
  if (!props.source) {
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    schema.value = await fetchDoc<Schema>(props.connectionId, props.source, {
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
  <div class="h-full overflow-auto p-5">
    <p v-if="loading" class="text-sm text-surface-400">Loading…</p>
    <p v-else-if="error" class="text-sm text-red-500">{{ error }}</p>
    <SchemaForm v-else-if="schema" :schema="schema" />
  </div>
</template>
