import type * as GuacamoleTypes from "guacamole-common-js";
import type { Event as GuacamoleEvent } from "guacamole-common-js";
import type { RemoteDesktopPanelConfig } from "../../../types/projection";
import type { RemoteDesktopEngineOptions, RemoteDesktopSession } from "./types";

type GuacamoleAPI = typeof GuacamoleTypes;

function resolveGuacamoleAPI(mod: unknown): GuacamoleAPI {
  const maybe = mod as { default?: GuacamoleAPI } & GuacamoleAPI;
  return maybe.default ?? maybe;
}

function statusMessage(status: { message?: string; code?: number }): string {
  if (status.message) return status.message;
  if (status.code !== undefined)
    return `Guacamole tunnel closed with code ${status.code}.`;
  return "Guacamole tunnel closed.";
}

function fitDisplay(
  client: GuacamoleTypes.Client,
  target: HTMLElement,
  config: Partial<RemoteDesktopPanelConfig>,
  connected: () => boolean,
): void {
  const display = client.getDisplay();
  const width = display.getWidth();
  const height = display.getHeight();
  const availableWidth = target.clientWidth;
  const availableHeight = target.clientHeight;

  if (width > 0 && height > 0 && availableWidth > 0 && availableHeight > 0) {
    display.scale(
      Math.min(availableWidth / width, availableHeight / height, 1),
    );
  }

  if (
    config.resize &&
    connected() &&
    availableWidth > 0 &&
    availableHeight > 0
  ) {
    client.sendSize(Math.floor(availableWidth), Math.floor(availableHeight));
  }
}

export async function connectGuacamoleDesktop({
  target,
  url,
  config,
  hooks,
}: RemoteDesktopEngineOptions): Promise<RemoteDesktopSession> {
  const Guacamole = resolveGuacamoleAPI(await import("guacamole-common-js"));
  const tunnel = new Guacamole.WebSocketTunnel(url);
  const client = new Guacamole.Client(tunnel);
  const display = client.getDisplay();
  const displayElement = display.getElement();
  let manuallyClosed = false;
  let isConnected = false;

  displayElement.tabIndex = 0;
  displayElement.classList.add("h-full", "w-full", "outline-none");
  target.replaceChildren(displayElement);

  const resizeObserver = new ResizeObserver(() => {
    fitDisplay(client, target, config, () => isConnected);
  });
  resizeObserver.observe(target);
  display.onresize = () => {
    fitDisplay(client, target, config, () => isConnected);
  };

  const mouse = new Guacamole.Mouse(displayElement);
  mouse.onEach(
    ["mousedown", "mousemove", "mouseup"],
    (event: GuacamoleEvent) => {
      displayElement.focus();
      client.sendMouseState((event as GuacamoleTypes.Mouse.Event).state, true);
    },
  );

  const touch = new Guacamole.Mouse.Touchscreen(displayElement);
  touch.onEach(
    ["mousedown", "mousemove", "mouseup"],
    (event: GuacamoleEvent) => {
      client.sendMouseState((event as GuacamoleTypes.Mouse.Event).state, true);
    },
  );

  const keyboard = new Guacamole.Keyboard(displayElement);
  keyboard.onkeydown = (keysym) => {
    client.sendKeyEvent(1, keysym);
    return false;
  };
  keyboard.onkeyup = (keysym) => {
    client.sendKeyEvent(0, keysym);
  };

  tunnel.onerror = (status) => {
    hooks.error(statusMessage(status));
  };
  client.onerror = (status) => {
    hooks.error(statusMessage(status));
    hooks.status("connection-lost");
  };
  client.onrequired = () => {
    hooks.status("credentials-required");
  };
  client.onstatechange = (state) => {
    switch (state) {
      case Guacamole.Client.State.CONNECTING:
      case Guacamole.Client.State.WAITING:
        hooks.status("connecting");
        break;
      case Guacamole.Client.State.CONNECTED:
        isConnected = true;
        fitDisplay(client, target, config, () => isConnected);
        hooks.status("ready");
        break;
      case Guacamole.Client.State.DISCONNECTED:
        isConnected = false;
        hooks.status(manuallyClosed ? "disconnected" : "connection-lost");
        break;
    }
  };

  client.connect("");

  return {
    disconnect() {
      manuallyClosed = true;
      isConnected = false;
      resizeObserver.disconnect();
      keyboard.reset();
      keyboard.onkeydown = null;
      keyboard.onkeyup = null;
      display.onresize = null;
      client.disconnect();
      target.replaceChildren();
    },
  };
}
