import { computed, onMounted, onUnmounted, ref } from "vue";
import { prepareStream, type ResolveContext } from "../api/dataSource";
import { useSessionsStore, type ChannelStatus } from "../stores/sessions";
import type { DataSource } from "../types/projection";

// Wires a panel to a store-owned channel: opens (once) via the resolver, replays
// the buffer, subscribes for new frames, and detaches on unmount WITHOUT closing
// the channel — so the stream survives tab switches and remounts.
export function useStream(
  connectionId: string,
  source: DataSource | undefined,
  ctx: ResolveContext,
  onFrame?: (data: string) => void,
) {
  const store = useSessionsStore();
  const key = ref<string | null>(null);
  const error = ref<string | null>(null);
  let unsub: (() => void) | undefined;

  onMounted(async () => {
    if (!source) return;
    try {
      const handle = await prepareStream(connectionId, source, ctx);
      key.value = handle.key;
      store.ensure(handle.key, () => new WebSocket(handle.url) as never);
      if (onFrame) for (const frame of store.buffer(handle.key)) onFrame(frame);
      unsub = store.subscribe(handle.key, (d) => onFrame?.(d));
    } catch (e) {
      error.value = (e as Error).message;
    }
  });

  onUnmounted(() => unsub?.());

  const status = computed<ChannelStatus>(() =>
    key.value
      ? (store.status(key.value) ?? "connecting")
      : error.value
        ? "error"
        : "connecting",
  );

  function send(data: string): void {
    if (key.value) store.send(key.value, data);
  }

  return { status, error, send };
}
