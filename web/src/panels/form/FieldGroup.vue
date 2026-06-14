<script setup lang="ts">
import { computed } from "vue";
import type { Field, ResourceIdentity, Row } from "@/types/projection";
import FormField from "./FormField.vue";
import { isVisible } from "./condition";

const props = defineProps<{
  fields: Field[];
  values: Record<string, unknown>;
  connectionId?: string;
  resource?: ResourceIdentity | null;
  record?: Row | null;
}>();
const emit = defineEmits<{ update: [key: string, value: unknown] }>();

const visible = computed(() =>
  props.fields.filter((f) => isVisible(f.visibleWhen, props.values)),
);
</script>

<template>
  <div class="flex min-w-0 flex-col gap-4">
    <FormField
      v-for="field in visible"
      :key="field.key"
      :field="field"
      :model-value="values[field.key]"
      :connection-id="connectionId"
      :resource="resource"
      :record="record"
      @update:model-value="emit('update', field.key, $event)"
    />
  </div>
</template>
