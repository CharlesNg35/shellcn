import { defineStore } from "pinia";
import { reactive, ref } from "vue";
import { STREAM_CHANNEL_BUFFER_LIMIT } from "./sessionLimits";

export const ChannelStatus = {
  Connecting: "connecting",
  Open: "open",
  Closed: "closed",
  Error: "error",
} as const;
export type ChannelStatus = (typeof ChannelStatus)[keyof typeof ChannelStatus];

export interface SocketLike {
  readyState: number;
  send(data: string): void;
  close(): void;
  addEventListener(type: string, listener: (ev: unknown) => void): void;
}

const WS_OPEN = 1;

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

export const useStreamChannelsStore = defineStore("streamChannels", () => {
  const channels = new Map<string, Channel>();
  const statuses = reactive<Record<string, ChannelStatus>>({});
  const reasons = reactive<Record<string, string>>({});
  const generation = ref(0);

  function ensure(key: string, factory: () => SocketLike): void {
    if (channels.has(key)) return;
    const socket = factory();
    const channel: Channel = { socket, listeners: new Set(), buffer: [] };
    channels.set(key, channel);
    statuses[key] = ChannelStatus.Connecting;

    socket.addEventListener("open", () => {
      statuses[key] = ChannelStatus.Open;
      delete reasons[key];
    });
    socket.addEventListener("error", () => {
      statuses[key] = ChannelStatus.Error;
      reasons[key] = "The stream connection failed.";
    });
    socket.addEventListener("close", (ev) => {
      statuses[key] = ChannelStatus.Closed;
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
    const channel = channels.get(key);
    if (channel && channel.socket.readyState === WS_OPEN)
      channel.socket.send(data);
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

  function closeForConnection(connectionId: string): void {
    closeWhere((key) => key.startsWith(`${connectionId}:`));
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
    closeForConnection,
  };
});
