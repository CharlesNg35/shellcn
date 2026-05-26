<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from "vue";
import { prepareStream, resolveParams } from "../../api/dataSource";
import {
  useDesktopRecorder,
  desktopRecordingSupported,
} from "../../composables/useDesktopRecorder";
import type { RecordingDescriptor } from "../../composables/useRecordingControl";
import AppIcon from "../../components/AppIcon.vue";
import type { PanelProps } from "../core/types";
import StreamStatusBar from "./StreamStatusBar.vue";

const props = defineProps<PanelProps>();

const loaded = ref(false);
const status = ref("connecting");
const error = ref<string | null>(null);
const reconnecting = ref(false);
const container = ref<HTMLElement | null>(null);

interface RfbLike {
  scaleViewport: boolean;
  clipViewport: boolean;
  background: string;
  disconnect(): void;
  addEventListener(type: string, listener: (e: CustomEvent) => void): void;
}
let rfb: RfbLike | null = null;

const descriptor = computed(
  () => (props.config?._recording as RecordingDescriptor | undefined) ?? null,
);
const recordable = computed(
  () => descriptor.value && descriptor.value.policy !== "disabled",
);
const forced = computed(() => descriptor.value?.policy === "auto");
const unsupported = computed(
  () => Boolean(recordable.value) && !desktopRecordingSupported(),
);

const streamRef = computed(() => ({
  routeId: props.source?.routeId ?? "",
  params: resolveParams(props.source?.params, { resource: props.resource }),
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

function disconnectRemote(): void {
  recorder.stop();
  const current = rfb;
  rfb = null;
  current?.disconnect();
}

async function connectRemote(): Promise<void> {
  // A forced recording on a browser that cannot capture is denied before connect.
  error.value = null;
  if (forced.value && unsupported.value) {
    status.value = "recording-unsupported";
    return;
  }
  disconnectRemote();
  status.value = "connecting";
  loaded.value = false;
  try {
    if (!props.source || !container.value) {
      status.value = "missing-route";
      return;
    }
    const mod = await import("@novnc/novnc");
    const RFB = mod.default as new (
      target: HTMLElement,
      url: string,
      opts?: Record<string, unknown>,
    ) => RfbLike;
    const stream = await prepareStream(props.connectionId, props.source, {
      resource: props.resource,
    });
    rfb = new RFB(container.value, stream.url, {
      shared: true,
      repeaterID: props.config?.repeaterID,
    });
    const current = rfb;
    // Scale the framebuffer to fit the panel rather than clipping/scrolling.
    rfb.scaleViewport = true;
    rfb.clipViewport = false;
    rfb.background = "#000";

    // Ready is driven by the protocol handshake, not by object construction.
    rfb.addEventListener("connect", async () => {
      if (rfb !== current) return;
      status.value = "ready";
      loaded.value = true;
      if (forced.value) {
        const recordingStarted = await beginCapture();
        if (!recordingStarted) {
          status.value = "recording-failed";
          loaded.value = false;
          rfb?.disconnect();
          rfb = null;
        }
      }
    });
    rfb.addEventListener("disconnect", (e) => {
      if (rfb !== current) return;
      recorder.stop();
      loaded.value = false;
      // Preserve a more specific terminal state (auth/recording) if already set.
      if (status.value === "ready" || status.value === "connecting") {
        status.value = e.detail?.clean ? "disconnected" : "connection-lost";
      }
    });
    rfb.addEventListener("securityfailure", () => {
      if (rfb !== current) return;
      status.value = "auth-failed";
      loaded.value = false;
    });
    rfb.addEventListener("credentialsrequired", () => {
      if (rfb !== current) return;
      status.value = "credentials-required";
    });
  } catch (e) {
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
  void connectRemote();
});

onUnmounted(() => {
  disconnectRemote();
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
        class="inline-flex items-center gap-1.5 rounded-full bg-red-500/10 px-2 py-0.5 font-medium text-red-400"
        role="status"
        aria-label="Recording this desktop session"
      >
        <span
          class="h-2 w-2 animate-pulse rounded-full bg-red-500"
          aria-hidden="true"
        />
        REC
      </span>
      <button
        v-if="!forced && !recorder.recording.value"
        type="button"
        :disabled="unsupported || !loaded"
        class="inline-flex items-center gap-1.5 rounded-md border border-surface-600 px-2 py-1 text-surface-300 hover:border-red-400 hover:text-red-400 disabled:opacity-50"
        @click="beginCapture"
      >
        <span class="h-2 w-2 rounded-full bg-red-500" />
        Record
      </button>
      <button
        v-if="!forced && recorder.recording.value"
        type="button"
        class="inline-flex items-center gap-1.5 rounded-md border border-surface-600 px-2 py-1 text-surface-300 hover:bg-white/5"
        @click="recorder.stop()"
      >
        <AppIcon :icon="{ type: 'name', value: 'stop' }" :size="12" />
        Stop
      </button>
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
    <div ref="container" class="min-h-0 flex-1">
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
        class="p-4 text-sm text-red-400"
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
        class="p-4 text-sm text-red-400"
        role="alert"
      >
        Connection to the remote desktop was lost.
      </p>
      <p v-else-if="!loaded" class="p-4 text-sm text-surface-400">
        Remote desktop session is waiting for a VNC route.
      </p>
    </div>
  </div>
</template>
