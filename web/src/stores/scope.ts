import { reactive } from "vue";
import { defineStore } from "pinia";

// Wire convention that joins a multiselect scope's values into the one param
// string; mirrors the backend plugin.ScopeSeparator so handlers split the same way.
export const SCOPE_SEPARATOR = ",";

interface ScopeDefinition {
  param: string;
}

export const useScopeStore = defineStore("scope", () => {
  const byConnection = reactive<Record<string, Record<string, string>>>({});
  const allowed = reactive<Record<string, Record<string, true>>>({});

  function configure(connectionId: string, scope: ScopeDefinition[]): void {
    const next: Record<string, true> = {};
    for (const filter of scope) {
      if (filter.param) next[filter.param] = true;
    }
    allowed[connectionId] = next;
    const current = byConnection[connectionId];
    if (!current) return;
    for (const param of Object.keys(current)) {
      if (!next[param]) delete current[param];
    }
  }

  function params(connectionId: string): Record<string, string> {
    const current = byConnection[connectionId] ?? {};
    const declared = allowed[connectionId] ?? {};
    const out: Record<string, string> = {};
    for (const [param, value] of Object.entries(current)) {
      if (declared[param]) out[param] = value;
    }
    return out;
  }

  function set(connectionId: string, param: string, value: string): void {
    if (!allowed[connectionId]?.[param]) return;
    const current =
      byConnection[connectionId] ?? (byConnection[connectionId] = {});
    if (value) current[param] = value;
    else delete current[param];
  }

  function clear(connectionId: string): void {
    delete byConnection[connectionId];
    delete allowed[connectionId];
  }

  return { byConnection, configure, params, set, clear };
});
