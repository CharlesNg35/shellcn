import { ref } from "vue";
import { recordingsApi, type StreamRef } from "../api/recordings";

const MIME = "video/webm";
const TIMESLICE_MS = 2000;

interface CapturableCanvas extends HTMLCanvasElement {
  captureStream(frameRate?: number): MediaStream;
}

// desktopRecordingSupported reports whether the browser can capture a canvas to
// WebM. Callers deny a forced desktop recording up front when this is false.
export function desktopRecordingSupported(): boolean {
  return (
    typeof MediaRecorder !== "undefined" &&
    typeof HTMLCanvasElement !== "undefined" &&
    typeof (HTMLCanvasElement.prototype as Partial<CapturableCanvas>)
      .captureStream === "function" &&
    MediaRecorder.isTypeSupported(MIME)
  );
}

// useDesktopRecorder captures a remote-desktop canvas to WebM via MediaRecorder
// and uploads it as ordered chunks. The result is non-authoritative (browser
// capture), which the server marks accordingly.
export function useDesktopRecorder(connectionId: string, streamRef: StreamRef) {
  const recording = ref(false);
  const failed = ref(false);
  let recorder: MediaRecorder | null = null;
  let stream: MediaStream | null = null;
  let recordingID = "";
  let index = 0;
  let chain: Promise<void> = Promise.resolve();
  let uploadFailed = false;

  // Release the captured canvas stream so the browser stops the capture track;
  // MediaRecorder.stop() alone leaves the track live.
  function stopTracks(): void {
    if (stream) {
      for (const track of stream.getTracks()) track.stop();
      stream = null;
    }
  }

  async function start(canvas: CapturableCanvas, fps = 5): Promise<boolean> {
    if (recording.value) return true;
    if (!desktopRecordingSupported()) {
      failed.value = true;
      return false;
    }
    try {
      const rec = await recordingsApi.startDesktop(
        connectionId,
        streamRef,
        "webm_canvas",
      );
      recordingID = rec.id;
      index = 0;
      chain = Promise.resolve();
      uploadFailed = false;
      stream = canvas.captureStream(fps);
      recorder = new MediaRecorder(stream, { mimeType: MIME });
      recorder.ondataavailable = (e: BlobEvent) => {
        if (uploadFailed) return;
        if (e.data && e.data.size > 0) {
          const i = index++;
          chain = chain
            .then(async () => {
              await recordingsApi.uploadChunk(recordingID, i, e.data);
            })
            .catch(() => {
              uploadFailed = true;
              failed.value = true;
            });
        }
      };
      recorder.onstop = () => {
        const id = recordingID;
        chain = chain
          .then(async () => {
            if (uploadFailed) await recordingsApi.abort(id);
            else await recordingsApi.finalize(id);
          })
          .catch(async () => {
            failed.value = true;
            if (id) await recordingsApi.abort(id).catch(() => undefined);
          })
          .finally(() => {
            if (recordingID === id) recordingID = "";
          });
        stopTracks();
      };
      recorder.start(TIMESLICE_MS);
      recording.value = true;
      return true;
    } catch {
      const id = recordingID;
      recordingID = "";
      stopTracks();
      if (id) await recordingsApi.abort(id).catch(() => undefined);
      failed.value = true;
      return false;
    }
  }

  function stop(): void {
    if (recorder && recording.value) {
      recorder.stop();
      recording.value = false;
    }
  }

  return { recording, failed, start, stop };
}
