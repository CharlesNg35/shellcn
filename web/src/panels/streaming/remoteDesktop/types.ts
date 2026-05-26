import type { RemoteDesktopPanelConfig } from "../../../types/projection";

export type RemoteDesktopStatus =
  | "connecting"
  | "ready"
  | "disconnected"
  | "connection-lost"
  | "auth-failed"
  | "credentials-required"
  | "error";

export interface RemoteDesktopHooks {
  status(status: RemoteDesktopStatus): void;
  error(message: string): void;
}

export interface RemoteDesktopConnectOptions {
  target: HTMLElement;
  url: string;
  config: Partial<RemoteDesktopPanelConfig>;
  hooks: RemoteDesktopHooks;
}

export interface RemoteDesktopSession {
  disconnect(): void;
}
