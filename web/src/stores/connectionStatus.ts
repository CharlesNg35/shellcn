import { defineStore } from "pinia";
import { reactive } from "vue";
import {
  ConnectionSessionState,
  type ConnectionSession,
} from "../api/connectionSession";
import { registerSessionCleanup } from "./session";

export const ConnLiveState = {
  Connecting: "connecting",
  Connected: "connected",
  Error: "error",
} as const;
export type ConnLiveState = (typeof ConnLiveState)[keyof typeof ConnLiveState];

export interface ConnLive {
  state: ConnLiveState;
  reason?: string;
}

// The live health of connections the user has opened. The workspace keepalive is
// authoritative for the pooled backend session; HTTP data outcomes can reinforce
// it. Individual stream closes stay local to their panel.
export const useConnectionStatusStore = defineStore("connectionStatus", () => {
  const live = reactive<Record<string, ConnLive>>({});

  function connecting(id: string): void {
    // Don't downgrade an already-established connection back to "connecting".
    if (live[id]?.state === ConnLiveState.Connected) return;
    live[id] = { state: ConnLiveState.Connecting };
  }

  function connected(id: string): void {
    live[id] = { state: ConnLiveState.Connected };
  }

  function failed(id: string, reason?: string): void {
    live[id] = {
      state: ConnLiveState.Error,
      reason: reason || live[id]?.reason,
    };
  }

  function applySession(id: string, session: ConnectionSession): void {
    switch (session.state) {
      case ConnectionSessionState.Connected:
        connected(id);
        return;
      case ConnectionSessionState.Connecting:
        connecting(id);
        return;
      case ConnectionSessionState.Error:
        failed(id, session.reason);
        return;
      default:
        clear(id);
    }
  }

  function clear(id: string): void {
    delete live[id];
  }

  function get(id: string): ConnLive | undefined {
    return live[id];
  }

  function reset(): void {
    for (const id of Object.keys(live)) delete live[id];
  }

  registerSessionCleanup("connectionStatus", reset);

  return {
    live,
    connecting,
    connected,
    failed,
    applySession,
    clear,
    get,
    reset,
  };
});
