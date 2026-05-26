<script setup lang="ts">
import AppIcon from "../../components/AppIcon.vue";

defineProps<{ path: string }>();
const emit = defineEmits<{ navigate: [path: string] }>();

function crumbs(path: string): { label: string; path: string }[] {
  const parts = path.split("/").filter(Boolean);
  const out: { label: string; path: string }[] = [{ label: "Root", path: "/" }];
  let current = "";
  for (const part of parts) {
    current += `/${part}`;
    out.push({ label: part, path: current });
  }
  return out;
}
</script>

<template>
  <div
    class="flex items-center gap-1 overflow-x-auto border-b border-surface-200 px-3 py-2 text-sm dark:border-surface-800"
  >
    <template v-for="(c, i) in crumbs(path)" :key="c.path">
      <AppIcon
        v-if="i > 0"
        :icon="{ type: 'name', value: 'chevron-right' }"
        :size="14"
        class="text-surface-300"
      />
      <button
        type="button"
        class="rounded-md px-1.5 py-0.5 text-surface-500 transition-colors hover:bg-surface-100 hover:text-surface-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/35 dark:hover:bg-surface-800 dark:hover:text-surface-100"
        @click="emit('navigate', c.path)"
      >
        {{ c.label }}
      </button>
    </template>
  </div>
</template>
