import { api } from "./client";

export const ConnectionSessionState = {
  Idle: "idle",
  Connecting: "connecting",
  Connected: "connected",
  Closed: "closed",
  Error: "error",
} as const;
export type ConnectionSessionState =
  (typeof ConnectionSessionState)[keyof typeof ConnectionSessionState];

export interface ConnectionSession {
  state: ConnectionSessionState;
  reason?: string;
  channels: number;
  streams: number;
  lastSeen?: string;
  lastHealthCheck?: string;
  idleExpiresIn?: number;
}

export function keepaliveConnectionSession(
  connectionId: string,
): Promise<ConnectionSession> {
  return api.post<ConnectionSession>(
    `/connections/${encodeURIComponent(connectionId)}/session`,
  );
}

export function closeConnectionSession(connectionId: string): Promise<unknown> {
  return api.del(`/connections/${encodeURIComponent(connectionId)}/session`);
}
