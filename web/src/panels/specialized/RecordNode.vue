<script setup lang="ts">
import { Handle, Position } from "@vue-flow/core";
import AppIcon from "@/components/AppIcon.vue";
import type { GraphField } from "./graphLayout";

defineProps<{
  data: { label: string; group?: string; fields: GraphField[] };
  selected?: boolean;
}>();
</script>

<template>
  <div class="record-node" :class="{ 'record-node-selected': selected }">
    <Handle type="target" :position="Position.Left" class="record-handle" />
    <div class="record-head">
      <span class="truncate">{{ data.label }}</span>
    </div>
    <ul class="record-fields">
      <li v-for="f in data.fields" :key="f.name" class="record-field">
        <span class="record-field-name" :class="{ 'record-field-key': f.key }">
          <AppIcon
            v-if="f.key"
            :icon="{ type: 'lucide', value: 'key-round' }"
            :size="11"
            :title="f.key"
          />
          <span class="truncate">{{ f.name }}</span>
        </span>
        <span class="record-field-type truncate">{{ f.type }}</span>
      </li>
    </ul>
    <Handle type="source" :position="Position.Right" class="record-handle" />
  </div>
</template>

<style scoped>
.record-node {
  width: 220px;
  border: 1px solid var(--p-surface-300);
  border-radius: 8px;
  background: var(--p-surface-0);
  font-size: 12px;
  overflow: hidden;
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.06);
}
:global(.dark) .record-node {
  border-color: var(--p-surface-700);
  background: var(--p-surface-900);
}
.record-node-selected {
  border-color: var(--p-primary-color);
  box-shadow: 0 0 0 2px
    color-mix(in srgb, var(--p-primary-color) 35%, transparent);
}
.record-head {
  padding: 6px 10px;
  font-weight: 600;
  color: var(--p-surface-700);
  background: var(--p-surface-100);
  border-bottom: 1px solid var(--p-surface-200);
}
:global(.dark) .record-head {
  color: var(--p-surface-100);
  background: var(--p-surface-800);
  border-bottom-color: var(--p-surface-700);
}
.record-fields {
  list-style: none;
  margin: 0;
  padding: 0;
}
.record-field {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 3px 10px;
  border-bottom: 1px solid var(--p-surface-100);
}
:global(.dark) .record-field {
  border-bottom-color: var(--p-surface-800);
}
.record-field:last-child {
  border-bottom: none;
}
.record-field-name {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  color: var(--p-surface-700);
}
:global(.dark) .record-field-name {
  color: var(--p-surface-200);
}
.record-field-key {
  color: var(--p-primary-color);
  font-weight: 500;
}
.record-field-type {
  color: var(--p-surface-400);
  font-size: 11px;
}
.record-handle {
  width: 7px;
  height: 7px;
  background: var(--p-primary-color);
  border: none;
}
</style>
