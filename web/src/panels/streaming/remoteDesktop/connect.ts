import type { RemoteDesktopEngineOptions, RemoteDesktopSession } from "./types";

// Remote desktops render through noVNC. VNC streams raw RFB; RDP is bridged to
// RFB server-side, so both protocols share this single browser engine.
export async function connectRemoteDesktop(
  options: RemoteDesktopEngineOptions,
): Promise<RemoteDesktopSession> {
  const { connectNoVNCDesktop } = await import("./novncEngine");
  return connectNoVNCDesktop(options);
}

export type { RemoteDesktopSession, RemoteDesktopStatus } from "./types";
