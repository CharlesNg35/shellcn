import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { aiApi, type AiGlobalStatus, type AiProviderSummary } from "../api/ai";
import { registerSessionCleanup } from "./session";

export const useAiProvidersStore = defineStore("aiProviders", () => {
  const providers = ref<AiProviderSummary[]>([]);
  const global = ref<AiGlobalStatus | null>(null);
  const ready = ref(false);
  const loading = ref(false);
  const error = ref<string | null>(null);
  let loaded = false;
  let inFlight: Promise<void> | null = null;
  let requestSeq = 0;

  const available = computed(
    () =>
      Boolean(global.value?.configured && (global.value.usable ?? true)) ||
      providers.value.length > 0,
  );

  async function load(force = false): Promise<void> {
    if (loaded && !force) return;
    if (inFlight && !force) return inFlight;
    loading.value = true;
    error.value = null;
    const seq = ++requestSeq;
    const promise = Promise.all([aiApi.global(), aiApi.list()])
      .then(([g, list]) => {
        if (seq !== requestSeq) return;
        global.value = g;
        providers.value = list;
        ready.value = true;
        loaded = true;
      })
      .catch((err: unknown) => {
        if (seq === requestSeq) {
          ready.value = false;
          loaded = false;
          error.value =
            err instanceof Error ? err.message : "Failed to load AI settings";
        }
        throw err;
      })
      .finally(() => {
        if (seq === requestSeq) {
          loading.value = false;
          inFlight = null;
        }
      });
    inFlight = promise;
    return inFlight;
  }

  function refresh(): Promise<void> {
    return load(true);
  }

  function reset(): void {
    providers.value = [];
    global.value = null;
    ready.value = false;
    loading.value = false;
    error.value = null;
    loaded = false;
    inFlight = null;
    requestSeq++;
  }

  registerSessionCleanup("aiProviders", reset);

  return {
    providers,
    global,
    ready,
    loading,
    error,
    available,
    load,
    refresh,
    reset,
  };
});
