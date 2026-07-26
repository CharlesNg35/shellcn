<script setup lang="ts">
import { computed } from "vue";
import AppIcon from "./AppIcon.vue";
import { useWorkspaceStore } from "../stores/workspace";
import {
  ConnLiveState,
  useConnectionStatusStore,
} from "../stores/connectionStatus";
import { ConnectionStatus, type ConnectionSummary } from "../types/projection";

const props = defineProps<{
  connection: ConnectionSummary;
  active: boolean;
  dragging?: boolean;
  highlighted?: boolean;
  collapsed?: boolean;
}>();

const emit = defineEmits<{
  open: [connection: ConnectionSummary];
}>();

const ws = useWorkspaceStore();
const live = useConnectionStatusStore();

const DotState = {
  Offline: "offline",
  Error: "error",
  Connecting: "connecting",
  Connected: "connected",
  Idle: "idle",
} as const;
type DotState = (typeof DotState)[keyof typeof DotState];
// The dot reflects the pooled backend session: agent reachability first, then
// the workspace keepalive/HTTP health state.
const dotState = computed<DotState>(() => {
  const c = props.connection;
  if (c.status === ConnectionStatus.Offline) return DotState.Offline;
  const state = live.get(c.id)?.state;
  if (state === ConnLiveState.Error) return DotState.Error;
  if (!ws.isConnected(c.id)) return DotState.Idle;
  if (state === ConnLiveState.Connected) return DotState.Connected;
  return DotState.Connecting;
});

const dotClass = computed(() => {
  switch (dotState.value) {
    case DotState.Connected:
      return "bg-emerald-400";
    case DotState.Connecting:
      return "bg-amber-400 animate-pulse";
    case DotState.Error:
    case DotState.Offline:
      return "bg-rose-400";
    default:
      return "bg-surface-300 dark:bg-surface-600";
  }
});

const dotTitle = computed(() => {
  switch (dotState.value) {
    case DotState.Connected:
      return "Connected";
    case DotState.Connecting:
      return "Connecting…";
    case DotState.Error:
      return live.get(props.connection.id)?.reason ?? "Connection failed";
    case DotState.Offline:
      return "Agent offline";
    default:
      return "Idle";
  }
});

function shareTitle(c: ConnectionSummary): string {
  if (c.sharedWithMe) {
    const by = c.ownerName ? `Shared by ${c.ownerName}` : "Shared with you";
    return `${by} · ${c.access ?? "view"}`;
  }
  if (c.sharedByMe) return "Shared by you";
  return "";
}
</script>

<template>
  <div
    class="connection-sidebar-drag-item flex items-center rounded-md text-left text-sm transition-colors"
    :data-connection-id="connection.id"
    :class="[
      collapsed
        ? 'mx-auto h-10 w-10 justify-center'
        : 'mx-1 min-h-10 w-[calc(100%-0.5rem)] gap-2.5 overflow-hidden px-2 py-1.5',
      !collapsed &&
        !dragging &&
        'hover:bg-surface-100 dark:hover:bg-surface-800',
      !collapsed &&
        !active &&
        highlighted &&
        'bg-surface-100 dark:bg-surface-800',
      !collapsed &&
        active &&
        'bg-primary-50 font-medium text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60',
    ]"
  >
    <button
      v-if="collapsed"
      type="button"
      class="relative flex h-9 w-9 items-center justify-center rounded-full text-surface-500 transition-colors"
      :class="[
        !dragging &&
          !active &&
          'hover:bg-surface-100 dark:hover:bg-surface-800',
        !active && highlighted && 'bg-surface-100 dark:bg-surface-800',
        active &&
          'bg-primary-50 text-primary-700 ring-1 ring-primary-200/70 dark:bg-primary-950/40 dark:text-primary-200 dark:ring-primary-900/60',
      ]"
      v-tooltip.right="connection.name"
      :aria-label="`Open ${connection.name}`"
      :aria-current="active ? 'page' : undefined"
      @click="emit('open', connection)"
    >
      <AppIcon :icon="connection.icon" :size="16" />
      <span
        class="absolute right-0 bottom-0 h-2 w-2 rounded-full ring-2 ring-surface-50 dark:ring-surface-900"
        :class="dotClass"
        :title="dotTitle"
      />
    </button>

    <template v-else>
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
      <span
        class="h-2 w-2 shrink-0 rounded-full"
        :class="dotClass"
        :title="dotTitle"
      />
    </template>
  </div>
</template>
