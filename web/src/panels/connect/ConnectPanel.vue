<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import Button from "primevue/button";
import { api } from "../../api/client";
import { useAgentState } from "../../composables/useAgentState";
import AppIcon from "../../components/AppIcon.vue";
import type {
  ConnectionDetail,
  ConnectionSummary,
} from "../../types/projection";

const props = defineProps<{
  connectionId: string;
  connection?: ConnectionSummary;
}>();
const emit = defineEmits<{ connect: []; enroll: [] }>();

const isAgent = computed(() => props.connection?.transport === "agent");

const detail = ref<ConnectionDetail | null>(null);
const details = computed<{ key: string; label: string; value: string }[]>(
  () => {
    const config = detail.value?.config ?? {};
    return Object.entries(config)
      .filter(
        ([, v]) =>
          v !== null && v !== undefined && v !== "" && typeof v !== "object",
      )
      .map(([key, v]) => ({ key, label: prettify(key), value: String(v) }));
  },
);

function prettify(key: string): string {
  const spaced = key.replace(/_/g, " ");
  return spaced.charAt(0).toUpperCase() + spaced.slice(1);
}

const agent = useAgentState(props.connectionId);
const canConnect = computed(() => !isAgent.value || agent.online.value);
// Agent is the gate: until its tunnel is up, "Connect" is not actionable and
// the real next step is enrolling the agent.
const gated = computed(() => isAgent.value && !agent.online.value);

const agentTone = computed(() => {
  switch (agent.status.value) {
    case "online":
      return "bg-emerald-400";
    case "pending":
      return "bg-amber-400 animate-pulse";
    default:
      return "bg-red-500";
  }
});
const agentLabel = computed(() => {
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

onMounted(async () => {
  if (isAgent.value) agent.start();
  try {
    detail.value = await api.get<ConnectionDetail>(
      `/connections/${props.connectionId}`,
    );
  } catch {
    // details are best-effort; the connect action does not depend on them
  }
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

    <!-- Connection details (non-secret config). -->
    <dl
      v-if="details.length"
      class="w-full max-w-xs divide-y divide-surface-200 rounded-lg border border-surface-200 text-left text-xs dark:divide-surface-800 dark:border-surface-800"
    >
      <div
        v-for="d in details"
        :key="d.key"
        class="flex items-center justify-between gap-3 px-3 py-1.5"
      >
        <dt class="text-surface-400">{{ d.label }}</dt>
        <dd
          class="min-w-0 truncate font-medium text-surface-700 dark:text-surface-200"
        >
          {{ d.value }}
        </dd>
      </div>
    </dl>

    <div class="flex items-center gap-2">
      <Button v-if="gated" @click="emit('enroll')">Set up agent</Button>
      <Button
        :disabled="!canConnect"
        :severity="gated ? 'secondary' : undefined"
        :outlined="gated"
        @click="emit('connect')"
      >
        <AppIcon :icon="{ type: 'name', value: 'play' }" :size="16" />
        Connect
      </Button>
    </div>
    <p v-if="gated" class="text-xs text-surface-400">
      Connect becomes available once the agent is online.
    </p>
  </div>
</template>
