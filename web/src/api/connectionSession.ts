import { api } from "./client";

export type ConnectionSessionState =
  | "idle"
  | "connecting"
  | "connected"
  | "closed"
  | "error";

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
