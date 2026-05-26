import type { RemoteDesktopPanelConfig } from "../../../types/projection";

export type RemoteDesktopStatus =
  | "connecting"
  | "ready"
  | "disconnected"
  | "connection-lost"
  | "auth-failed"
  | "credentials-required"
  | "error";

export interface RemoteDesktopEngineHooks {
  status(status: RemoteDesktopStatus): void;
  error(message: string): void;
}

export interface RemoteDesktopEngineOptions {
  target: HTMLElement;
  url: string;
  config: Partial<RemoteDesktopPanelConfig>;
  hooks: RemoteDesktopEngineHooks;
}

export interface RemoteDesktopSession {
  disconnect(): void;
}
