<script setup lang="ts">
import {
  computed,
  onActivated,
  onDeactivated,
  onUnmounted,
  ref,
  toRef,
  watch,
} from "vue";
import Button from "primevue/button";
import { agentApi } from "@/api/agent";
import type { Enrollment, InstallArtifact } from "@/types/projection";
import type { PanelProps } from "../core/types";
import { useAgentState } from "@/composables/useAgentState";
import AppIcon from "@/components/AppIcon.vue";

const props = defineProps<PanelProps>();
const emit = defineEmits<{ online: [] }>();

const enrollment = ref<Enrollment | null>(null);
const error = ref<string | null>(null);
const copied = ref<string | null>(null);
const active = ref(true);
let copiedTimer: ReturnType<typeof setTimeout> | undefined;

const { status, message, online, refresh, start, stop } = useAgentState(
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

// The binary download only helps for host-run installs (a shell/PowerShell
// command); container or orchestrator installs (docker, compose, k8s) pull the
// image themselves, so the link would just be noise.
const hostInstallKinds = [
  "shell",
  "powershell",
  "bash",
  "terminal",
  "script",
  "binary",
  "native",
];
const showAgentDownload = computed(
  () =>
    status.value !== "online" &&
    Boolean(enrollment.value?.downloadUrl) &&
    (enrollment.value?.artifacts ?? []).some((artifact) =>
      hostInstallKinds.some((kind) =>
        artifact.kind.toLowerCase().includes(kind),
      ),
    ),
);

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
    enrollment.value = await agentApi.enroll(props.connectionId);
    void refresh();
  } catch (e) {
    error.value = (e as Error).message;
  }
}

async function copy(artifact: InstallArtifact): Promise<void> {
  const text = artifact.content ?? artifact.command;
  if (!text) return;
  try {
    await navigator.clipboard?.writeText(text);
    copied.value = artifact.kind;
    clearCopiedTimer();
    copiedTimer = setTimeout(() => (copied.value = null), 1500);
  } catch {
    // clipboard unavailable
  }
}

function download(artifact: InstallArtifact): void {
  if (!artifact.content) return;
  const blob = new Blob([artifact.content], {
    type: "text/yaml;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = artifact.filename || `${artifact.kind}.yml`;
  a.click();
  URL.revokeObjectURL(url);
}

watch(
  () => props.connectionId,
  () => {
    clearCopiedTimer();
    copied.value = null;
    enrollment.value = null;
    error.value = null;
    if (active.value) start();
    else stop();
  },
  { immediate: true },
);

onActivated(() => {
  if (active.value) return;
  active.value = true;
  start();
});
onDeactivated(() => {
  active.value = false;
  stop();
});

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
    <p v-if="showAgentDownload" class="text-sm text-surface-500">
      Don't have the agent yet?
      <a
        :href="enrollment?.downloadUrl"
        target="_blank"
        rel="noopener noreferrer"
        class="font-medium text-primary-600 hover:underline dark:text-primary-400"
      >
        Download the latest release</a
      >.
    </p>
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
        class="flex items-center justify-between gap-2 border-b border-surface-200 px-3 py-2 text-sm font-medium dark:border-surface-800"
      >
        <span class="truncate">{{ artifact.label }}</span>
        <div class="flex shrink-0 items-center gap-1">
          <Button
            v-if="artifact.content"
            type="button"
            size="small"
            variant="text"
            :title="`Download ${artifact.filename || artifact.kind}`"
            @click="download(artifact)"
          >
            <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="13" />
            Download
          </Button>
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
      </div>
      <pre
        class="m-0 overflow-auto p-3 font-mono text-xs text-surface-700 dark:text-surface-200"
        >{{ artifact.content || artifact.command }}</pre
      >
    </div>
  </div>
</template>
