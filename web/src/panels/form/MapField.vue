<script setup lang="ts">
import { ref, watch } from "vue";
import Button from "primevue/button";
import InputText from "primevue/inputtext";
import type {
  Field,
  ResourceIdentity,
  Row as ProjectionRow,
} from "@/types/projection";
import FormField from "./FormField.vue";
import AppIcon from "@/components/AppIcon.vue";
import { defaultForField } from "./defaults";

const props = defineProps<{
  field: Field;
  modelValue: unknown;
  connectionId?: string;
  resource?: ResourceIdentity | null;
  record?: ProjectionRow | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: Record<string, unknown>];
}>();

interface Row {
  key: string;
  value: unknown;
}

function toRows(obj: unknown): Row[] {
  return Object.entries((obj ?? {}) as Record<string, unknown>).map(
    ([key, value]) => ({ key, value }),
  );
}
function toObject(rows: Row[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const r of rows) if (r.key.trim() !== "") out[r.key] = r.value;
  return out;
}
function canon(obj: Record<string, unknown>): string {
  return JSON.stringify(
    Object.entries(obj).sort(([a], [b]) => (a < b ? -1 : 1)),
  );
}

const rows = ref<Row[]>(toRows(props.modelValue));

// Re-seed rows only on a genuine external change, so editing keystrokes (which
// echo back through modelValue) don't reset the row order or focus.
watch(
  () => props.modelValue,
  (v) => {
    if (
      canon((v ?? {}) as Record<string, unknown>) !==
      canon(toObject(rows.value))
    )
      rows.value = toRows(v);
  },
);

function emitRows(): void {
  emit("update:modelValue", toObject(rows.value));
}
function setKey(index: number, key: string): void {
  rows.value[index] = { ...rows.value[index], key };
  emitRows();
}
function setValue(index: number, value: unknown): void {
  rows.value[index] = { ...rows.value[index], value };
  emitRows();
}
function add(): void {
  rows.value = [
    ...rows.value,
    {
      key: "",
      value: props.field.item
        ? defaultForField(props.field.item, {
            resource: props.resource,
            record: props.record,
          })
        : "",
    },
  ];
}
function removeAt(index: number): void {
  rows.value = rows.value.filter((_, i) => i !== index);
  emitRows();
}
</script>

<template>
  <div class="flex min-w-0 flex-col gap-2">
    <p v-if="!rows.length" class="text-xs text-surface-400">No entries yet.</p>

    <div
      v-for="(row, index) in rows"
      :key="index"
      class="flex min-w-0 items-start gap-2"
    >
      <InputText
        :model-value="row.key"
        :placeholder="field.keyPlaceholder ?? field.keyLabel ?? 'Key'"
        :aria-label="`${field.keyLabel ?? 'Key'} ${index + 1}`"
        class="w-1/3 shrink-0"
        @update:model-value="setKey(index, $event ?? '')"
      />
      <div
        class="min-w-0 flex-1"
        role="group"
        :aria-label="`Value ${index + 1}`"
      >
        <FormField
          v-if="field.item"
          :field="field.item"
          :model-value="row.value"
          :connection-id="connectionId"
          :resource="resource"
          :record="record"
          hide-label
          @update:model-value="(value) => setValue(index, value)"
        />
      </div>
      <Button
        type="button"
        severity="danger"
        text
        :aria-label="`Remove entry ${index + 1}`"
        @click="removeAt(index)"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'trash-2' }" :size="15" />
      </Button>
    </div>

    <div>
      <Button
        type="button"
        severity="secondary"
        size="small"
        :aria-label="field.addLabel ?? 'Add entry'"
        @click="add"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'plus' }" :size="14" />
        {{ field.addLabel ?? "Add entry" }}
      </Button>
    </div>
  </div>
</template>
