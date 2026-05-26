import { computed, onMounted, onUnmounted, ref } from "vue";
import {
  channelKey,
  prepareStream,
  type ResolveContext,
} from "../api/dataSource";
import { useSessionsStore, type ChannelStatus } from "../stores/sessions";
import { useConnectionStatusStore } from "../stores/connectionStatus";
import type { DataSource } from "../types/projection";

// Wires a panel to a store-owned channel. If the channel is already open (e.g.
// the user switched away and came back), it re-attaches and replays the buffer
// WITHOUT minting a new ticket or reconnecting. Otherwise it opens once via the
// resolver. On unmount it only detaches — the channel persists so the stream
// survives tab switches, pane moves, and navigating between connections.
export function useStream(
  connectionId: string,
  source: DataSource | undefined,
  ctx: ResolveContext,
  onFrame?: (data: string) => void,
) {
  const store = useSessionsStore();
  const live = useConnectionStatusStore();
  const key = ref<string | null>(null);
  const localError = ref<string | null>(null);
  // Prefer a setup failure (no ticket); otherwise surface the close reason so the
  // status bar can explain *why* the stream dropped — from the channel, and
  // falling back to the connection's last failure (the same source the sidebar
  // dot uses) so a dial/handshake failure is always explained.
  const error = computed(
    () =>
      localError.value ??
      (key.value ? store.reason(key.value) : undefined) ??
      live.get(connectionId)?.reason ??
      null,
  );
  let unsub: (() => void) | undefined;

  function attach(k: string): void {
    unsub?.();
    key.value = k;
    if (onFrame) for (const frame of store.buffer(k)) onFrame(frame);
    unsub = store.subscribe(k, (d) => onFrame?.(d));
  }

  async function connect(force = false): Promise<void> {
    if (!source) return;
    try {
      localError.value = null;
      const existing = channelKey(connectionId, source, ctx);
      if (force) {
        unsub?.();
        unsub = undefined;
        store.close(existing);
        key.value = null;
      }
      if (store.has(existing)) {
        const current = store.status(existing);
        if (current === "open" || current === "connecting") {
          attach(existing); // resume an already-open stream — no new ticket
          return;
        }
        store.close(existing);
      }
      const handle = await prepareStream(connectionId, source, ctx);
      store.ensure(handle.key, () => new WebSocket(handle.url) as never);
      attach(handle.key);
    } catch (e) {
      localError.value = (e as Error).message;
    }
  }

  onMounted(() => void connect());

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

  function reconnect(): Promise<void> {
    return connect(true);
  }

  return { status, error, send, reconnect };
}
