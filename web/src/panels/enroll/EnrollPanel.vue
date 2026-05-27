<script setup lang="ts">
import { computed, onUnmounted, ref, toRef, watch } from "vue";
import Button from "primevue/button";
import { api } from "../../api/client";
import type { Enrollment, InstallArtifact } from "../../types/projection";
import type { PanelProps } from "../core/types";
import { useAgentState } from "../../composables/useAgentState";
import AppIcon from "../../components/AppIcon.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{ online: [] }>();

const enrollment = ref<Enrollment | null>(null);
const error = ref<string | null>(null);
const copied = ref<string | null>(null);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

const { status, message, online, refresh, start } = useAgentState(
  toRef(props, "connectionId"),
);

function clearCopiedTimer(): void {
  if (copiedTimer) clearTimeout(copiedTimer);
  copiedTimer = undefined;
}

const statusTone = computed(() => {
  switch (status.value) {
    case "online":
      return "bg-emerald-400";
    case "offline":
    case "error":
      return "bg-rose-400";
    default:
      return "animate-pulse bg-amber-400";
  }
});

const heading = computed(() => {
  switch (status.value) {
    case "online":
      return "Agent online";
    case "offline":
    case "error":
      return "Agent disconnected";
    default:
      return "Connect the agent";
  }
});

const guidance = computed(() => {
  switch (status.value) {
    case "offline":
    case "error":
      return "Restart the installed agent on the target host. Generate a new command only if the previous install command was lost or intentionally rotated.";
    case "online":
      return "";
    default:
      return "This connection reaches a private target through an agent. Run the command on the target host; this page updates when the agent dials back.";
  }
});

watch(online, (isOnline) => {
  if (isOnline) {
    emit("online");
  }
});

async function enroll(): Promise<void> {
  error.value = null;
  try {
    enrollment.value = await api.post<Enrollment>(
      `/connections/${props.connectionId}/agent/enrollments`,
    );
    void refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function copy(artifact: InstallArtifact): Promise<void> {
  if (!artifact.command) return;
  try {
    await navigator.clipboard?.writeText(artifact.command);
    copied.value = artifact.kind;
    clearCopiedTimer();
    copiedTimer = setTimeout(() => (copied.value = null), 1500);
  } catch {
    // clipboard unavailable
  }
}

watch(
  () => props.connectionId,
  () => {
    clearCopiedTimer();
    copied.value = null;
    enrollment.value = null;
    error.value = null;
    start();
  },
  { immediate: true },
);

onUnmounted(clearCopiedTimer);
</script>

<template>
  <div class="mx-auto flex h-full max-w-2xl flex-col gap-5 overflow-auto p-6">
    <div class="flex items-center gap-3">
      <span class="h-2.5 w-2.5 rounded-full" :class="statusTone" />
      <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-0">
        {{ heading }}
      </h2>
    </div>

    <p v-if="guidance" class="text-sm text-surface-500">{{ guidance }}</p>
    <p v-if="message" class="text-sm text-surface-400">{{ message }}</p>
    <p v-if="error" class="text-sm text-red-500">{{ error }}</p>

    <Button
      v-if="!enrollment && status !== 'online'"
      type="button"
      class="self-start"
      @click="enroll"
    >
      {{
        status === "pending"
          ? "Generate install command"
          : "Generate new install command"
      }}
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
          <AppIcon :icon="{ type: 'lucide', value: 'copy' }" :size="13" />
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
