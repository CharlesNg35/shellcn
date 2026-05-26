import type { RemoteDesktopEngineOptions, RemoteDesktopSession } from "./types";

interface RfbLike {
  scaleViewport: boolean;
  clipViewport: boolean;
  resizeSession: boolean;
  background: string;
  disconnect(): void;
  addEventListener(type: string, listener: (e: CustomEvent) => void): void;
}

export async function connectNoVNCDesktop({
  target,
  url,
  config,
  hooks,
}: RemoteDesktopEngineOptions): Promise<RemoteDesktopSession> {
  const mod = await import("@novnc/novnc");
  const RFB = mod.default as new (
    target: HTMLElement,
    urlOrSocket: string | WebSocket,
    opts?: Record<string, unknown>,
  ) => RfbLike;

  const socket = new WebSocket(url);
  socket.binaryType = "arraybuffer";
  socket.addEventListener("close", (ev) => {
    if (ev.reason) hooks.error(ev.reason);
  });

  const rfb = new RFB(target, socket, {
    shared: true,
    repeaterID: config.repeaterID,
  });

  rfb.scaleViewport = true;
  rfb.clipViewport = false;
  rfb.resizeSession = config.resize ?? false;
  rfb.background = "#000";

  rfb.addEventListener("connect", () => {
    hooks.status("ready");
  });
  rfb.addEventListener("disconnect", (event) => {
    hooks.status(event.detail?.clean ? "disconnected" : "connection-lost");
  });
  rfb.addEventListener("securityfailure", (event) => {
    const reason = (event.detail as { reason?: string } | undefined)?.reason;
    hooks.error(reason || "Authentication failed.");
    hooks.status("auth-failed");
  });
  rfb.addEventListener("credentialsrequired", () => {
    hooks.status("credentials-required");
  });

  return {
    disconnect() {
      rfb.disconnect();
    },
  };
}
