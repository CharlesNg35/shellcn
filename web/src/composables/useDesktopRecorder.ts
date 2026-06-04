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

// Extracts image URL + hotspot from a cursor value like `url("data:...") 4 4, default`.
function parseCursorStyle(
  value: string,
): { url: string; hotX: number; hotY: number } | null {
  const m = /url\(["']?(.*?)["']?\)(?:\s+(\d+)\s+(\d+))?/.exec(value);
  if (!m) return null;
  return { url: m[1], hotX: Number(m[2] ?? 0), hotY: Number(m[3] ?? 0) };
}

// noVNC paints the cursor as a CSS overlay, never into the canvas pixels, so a
// recording only contains it if we capture a composite of canvas + cursor.
function startCursorCompositor(
  source: CapturableCanvas,
  fps: number,
): { stream: MediaStream; stop: () => void } | null {
  const composite = document.createElement("canvas") as CapturableCanvas;
  composite.width = source.width || 1;
  composite.height = source.height || 1;
  const ctx = composite.getContext("2d");
  if (!ctx) return null;

  const stream = composite.captureStream(0); // manual frames via requestFrame
  const track = (stream.getVideoTracks?.()[0] ?? stream.getTracks()[0]) as
    | (MediaStreamTrack & { requestFrame?: () => void })
    | undefined;

  let pointerX = (source.width || 0) / 2;
  let pointerY = (source.height || 0) / 2;
  let hasPointer = false;
  const onMove = (e: PointerEvent) => {
    const rect = source.getBoundingClientRect();
    if (!rect.width || !rect.height) return;
    pointerX = ((e.clientX - rect.left) / rect.width) * source.width;
    pointerY = ((e.clientY - rect.top) / rect.height) * source.height;
    hasPointer = true;
  };
  source.addEventListener("pointermove", onMove, { passive: true });

  let cursorURL = "";
  let cursorImg: HTMLImageElement | null = null;
  let hotX = 0;
  let hotY = 0;
  function refreshCursor() {
    const parsed = parseCursorStyle(getComputedStyle(source).cursor);
    if (!parsed) {
      cursorImg = null;
      cursorURL = "";
      return;
    }
    hotX = parsed.hotX;
    hotY = parsed.hotY;
    if (parsed.url !== cursorURL) {
      cursorURL = parsed.url;
      const img = new Image();
      img.onload = () => {
        if (cursorURL === parsed.url) cursorImg = img;
      };
      img.src = parsed.url;
    }
  }

  function draw() {
    if (composite.width !== (source.width || 1))
      composite.width = source.width || 1;
    if (composite.height !== (source.height || 1))
      composite.height = source.height || 1;
    ctx!.drawImage(source, 0, 0, composite.width, composite.height);
    refreshCursor();
    if (hasPointer && cursorImg) {
      ctx!.drawImage(
        cursorImg,
        Math.round(pointerX - hotX),
        Math.round(pointerY - hotY),
      );
    }
    try {
      track?.requestFrame?.();
    } catch {
      // Track ended between ticks; stop() clears the timer.
    }
  }

  draw();
  const timer = setInterval(
    draw,
    Math.max(100, Math.round(1000 / Math.max(1, fps))),
  );

  return {
    stream,
    stop() {
      clearInterval(timer);
      source.removeEventListener("pointermove", onMove);
      for (const t of stream.getTracks()) t.stop();
    },
  };
}

// useDesktopRecorder captures a remote-desktop canvas to WebM via MediaRecorder
// and uploads it as ordered chunks. The result is non-authoritative (browser
// capture), which the server marks accordingly.
export function useDesktopRecorder(connectionId: string, streamRef: StreamRef) {
  const recording = ref(false);
  const failed = ref(false);
  let recorder: MediaRecorder | null = null;
  let stream: MediaStream | null = null;
  let compositorStop: (() => void) | null = null;
  let recordingID = "";
  let index = 0;
  let chain: Promise<void> = Promise.resolve();
  let stopped: Promise<void> = Promise.resolve();
  let resolveStopped: (() => void) | null = null;
  let uploadFailed = false;
  let keepalive = false;

  // Release the captured stream so the browser stops the capture track and the
  // compositor loop; MediaRecorder.stop() alone leaves both live.
  function stopTracks(): void {
    if (compositorStop) {
      compositorStop();
      compositorStop = null;
    }
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
      stopped = new Promise((resolve) => {
        resolveStopped = resolve;
      });
      uploadFailed = false;
      keepalive = false;
      // The compositor's draw loop forces a frame per tick, so an idle desktop
      // still produces chunks.
      const compositor = startCursorCompositor(canvas, fps);
      if (compositor) {
        stream = compositor.stream;
        compositorStop = compositor.stop;
      } else {
        stream = canvas.captureStream(fps);
      }
      recorder = new MediaRecorder(stream, { mimeType: MIME });
      recorder.ondataavailable = (e: BlobEvent) => {
        if (uploadFailed) return;
        if (e.data && e.data.size > 0) {
          const i = index++;
          chain = chain
            .then(async () => {
              await recordingsApi.uploadChunk(
                recordingID,
                i,
                e.data,
                keepalive ? { keepalive: true } : undefined,
              );
            })
            .catch(() => {
              uploadFailed = true;
              failed.value = true;
            });
        }
      };
      recorder.onstop = () => {
        const id = recordingID;
        const requestOptions = keepalive ? { keepalive: true } : undefined;
        recording.value = false;
        chain = chain
          .then(async () => {
            if (uploadFailed) {
              if (requestOptions) await recordingsApi.abort(id, requestOptions);
              else await recordingsApi.abort(id);
            } else if (requestOptions) {
              await recordingsApi.finalize(id, requestOptions);
            } else {
              await recordingsApi.finalize(id);
            }
          })
          .catch(async () => {
            failed.value = true;
            if (!id) return;
            if (requestOptions) {
              await recordingsApi
                .abort(id, requestOptions)
                .catch(() => undefined);
            } else {
              await recordingsApi.abort(id).catch(() => undefined);
            }
          })
          .finally(() => {
            if (recordingID === id) recordingID = "";
            recorder = null;
            keepalive = false;
            resolveStopped?.();
            resolveStopped = null;
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

  async function stop(options: { keepalive?: boolean } = {}): Promise<void> {
    if (recorder && recording.value) {
      keepalive = options.keepalive === true;
      try {
        if (recorder.state === "recording") recorder.requestData();
      } catch {
        // Some browser implementations throw if no data is currently buffered.
      }
      recorder.stop();
      recording.value = false;
      await stopped;
      return;
    }
    await chain;
  }

  function stopForPageHide(): void {
    if (!recording.value) return;
    void stop({ keepalive: true });
  }

  if (typeof window !== "undefined") {
    window.addEventListener("pagehide", stopForPageHide);
    window.addEventListener("beforeunload", stopForPageHide);
  }

  function dispose(): void {
    if (typeof window !== "undefined") {
      window.removeEventListener("pagehide", stopForPageHide);
      window.removeEventListener("beforeunload", stopForPageHide);
    }
    void stop();
  }

  return { recording, failed, start, stop, dispose };
}
