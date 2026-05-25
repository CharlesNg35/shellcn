import { computed, ref } from "vue";
import { recordingsApi, type StreamRef } from "../api/recordings";
import type {
  ConnectionSummary,
  PluginProjection,
  RecordingClass,
  RecordingPolicy,
} from "../types/projection";

export interface RecordingDescriptor {
  class: RecordingClass;
  policy: RecordingPolicy;
  authoritative: boolean;
}

// Resolves whether a stream is recordable and under which policy, using only the
// projection (stream kind + declared capability) and the connection's policy —
// never inferred from the panel type alone.
export function recordingForStream(
  projection: PluginProjection,
  connection: ConnectionSummary | undefined,
  routeId: string,
): RecordingDescriptor | null {
  const stream = projection.streams?.find((s) => s.routeId === routeId);
  if (!stream) return null;
  const cls: RecordingClass | null =
    stream.kind === "terminal"
      ? "terminal"
      : stream.kind === "desktop"
        ? "desktop"
        : null;
  if (!cls) return null;
  const cap = projection.recording?.find((c) => c.class === cls);
  if (!cap) return null;
  const policy =
    (connection?.recording?.[cls] as RecordingPolicy) ?? "disabled";
  return { class: cls, policy, authoritative: cap.authoritative };
}

// Manual terminal recording control. Forced (`auto`) recordings are already
// running server-side and cannot be stopped from the client.
export function useRecordingControl(
  connectionId: string,
  ref0: StreamRef,
  descriptor: RecordingDescriptor,
) {
  const forced = descriptor.policy === "auto";
  const recording = ref(forced);
  const failed = ref(false);
  const busy = ref(false);

  const canControl = computed(() => descriptor.policy === "manual");

  async function start(): Promise<void> {
    if (busy.value || recording.value) return;
    busy.value = true;
    try {
      await recordingsApi.control(connectionId, ref0, "start");
      recording.value = true;
      failed.value = false;
    } catch {
      failed.value = true;
    } finally {
      busy.value = false;
    }
  }

  async function stop(): Promise<void> {
    if (busy.value || forced || !recording.value) return;
    busy.value = true;
    try {
      await recordingsApi.control(connectionId, ref0, "stop");
      recording.value = false;
    } catch {
      failed.value = true;
    } finally {
      busy.value = false;
    }
  }

  return { recording, forced, failed, busy, canControl, start, stop };
}
