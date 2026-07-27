import { defineStore } from "pinia";
import { ref } from "vue";
import { adminMarketApi } from "../api/admin";
import { useNotify } from "../composables/useNotify";
import type { MarketEntry } from "../types/projection/market";

// market owns the plugin marketplace: the shared catalog (fetched once and
// reused by the Protocols page and the workspace update badge), the install /
// uninstall actions, and the per-plugin update lookup. The market is admin-only,
// so callers gate the fetch; failures resolve to "disabled / no updates".
export const useMarketStore = defineStore("market", () => {
  const notify = useNotify();

  const enabled = ref(false);
  const entries = ref<MarketEntry[]>([]);
  const loading = ref(false);
  const loaded = ref(false);
  const installing = ref<Record<string, boolean>>({});
  const uninstalling = ref<Record<string, boolean>>({});
  let inflight: Promise<void> | null = null;

  async function load({ silent = false } = {}): Promise<void> {
    if (!silent) loading.value = true;
    try {
      const res = await adminMarketApi.list();
      enabled.value = res.enabled;
      entries.value = res.plugins;
      loaded.value = true;
    } catch {
      if (!silent) {
        enabled.value = false;
        entries.value = [];
      }
    } finally {
      if (!silent) loading.value = false;
    }
  }

  // ensureLoaded fetches the catalog at most once, sharing a single request
  // across concurrent callers. Used where the data is incidental (the badge).
  async function ensureLoaded(): Promise<void> {
    if (loaded.value) return;
    if (!inflight) {
      inflight = load({ silent: true }).finally(() => {
        inflight = null;
      });
    }
    await inflight;
  }

  function updateFor(name: string | undefined): MarketEntry | null {
    if (!name) return null;
    const entry = entries.value.find((p) => p.name === name);
    return entry && entry.managed && entry.updateAvailable ? entry : null;
  }

  function patchEntry(name: string, patch: Partial<MarketEntry>): void {
    entries.value = entries.value.map((entry) =>
      entry.name === name ? { ...entry, ...patch } : entry,
    );
  }

  async function install(
    entry: MarketEntry,
    onChanged?: () => Promise<void> | void,
  ): Promise<void> {
    installing.value = { ...installing.value, [entry.name]: true };
    try {
      const res = await adminMarketApi.install(entry.name);
      patchEntry(entry.name, {
        managed: true,
        installedVersion: res.version,
        updateAvailable: false,
      });
      notify.success(
        res.updated ? "Plugin updated" : "Plugin installed",
        `${entry.displayName} v${res.version}`,
      );
      await Promise.all([load({ silent: true }), onChanged?.()]);
    } catch {
      notify.error("Installation failed", entry.displayName);
    } finally {
      installing.value = { ...installing.value, [entry.name]: false };
    }
  }

  async function uninstall(
    entry: MarketEntry,
    onChanged?: () => Promise<void> | void,
  ): Promise<void> {
    uninstalling.value = { ...uninstalling.value, [entry.name]: true };
    try {
      await adminMarketApi.uninstall(entry.name);
      patchEntry(entry.name, {
        managed: false,
        installedVersion: undefined,
        updateAvailable: false,
      });
      notify.success("Plugin uninstalled", entry.displayName);
      await Promise.all([load({ silent: true }), onChanged?.()]);
    } catch {
      notify.error("Uninstall failed", entry.displayName);
    } finally {
      uninstalling.value = { ...uninstalling.value, [entry.name]: false };
    }
  }

  return {
    enabled,
    entries,
    loading,
    installing,
    uninstalling,
    load,
    ensureLoaded,
    updateFor,
    install,
    uninstall,
  };
});
