<script setup lang="ts">
import Button from "primevue/button";
import AppIcon from "../../components/AppIcon.vue";

withDefaults(
  defineProps<{
    status?: string;
    error?: string | null;
    reconnecting?: boolean;
    canReconnect?: boolean;
  }>(),
  {
    status: "connecting",
    error: null,
    reconnecting: false,
    canReconnect: false,
  },
);

const emit = defineEmits<{ reconnect: [] }>();

function label(status: string): string {
  switch (status) {
    case "open":
    case "ready":
      return "Connected";
    case "connecting":
      return "Connecting";
    case "closed":
    case "disconnected":
      return "Disconnected";
    case "error":
    case "connection-lost":
      return "Connection lost";
    case "auth-failed":
      return "Authentication failed";
    case "credentials-required":
      return "Credentials required";
    case "recording-unsupported":
      return "Recording unavailable";
    case "recording-failed":
      return "Recording failed";
    case "missing-route":
      return "Missing stream route";
    default:
      return status;
  }
}

function tone(status: string): string {
  if (status === "open" || status === "ready") {
    return "bg-emerald-500";
  }
  if (status === "connecting") {
    return "bg-amber-400";
  }
  return "bg-red-500";
}

function reconnectable(status: string): boolean {
  return ["closed", "disconnected", "error", "connection-lost"].includes(
    status,
  );
}
</script>

<template>
  <div
    class="flex min-h-10 items-center justify-between gap-3 border-b border-surface-200 bg-surface-0 px-3 py-1.5 text-xs text-surface-600 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-300"
  >
    <div class="flex min-w-0 items-center gap-2">
      <span
        class="h-2 w-2 shrink-0 rounded-full"
        :class="[tone(status), status === 'connecting' ? 'animate-pulse' : '']"
        aria-hidden="true"
      />
      <span class="font-medium">{{ label(status) }}</span>
      <span v-if="error" class="truncate text-red-600 dark:text-red-300">
        {{ error }}
      </span>
    </div>
    <Button
      v-if="canReconnect && reconnectable(status)"
      type="button"
      size="small"
      severity="secondary"
      :disabled="reconnecting"
      @click="emit('reconnect')"
    >
      <AppIcon :icon="{ type: 'name', value: 'refresh-cw' }" :size="14" />
      {{ reconnecting ? "Reconnecting..." : "Reconnect" }}
    </Button>
  </div>
</template>
