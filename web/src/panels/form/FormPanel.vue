<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useToast } from "primevue/usetoast";
import { fetchDoc, runFormAction } from "@/api/dataSource";
import type { FormPanelConfig, Schema } from "@/types/projection";
import type { PanelProps } from "../core/types";
import PanelError from "../shared/PanelError.vue";
import SkeletonList from "@/components/SkeletonList.vue";
import SchemaForm from "./SchemaForm.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{ close: [] }>();
const toast = useToast();

const formConfig = computed(() => props.config as FormPanelConfig | undefined);
const schema = ref<Schema | null>(null);
const loading = ref(true);
const submitting = ref(false);
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
      record: props.record,
    });
  } catch (e) {
    error.value = (e as Error).message;
  } finally {
    loading.value = false;
  }
}

async function submit(value: Record<string, unknown>): Promise<void> {
  const routeId = formConfig.value?.submitRouteId;
  if (!routeId) return;
  submitting.value = true;
  try {
    await runFormAction(
      props.connectionId,
      routeId,
      { resource: props.resource, record: props.record },
      value,
      formConfig.value?.params ?? props.source?.params ?? {},
      formConfig.value?.submitMethod ?? "PATCH",
    );
    const feedback = formConfig.value?.saveToast;
    toast.add({
      severity: feedback?.severity ?? "success",
      summary: feedback?.summary ?? "Saved",
      detail: feedback?.detail ?? "Changes were submitted.",
      life: 2200,
    });
    if (formConfig.value?.saveDismiss === "close") {
      emit("close");
    }
  } catch (e) {
    toast.add({
      severity: "error",
      summary: "Save failed",
      detail: (e as Error).message,
      life: 4000,
    });
  } finally {
    submitting.value = false;
  }
}

watch(() => [props.connectionId, props.resource?.uid], load, {
  immediate: true,
});
</script>

<template>
  <div class="h-full overflow-auto p-5">
    <SkeletonList v-if="loading" />
    <PanelError v-else-if="error" :message="error" retryable @retry="load" />
    <SchemaForm
      v-else-if="schema"
      :schema="schema"
      :submit-label="
        formConfig?.submitLabel ??
        (formConfig?.submitRouteId ? 'Save' : undefined)
      "
      :busy="submitting"
      :connection-id="connectionId"
      :resource="resource"
      :record="record"
      @submit="submit"
    />
  </div>
</template>
