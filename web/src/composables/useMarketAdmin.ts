import { ref } from "vue";
import { adminMarketApi } from "../api/admin";
import { useNotify } from "./useNotify";
import type { MarketEntry } from "../types/projection";

// useMarketAdmin owns the plugin marketplace state and install/update actions.
export function useMarketAdmin(onChanged?: () => Promise<void> | void) {
  const notify = useNotify();

  const enabled = ref(false);
  const entries = ref<MarketEntry[]>([]);
  const loading = ref(true);
  const installing = ref<Record<string, boolean>>({});
  const uninstalling = ref<Record<string, boolean>>({});

  async function load(): Promise<void> {
    loading.value = true;
    try {
      const res = await adminMarketApi.list();
      enabled.value = res.enabled;
      entries.value = res.plugins;
    } catch {
      enabled.value = false;
      entries.value = [];
    } finally {
      loading.value = false;
    }
  }

  async function install(entry: MarketEntry): Promise<void> {
    installing.value = { ...installing.value, [entry.name]: true };
    try {
      const res = await adminMarketApi.install(entry.name);
      notify.success(
        res.updated ? "Plugin updated" : "Plugin installed",
        `${entry.displayName} v${res.version}`,
      );
      await Promise.all([load(), onChanged?.()]);
    } catch {
      notify.error("Installation failed", entry.displayName);
    } finally {
      installing.value = { ...installing.value, [entry.name]: false };
    }
  }

  async function uninstall(entry: MarketEntry): Promise<void> {
    uninstalling.value = { ...uninstalling.value, [entry.name]: true };
    try {
      await adminMarketApi.uninstall(entry.name);
      notify.success("Plugin uninstalled", entry.displayName);
      await Promise.all([load(), onChanged?.()]);
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
    install,
    uninstall,
  };
}
