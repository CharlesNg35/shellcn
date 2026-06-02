import { defineStore } from "pinia";
import { useStorage } from "@vueuse/core";
import { computed, reactive, ref, watch } from "vue";
import {
  aiApi,
  chatSocketUrl,
  type AiConversation,
  type AiStoredMessage,
  type AiStreamEvent,
} from "../api/ai";
import { useAiProvidersStore } from "./aiProviders";

export type AiRunState = "idle" | "starting" | "streaming" | "stopping";

export interface AiToolCall {
  id: string;
  name: string;
  status: "running" | "done" | "error";
  output?: unknown;
  err?: string;
  subagent?: string;
}

export interface AiMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  reasoning: string;
  toolCalls: AiToolCall[];
  error?: string;
  truncated?: boolean;
}

export interface PendingConfirm {
  toolId: string;
  toolName: string;
  routeId: string;
  risk: string;
  destructive: boolean;
  params: Record<string, string>;
  body: Record<string, unknown>;
}

interface ChatState {
  socket: WebSocket | null;
  connected: boolean;
  messages: AiMessage[];
  runState: AiRunState;
  current: AiMessage | null;
  error: string | null;
  providerId: string;
  conversations: AiConversation[];
  activeId: string | null;
  pendingConfirm: PendingConfirm | null;
  queue: string[];
  hasMore: boolean;
  loadingOlder: boolean;
}

function newState(): ChatState {
  return {
    socket: null,
    connected: false,
    messages: [],
    runState: "idle",
    current: null,
    error: null,
    providerId: "",
    conversations: [],
    activeId: null,
    pendingConfirm: null,
    queue: [],
    hasMore: false,
    loadingOlder: false,
  };
}

function mapStored(m: AiStoredMessage): AiMessage {
  return {
    id: m.id,
    role: m.role,
    content: m.content,
    reasoning: m.reasoning ?? "",
    truncated: m.truncated,
    toolCalls: (m.toolCalls ?? []).map((t) => ({
      id: t.id,
      name: t.name,
      status: t.err ? "error" : "done",
      output: t.output,
      err: t.err,
    })),
  };
}

let seq = 0;
const nextId = (): string => `m-${Date.now()}-${seq++}`;
const isOpenSocket = (socket: WebSocket | null): socket is WebSocket =>
  !!socket && socket.readyState === WebSocket.OPEN;

export const useAiChatStore = defineStore("aiChat", () => {
  const aiProviders = useAiProvidersStore();
  const selectedProviders = useStorage<Record<string, string>>(
    "shellcn:ai:selected-provider",
    {},
  );
  const byConn = reactive<Record<string, ChatState>>({});
  const version = ref(0);
  const providers = computed(() => aiProviders.providers);
  const global = computed(() => aiProviders.global);
  const providersReady = computed(() => aiProviders.ready);

  async function loadProviders(force = false): Promise<void> {
    try {
      await aiProviders.load(force);
      Object.entries(byConn).forEach(([connId, st]) =>
        ensureProviderSelection(connId, st),
      );
    } catch {
      return;
    }
  }

  function setProvider(connId: string, providerId: string): void {
    const st = state(connId);
    st.providerId = providerId;
    rememberProvider(connId, providerId);
  }

  function state(connId: string): ChatState {
    if (!byConn[connId]) {
      const st = newState();
      st.providerId = storedProvider(connId);
      byConn[connId] = st;
      ensureProviderSelection(connId, st);
    }
    return byConn[connId];
  }

  function sendControlFrame(
    connId: string,
    payload: Record<string, unknown>,
  ): boolean {
    const st = state(connId);
    if (!isOpenSocket(st.socket)) {
      st.error = "Assistant is not connected.";
      return false;
    }
    try {
      st.socket.send(JSON.stringify(payload));
      st.error = null;
      return true;
    } catch (err) {
      st.error = err instanceof Error ? err.message : "Failed to send command";
      return false;
    }
  }

  function storedProvider(connId: string): string {
    return Object.prototype.hasOwnProperty.call(selectedProviders.value, connId)
      ? (selectedProviders.value[connId] ?? "")
      : "";
  }

  function rememberProvider(connId: string, providerId: string): void {
    selectedProviders.value = {
      ...selectedProviders.value,
      [connId]: providerId,
    };
  }

  function ensureProviderSelection(connId: string, st: ChatState): void {
    if (!aiProviders.ready) return;
    if (
      st.providerId &&
      aiProviders.providers.some((provider) => provider.id === st.providerId)
    ) {
      rememberProvider(connId, st.providerId);
      return;
    }
    if (st.providerId === "" && aiProviders.global?.configured) {
      rememberProvider(connId, "");
      return;
    }
    if (aiProviders.global?.configured) {
      st.providerId = "";
      rememberProvider(connId, "");
      return;
    }
    st.providerId = aiProviders.providers[0]?.id ?? "";
    rememberProvider(connId, st.providerId);
  }

  watch(
    () => [
      aiProviders.global?.configured ?? false,
      aiProviders.providers.map((provider) => provider.id).join("\u0000"),
    ],
    () =>
      Object.entries(byConn).forEach(([connId, st]) =>
        ensureProviderSelection(connId, st),
      ),
  );

  async function connect(connId: string): Promise<void> {
    const st = state(connId);
    if (
      st.socket &&
      (st.connected || st.socket.readyState === WebSocket.CONNECTING)
    )
      return;
    st.error = null;
    try {
      const { ticket } = await aiApi.chatTicket(connId);
      const ws = new WebSocket(chatSocketUrl(connId, ticket));
      st.socket = ws;
      ws.addEventListener("open", () => {
        st.connected = true;
        version.value++;
        void loadConversations(connId);
      });
      ws.addEventListener("message", (ev) => {
        try {
          const frame = JSON.parse((ev as MessageEvent).data) as
            | AiStreamEvent
            | { type: "conversation"; conversationId: string; title: string }
            | ({ type: "needs_confirmation" } & PendingConfirm);
          if (frame.type === "conversation") {
            if (!frame.conversationId) return;
            st.activeId = frame.conversationId;
            void loadConversations(connId);
            return;
          }
          if (frame.type === "needs_confirmation") {
            const { type: _t, ...rest } = frame;
            void _t;
            st.pendingConfirm = rest;
            return;
          }
          apply(connId, frame);
        } catch {
          return;
        }
      });
      ws.addEventListener("close", () => {
        st.connected = false;
        st.socket = null;
        if (st.runState !== "idle") {
          st.error = "Assistant disconnected.";
          finalize(connId);
        }
      });
      ws.addEventListener("error", () => {
        st.error = "Connection error";
      });
    } catch (err) {
      st.error = err instanceof Error ? err.message : "Failed to start chat";
    }
  }

  function send(connId: string, content: string): void {
    const text = content.trim();
    if (!text) return;
    const st = state(connId);
    ensureProviderSelection(connId, st);
    if (st.runState !== "idle") {
      if (!st.connected || !isOpenSocket(st.socket)) {
        st.error = "Assistant is not connected.";
        return;
      }
      st.queue.push(text);
      return;
    }
    if (!st.connected || !isOpenSocket(st.socket)) {
      st.error = "Assistant is not connected.";
      return;
    }
    st.error = null;
    st.messages.push({
      id: nextId(),
      role: "user",
      content: text,
      reasoning: "",
      toolCalls: [],
    });
    const assistant: AiMessage = {
      id: nextId(),
      role: "assistant",
      content: "",
      reasoning: "",
      toolCalls: [],
    };
    st.messages.push(assistant);
    st.current = assistant;
    st.runState = "starting";
    try {
      st.socket.send(
        JSON.stringify({
          type: "user_message",
          content: text,
          providerId: st.providerId,
          conversationId: st.activeId ?? "",
        }),
      );
    } catch (err) {
      assistant.error =
        err instanceof Error ? err.message : "Failed to send message";
      st.error = "Failed to send message";
      st.current = null;
      st.runState = "idle";
    }
  }

  async function loadConversations(connId: string): Promise<void> {
    const st = state(connId);
    try {
      st.conversations = await aiApi.listConversations(connId);
    } catch {
      return;
    }
  }

  async function selectConversation(
    connId: string,
    cid: string,
  ): Promise<void> {
    const st = state(connId);
    if (!cid) {
      st.error = "Conversation id is required.";
      return;
    }
    if (st.runState !== "idle") return;
    try {
      const { conversation, page } = await aiApi.getConversation(connId, cid);
      st.activeId = cid;
      st.providerId = conversation.providerId ?? "";
      ensureProviderSelection(connId, st);
      st.messages = page.messages.map(mapStored);
      st.hasMore = page.hasMore;
      st.current = null;
    } catch {
      st.error = "Failed to load conversation";
    }
  }

  async function loadOlder(connId: string): Promise<void> {
    const st = state(connId);
    if (!st.activeId || !st.hasMore || st.loadingOlder) return;
    st.loadingOlder = true;
    try {
      const page = await aiApi.messages(
        connId,
        st.activeId,
        st.messages.length,
      );
      st.messages = [...page.messages.map(mapStored), ...st.messages];
      st.hasMore = page.hasMore;
    } catch {
      return;
    } finally {
      st.loadingOlder = false;
    }
  }

  function newChat(connId: string): void {
    const st = state(connId);
    if (st.runState !== "idle") return;
    st.activeId = null;
    st.messages = [];
    st.current = null;
    st.error = null;
    st.hasMore = false;
  }

  async function renameConversation(
    connId: string,
    cid: string,
    title: string,
  ): Promise<void> {
    const st = state(connId);
    const next = title.trim();
    if (!cid || !next) {
      st.error = "Conversation title is required.";
      return;
    }
    try {
      await aiApi.renameConversation(connId, cid, next);
      await loadConversations(connId);
    } catch (err) {
      st.error =
        err instanceof Error ? err.message : "Failed to rename conversation";
    }
  }

  async function deleteConversation(
    connId: string,
    cid: string,
  ): Promise<void> {
    const st = state(connId);
    if (!cid) {
      st.error = "Conversation id is required.";
      return;
    }
    try {
      await aiApi.deleteConversation(connId, cid);
      if (st.activeId === cid) newChat(connId);
      await loadConversations(connId);
    } catch (err) {
      st.error =
        err instanceof Error ? err.message : "Failed to delete conversation";
    }
  }

  function stop(connId: string): void {
    const st = state(connId);
    if (st.runState === "idle") return;
    if (sendControlFrame(connId, { type: "stop" })) {
      st.runState = "stopping";
    }
  }

  function resolveConfirm(connId: string, approve: boolean): void {
    const st = state(connId);
    const pending = st.pendingConfirm;
    if (!pending) return;
    if (
      sendControlFrame(connId, {
        type: approve ? "confirm" : "reject",
        toolId: pending.toolId,
      })
    ) {
      st.pendingConfirm = null;
    }
  }

  function apply(connId: string, ev: AiStreamEvent): void {
    const st = state(connId);
    const cur = st.current;
    switch (ev.type) {
      case "text_delta":
        if (cur) cur.content += ev.text ?? "";
        st.runState = "streaming";
        break;
      case "reasoning_delta":
        if (cur) cur.reasoning += ev.text ?? "";
        break;
      case "tool_call":
        if (cur && ev.toolId)
          cur.toolCalls.push({
            id: ev.toolId,
            name: ev.toolName ?? "tool",
            status: "running",
            subagent: ev.subagent,
          });
        st.runState = "streaming";
        break;
      case "tool_result":
        if (cur && ev.toolId) {
          const tc = cur.toolCalls.find((t) => t.id === ev.toolId);
          if (tc) {
            tc.status = ev.err ? "error" : "done";
            tc.output = ev.output;
            tc.err = ev.err;
          }
        }
        break;
      case "error":
        if (cur) cur.error = ev.err ?? "error";
        st.error = ev.err ?? "error";
        break;
      case "done":
        if (cur && ev.truncated) cur.truncated = true;
        finalize(connId);
        break;
    }
  }

  function finalize(connId: string): void {
    const st = state(connId);
    if (
      st.current &&
      !st.current.content &&
      !st.current.toolCalls.length &&
      !st.current.error
    ) {
      st.messages = st.messages.filter((m) => m.id !== st.current?.id);
    }
    st.current = null;
    st.runState = "idle";
    st.pendingConfirm = null;
    if (st.connected && isOpenSocket(st.socket)) {
      const next = st.queue.shift();
      if (next) send(connId, next);
    }
  }

  function dequeue(connId: string, index: number): void {
    state(connId).queue.splice(index, 1);
  }

  function disconnect(connId: string): void {
    const st = byConn[connId];
    if (!st) return;
    st.socket?.close();
    st.socket = null;
    st.connected = false;
  }

  function reset(connId: string): void {
    const st = state(connId);
    st.messages = [];
    st.current = null;
    st.runState = "idle";
    st.error = null;
    st.pendingConfirm = null;
    st.queue = [];
    st.hasMore = false;
  }

  return {
    byConn,
    version,
    providers,
    global,
    providersReady,
    loadProviders,
    setProvider,
    state,
    connect,
    send,
    stop,
    resolveConfirm,
    dequeue,
    disconnect,
    reset,
    apply,
    loadConversations,
    selectConversation,
    loadOlder,
    newChat,
    renameConversation,
    deleteConversation,
  };
});
