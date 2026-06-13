export const RecordingClass = {
  Terminal: "terminal",
  Desktop: "desktop",
} as const;
export type RecordingClass =
  (typeof RecordingClass)[keyof typeof RecordingClass];

export const RecordingFormat = {
  AsciicastV2: "asciicast_v2",
  WebmCanvas: "webm_canvas",
} as const;
export type RecordingFormat =
  (typeof RecordingFormat)[keyof typeof RecordingFormat];

export const RecordingPolicy = {
  Disabled: "disabled",
  Manual: "manual",
  Auto: "auto",
} as const;
export type RecordingPolicy =
  (typeof RecordingPolicy)[keyof typeof RecordingPolicy];

export interface RecordingCapability {
  class: RecordingClass;
  formats: RecordingFormat[];
  authoritative: boolean;
  inputCapture: boolean;
}

export const RecordingStatus = {
  Pending: "pending",
  Active: "active",
  Finalized: "finalized",
  Failed: "failed",
  Discarded: "discarded",
} as const;
export type RecordingStatus =
  (typeof RecordingStatus)[keyof typeof RecordingStatus];

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
