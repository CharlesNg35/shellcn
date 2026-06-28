import { defineStore } from "pinia";
import { API_BASE, getCsrfToken } from "../api/client";
import {
  closeConnectionSession,
  keepaliveConnectionSession,
  type ConnectionSession,
} from "../api/connectionSession";
import {
  CONNECTION_SESSION_HEARTBEAT_MS,
  MAX_LIVE_CONNECTION_SESSIONS,
} from "./sessionLimits";
import { cleanupConnection } from "./connectionCleanup";
import { useConnectionStatusStore } from "./connectionStatus";
import { useStreamChannelsStore } from "./streamChannels";
import { useWorkspaceStore } from "./workspace";
import { registerSessionCleanup } from "./session";

export const useConnectionSessionsStore = defineStore(
  "connectionSessions",
  () => {
    const inFlight = new Map<string, Promise<boolean>>();
    let heartbeat: ReturnType<typeof setInterval> | undefined;
    let started = false;

    const ws = useWorkspaceStore();
    const live = useConnectionStatusStore();
    const streams = useStreamChannelsStore();

    function connectedIds(): string[] {
      return ws.connectedIds();
    }

    function applyBackendSession(
      connectionId: string,
      session: ConnectionSession,
    ): boolean {
      live.applySession(connectionId, session);
      switch (session.state) {
        case "connected":
        case "connecting":
          ws.setConnected(connectionId, true);
          if (started) startHeartbeat();
          return true;
        case "error":
          return ws.isConnected(connectionId);
        default:
          ws.setConnected(connectionId, false);
          cleanupConnection(connectionId);
          if (connectedIds().length === 0) stopHeartbeat();
          return false;
      }
    }

    async function keepAlive(
      connectionId: string,
      reportError = false,
    ): Promise<boolean> {
      const existing = inFlight.get(connectionId);
      if (existing) return existing;

      const run = (async () => {
        const wasConnected = ws.isConnected(connectionId);
        live.connecting(connectionId);
        try {
          const session = await keepaliveConnectionSession(connectionId);
          return applyBackendSession(connectionId, session);
        } catch (e) {
          const message = (e as Error).message;
          live.failed(connectionId, message);
          if (!wasConnected) ws.setConnected(connectionId, false);
          if (reportError) throw e;
          return wasConnected;
        } finally {
          inFlight.delete(connectionId);
        }
      })();

      inFlight.set(connectionId, run);
      return run;
    }

    function pulse(): void {
      for (const id of connectedIds()) {
        void keepAlive(id);
      }
    }

    function startHeartbeat(): void {
      if (heartbeat || connectedIds().length === 0) return;
      heartbeat = setInterval(pulse, CONNECTION_SESSION_HEARTBEAT_MS);
    }

    function stopHeartbeat(): void {
      if (!heartbeat) return;
      clearInterval(heartbeat);
      heartbeat = undefined;
    }

    async function connect(
      connectionId: string,
      reportError = false,
    ): Promise<boolean> {
      try {
        const ok = await keepAlive(connectionId, reportError);
        if (ok) await enforceLiveLimit(connectionId);
        return ok;
      } catch {
        return false;
      }
    }

    async function closeLiveSession(connectionId: string): Promise<void> {
      streams.closeWhere((key) => key.startsWith(`${connectionId}:`));
      ws.setConnected(connectionId, false);
      live.clear(connectionId);
      cleanupConnection(connectionId);
      await closeConnectionSession(connectionId);
    }

    async function enforceLiveLimit(
      preferredConnectionId: string,
    ): Promise<void> {
      while (connectedIds().length > MAX_LIVE_CONNECTION_SESSIONS) {
        const victim = connectedIds().find(
          (id) => id !== preferredConnectionId,
        );
        if (!victim) return;
        try {
          await closeLiveSession(victim);
        } catch {
          ws.setConnected(victim, false);
          live.clear(victim);
        }
      }
      if (connectedIds().length === 0) stopHeartbeat();
    }

    async function disconnect(connectionId: string): Promise<void> {
      await closeLiveSession(connectionId);
      if (connectedIds().length === 0) stopHeartbeat();
    }

    function closeAllOnPageHide(): void {
      const headers = new Headers();
      const csrf = getCsrfToken();
      if (csrf) headers.set("X-CSRF-Token", csrf);
      for (const id of connectedIds()) {
        void fetch(
          `${API_BASE}/connections/${encodeURIComponent(id)}/session`,
          {
            method: "DELETE",
            headers,
            keepalive: true,
          },
        ).catch(() => {});
      }
    }

    function confirmBeforePageUnload(event: BeforeUnloadEvent): void {
      if (connectedIds().length === 0) return;
      event.preventDefault();
      event.returnValue = "";
    }

    function start(): void {
      if (started) return;
      started = true;
      window.addEventListener("beforeunload", confirmBeforePageUnload);
      window.addEventListener("pagehide", closeAllOnPageHide);
      startHeartbeat();
    }

    function stop(): void {
      if (!started) return;
      started = false;
      stopHeartbeat();
      window.removeEventListener("beforeunload", confirmBeforePageUnload);
      window.removeEventListener("pagehide", closeAllOnPageHide);
    }

    function reset(): void {
      stop();
      inFlight.clear();
    }

    registerSessionCleanup("connectionSessions", reset);

    return {
      connect,
      disconnect,
      connectedIds,
      keepAlive,
      start,
      stop,
      reset,
    };
  },
);
