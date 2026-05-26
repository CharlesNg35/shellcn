import type { RemoteDesktopEngine } from "../../../types/projection";
import type { RemoteDesktopEngineOptions, RemoteDesktopSession } from "./types";

export async function connectRemoteDesktop(
  engine: RemoteDesktopEngine,
  options: RemoteDesktopEngineOptions,
): Promise<RemoteDesktopSession> {
  switch (engine) {
    case "novnc": {
      const { connectNoVNCDesktop } = await import("./novncEngine");
      return connectNoVNCDesktop(options);
    }
    case "guacamole": {
      const { connectGuacamoleDesktop } = await import("./guacamoleEngine");
      return connectGuacamoleDesktop(options);
    }
  }
}

export type { RemoteDesktopSession, RemoteDesktopStatus } from "./types";
