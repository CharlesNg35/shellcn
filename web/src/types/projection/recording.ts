export type RecordingClass = "terminal" | "desktop";

export type RecordingFormat = "asciicast_v2" | "webm_canvas";

export type RecordingPolicy = "disabled" | "manual" | "auto";

export interface RecordingCapability {
  class: RecordingClass;
  formats: RecordingFormat[];
  authoritative: boolean;
  inputCapture: boolean;
}

export type RecordingStatus =
  | "pending"
  | "active"
  | "finalized"
  | "failed"
  | "discarded";

export interface RecordingSummary {
  id: string;
  userId: string;
  username?: string;
  connectionId: string;
  connectionName?: string;
  protocol: string;
  class: RecordingClass;
  format: RecordingFormat;
  authoritative: boolean;
  status: RecordingStatus;
  title?: string;
  startedAt: string;
  endedAt?: string;
  durationMs: number;
  size: number;
}

export interface RecordingFilters {
  user?: string;
  connection?: string;
  protocol?: string;
  class?: RecordingClass;
  status?: RecordingStatus;
}
