import {
  computed,
  onUnmounted,
  ref,
  toValue,
  type MaybeRefOrGetter,
} from "vue";
import { agentApi } from "../api/agent";
import type { AgentStatus } from "../types/projection";

// Owns polling an agent connection's tunnel state (pending → online → offline).
// Shared by the connect gate and the enroll panel so the lifecycle lives in one
// place; callers drive start()/stop() from their own lifecycle.
export function useAgentState(connectionId: MaybeRefOrGetter<string>) {
  const status = ref<AgentStatus>("pending");
  const message = ref<string | undefined>();
  const online = computed(() => status.value === "online");

  let timer: ReturnType<typeof setInterval> | undefined;
  let generation = 0;

  async function refresh(): Promise<void> {
    const id = toValue(connectionId);
    const currentGeneration = generation;
    try {
      const state = await agentApi.state(id);
      if (currentGeneration !== generation || id !== toValue(connectionId)) {
        return;
      }
      status.value = state.status;
      message.value = state.message;
    } catch {
      // transient; the next tick retries
    }
  }

  function stop(): void {
    if (timer) clearInterval(timer);
    timer = undefined;
    generation++;
  }

  function start(intervalMs = 2000): void {
    stop();
    status.value = "pending";
    message.value = undefined;
    void refresh();
    timer = setInterval(() => void refresh(), intervalMs);
  }

  onUnmounted(stop);

  return { status, message, online, refresh, start, stop };
}
