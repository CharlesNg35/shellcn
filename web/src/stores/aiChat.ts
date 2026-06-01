import { defineStore } from "pinia";
import { reactive, ref } from "vue";
import {
  aiApi,
  chatSocketUrl,
  type AiConversation,
  type AiGlobalStatus,
  type AiProviderSummary,
  type AiStreamEvent,
} from "../api/ai";

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
  current: AiMessage | null; // the in-flight assistant message
  error: string | null;
  providerId: string;
  model: string;
  conversations: AiConversation[];
  activeId: string | null;
  pendingConfirm: PendingConfirm | null;
  queue: string[];
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
    model: "",
    conversations: [],
    activeId: null,
    pendingConfirm: null,
    queue: [],
  };
}

let seq = 0;
const nextId = (): string => `m-${Date.now()}-${seq++}`;

// The chat store owns one WebSocket per connection. State is keyed by connection
// id so a chat survives the drawer closing and reopening within a workspace.
export const useAiChatStore = defineStore("aiChat", () => {
  const byConn = reactive<Record<string, ChatState>>({});
  const version = ref(0);
  // Provider catalogue is shared across connections (the user's own providers +
  // the shared/global indicator), loaded once.
  const providers = ref<AiProviderSummary[]>([]);
  const global = ref<AiGlobalStatus | null>(null);
  let providersLoaded = false;

  async function loadProviders(): Promise<void> {
    if (providersLoaded) return;
    providersLoaded = true;
    try {
      const [g, list] = await Promise.all([aiApi.global(), aiApi.list()]);
      global.value = g;
      providers.value = list;
    } catch {
      providersLoaded = false;
    }
  }

  function setProvider(connId: string, providerId: string, model = ""): void {
    const st = state(connId);
    st.providerId = providerId;
    st.model = model;
  }

  function state(connId: string): ChatState {
    if (!byConn[connId]) byConn[connId] = newState();
    return byConn[connId];
  }

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
          // ignore malformed frame
        }
      });
      ws.addEventListener("close", () => {
        st.connected = false;
        st.socket = null;
        if (st.runState !== "idle") finalize(connId);
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
    // Typed mid-stream: queue it and flush when the current turn finishes.
    if (st.runState !== "idle") {
      st.queue.push(text);
      return;
    }
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
    st.socket?.send(
      JSON.stringify({
        type: "user_message",
        content: text,
        providerId: st.providerId,
        model: st.model,
        conversationId: st.activeId ?? "",
      }),
    );
  }

  async function loadConversations(connId: string): Promise<void> {
    const st = state(connId);
    try {
      st.conversations = await aiApi.listConversations(connId);
    } catch {
      // leave existing list on failure
    }
  }

  async function selectConversation(
    connId: string,
    cid: string,
  ): Promise<void> {
    const st = state(connId);
    if (st.runState !== "idle") return;
    try {
      const { messages } = await aiApi.getConversation(connId, cid);
      st.activeId = cid;
      st.messages = messages.map((m) => ({
        id: m.id,
        role: m.role,
        content: m.content,
        reasoning: m.reasoning ?? "",
        truncated: m.truncated,
        toolCalls: (m.toolCalls ?? []).map((t) => ({
          id: t.id,
          name: t.name,
          status: t.err ? ("error" as const) : ("done" as const),
          output: t.output,
          err: t.err,
        })),
      }));
      st.current = null;
    } catch {
      st.error = "Failed to load conversation";
    }
  }

  function newChat(connId: string): void {
    const st = state(connId);
    if (st.runState !== "idle") return;
    st.activeId = null;
    st.messages = [];
    st.current = null;
    st.error = null;
  }

  async function renameConversation(
    connId: string,
    cid: string,
    title: string,
  ): Promise<void> {
    await aiApi.renameConversation(connId, cid, title);
    await loadConversations(connId);
  }

  async function deleteConversation(
    connId: string,
    cid: string,
  ): Promise<void> {
    await aiApi.deleteConversation(connId, cid);
    const st = state(connId);
    if (st.activeId === cid) newChat(connId);
    await loadConversations(connId);
  }

  function stop(connId: string): void {
    const st = state(connId);
    if (st.runState === "idle") return;
    st.runState = "stopping";
    st.socket?.send(JSON.stringify({ type: "stop" }));
  }

  function resolveConfirm(connId: string, approve: boolean): void {
    const st = state(connId);
    const pending = st.pendingConfirm;
    if (!pending) return;
    st.socket?.send(
      JSON.stringify({
        type: approve ? "confirm" : "reject",
        toolId: pending.toolId,
      }),
    );
    st.pendingConfirm = null;
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
    // Drop an assistant message that produced nothing (e.g. immediate error).
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
    // Auto-send the next queued message, if any.
    const next = st.queue.shift();
    if (next) send(connId, next);
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
  }

  return {
    byConn,
    version,
    providers,
    global,
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
    newChat,
    renameConversation,
    deleteConversation,
  };
});
