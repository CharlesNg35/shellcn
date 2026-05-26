import { defineStore } from "pinia";
import { reactive } from "vue";

export type ConnLiveState = "connecting" | "connected" | "error";

export interface ConnLive {
  state: ConnLiveState;
  reason?: string;
}

// The live health of connections the user has opened, derived from the actual
// traffic: WS stream status (sessions store) and HTTP data outcomes (dataSource).
// This is the source of truth for the sidebar presence dot — not the user's
// intent to connect — so a failed connection reads as failed, not green.
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

  function clear(id: string): void {
    delete live[id];
  }

  function get(id: string): ConnLive | undefined {
    return live[id];
  }

  return { live, connecting, connected, failed, clear, get };
});
