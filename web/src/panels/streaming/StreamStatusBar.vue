<script setup lang="ts">
import { ref } from "vue";
import Button from "primevue/button";
import Popover from "primevue/popover";
import AppIcon from "@/components/AppIcon.vue";

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

const errorPopover = ref();
function showError(event: Event): void {
  errorPopover.value?.toggle(event);
}

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
  return "bg-rose-400";
}

function reconnectable(status: string): boolean {
  return ["closed", "disconnected", "error", "connection-lost"].includes(
    status,
  );
}
</script>

<template>
  <!-- Once connected the bar carries no useful signal, so it hides and hands the
       space back to the panel; it reappears only while connecting/lost/errored. -->
  <div
    v-if="status !== 'open' && status !== 'ready'"
    class="flex min-h-10 items-center justify-between gap-3 border-b border-surface-200 bg-surface-0 px-3 py-1.5 text-xs text-surface-600 dark:border-surface-800 dark:bg-surface-950 dark:text-surface-300"
  >
    <div class="flex min-w-0 items-center gap-2">
      <span
        class="h-2 w-2 shrink-0 rounded-full"
        :class="[tone(status), status === 'connecting' ? 'animate-pulse' : '']"
        aria-hidden="true"
      />
      <span class="font-medium">{{ label(status) }}</span>
      <Button
        v-if="error"
        type="button"
        text
        severity="secondary"
        size="small"
        title="Why did it fail?"
        aria-label="Show error details"
        class="h-7 px-1.5 py-0 text-surface-400"
        @click="showError"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'info' }" :size="14" />
        Details
      </Button>
    </div>
    <Popover ref="errorPopover">
      <div class="max-w-xs space-y-1">
        <p
          class="flex items-center gap-1.5 text-xs font-semibold text-surface-700 dark:text-surface-100"
        >
          <AppIcon
            :icon="{ type: 'lucide', value: 'triangle-alert' }"
            :size="13"
            class="text-amber-500"
          />
          {{ label(status) }}
        </p>
        <p class="text-xs text-surface-500 dark:text-surface-400">
          {{ error }}
        </p>
      </div>
    </Popover>
    <Button
      v-if="canReconnect && reconnectable(status)"
      type="button"
      size="small"
      severity="secondary"
      :disabled="reconnecting"
      @click="emit('reconnect')"
    >
      <AppIcon :icon="{ type: 'lucide', value: 'refresh-cw' }" :size="14" />
      {{ reconnecting ? "Reconnecting..." : "Reconnect" }}
    </Button>
  </div>
</template>
