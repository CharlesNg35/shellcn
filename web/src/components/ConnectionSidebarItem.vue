<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "./AppIcon.vue";
import { useWorkspaceStore } from "../stores/workspace";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import type { ConnectionSummary } from "../types/projection";

const props = defineProps<{
  connection: ConnectionSummary;
  active: boolean;
  dragging?: boolean;
  highlighted?: boolean;
}>();

const emit = defineEmits<{
  open: [connection: ConnectionSummary];
}>();

const ws = useWorkspaceStore();
const live = useConnectionStatusStore();

type DotState = "offline" | "error" | "connecting" | "connected" | "idle";
// The dot reflects the pooled backend session: agent reachability first, then
// the workspace keepalive/HTTP health state.
const dotState = computed<DotState>(() => {
  const c = props.connection;
  if (c.status === "offline") return "offline";
  const state = live.get(c.id)?.state;
  if (state === "error") return "error";
  if (!ws.isConnected(c.id)) return "idle";
  if (state === "connected") return "connected";
  return "connecting";
});

const dotClass = computed(() => {
  switch (dotState.value) {
    case "connected":
      return "bg-emerald-400";
    case "connecting":
      return "bg-amber-400 animate-pulse";
    case "error":
    case "offline":
      return "bg-rose-400";
    default:
      return "bg-surface-300 dark:bg-surface-600";
  }
});

const dotTitle = computed(() => {
  switch (dotState.value) {
    case "connected":
      return "Connected";
    case "connecting":
      return "Connecting…";
    case "error":
      return live.get(props.connection.id)?.reason ?? "Connection failed";
    case "offline":
      return "Agent offline";
    default:
      return "Idle";
  }
});

function shareTitle(c: ConnectionSummary): string {
  if (c.sharedWithMe) {
    const by = c.ownerName ? `Shared by ${c.ownerName}` : "Shared with you";
    return `${by} · ${c.access ?? "use"}`;
  }
  if (c.sharedByMe) return "Shared by you";
  return "";
}
</script>

<template>
  <div
    class="connection-sidebar-drag-item mx-1 flex min-h-10 w-[calc(100%-0.5rem)] items-center gap-2.5 overflow-hidden rounded-md px-2 py-1.5 text-left text-sm transition-colors"
    :data-connection-id="connection.id"
    :class="[
      !dragging && 'hover:bg-surface-100 dark:hover:bg-surface-800',
      !active && highlighted && 'bg-surface-100 dark:bg-surface-800',
      active &&
        'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60',
    ]"
  >
    <span class="shrink-0 rounded p-0.5 text-surface-500" aria-hidden="true">
      <AppIcon :icon="connection.icon" :size="16" />
    </span>
    <button
      type="button"
      class="flex min-w-0 flex-1 flex-col overflow-hidden text-left"
      :title="connection.name"
      :aria-label="`Open ${connection.name}`"
      @click="emit('open', connection)"
    >
      <span
        class="block max-w-full truncate text-surface-800 dark:text-surface-100"
        :title="connection.name"
      >
        {{ connection.name }}
      </span>
      <span
        class="block max-w-full truncate text-xs text-surface-400"
        :title="connection.protocol"
      >
        {{ connection.protocol }}
      </span>
    </button>
    <span
      class="h-2 w-2 shrink-0 rounded-full"
      :class="dotClass"
      :title="dotTitle"
    />
    <AppIcon
      v-if="connection.sharedWithMe || connection.sharedByMe"
      :icon="{
        type: 'lucide',
        value: connection.sharedWithMe ? 'users' : 'share-2',
      }"
      :size="14"
      class="shrink-0 text-surface-400"
      :title="shareTitle(connection)"
    />
  </div>
</template>
