import { defineStore } from "pinia";
import { useStorage } from "@vueuse/core";
import { computed, reactive, watch } from "vue";
import {
  aiApi,
  isAbort,
  streamAiTurn,
  type AiConversation,
  type AiStoredMessage,
  type AiStreamEvent,
  type AiTurnStreamEvent,
} from "../api/ai";
import { useAiProvidersStore } from "./aiProviders";
import { RiskLevel } from "../types/projection";

export type AiRunState = "idle" | "starting" | "streaming" | "stopping";

const defaultConversationTitle = "New conversation";

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
  abort: AbortController | null;
  turnId: string;
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
    abort: null,
    turnId: "",
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

export const useAiChatStore = defineStore("aiChat", () => {
  const aiProviders = useAiProvidersStore();
  const selectedProviders = useStorage<Record<string, string>>(
    "shellcn:ai:selected-provider",
    {},
  );
  const rememberedConfirmations = useStorage<Record<string, string[]>>(
    "shellcn:ai:auto-confirm-write-routes",
    {},
  );
  const byConn = reactive<Record<string, ChatState>>({});
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

  function send(connId: string, content: string): void {
    const text = content.trim();
    if (!text) return;
    const st = state(connId);
    ensureProviderSelection(connId, st);
    if (st.runState !== "idle") {
      st.queue.push(text);
      return;
    }

    st.error = null;
    st.pendingConfirm = null;
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
    st.turnId = "";
    st.abort = new AbortController();
    void runTurn(connId, text, assistant, st.abort);
  }

  async function runTurn(
    connId: string,
    content: string,
    assistant: AiMessage,
    controller: AbortController,
  ): Promise<void> {
    const st = state(connId);
    let completed = false;
    try {
      await streamAiTurn(
        connId,
        {
          content,
          providerId: st.providerId,
          conversationId: st.activeId ?? "",
        },
        {
          signal: controller.signal,
          onTurnId: (turnId) => {
            st.turnId = turnId;
          },
          onEvent: (event) => {
            if (event.type === "done") completed = true;
            if (state(connId).current?.id !== assistant.id) return;
            applyStreamEvent(connId, event);
          },
        },
      );
    } catch (err) {
      if (!isAbort(err)) {
        assistant.error =
          err instanceof Error ? err.message : "Failed to send message";
        st.error = assistant.error;
      }
    } finally {
      if (st.abort === controller) st.abort = null;
      if (st.current === assistant && (!completed || st.runState !== "idle")) {
        finalize(connId);
      }
    }
  }

  async function loadConversations(connId: string): Promise<void> {
    const st = state(connId);
    try {
      st.conversations = mergeConversations(
        st,
        await aiApi.listConversations(connId),
      );
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

  function stop(connId: string, options: { clearQueue?: boolean } = {}): void {
    const st = state(connId);
    if (options.clearQueue) st.queue = [];
    if (st.runState === "idle") return;

    st.runState = "stopping";

    if (st.turnId) {
      void aiApi.turnControl(connId, st.turnId, { type: "stop" });
    }

    st.abort?.abort();
    st.abort = null;
    finalize(connId, false);
  }

  function resolveConfirm(
    connId: string,
    approve: boolean,
    options: { remember?: boolean } = {},
  ): void {
    const st = state(connId);
    const pending = st.pendingConfirm;
    if (!pending) return;
    if (!st.turnId) {
      st.error = "Assistant turn is no longer active.";
      return;
    }
    if (approve && options.remember) {
      rememberConfirmation(connId, pending);
    }
    st.pendingConfirm = null;
    void aiApi.turnControl(connId, st.turnId, {
      type: approve ? "confirm" : "reject",
      toolId: pending.toolId,
    });
  }

  function applyStreamEvent(connId: string, ev: AiTurnStreamEvent): void {
    const st = state(connId);
    if (ev.type === "turn") {
      st.turnId = ev.turnId;
      return;
    }
    if (ev.type === "conversation") {
      if (!ev.conversationId) return;
      st.activeId = ev.conversationId;
      applyConversationTitle(st, connId, ev.conversationId, ev.title);
      void loadConversations(connId);
      return;
    }
    if (ev.type === "needs_confirmation") {
      const { type: _type, turnId, ...rest } = ev;
      void _type;
      st.turnId = turnId || st.turnId;
      if (st.turnId && shouldAutoConfirm(connId, rest)) {
        void aiApi.turnControl(connId, st.turnId, {
          type: "confirm",
          toolId: rest.toolId,
        });
        return;
      }
      st.pendingConfirm = rest;
      return;
    }
    apply(connId, ev);
  }

  function apply(connId: string, ev: AiStreamEvent): void {
    const st = state(connId);
    const cur = st.current;
    switch (ev.type) {
      case "text_delta":
        if (cur) cur.content += ev.text ?? "";
        if (cur) st.runState = "streaming";
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
        if (cur) st.runState = "streaming";
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
        finalize(connId);
        break;
      case "done":
        if (cur && ev.truncated) cur.truncated = true;
        finalize(connId);
        break;
    }
  }

  function finalize(connId: string, flushQueue = true): void {
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
    st.turnId = "";
    if (!flushQueue) return;

    const next = st.queue.shift();
    if (next) send(connId, next);
  }

  function dequeue(connId: string, index: number): void {
    state(connId).queue.splice(index, 1);
  }

  function reset(connId: string): void {
    const st = state(connId);
    st.abort?.abort();
    st.abort = null;
    st.turnId = "";
    st.messages = [];
    st.current = null;
    st.runState = "idle";
    st.error = null;
    st.pendingConfirm = null;
    st.queue = [];
    st.hasMore = false;
  }

  function applyConversationTitle(
    st: ChatState,
    connId: string,
    conversationId: string,
    title?: string,
  ): void {
    const next = title?.trim();
    if (!next) return;
    const existing = st.conversations.find((c) => c.id === conversationId);
    if (existing) {
      existing.title = next;
      existing.titleResolved = true;
      existing.updatedAt = new Date().toISOString();
      return;
    }
    const now = new Date().toISOString();
    st.conversations = [
      {
        id: conversationId,
        ownerId: "",
        connectionId: connId,
        title: next,
        titleResolved: true,
        providerId: "",
        model: "",
        createdAt: now,
        updatedAt: now,
      },
      ...st.conversations,
    ];
  }

  function mergeConversations(
    st: ChatState,
    loaded: AiConversation[],
  ): AiConversation[] {
    const local = new Map(st.conversations.map((c) => [c.id, c]));
    const seen = new Set<string>();
    const merged = loaded.map((conversation) => {
      seen.add(conversation.id);
      const cached = local.get(conversation.id);
      if (
        cached?.titleResolved &&
        cached.title &&
        isUnresolvedDefaultTitle(conversation) &&
        isCachedTitleNewer(cached, conversation)
      ) {
        return {
          ...conversation,
          title: cached.title,
          titleResolved: true,
          updatedAt: cached.updatedAt,
        };
      }
      return conversation;
    });
    for (const conversation of st.conversations) {
      if (
        conversation.titleResolved &&
        conversation.id === st.activeId &&
        !seen.has(conversation.id)
      ) {
        merged.unshift(conversation);
      }
    }
    return merged;
  }

  function isCachedTitleNewer(
    cached: AiConversation,
    conversation: AiConversation,
  ): boolean {
    if (!cached.updatedAt || !conversation.updatedAt) return true;
    return cached.updatedAt >= conversation.updatedAt;
  }

  function isUnresolvedDefaultTitle(conversation: AiConversation): boolean {
    if (conversation.title !== defaultConversationTitle) return false;
    return !conversation.titleResolved;
  }

  function canRememberConfirmation(pending: PendingConfirm): boolean {
    return !pending.destructive && pending.risk === RiskLevel.Write;
  }

  function rememberedRoutes(connId: string): string[] {
    return rememberedConfirmations.value[connId] ?? [];
  }

  function shouldAutoConfirm(connId: string, pending: PendingConfirm): boolean {
    return (
      canRememberConfirmation(pending) &&
      rememberedRoutes(connId).includes(pending.routeId)
    );
  }

  function rememberConfirmation(connId: string, pending: PendingConfirm): void {
    if (!canRememberConfirmation(pending)) return;
    const routes = new Set(rememberedRoutes(connId));
    routes.add(pending.routeId);
    rememberedConfirmations.value = {
      ...rememberedConfirmations.value,
      [connId]: [...routes].sort(),
    };
  }

  return {
    byConn,
    providers,
    global,
    providersReady,
    loadProviders,
    setProvider,
    state,
    send,
    stop,
    resolveConfirm,
    dequeue,
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
