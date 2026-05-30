<script setup lang="ts">
import Button from "primevue/button";
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
  <nav
    aria-label="Breadcrumb"
    class="flex items-center gap-1 overflow-x-auto border-b border-surface-200 px-3 py-2 text-sm dark:border-surface-800"
  >
    <template v-for="(c, i) in crumbs(path)" :key="c.path">
      <AppIcon
        v-if="i > 0"
        :icon="{ type: 'lucide', value: 'chevron-right' }"
        :size="14"
        class="text-surface-300"
      />
      <span
        v-if="i === crumbs(path).length - 1"
        aria-current="page"
        class="truncate px-2 py-1 font-medium text-surface-700 dark:text-surface-200"
      >
        {{ c.label }}
      </span>
      <Button
        v-else
        text
        severity="secondary"
        size="small"
        @click="emit('navigate', c.path)"
      >
        {{ c.label }}
      </Button>
    </template>
  </nav>
</template>
