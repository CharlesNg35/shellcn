<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import Button from "primevue/button";
import { prepareStream, resolveParams } from "@/api/dataSource";
import { registerConnectionCleanup } from "@/stores/connectionCleanup";
import {
  useDesktopRecorder,
  desktopRecordingSupported,
} from "@/composables/useDesktopRecorder";
import AppIcon from "@/components/AppIcon.vue";
import PanelLoader from "@/components/PanelLoader.vue";
import {
  RecordingPolicy,
  type RemoteDesktopPanelConfig,
} from "@/types/projection";
import type { PanelProps } from "../core/types";
import {
  connectRemoteDesktop,
  type RemoteDesktopSession,
  type RemoteDesktopStatus,
} from "./remoteDesktop/connect";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const loaded = ref(false);
const status = ref("connecting");
const error = ref<string | null>(null);
const reconnecting = ref(false);
const container = ref<HTMLElement | null>(null);
const resumeRecording = ref(false);
let remoteSession: RemoteDesktopSession | null = null;
let activeRun = 0;
let unregisterConnectionCleanup: (() => void) | undefined;

const remoteConfig = computed(
  () => props.config as Partial<RemoteDesktopPanelConfig> | undefined,
);
const descriptor = computed(() => props.recording ?? null);
const recordable = computed(
  () =>
    descriptor.value && descriptor.value.policy !== RecordingPolicy.Disabled,
);
const forced = computed(
  () => descriptor.value?.policy === RecordingPolicy.Auto,
);
const unsupported = computed(
  () => Boolean(recordable.value) && !desktopRecordingSupported(),
);

const streamRef = computed(() => ({
  routeId: props.source?.routeId ?? "",
  params: resolveParams(props.source?.params, {
    resource: props.resource,
    record: props.record,
  }),
}));

const recorder = useDesktopRecorder(props.connectionId, streamRef.value);

function findCanvas():
  | (HTMLCanvasElement & { captureStream(): MediaStream })
  | null {
  return (
    (container.value?.querySelector("canvas") as
      | (HTMLCanvasElement & { captureStream(): MediaStream })
      | null) ?? null
  );
}

async function beginCapture(): Promise<boolean> {
  const canvas = findCanvas();
  if (!canvas) return false;
  return recorder.start(canvas);
}

async function startDesiredRecording(): Promise<boolean> {
  resumeRecording.value = true;
  const started = await beginCapture();
  if (!started) resumeRecording.value = false;
  return started;
}

async function stopDesiredRecording(): Promise<void> {
  resumeRecording.value = false;
  await recorder.stop();
}

function disconnectRemote(): void {
  activeRun += 1;
  void recorder.stop();
  const current = remoteSession;
  remoteSession = null;
  current?.disconnect();
}

async function handleEngineStatus(
  run: number,
  nextStatus: RemoteDesktopStatus,
): Promise<void> {
  if (run !== activeRun) return;
  if (nextStatus === "ready") {
    status.value = "ready";
    loaded.value = true;
    error.value = null;
    if (forced.value || resumeRecording.value) {
      const recordingStarted = await beginCapture();
      if (!recordingStarted && forced.value && run === activeRun) {
        status.value = "recording-failed";
        loaded.value = false;
        remoteSession?.disconnect();
        remoteSession = null;
      }
    }
    return;
  }

  if (recorder.recording.value) resumeRecording.value = true;
  void recorder.stop();
  loaded.value = false;
  if (
    nextStatus === "disconnected" &&
    status.value !== "ready" &&
    status.value !== "connecting"
  ) {
    return;
  }
  status.value = nextStatus;
}

async function connectRemote(): Promise<void> {
  // A forced recording on a browser that cannot capture is denied before connect.
  error.value = null;
  if (forced.value && unsupported.value) {
    status.value = "recording-unsupported";
    return;
  }
  disconnectRemote();
  const run = ++activeRun;
  status.value = "connecting";
  loaded.value = false;
  try {
    if (!props.source || !container.value) {
      status.value = "missing-route";
      return;
    }
    const stream = await prepareStream(props.connectionId, props.source, {
      resource: props.resource,
      record: props.record,
    });
    remoteSession = await connectRemoteDesktop({
      target: container.value,
      url: stream.url,
      config: remoteConfig.value ?? {},
      hooks: {
        status: (nextStatus) => {
          void handleEngineStatus(run, nextStatus);
        },
        error: (message) => {
          if (run !== activeRun) return;
          error.value = message;
        },
      },
    });
  } catch (e) {
    if (run !== activeRun) return;
    status.value = "error";
    error.value = (e as Error).message || "Remote desktop unavailable";
    loaded.value = false;
  }
}

async function onReconnect(): Promise<void> {
  reconnecting.value = true;
  try {
    await connectRemote();
  } finally {
    reconnecting.value = false;
  }
}

onMounted(() => {
  unregisterConnectionCleanup = registerConnectionCleanup((connectionId) => {
    if (connectionId === props.connectionId) disconnectRemote();
  });
  void connectRemote();
});

onUnmounted(() => {
  unregisterConnectionCleanup?.();
  unregisterConnectionCleanup = undefined;
  disconnectRemote();
  recorder.dispose();
  loaded.value = false;
});
</script>

<template>
  <div class="flex h-full flex-col bg-black">
    <div
      v-if="recordable"
      class="flex items-center justify-end gap-2 border-b border-white/5 px-3 py-1.5 text-xs"
    >
      <span
        v-if="recorder.recording.value"
        class="inline-flex items-center gap-1.5 rounded-full bg-rose-500/10 px-2 py-0.5 font-medium text-rose-300"
        role="status"
        aria-label="Recording this desktop session"
      >
        <span
          class="h-2 w-2 rounded-full bg-rose-400 motion-safe:animate-pulse"
          aria-hidden="true"
        />
        REC
      </span>
      <Button
        v-if="!forced && !recorder.recording.value"
        type="button"
        :disabled="unsupported || !loaded"
        size="small"
        severity="secondary"
        outlined
        @click="startDesiredRecording"
      >
        <span class="h-2 w-2 rounded-full bg-rose-400" aria-hidden="true" />
        Record
      </Button>
      <Button
        v-if="!forced && recorder.recording.value"
        type="button"
        size="small"
        severity="secondary"
        outlined
        @click="stopDesiredRecording"
      >
        <AppIcon :icon="{ type: 'lucide', value: 'square' }" :size="12" />
        Stop
      </Button>
      <span v-if="unsupported" class="text-amber-400">
        Recording unavailable in this browser
      </span>
    </div>

    <StreamStatusBar
      :status="loaded ? 'ready' : status"
      :error="error"
      :reconnecting="reconnecting"
      can-reconnect
      @reconnect="onReconnect"
    />
    <div
      ref="container"
      class="relative min-h-0 flex-1 focus-visible:outline focus-visible:-outline-offset-2 focus-visible:outline-primary-500"
      role="application"
      tabindex="0"
      aria-label="Remote desktop viewport"
    >
      <PanelLoader
        v-if="!loaded && status === 'connecting'"
        label="Connecting"
        class="absolute inset-0"
      />
      <p
        v-if="status === 'recording-unsupported'"
        class="p-4 text-sm text-amber-400"
        role="alert"
      >
        This connection requires recording, but your browser cannot capture the
        session. Connection blocked.
      </p>
      <p
        v-else-if="status === 'recording-failed'"
        class="p-4 text-sm text-amber-400"
        role="alert"
      >
        Recording could not start. Connection blocked.
      </p>
      <p
        v-else-if="status === 'auth-failed'"
        class="p-4 text-sm text-rose-300"
        role="alert"
      >
        Authentication with the remote desktop failed.
      </p>
      <p
        v-else-if="status === 'credentials-required'"
        class="p-4 text-sm text-amber-400"
        role="alert"
      >
        The remote desktop requires credentials that were not provided.
      </p>
      <p
        v-else-if="status === 'connection-lost'"
        class="p-4 text-sm text-rose-300"
        role="alert"
      >
        Connection to the remote desktop was lost.
      </p>
      <p
        v-else-if="!loaded && status !== 'connecting'"
        class="p-4 text-sm text-surface-400"
      >
        Remote desktop session is waiting for a stream route.
      </p>
    </div>
  </div>
</template>
