import type {
  RemoteDesktopConnectOptions,
  RemoteDesktopSession,
} from "./types";

// Protocol adapters stay server-side; the browser receives an RFB stream.
export async function connectRemoteDesktop(
  options: RemoteDesktopConnectOptions,
): Promise<RemoteDesktopSession> {
  const { connectNoVNCDesktop } = await import("./novncClient");
  return connectNoVNCDesktop(options);
}

export type { RemoteDesktopSession, RemoteDesktopStatus } from "./types";
