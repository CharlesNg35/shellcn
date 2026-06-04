import { computed, ref } from "vue";
import { adminProtocolsApi } from "../api/admin";
import { useNotify } from "./useNotify";
import type {
  ProtocolAdminItem,
  ProtocolAvailability,
} from "../types/projection";

// useProtocolsAdmin owns the admin protocol list state and availability edits.
export function useProtocolsAdmin() {
  const notify = useNotify();

  const protocols = ref<ProtocolAdminItem[]>([]);
  const pluginsDir = ref("");
  const loading = ref(true);
  const saving = ref<Record<string, boolean>>({});

  const builtIn = computed(() => protocols.value.filter((p) => !p.external));
  const external = computed(() => protocols.value.filter((p) => p.external));

  async function load(): Promise<void> {
    loading.value = true;
    try {
      const res = await adminProtocolsApi.list();
      protocols.value = res.protocols;
      pluginsDir.value = res.dir;
    } finally {
      loading.value = false;
    }
  }

  async function setAvailability(
    item: ProtocolAdminItem,
    next: ProtocolAvailability,
  ): Promise<void> {
    const previous = item.availability;
    if (next === previous) return;
    saving.value = { ...saving.value, [item.name]: true };
    item.availability = next;
    try {
      await adminProtocolsApi.setAvailability(item.name, next);
      notify.success("Protocol updated", item.title);
    } catch {
      item.availability = previous;
      notify.error("Could not update protocol", item.title);
    } finally {
      saving.value = { ...saving.value, [item.name]: false };
    }
  }

  return {
    protocols,
    pluginsDir,
    loading,
    saving,
    builtIn,
    external,
    load,
    setAvailability,
  };
}
