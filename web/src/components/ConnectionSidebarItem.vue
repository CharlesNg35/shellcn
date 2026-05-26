<script setup lang="ts">
import AppIcon from "./AppIcon.vue";
import { useWorkspaceStore } from "../stores/workspace";
import type { ConnectionSummary } from "../types/projection";

defineProps<{
  connection: ConnectionSummary;
  active: boolean;
}>();

const emit = defineEmits<{
  open: [connection: ConnectionSummary];
}>();

const ws = useWorkspaceStore();

function dotClass(c: ConnectionSummary): string {
  if (c.status === "offline") return "bg-red-500";
  if (ws.isConnected(c.id)) return "bg-emerald-400";
  return "bg-surface-300 dark:bg-surface-600";
}

function dotTitle(c: ConnectionSummary): string {
  if (c.status === "offline") return "Agent offline";
  if (ws.isConnected(c.id)) return "Connected";
  return "Idle";
}

function shareTitle(c: ConnectionSummary): string {
  if (c.sharedWithMe) return `Shared with you · ${c.access ?? "use"}`;
  if (c.sharedByMe) return "Shared by you";
  return "";
}
</script>

<template>
  <div
    class="flex min-h-10 w-full items-center gap-2.5 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-surface-100 dark:hover:bg-surface-800"
    :class="
      active
        ? 'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60'
        : ''
    "
  >
    <span
      class="connection-drag-handle cursor-grab touch-none rounded p-0.5 text-surface-500 active:cursor-grabbing"
      title="Drag connection"
      aria-label="Drag connection"
    >
      <AppIcon :icon="connection.icon" :size="16" />
    </span>
    <button
      type="button"
      class="flex min-w-0 flex-1 flex-col text-left"
      @click="emit('open', connection)"
    >
      <span class="truncate text-surface-800 dark:text-surface-100">
        {{ connection.name }}
      </span>
      <span class="truncate text-xs text-surface-400">
        {{ connection.protocol }}
      </span>
    </button>
    <span
      class="h-2 w-2 shrink-0 rounded-full"
      :class="dotClass(connection)"
      :title="dotTitle(connection)"
    />
    <AppIcon
      v-if="connection.sharedWithMe || connection.sharedByMe"
      :icon="{
        type: 'name',
        value: connection.sharedWithMe ? 'users' : 'share-2',
      }"
      :size="14"
      class="shrink-0 text-surface-400"
      :title="shareTitle(connection)"
    />
  </div>
</template>
