<script setup lang="ts">
import { computed, ref } from "vue";
import Button from "primevue/button";
import AppIcon from "@/components/AppIcon.vue";
import type { AiToolCall } from "@/stores/aiChat";

const props = defineProps<{ calls: AiToolCall[] }>();
const expanded = ref(false);

const summary = computed(() => {
  const running = props.calls.filter((c) => c.status === "running").length;
  const errored = props.calls.filter((c) => c.status === "error").length;
  if (running) return `${running} tool${running > 1 ? "s" : ""} running…`;
  if (errored) return `${props.calls.length} tool calls · ${errored} failed`;
  return `${props.calls.length} tool call${props.calls.length > 1 ? "s" : ""}`;
});

function statusIcon(status: AiToolCall["status"]): string {
  return status === "running"
    ? "loader"
    : status === "error"
      ? "circle-x"
      : "circle-check";
}
function statusColor(status: AiToolCall["status"]): string {
  return status === "running"
    ? "text-primary-500"
    : status === "error"
      ? "text-red-500"
      : "text-emerald-500";
}
</script>

<template>
  <div
    v-if="calls.length"
    class="my-1.5 rounded-md border border-surface-200 dark:border-surface-700"
  >
    <Button
      type="button"
      text
      severity="secondary"
      class="flex w-full items-center gap-2 px-2.5 py-1.5 text-xs font-medium text-surface-600 dark:text-surface-300"
      :aria-expanded="expanded"
      @click="expanded = !expanded"
    >
      <AppIcon
        :icon="{ type: 'lucide', value: 'wrench' }"
        :size="13"
        class="text-surface-400"
      />
      <span class="flex-1 text-left">{{ summary }}</span>
      <AppIcon
        :icon="{
          type: 'lucide',
          value: expanded ? 'chevron-up' : 'chevron-down',
        }"
        :size="14"
        class="text-surface-400"
      />
    </Button>
    <ul
      v-if="expanded"
      class="flex flex-col gap-1 border-t border-surface-200 px-2.5 py-1.5 dark:border-surface-700"
    >
      <li
        v-for="c in calls"
        :key="c.id"
        class="flex items-center gap-2 text-xs"
      >
        <span
          v-if="c.subagent"
          class="text-violet-500 dark:text-violet-400"
          :title="`subagent: ${c.subagent}`"
          aria-hidden="true"
          >▸</span
        >
        <AppIcon
          :icon="{ type: 'lucide', value: statusIcon(c.status) }"
          :size="13"
          :class="[
            statusColor(c.status),
            c.status === 'running' ? 'animate-spin' : '',
          ]"
        />
        <code
          class="min-w-0 shrink truncate"
          :class="
            c.subagent
              ? 'text-violet-600 dark:text-violet-300'
              : 'text-surface-700 dark:text-surface-200'
          "
          >{{ c.name }}</code
        >
        <span v-if="c.err" class="min-w-0 flex-1 truncate text-red-500">{{
          c.err
        }}</span>
      </li>
    </ul>
  </div>
</template>
