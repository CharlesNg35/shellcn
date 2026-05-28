import { defineStore } from "pinia";
import { reactive, ref } from "vue";
import { STREAM_CHANNEL_BUFFER_LIMIT } from "./sessionLimits";

export type ChannelStatus = "connecting" | "open" | "closed" | "error";

// The browser WebSocket satisfies this; tests inject a fake.
export interface SocketLike {
  send(data: string): void;
  close(): void;
  addEventListener(type: string, listener: (ev: unknown) => void): void;
}

type Listener = (data: string) => void;

interface Channel {
  socket: SocketLike;
  listeners: Set<Listener>;
  buffer: string[];
}

function frameText(ev: unknown): string {
  const data = (ev as { data?: unknown }).data;
  return typeof data === "string" ? data : String(data ?? "");
}

// Streams are owned here, not by components: a panel attaches on mount and
// detaches on unmount, but the underlying channel persists across remounts and
// tab switches. Channels are torn down explicitly when a connection closes.
// A channel key is `${connectionId}:${routeId}:${params}`; the connection id is
// everything before the first colon.
export const useStreamChannelsStore = defineStore("streamChannels", () => {
  const channels = new Map<string, Channel>();
  const statuses = reactive<Record<string, ChannelStatus>>({});
  // Per-channel failure reason (from the WS close frame), surfaced by panels.
  const reasons = reactive<Record<string, string>>({});
  const generation = ref(0);

  function ensure(key: string, factory: () => SocketLike): void {
    if (channels.has(key)) return;
    const socket = factory();
    const channel: Channel = { socket, listeners: new Set(), buffer: [] };
    channels.set(key, channel);
    statuses[key] = "connecting";

    socket.addEventListener("open", () => {
      statuses[key] = "open";
      delete reasons[key];
    });
    socket.addEventListener("error", () => {
      statuses[key] = "error";
      reasons[key] = "The stream connection failed.";
    });
    socket.addEventListener("close", (ev) => {
      statuses[key] = "closed";
      const reason =
        (ev as { reason?: string }).reason || "The connection was closed.";
      reasons[key] = reason;
    });
    socket.addEventListener("message", (ev) => {
      const text = frameText(ev);
      channel.buffer.push(text);
      if (channel.buffer.length > STREAM_CHANNEL_BUFFER_LIMIT) {
        channel.buffer.shift();
      }
      generation.value += 1;
      for (const fn of channel.listeners) fn(text);
    });
  }

  function has(key: string): boolean {
    return channels.has(key);
  }

  function subscribe(key: string, fn: Listener): () => void {
    const channel = channels.get(key);
    if (!channel) return () => {};
    channel.listeners.add(fn);
    return () => channel.listeners.delete(fn);
  }

  function send(key: string, data: string): void {
    channels.get(key)?.socket.send(data);
  }

  function buffer(key: string): string[] {
    return channels.get(key)?.buffer ?? [];
  }

  function status(key: string): ChannelStatus | undefined {
    return statuses[key];
  }

  function reason(key: string): string | undefined {
    return reasons[key];
  }

  function close(key: string): void {
    const channel = channels.get(key);
    if (!channel) return;
    channel.socket.close();
    channel.listeners.clear();
    channels.delete(key);
    delete statuses[key];
    delete reasons[key];
  }

  function closeWhere(predicate: (key: string) => boolean): void {
    for (const key of [...channels.keys()]) {
      if (predicate(key)) close(key);
    }
  }

  return {
    statuses,
    generation,
    ensure,
    has,
    subscribe,
    send,
    buffer,
    status,
    reason,
    close,
    closeWhere,
  };
});
