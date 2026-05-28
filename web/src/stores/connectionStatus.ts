import { defineStore } from "pinia";
import { reactive } from "vue";
import type { ConnectionSession } from "../api/connectionSession";

export type ConnLiveState = "connecting" | "connected" | "error";

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
    if (live[id]?.state === "connected") return;
    live[id] = { state: "connecting" };
  }

  function connected(id: string): void {
    live[id] = { state: "connected" };
  }

  function failed(id: string, reason?: string): void {
    live[id] = { state: "error", reason: reason || live[id]?.reason };
  }

  function applySession(id: string, session: ConnectionSession): void {
    switch (session.state) {
      case "connected":
        connected(id);
        return;
      case "connecting":
        connecting(id);
        return;
      case "error":
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

  return { live, connecting, connected, failed, applySession, clear, get };
});
