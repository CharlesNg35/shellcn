<script setup lang="ts">
import { computed, ref, watch } from "vue";
import Button from "primevue/button";
import Dialog from "primevue/dialog";
import InputText from "primevue/inputtext";
import ProgressBar from "primevue/progressbar";
import { useStream } from "@/composables/useStream";
import type { ResolveContext } from "@/api/dataSource";
import { FileJobOperation, type FileJobConfig } from "@/types/projection";
import AppIcon from "@/components/AppIcon.vue";
import StreamStatusBar from "../streaming/StreamStatusBar.vue";
import { formatBytes } from "./fileTypes";

interface FileJobFrame {
  type?: string;
  jobId?: string;
  status?: string;
  message?: string;
  path?: string;
  operation?: string;
  percent?: number;
  bytesDone?: number;
  bytesTotal?: number;
  filesDone?: number;
  filesTotal?: number;
  rateBps?: number;
  downloadUrl?: string;
  error?: string;
}

const visible = defineModel<boolean>("visible", { default: false });

const props = defineProps<{
  connectionId: string;
  ctx: ResolveContext;
  config: FileJobConfig;
  operation: FileJobOperation;
  paths: string[];
  defaultDestination?: string;
}>();

const emit = defineEmits<{
  complete: [];
}>();

const destination = ref(props.defaultDestination ?? "");
const jobId = ref("");
const active = ref(false);
const finished = ref(false);
const failed = ref(false);
const statusText = ref("");
const percent = ref<number | null>(null);
const bytesDone = ref(0);
const bytesTotal = ref(0);
const filesDone = ref(0);
const filesTotal = ref(0);
const rateBps = ref(0);
const downloadUrl = ref("");
const lines = ref<string[]>([]);

const { status, error, send, reconnect } = useStream(
  props.connectionId,
  props.config.source,
  props.ctx,
  handleFrame,
  { keySuffix: "file-job" },
);

const operationLabel = computed(() => {
  switch (props.operation) {
    case FileJobOperation.Move:
      return "Move";
    case FileJobOperation.Copy:
      return "Copy";
    case FileJobOperation.Archive:
      return "Archive";
    case FileJobOperation.Extract:
      return "Extract";
    case FileJobOperation.Sync:
      return "Sync";
    default:
      return "File operation";
  }
});

const itemLabel = computed(
  () => `${props.paths.length} item${props.paths.length === 1 ? "" : "s"}`,
);

const headerText = computed(() => `${operationLabel.value} ${itemLabel.value}`);

const destinationLabel = computed(() => {
  switch (props.operation) {
    case FileJobOperation.Move:
      return "Move to folder";
    case FileJobOperation.Copy:
      return "Copy to folder";
    case FileJobOperation.Extract:
      return "Extract to folder";
    case FileJobOperation.Sync:
      return "Sync with folder";
    default:
      return "Destination folder";
  }
});

const destinationRequired = computed(() =>
  [
    FileJobOperation.Move,
    FileJobOperation.Copy,
    FileJobOperation.Extract,
    FileJobOperation.Sync,
  ].includes(props.operation),
);

const canStart = computed(
  () =>
    status.value === "open" &&
    props.paths.length > 0 &&
    !active.value &&
    (!destinationRequired.value || Boolean(destination.value.trim())),
);

const progressMode = computed(() =>
  percent.value == null ? "indeterminate" : "determinate",
);

const progressValue = computed(() =>
  percent.value == null ? undefined : Math.max(0, Math.min(100, percent.value)),
);

const detailText = computed(() => {
  const parts: string[] = [];
  if (filesTotal.value > 0)
    parts.push(`${filesDone.value}/${filesTotal.value} files`);
  if (bytesTotal.value > 0)
    parts.push(
      `${formatBytes(bytesDone.value)} / ${formatBytes(bytesTotal.value)}`,
    );
  else if (bytesDone.value > 0) parts.push(formatBytes(bytesDone.value));
  if (rateBps.value > 0) parts.push(`${formatBytes(rateBps.value)}/s`);
  return parts.join(" · ");
});

function reset(): void {
  jobId.value = "";
  active.value = false;
  finished.value = false;
  failed.value = false;
  statusText.value = "";
  percent.value = null;
  bytesDone.value = 0;
  bytesTotal.value = 0;
  filesDone.value = 0;
  filesTotal.value = 0;
  rateBps.value = 0;
  downloadUrl.value = "";
  lines.value = [];
}

function makeJobId(): string {
  return (
    globalThis.crypto?.randomUUID?.() ??
    `file-job-${Date.now()}-${Math.random().toString(36).slice(2)}`
  );
}

function startJob(): void {
  if (!canStart.value) return;
  reset();
  jobId.value = makeJobId();
  active.value = true;
  statusText.value = `${operationLabel.value} started`;
  send(
    JSON.stringify({
      type: "start",
      jobId: jobId.value,
      operation: props.operation,
      paths: props.paths,
      destination: destination.value.trim(),
    }),
  );
}

function cancelJob(): void {
  if (!active.value || !jobId.value) return;
  send(JSON.stringify({ type: "cancel", jobId: jobId.value }));
  statusText.value = "Cancel requested";
}

function appendLine(line: string): void {
  lines.value = [...lines.value.slice(-80), line];
}

function handleFrame(raw: string): void {
  let frame: FileJobFrame;
  try {
    frame = JSON.parse(raw) as FileJobFrame;
  } catch {
    appendLine(raw);
    return;
  }
  if (frame.jobId && jobId.value && frame.jobId !== jobId.value) return;
  if (frame.message) appendLine(frame.message);
  if (frame.path) statusText.value = frame.path;
  if (frame.status) statusText.value = frame.status;
  if (typeof frame.percent === "number") percent.value = frame.percent;
  if (typeof frame.bytesDone === "number") bytesDone.value = frame.bytesDone;
  if (typeof frame.bytesTotal === "number") bytesTotal.value = frame.bytesTotal;
  if (typeof frame.filesDone === "number") filesDone.value = frame.filesDone;
  if (typeof frame.filesTotal === "number") filesTotal.value = frame.filesTotal;
  if (typeof frame.rateBps === "number") rateBps.value = frame.rateBps;
  if (frame.downloadUrl) downloadUrl.value = frame.downloadUrl;
  if (frame.error) {
    failed.value = true;
    active.value = false;
    statusText.value = frame.error;
    appendLine(frame.error);
  }
  if (frame.type === "complete") {
    active.value = false;
    finished.value = true;
    statusText.value = frame.message || `${operationLabel.value} complete`;
    emit("complete");
  }
  if (frame.type === "error") {
    failed.value = true;
    active.value = false;
  }
}

watch(
  () => [visible.value, props.operation, props.defaultDestination],
  () => {
    if (!visible.value) return;
    destination.value = props.defaultDestination ?? "";
    reset();
  },
  { immediate: true },
);
</script>

<template>
  <Dialog
    v-model:visible="visible"
    modal
    :header="headerText"
    :style="{ width: '34rem' }"
    :breakpoints="{ '640px': '94vw' }"
  >
    <div class="space-y-4">
      <StreamStatusBar :status="status" :error="error" @reconnect="reconnect" />

      <div
        class="rounded-md border border-surface-200 p-3 text-sm dark:border-surface-800"
      >
        <div class="font-medium text-surface-900 dark:text-surface-50">
          {{ itemLabel }}
        </div>
        <div
          class="mt-2 max-h-28 space-y-1 overflow-auto text-surface-500 dark:text-surface-400"
        >
          <div v-for="path in paths" :key="path" class="truncate">
            {{ path }}
          </div>
        </div>
      </div>

      <label v-if="destinationRequired" class="block space-y-1">
        <span
          class="text-xs font-medium tracking-wide text-surface-500 uppercase dark:text-surface-400"
        >
          {{ destinationLabel }}
        </span>
        <InputText
          v-model="destination"
          class="w-full"
          aria-label="Job destination"
          :disabled="active"
        />
        <span class="block text-xs text-surface-500 dark:text-surface-400">
          Each item keeps its current name.
        </span>
      </label>

      <div v-if="active || finished || failed" class="space-y-2">
        <div class="flex items-center justify-between gap-3 text-sm">
          <span
            class="min-w-0 truncate font-medium text-surface-800 dark:text-surface-100"
          >
            {{ statusText || operationLabel }}
          </span>
          <span
            v-if="detailText"
            class="shrink-0 text-xs text-surface-500 dark:text-surface-400"
          >
            {{ detailText }}
          </span>
        </div>
        <ProgressBar
          :value="progressValue"
          :mode="progressMode"
          :show-value="false"
          aria-label="File job progress"
          style="height: 0.5rem"
        />
      </div>

      <div
        v-if="lines.length"
        class="max-h-36 overflow-auto rounded-md bg-surface-950 p-3 font-mono text-xs text-surface-100"
      >
        <div v-for="(line, index) in lines" :key="index">
          {{ line }}
        </div>
      </div>

      <div class="flex justify-end gap-2">
        <Button
          v-if="downloadUrl"
          as="a"
          severity="secondary"
          :href="downloadUrl"
          download
        >
          <AppIcon :icon="{ type: 'lucide', value: 'download' }" :size="15" />
          Download
        </Button>
        <Button
          v-if="active"
          type="button"
          severity="danger"
          outlined
          @click="cancelJob"
        >
          <AppIcon :icon="{ type: 'lucide', value: 'x' }" :size="15" />
          Cancel
        </Button>
        <Button v-else type="button" :disabled="!canStart" @click="startJob">
          <AppIcon :icon="{ type: 'lucide', value: 'play' }" :size="15" />
          {{ operationLabel }}
        </Button>
      </div>
    </div>
  </Dialog>
</template>
