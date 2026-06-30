import { describe, it, expect, beforeEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";
import { nextTick } from "vue";
import type {
  AiGlobalStatus,
  AiTurnRequest,
  StreamAiTurnOptions,
} from "../api/ai";
import { RiskLevel } from "../types/projection";

const listConversations = vi.fn(async () => [] as unknown[]);
const getConversation = vi.fn();
const messages = vi.fn();
const renameConversation = vi.fn();
const deleteConversation = vi.fn();
const turnControl = vi.fn();
const global = vi.fn(
  async (): Promise<AiGlobalStatus> => ({
    configured: false,
  }),
);
const listProviders = vi.fn(async () => [] as unknown[]);

interface StreamCall {
  connectionId: string;
  body: AiTurnRequest;
  options: StreamAiTurnOptions;
  resolve: () => void;
  reject: (err: unknown) => void;
}

let turnSeq = 0;
const streamCalls: StreamCall[] = [];
const streamAiTurn = vi.fn(
  async (
    connectionId: string,
    body: AiTurnRequest,
    options: StreamAiTurnOptions,
  ) =>
    new Promise<void>((resolve, reject) => {
      const turnId = `turn-${++turnSeq}`;
      options.onTurnId?.(turnId);
      options.onEvent({ type: "turn", turnId });
      streamCalls.push({ connectionId, body, options, resolve, reject });
    }),
);

vi.mock("../api/ai", () => ({
  aiApi: {
    global: () => global(),
    list: () => listProviders(),
    turnControl: (...a: unknown[]) => turnControl(...a),
    listConversations: () => listConversations(),
    getConversation: (...a: unknown[]) => getConversation(...a),
    messages: (...a: unknown[]) => messages(...a),
    renameConversation: (...a: unknown[]) => renameConversation(...a),
    deleteConversation: (...a: unknown[]) => deleteConversation(...a),
  },
  streamAiTurn: (...a: Parameters<typeof streamAiTurn>) => streamAiTurn(...a),
  isAbort: (err: unknown) =>
    typeof err === "object" &&
    err !== null &&
    "name" in err &&
    (err as { name?: string }).name === "AbortError",
}));

import { useAiChatStore } from "./aiChat";
import { useWorkspaceInvalidationStore } from "./workspaceInvalidation";

const CONN = "c1";

beforeEach(() => {
  localStorage.clear();
  setActivePinia(createPinia());
  listConversations.mockClear();
  getConversation.mockReset();
  messages.mockReset();
  renameConversation.mockReset();
  deleteConversation.mockReset();
  turnControl.mockReset();
  global.mockReset();
  global.mockResolvedValue({ configured: false });
  listProviders.mockReset();
  listProviders.mockResolvedValue([]);
  streamCalls.splice(0);
  streamAiTurn.mockClear();
  turnSeq = 0;
});

const storedMsg = (id: string, content: string) => ({
  id,
  conversationId: "cv",
  seq: 0,
  role: "user" as const,
  content,
  createdAt: "",
});

const conversation = (id: string, title = "New conversation") => ({
  id,
  ownerId: "u1",
  connectionId: CONN,
  title,
  titleResolved: title !== "New conversation",
  providerId: "",
  model: "gpt-4o",
  createdAt: "",
  updatedAt: "",
});

describe("aiChat store", () => {
  it("creates user + assistant messages on send and streams text", () => {
    const store = useAiChatStore();
    store.send(CONN, "list resources");
    const st = store.state(CONN);
    expect(st.messages).toHaveLength(2);
    expect(st.messages[0]).toMatchObject({
      role: "user",
      content: "list resources",
    });
    expect(st.runState).toBe("starting");

    streamCalls[0].options.onEvent({ type: "text_delta", text: "Hello " });
    streamCalls[0].options.onEvent({ type: "text_delta", text: "world" });
    expect(st.messages[1].content).toBe("Hello world");
    expect(st.runState).toBe("streaming");
  });

  it("sends workspace context with the turn payload", () => {
    const store = useAiChatStore();
    store.send(CONN, "explain this pod", {
      query:
        "?v=detail:pod:7f1e0127-cbc5-4432-aca2-59ab0476e33c:n=kube-controller-manager-kind-control-plane,ns=kube-system",
    });

    expect(streamCalls[0].body.workspaceContext).toEqual({
      query:
        "?v=detail:pod:7f1e0127-cbc5-4432-aca2-59ab0476e33c:n=kube-controller-manager-kind-control-plane,ns=kube-system",
    });
  });

  it("tracks tool calls and their results", () => {
    const store = useAiChatStore();
    store.send(CONN, "go");
    store.apply(CONN, {
      type: "tool_call",
      toolId: "t1",
      toolName: "demo_list",
    });
    const assistant = store.state(CONN).messages[1];
    expect(assistant.toolCalls).toHaveLength(1);
    expect(assistant.toolCalls[0]).toMatchObject({
      name: "demo_list",
      status: "running",
    });

    store.apply(CONN, {
      type: "tool_result",
      toolId: "t1",
      output: { ok: true },
    });
    expect(assistant.toolCalls[0].status).toBe("done");

    store.apply(CONN, { type: "done" });
    expect(store.state(CONN).runState).toBe("idle");
  });

  it("publishes workspace invalidations from the AI stream", () => {
    const store = useAiChatStore();
    const invalidations = useWorkspaceInvalidationStore();

    store.apply(CONN, {
      type: "workspace_invalidated",
      invalidation: {
        connectionId: CONN,
        routeId: "demo.create",
        risk: RiskLevel.Write,
        params: { name: "created" },
        toolName: "demo_create",
        toolId: "t1",
      },
    });

    expect(invalidations.version(CONN)).toBe(1);
    expect(invalidations.last(CONN)).toMatchObject({
      connectionId: CONN,
      routeId: "demo.create",
      risk: RiskLevel.Write,
      source: "ai",
    });
  });

  it("marks an error and keeps the partial assistant message", () => {
    const store = useAiChatStore();
    store.send(CONN, "go");
    store.apply(CONN, { type: "text_delta", text: "partial" });
    store.apply(CONN, { type: "error", err: "boom" });
    const st = store.state(CONN);
    expect(st.messages[1].error).toBe("boom");
    expect(st.messages[1].content).toBe("partial");
    expect(st.runState).toBe("idle");
    expect(st.current).toBeNull();

    store.apply(CONN, { type: "text_delta", text: "late" });
    store.apply(CONN, { type: "done" });
    expect(st.messages[1].content).toBe("partial");
    expect(st.runState).toBe("idle");
  });

  it("drops an empty assistant message that produced nothing", () => {
    const store = useAiChatStore();
    store.send(CONN, "go");
    store.apply(CONN, { type: "done" });
    // Only the user message survives; the empty assistant bubble is pruned.
    const st = store.state(CONN);
    expect(st.messages).toHaveLength(1);
    expect(st.messages[0].role).toBe("user");
  });

  it("flags a truncated response", () => {
    const store = useAiChatStore();
    store.send(CONN, "go");
    store.apply(CONN, { type: "text_delta", text: "capped" });
    store.apply(CONN, { type: "done", truncated: true });
    expect(store.state(CONN).messages[1].truncated).toBe(true);
  });

  it("tags nested subagent tool calls", () => {
    const store = useAiChatStore();
    store.send(CONN, "investigate");
    store.apply(CONN, {
      type: "tool_call",
      toolId: "p1",
      toolName: "investigate",
    });
    store.apply(CONN, {
      type: "tool_call",
      toolId: "n1",
      toolName: "list_containers",
      subagent: "investigate",
    });
    const calls = store.state(CONN).messages[1].toolCalls;
    expect(calls).toHaveLength(2);
    expect(calls[0].subagent).toBeUndefined();
    expect(calls[1].subagent).toBe("investigate");
  });

  it("newChat clears the active conversation and messages", () => {
    const store = useAiChatStore();
    store.send(CONN, "hi");
    store.apply(CONN, { type: "text_delta", text: "yo" });
    store.apply(CONN, { type: "done" });
    store.state(CONN).activeId = "conv-1";
    store.newChat(CONN);
    const st = store.state(CONN);
    expect(st.activeId).toBeNull();
    expect(st.messages).toHaveLength(0);
  });

  it("sends the active conversation id and confirms/rejects a pending action", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    st.turnId = "turn-1";

    st.pendingConfirm = {
      toolId: "t1",
      toolName: "demo_delete",
      routeId: "demo.delete",
      risk: RiskLevel.Destructive,
      destructive: true,
      params: {},
      body: {},
    };
    store.resolveConfirm(CONN, false);
    expect(st.pendingConfirm).toBeNull();
    expect(turnControl).toHaveBeenLastCalledWith(CONN, "turn-1", {
      type: "reject",
      toolId: "t1",
    });

    st.pendingConfirm = {
      toolId: "t2",
      toolName: "demo_create",
      routeId: "demo.create",
      risk: RiskLevel.Write,
      destructive: false,
      params: {},
      body: {},
    };
    store.resolveConfirm(CONN, true);
    expect(turnControl).toHaveBeenLastCalledWith(CONN, "turn-1", {
      type: "confirm",
      toolId: "t2",
    });
  });

  it("remembers non-destructive write approvals by connection and route", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    st.turnId = "turn-1";
    st.pendingConfirm = {
      toolId: "t1",
      toolName: "demo_create",
      routeId: "demo.create",
      risk: RiskLevel.Write,
      destructive: false,
      params: {},
      body: {},
    };

    store.resolveConfirm(CONN, true, { remember: true });
    expect(turnControl).toHaveBeenLastCalledWith(CONN, "turn-1", {
      type: "confirm",
      toolId: "t1",
    });

    store.send(CONN, "create another");
    streamCalls[0].options.onEvent({
      type: "needs_confirmation",
      turnId: "turn-2",
      toolId: "t2",
      toolName: "demo_create",
      routeId: "demo.create",
      risk: RiskLevel.Write,
      destructive: false,
      params: {},
      body: {},
    });

    expect(st.pendingConfirm).toBeNull();
    expect(turnControl).toHaveBeenLastCalledWith(CONN, "turn-2", {
      type: "confirm",
      toolId: "t2",
    });
  });

  it("does not auto-confirm remembered destructive actions", () => {
    localStorage.setItem(
      "shellcn:ai:auto-confirm-write-routes",
      JSON.stringify({ [CONN]: ["demo.delete"] }),
    );
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "delete");
    streamCalls[0].options.onEvent({
      type: "needs_confirmation",
      turnId: "turn-1",
      toolId: "t1",
      toolName: "demo_delete",
      routeId: "demo.delete",
      risk: RiskLevel.Destructive,
      destructive: true,
      params: {},
      body: {},
    });

    expect(st.pendingConfirm).toMatchObject({
      routeId: "demo.delete",
      destructive: true,
    });
    expect(turnControl).not.toHaveBeenCalled();
  });

  it("sends the active conversation id with the message", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    st.activeId = "conv-9";
    store.send(CONN, "hello");
    expect(streamCalls[0].body.conversationId).toBe("conv-9");
  });

  it("applies generated conversation titles from stream events immediately", async () => {
    listConversations.mockResolvedValue([
      conversation("conv-1", "New conversation"),
    ]);
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "why did backup fail");
    streamCalls[0].options.onEvent({
      type: "conversation",
      conversationId: "conv-1",
      title: "Database Backup Failure",
    });
    await nextTick();

    expect(st.activeId).toBe("conv-1");
    expect(st.conversations.find((c) => c.id === "conv-1")?.title).toBe(
      "Database Backup Failure",
    );
  });

  it("applies generated conversation titles that arrive after done", async () => {
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "why did backup fail");
    streamCalls[0].options.onEvent({
      type: "conversation",
      conversationId: "conv-1",
    });
    streamCalls[0].options.onEvent({ type: "text_delta", text: "Check disk." });
    streamCalls[0].options.onEvent({ type: "done" });
    streamCalls[0].options.onEvent({
      type: "conversation",
      conversationId: "conv-1",
      title: "Database Backup Failure",
    });
    await nextTick();

    expect(st.runState).toBe("idle");
    expect(st.conversations.find((c) => c.id === "conv-1")?.title).toBe(
      "Database Backup Failure",
    );
  });

  it("loads a conversation page and prepends older messages", async () => {
    const store = useAiChatStore();
    getConversation.mockResolvedValue({
      conversation: {
        id: "cv",
        providerId: "p1",
        model: "gpt-4o-mini",
      },
      page: { messages: [storedMsg("m2", "second")], hasMore: true },
    });
    await store.selectConversation(CONN, "cv");
    const st = store.state(CONN);
    expect(st.providerId).toBe("p1");
    expect(st.messages.map((m) => m.content)).toEqual(["second"]);
    expect(st.hasMore).toBe(true);
    // Selecting a conversation bumps loadSeq so the list remounts (instant scroll).
    const seqAfterSelect = st.loadSeq;
    expect(seqAfterSelect).toBeGreaterThan(0);

    messages.mockResolvedValue({
      messages: [storedMsg("m1", "first")],
      hasMore: false,
    });
    await store.loadOlder(CONN);
    // Older page is prepended; loaded count was the current length.
    expect(messages).toHaveBeenCalledWith(CONN, "cv", 1);
    expect(st.messages.map((m) => m.content)).toEqual(["first", "second"]);
    expect(st.hasMore).toBe(false);
    // Loading older messages must NOT remount (would lose scroll position).
    expect(st.loadSeq).toBe(seqAfterSelect);
  });

  it("flags loadingConversation while a conversation page is in flight", async () => {
    const store = useAiChatStore();
    let resolvePage: (value: unknown) => void = () => {};
    getConversation.mockReturnValue(
      new Promise((resolve) => {
        resolvePage = resolve;
      }),
    );
    const st = store.state(CONN);
    const pending = store.selectConversation(CONN, "cv");
    expect(st.loadingConversation).toBe(true);

    resolvePage({
      conversation: { id: "cv", providerId: "p1", model: "m" },
      page: { messages: [], hasMore: false },
    });
    await pending;
    expect(st.loadingConversation).toBe(false);
  });

  it("queues messages typed mid-stream and flushes on completion", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "first"); // starts a turn
    expect(st.runState).toBe("starting");
    store.send(CONN, "second"); // queued (turn in flight)
    store.send(CONN, "third");
    expect(st.queue).toEqual(["second", "third"]);

    // Completing the turn auto-sends the next queued message.
    store.apply(CONN, { type: "text_delta", text: "ok" });
    store.apply(CONN, { type: "done" });
    expect(st.queue).toEqual(["third"]);
    expect(st.runState).toBe("starting");
    expect(streamCalls[1].body.content).toBe("second");
  });

  it("does not let stale frames from an errored turn affect the next queued turn", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "first");
    store.send(CONN, "second");

    streamCalls[0].options.onEvent({ type: "text_delta", text: "partial" });
    streamCalls[0].options.onEvent({ type: "error", err: "rate limited" });

    expect(st.runState).toBe("starting");
    expect(streamCalls).toHaveLength(2);
    expect(streamCalls[1].body.content).toBe("second");

    streamCalls[0].options.onEvent({ type: "done" });
    expect(st.runState).toBe("starting");
    expect(st.current).toBe(st.messages[3]);
    expect(st.messages[1]).toMatchObject({
      content: "partial",
      error: "rate limited",
    });
  });

  it("sends a stop control and returns to idle", () => {
    const store = useAiChatStore();

    store.send(CONN, "first");
    const st = store.state(CONN);
    store.apply(CONN, { type: "text_delta", text: "partial" });
    const controller = st.abort;

    store.stop(CONN);

    expect(st.runState).toBe("idle");
    expect(turnControl).toHaveBeenCalledWith(CONN, "turn-1", {
      type: "stop",
    });
    expect(controller?.signal.aborted).toBe(true);
    expect(st.abort).toBeNull();
    expect(st.messages[1]).toMatchObject({
      role: "assistant",
      content: "partial",
    });
  });

  it("can stop before the server turn id arrives", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    st.runState = "streaming";
    st.abort = new AbortController();
    const controller = st.abort;

    store.stop(CONN);

    expect(st.runState).toBe("idle");
    expect(turnControl).not.toHaveBeenCalled();
    expect(controller.signal.aborted).toBe(true);
    expect(st.abort).toBeNull();
  });

  it("does not flush queued messages when stopping", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);

    store.send(CONN, "first");
    store.send(CONN, "second");
    store.stop(CONN);

    expect(st.runState).toBe("idle");
    expect(st.queue).toEqual(["second"]);
    expect(streamCalls).toHaveLength(1);
  });

  it("sends only the selected provider", () => {
    const store = useAiChatStore();
    store.setProvider(CONN, "p1");
    store.send(CONN, "hi");
    expect(streamCalls.at(-1)?.body).toMatchObject({
      providerId: "p1",
    });
    expect(streamCalls.at(-1)?.body).not.toHaveProperty("model");
  });

  it("defaults to the first personal provider when no shared provider exists", async () => {
    listProviders.mockResolvedValue([
      {
        id: "p-local",
        kind: "openrouter",
        name: "OpenRouter",
        models: ["openai/gpt-4o"],
        model: "openai/gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
    ]);
    const store = useAiChatStore();

    await store.loadProviders();
    store.send(CONN, "hi");

    expect(streamCalls.at(-1)?.body).toMatchObject({
      providerId: "p-local",
    });
    expect(streamCalls.at(-1)?.body).not.toHaveProperty("model");
  });

  it("defaults to the first personal provider when the shared provider is unusable", async () => {
    global.mockResolvedValue({
      configured: true,
      usable: false,
      provider: "Shared AI",
      kind: "AI",
      model: "gpt-4o",
    });
    listProviders.mockResolvedValue([
      {
        id: "p-local",
        kind: "openrouter",
        name: "OpenRouter",
        models: ["openai/gpt-4o"],
        model: "openai/gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
    ]);
    const store = useAiChatStore();

    await store.loadProviders();
    store.send(CONN, "hi");

    expect(store.global?.configured).toBe(true);
    expect(streamCalls.at(-1)?.body).toMatchObject({
      providerId: "p-local",
    });
  });

  it("defaults a new connection to the last chosen provider", async () => {
    const make = (id: string) => ({
      id,
      kind: "openrouter" as const,
      name: id,
      models: ["m"],
      model: "m",
      hasKey: true,
      createdAt: "",
      updatedAt: "",
    });
    listProviders.mockResolvedValue([make("p1"), make("p2")]);
    const store = useAiChatStore();
    await store.loadProviders();

    store.setProvider("conn-a", "p2");
    expect(store.state("conn-b").providerId).toBe("p2");
  });

  it("can force-refresh providers after settings change", async () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    await store.loadProviders();
    expect(store.providers).toHaveLength(0);

    listProviders.mockResolvedValue([
      {
        id: "p-new",
        kind: "openrouter",
        name: "OpenRouter",
        models: ["openai/gpt-4o"],
        model: "openai/gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
    ]);
    await store.loadProviders(true);

    expect(store.providers).toHaveLength(1);
    expect(st.providerId).toBe("p-new");
  });

  it("remembers the selected provider across store instances", async () => {
    listProviders.mockResolvedValue([
      {
        id: "p1",
        kind: "openai",
        name: "OpenAI",
        models: ["gpt-4o"],
        model: "gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
      {
        id: "p2",
        kind: "openrouter",
        name: "OpenRouter",
        models: ["openai/gpt-4o"],
        model: "openai/gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
    ]);

    const store = useAiChatStore();
    await store.loadProviders();
    store.setProvider(CONN, "p2");

    setActivePinia(createPinia());
    const next = useAiChatStore();
    await next.loadProviders();

    expect(next.state(CONN).providerId).toBe("p2");
  });

  it("falls back when the remembered provider no longer exists", async () => {
    localStorage.setItem(
      "shellcn:ai:selected-provider",
      JSON.stringify({ [CONN]: "p-missing" }),
    );
    listProviders.mockResolvedValue([
      {
        id: "p-new",
        kind: "openrouter",
        name: "OpenRouter",
        models: ["openai/gpt-4o"],
        model: "openai/gpt-4o",
        hasKey: true,
        createdAt: "",
        updatedAt: "",
      },
    ]);

    const store = useAiChatStore();
    await store.loadProviders();

    expect(store.state(CONN).providerId).toBe("p-new");
    await nextTick();
    expect(
      JSON.parse(localStorage.getItem("shellcn:ai:selected-provider") ?? "{}"),
    ).toMatchObject({ [CONN]: "p-new" });
  });

  it("does not call conversation endpoints with an empty id", async () => {
    const store = useAiChatStore();
    await store.deleteConversation(CONN, "");
    await store.renameConversation(CONN, "", "Next");

    expect(deleteConversation).not.toHaveBeenCalled();
    expect(renameConversation).not.toHaveBeenCalled();
    expect(store.state(CONN).error).toBe("Conversation title is required.");
  });
});
