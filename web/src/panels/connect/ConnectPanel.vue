<script setup lang="ts">
import { computed, onActivated, onDeactivated, ref, toRef, watch } from "vue";
import Button from "primevue/button";
import { useAgentState } from "@/composables/useAgentState";
import AppIcon from "@/components/AppIcon.vue";
import AppAlert from "@/components/AppAlert.vue";
import type { ConnectionSummary } from "@/types/projection";

const props = defineProps<{
  connectionId: string;
  connection?: ConnectionSummary;
  connecting?: boolean;
  errorMessage?: string;
}>();
const emit = defineEmits<{ connect: []; enroll: [] }>();

const isAgent = computed(() => props.connection?.transport === "agent");
const connectionError = computed(() => props.errorMessage?.trim() ?? "");
const active = ref(true);

const agent = useAgentState(toRef(props, "connectionId"));
const canConnect = computed(() => !isAgent.value || agent.online.value);
// Until the first state resolves we don't know if the agent is up; show a neutral
// "checking" state so an already-online agent doesn't flash "waiting → connected".
const checking = computed(() => isAgent.value && !agent.ready.value);
// Agent is the gate: once we know it's offline, "Connect" isn't actionable and
// the real next step is enrolling the agent.
const gated = computed(
  () => isAgent.value && agent.ready.value && !agent.online.value,
);

const agentTone = computed(() => {
  if (checking.value) return "bg-surface-300 animate-pulse dark:bg-surface-600";
  switch (agent.status.value) {
    case "online":
      return "bg-emerald-400";
    case "pending":
      return "bg-amber-400 animate-pulse";
    default:
      return "bg-rose-400";
  }
});
const agentLabel = computed(() => {
  if (checking.value) return "Checking agent…";
  switch (agent.status.value) {
    case "online":
      return "Agent connected";
    case "pending":
      return "Waiting for agent";
    case "error":
      return "Agent error";
    default:
      return "Agent offline";
  }
});

function syncAgentPolling(): void {
  if (active.value && isAgent.value) agent.start();
  else agent.stop();
}

watch([() => props.connectionId, isAgent], syncAgentPolling, {
  immediate: true,
});
onActivated(() => {
  if (active.value) return;
  active.value = true;
  syncAgentPolling();
});
onDeactivated(() => {
  active.value = false;
  agent.stop();
});
</script>

<template>
  <div
    class="mx-auto flex h-full w-full max-w-md flex-col items-center justify-center gap-5 p-8 text-center"
  >
    <span
      class="flex h-16 w-16 items-center justify-center rounded-2xl bg-surface-100 text-surface-500 dark:bg-surface-800 dark:text-surface-400"
    >
      <AppIcon :icon="connection?.icon" :size="28" />
    </span>

    <div class="space-y-1">
      <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-0">
        Not connected
      </h2>
      <p class="text-sm text-surface-500 dark:text-surface-400">
        {{ connection?.name ?? connectionId }} · {{ connection?.protocol }}
      </p>
    </div>

    <!-- Agent reachability: a session can only open once the tunnel is up. -->
    <div
      v-if="isAgent"
      class="flex items-center gap-2 rounded-full border border-surface-200 px-3 py-1 text-xs dark:border-surface-800"
    >
      <span class="h-2 w-2 rounded-full" :class="agentTone" />
      <span class="font-medium text-surface-600 dark:text-surface-300">
        {{ agentLabel }}
      </span>
    </div>

    <div class="flex items-center gap-2">
      <Button v-if="gated" @click="emit('enroll')">Set up agent</Button>
      <Button
        :disabled="!canConnect || connecting"
        :severity="gated || checking ? 'secondary' : undefined"
        :outlined="gated || checking"
        @click="emit('connect')"
      >
        <AppIcon
          :icon="{ type: 'lucide', value: 'play' }"
          :size="16"
          :loading="connecting"
        />
        Connect
      </Button>
    </div>

    <AppAlert v-if="connectionError" tone="danger" title="Could not connect">
      {{ connectionError }}
    </AppAlert>

    <p v-if="gated" class="text-xs text-surface-400">
      Connect becomes available once the agent is online.
    </p>
  </div>
</template>
