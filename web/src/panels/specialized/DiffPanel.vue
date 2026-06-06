<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { fetchDoc } from "../../api/dataSource";
import type { DiffPanelConfig } from "../../types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "../../components/SkeletonList.vue";
import CodeDiffView from "../shared/CodeDiffView.vue";

const props = defineProps<PanelProps>();

const loading = ref(true);
const error = ref<string | null>(null);
const original = ref("");
const modified = ref("");

const config = computed(() => props.config as DiffPanelConfig | undefined);
const originalField = computed(() => config.value?.originalField ?? "original");
const modifiedField = computed(() => config.value?.modifiedField ?? "modified");
const mode = computed(() => config.value?.mode ?? "side_by_side");

function asRecord(value: unknown): Record<string, unknown> | null {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function textValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value == null) return "";
  return JSON.stringify(value, null, 2);
}

async function load(): Promise<void> {
  if (!props.source) {
    error.value = "Diff panel is missing a source.";
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = null;
  try {
    const doc = await fetchDoc(props.connectionId, props.source, {
      resource: props.resource,
    });
    const payload = asRecord(doc);
    if (!payload) {
      throw new Error("Diff route must return an object.");
    }
    original.value = textValue(payload[originalField.value]);
    modified.value = textValue(payload[modifiedField.value]);
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

watch(
  () => [
    props.connectionId,
    props.resource?.uid,
    props.source?.routeId,
    JSON.stringify(props.source?.params ?? {}),
    originalField.value,
    modifiedField.value,
  ],
  load,
  { immediate: true },
);
</script>

<template>
  <div class="h-full min-h-0">
    <SkeletonList v-if="loading" :rows="8" />
    <PanelError v-else-if="error" :message="error" retryable @retry="load" />
    <CodeDiffView
      v-else
      :original="original"
      :modified="modified"
      :language="config?.language"
      :original-label="config?.originalLabel ?? 'Original'"
      :modified-label="config?.modifiedLabel ?? 'Modified'"
      :mode="mode"
      :collapse-unchanged="config?.collapseUnchanged === true"
    />
  </div>
</template>
