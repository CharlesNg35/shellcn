import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { flushPromises } from "@vue/test-utils";

const startDesktop = vi.fn<(...a: unknown[]) => Promise<{ id: string }>>(
  async () => ({ id: "rec1" }),
);
const uploadChunk = vi.fn<
  (
    id: string,
    index: number,
    chunk: Blob,
    options?: { keepalive?: boolean },
  ) => Promise<void>
>(async () => undefined);
const finalize = vi.fn<(...a: unknown[]) => Promise<{ id: string }>>(
  async () => ({
    id: "rec1",
  }),
);
const abort = vi.fn<(...a: unknown[]) => Promise<{ ok: true }>>(async () => ({
  ok: true,
}));
vi.mock("../api/recordings", () => ({
  recordingsApi: {
    startDesktop: (...a: unknown[]) => startDesktop(...a),
    uploadChunk: (
      id: string,
      index: number,
      chunk: Blob,
      options?: { keepalive?: boolean },
    ) => uploadChunk(id, index, chunk, options),
    finalize: (...a: unknown[]) => finalize(...a),
    abort: (...a: unknown[]) => abort(...a),
  },
}));

import {
  useDesktopRecorder,
  desktopRecordingSupported,
} from "./useDesktopRecorder";

const instances: FakeMediaRecorder[] = [];

class FakeMediaRecorder {
  static isTypeSupported(): boolean {
    return true;
  }
  ondataavailable: ((e: { data: Blob }) => void) | null = null;
  onstop: (() => void) | null = null;
  state = "inactive";
  constructor() {
    instances.push(this);
  }
  start(): void {
    this.state = "recording";
  }
  stop(): void {
    this.state = "inactive";
    this.onstop?.();
  }
  requestData(): void {
    this.emit(fakeBlob(1));
  }
  emit(data: Blob): void {
    this.ondataavailable?.({ data });
  }
}

beforeEach(() => {
  instances.length = 0;
  startDesktop.mockClear();
  uploadChunk.mockClear();
  finalize.mockClear();
  abort.mockClear();
  vi.stubGlobal("MediaRecorder", FakeMediaRecorder);
  (
    HTMLCanvasElement.prototype as unknown as { captureStream: () => unknown }
  ).captureStream = () => ({ getTracks: () => [] }) as unknown as MediaStream;
});

afterEach(() => vi.unstubAllGlobals());

function fakeBlob(n: number): Blob {
  return { size: n } as Blob;
}

describe("useDesktopRecorder", () => {
  it("uploads ordered chunks and finalizes on stop", async () => {
    const rec = useDesktopRecorder("c1", { routeId: "vnc.screen" });
    const canvas = document.createElement("canvas") as HTMLCanvasElement & {
      captureStream(): MediaStream;
    };

    const ok = await rec.start(canvas, 5);
    expect(ok).toBe(true);
    expect(startDesktop).toHaveBeenCalledTimes(1);
    expect(rec.recording.value).toBe(true);

    const mr = instances[0];
    mr.emit(fakeBlob(10));
    mr.emit(fakeBlob(20));
    await flushPromises();

    expect(uploadChunk).toHaveBeenCalledTimes(2);
    expect(uploadChunk.mock.calls[0][1]).toBe(0);
    expect(uploadChunk.mock.calls[1][1]).toBe(1);

    await rec.stop();
    expect(finalize).toHaveBeenCalledWith("rec1");
    expect(rec.recording.value).toBe(false);
    rec.dispose();
  });

  it("reports unsupported when MediaRecorder is unavailable", async () => {
    vi.stubGlobal("MediaRecorder", undefined);
    expect(desktopRecordingSupported()).toBe(false);
    const rec = useDesktopRecorder("c1", { routeId: "vnc.screen" });
    const canvas = document.createElement("canvas") as HTMLCanvasElement & {
      captureStream(): MediaStream;
    };
    const ok = await rec.start(canvas);
    expect(ok).toBe(false);
    expect(rec.failed.value).toBe(true);
    rec.dispose();
  });

  it("aborts instead of finalizing when a chunk upload fails", async () => {
    uploadChunk.mockRejectedValueOnce(new Error("upload failed"));
    const rec = useDesktopRecorder("c1", { routeId: "vnc.screen" });
    const canvas = document.createElement("canvas") as HTMLCanvasElement & {
      captureStream(): MediaStream;
    };

    expect(await rec.start(canvas)).toBe(true);
    instances[0].emit(fakeBlob(10));
    await flushPromises();
    await rec.stop();

    expect(finalize).not.toHaveBeenCalled();
    expect(abort).toHaveBeenCalledWith("rec1");
    expect(rec.failed.value).toBe(true);
    rec.dispose();
  });

  it("stops active recording with keepalive when the page is hidden", async () => {
    const rec = useDesktopRecorder("c1", { routeId: "vnc.screen" });
    const canvas = document.createElement("canvas") as HTMLCanvasElement & {
      captureStream(): MediaStream;
    };

    expect(await rec.start(canvas)).toBe(true);
    window.dispatchEvent(new Event("pagehide"));
    await flushPromises();

    expect(uploadChunk).toHaveBeenCalledWith("rec1", 0, expect.anything(), {
      keepalive: true,
    });
    expect(finalize).toHaveBeenCalledWith("rec1", { keepalive: true });
    expect(rec.recording.value).toBe(false);
    rec.dispose();
  });
});
