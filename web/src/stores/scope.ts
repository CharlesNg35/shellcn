import { reactive } from "vue";
import { defineStore } from "pinia";

// Global, per-connection request scope. A manifest-declared header selector
// writes its chosen value here; the data layer merges these params into every
// read/stream for that connection, so all resources share one scope. The store
// only holds opaque param key/values.
export const useScopeStore = defineStore("scope", () => {
  const byConnection = reactive<Record<string, Record<string, string>>>({});

  function params(connectionId: string): Record<string, string> {
    return byConnection[connectionId] ?? {};
  }

  function set(connectionId: string, param: string, value: string): void {
    const current =
      byConnection[connectionId] ?? (byConnection[connectionId] = {});
    if (value) current[param] = value;
    else delete current[param];
  }

  function clear(connectionId: string): void {
    delete byConnection[connectionId];
  }

  return { byConnection, params, set, clear };
});
