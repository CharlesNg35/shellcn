import type { RemoteDesktopEngine } from "../../../types/projection";
import { connectGuacamoleDesktop } from "./guacamoleEngine";
import { connectNoVNCDesktop } from "./novncEngine";
import type { RemoteDesktopEngineOptions, RemoteDesktopSession } from "./types";

export async function connectRemoteDesktop(
  engine: RemoteDesktopEngine,
  options: RemoteDesktopEngineOptions,
): Promise<RemoteDesktopSession> {
  switch (engine) {
    case "novnc":
      return connectNoVNCDesktop(options);
    case "guacamole":
      return connectGuacamoleDesktop(options);
  }
}

export type { RemoteDesktopSession, RemoteDesktopStatus } from "./types";
