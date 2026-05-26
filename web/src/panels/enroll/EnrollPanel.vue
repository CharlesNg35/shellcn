<script setup lang="ts">
import { onMounted, onUnmounted, ref } from "vue";
import Button from "primevue/button";
import { api } from "../../api/client";
import type {
  AgentState,
  Enrollment,
  InstallArtifact,
} from "../../types/projection";
import type { PanelProps } from "../core/types";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{ online: [] }>();

const enrollment = ref<Enrollment | null>(null);
const status = ref<AgentState["status"]>("pending");
const message = ref<string | undefined>();
const error = ref<string | null>(null);
const copied = ref<string | null>(null);

let poll: ReturnType<typeof setInterval> | undefined;

async function checkStatus(): Promise<void> {
  try {
    const state = await api.get<AgentState>(
      `/connections/${props.connectionId}/agent/state`,
    );
    status.value = state.status;
    message.value = state.message;
    if (state.status === "online") {
      stopPolling();
      emit("online");
    }
  } catch {
    // transient; keep polling
  }
}

function stopPolling(): void {
  if (poll) clearInterval(poll);
  poll = undefined;
}

async function enroll(): Promise<void> {
  error.value = null;
  try {
    enrollment.value = await api.post<Enrollment>(
      `/connections/${props.connectionId}/agent/enrollments`,
    );
    stopPolling();
    poll = setInterval(checkStatus, 2000);
    void checkStatus();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function copy(artifact: InstallArtifact): Promise<void> {
  if (!artifact.command) return;
  try {
    await navigator.clipboard?.writeText(artifact.command);
    copied.value = artifact.kind;
    setTimeout(() => (copied.value = null), 1500);
  } catch {
    // clipboard unavailable
  }
}

onMounted(() => {
  void checkStatus();
});
onUnmounted(stopPolling);
</script>

<template>
  <div class="mx-auto flex h-full max-w-2xl flex-col gap-5 overflow-auto p-6">
    <div class="flex items-center gap-3">
      <span
        class="h-2.5 w-2.5 rounded-full"
        :class="
          status === 'online' ? 'bg-emerald-400' : 'bg-amber-400 animate-pulse'
        "
      />
      <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-0">
        {{ status === "online" ? "Agent online" : "Connect the agent" }}
      </h2>
    </div>

    <p v-if="status !== 'online'" class="text-sm text-surface-500">
      This connection reaches a private target through an agent. Run the command
      on the target host; this page updates when the agent dials back.
    </p>
    <p v-if="message" class="text-sm text-surface-400">{{ message }}</p>
    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>

    <Button
      v-if="!enrollment && status !== 'online'"
      type="button"
      class="self-start"
      @click="enroll"
    >
      Generate install command
    </Button>

    <div
      v-for="artifact in enrollment?.artifacts ?? []"
      :key="artifact.kind"
      class="rounded-lg border border-surface-200 dark:border-surface-800"
    >
      <div
        class="flex items-center justify-between border-b border-surface-200 px-3 py-2 text-sm font-medium dark:border-surface-800"
      >
        <span>{{ artifact.label }}</span>
        <Button
          type="button"
          size="small"
          variant="text"
          @click="copy(artifact)"
        >
          <AppIcon :icon="{ type: 'name', value: 'copy' }" :size="13" />
          {{ copied === artifact.kind ? "Copied" : "Copy" }}
        </Button>
      </div>
      <pre
        class="m-0 overflow-auto p-3 font-mono text-xs text-surface-700 dark:text-surface-200"
        >{{ artifact.command }}</pre
      >
    </div>
  </div>
</template>
