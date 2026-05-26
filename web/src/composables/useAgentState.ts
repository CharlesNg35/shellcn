import { computed, onUnmounted, ref } from "vue";
import { api } from "../api/client";
import type { AgentState, AgentStatus } from "../types/projection";

// Owns polling an agent connection's tunnel state (pending → online → offline).
// Shared by the connect gate and the enroll panel so the lifecycle lives in one
// place; callers drive start()/stop() from their own lifecycle.
export function useAgentState(connectionId: string) {
  const status = ref<AgentStatus>("pending");
  const message = ref<string | undefined>();
  const online = computed(() => status.value === "online");

  let timer: ReturnType<typeof setInterval> | undefined;

  async function refresh(): Promise<void> {
    try {
      const state = await api.get<AgentState>(
        `/connections/${connectionId}/agent/state`,
      );
      status.value = state.status;
      message.value = state.message;
    } catch {
      // transient; the next tick retries
    }
  }

  function stop(): void {
    if (timer) clearInterval(timer);
    timer = undefined;
  }

  function start(intervalMs = 2000): void {
    stop();
    void refresh();
    timer = setInterval(() => void refresh(), intervalMs);
  }

  onUnmounted(stop);

  return { status, message, online, refresh, start, stop };
}
