import { api, API_BASE, ApiError, getCsrfToken } from "./client";
import type {
  RecordingFilters,
  RecordingFormat,
  RecordingSummary,
} from "../types/projection";

function query(f: RecordingFilters): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(f)) if (v) sp.set(k, String(v));
  const s = sp.toString();
  return s ? `?${s}` : "";
}

export interface StreamRef {
  routeId: string;
  params?: Record<string, string>;
}

async function postBinary(path: string, body: BodyInit): Promise<void> {
  const res = await fetch(API_BASE + path, {
    method: "POST",
    headers: { "X-CSRF-Token": getCsrfToken() },
    body,
  });
  if (!res.ok) throw new ApiError(res.status, res.statusText);
}

export const recordingsApi = {
  list: (f: RecordingFilters = {}) =>
    api.get<RecordingSummary[]>(`/recordings${query(f)}`),
  forConnection: (id: string, f: RecordingFilters = {}) =>
    api.get<RecordingSummary[]>(`/connections/${id}/recordings${query(f)}`),
  get: (id: string) => api.get<RecordingSummary>(`/recordings/${id}`),
  remove: (id: string) => api.del(`/recordings/${id}`),
  contentUrl: (id: string) => `${API_BASE}/recordings/${id}/content`,

  // Manual terminal recording control on a live stream.
  control: (connectionId: string, ref: StreamRef, action: "start" | "stop") =>
    api.post<RecordingSummary | { ok: true }>(
      `/connections/${connectionId}/recordings/control`,
      { ...ref, action },
    ),

  // Desktop browser-capture chunk lifecycle.
  startDesktop: (
    connectionId: string,
    ref: StreamRef,
    format: RecordingFormat,
  ) =>
    api.post<RecordingSummary>(
      `/connections/${connectionId}/recordings/desktop`,
      { ...ref, format },
    ),
  uploadChunk: (recordingId: string, index: number, chunk: Blob) =>
    postBinary(`/recordings/${recordingId}/chunks?index=${index}`, chunk),
  finalize: (recordingId: string) =>
    api.post<RecordingSummary>(`/recordings/${recordingId}/finalize`),
  abort: (recordingId: string) =>
    api.post<{ ok: true }>(`/recordings/${recordingId}/abort`),
};
