import { describe, it, expect, beforeEach, vi } from "vitest";
import { setActivePinia, createPinia } from "pinia";

const listConversations = vi.fn(async () => [] as unknown[]);
const getConversation = vi.fn();
const messages = vi.fn();
const global = vi.fn(async () => ({ configured: false }));
const listProviders = vi.fn(async () => [] as unknown[]);
vi.mock("../api/ai", () => ({
  aiApi: {
    global: () => global(),
    list: () => listProviders(),
    listConversations: () => listConversations(),
    getConversation: (...a: unknown[]) => getConversation(...a),
    messages: (...a: unknown[]) => messages(...a),
  },
  chatSocketUrl: () => "ws://test",
}));

import { useAiChatStore } from "./aiChat";

const CONN = "c1";

beforeEach(() => {
  setActivePinia(createPinia());
  listConversations.mockClear();
  getConversation.mockReset();
  messages.mockReset();
  global.mockReset();
  global.mockResolvedValue({ configured: false });
  listProviders.mockReset();
  listProviders.mockResolvedValue([]);
});

const storedMsg = (id: string, content: string) => ({
  id,
  conversationId: "cv",
  seq: 0,
  role: "user" as const,
  content,
  createdAt: "",
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

    store.apply(CONN, { type: "text_delta", text: "Hello " });
    store.apply(CONN, { type: "text_delta", text: "world" });
    expect(st.messages[1].content).toBe("Hello world");
    expect(st.runState).toBe("streaming");
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

  it("marks an error and keeps the partial assistant message", () => {
    const store = useAiChatStore();
    store.send(CONN, "go");
    store.apply(CONN, { type: "text_delta", text: "partial" });
    store.apply(CONN, { type: "error", err: "boom" });
    store.apply(CONN, { type: "done" });
    const st = store.state(CONN);
    expect(st.messages[1].error).toBe("boom");
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
    const sent: Record<string, unknown>[] = [];
    st.socket = {
      send: (d: string) => sent.push(JSON.parse(d)),
    } as unknown as WebSocket;

    st.pendingConfirm = {
      toolId: "t1",
      toolName: "demo_delete",
      routeId: "demo.delete",
      risk: "destructive",
      destructive: true,
      params: {},
      body: {},
    };
    store.resolveConfirm(CONN, false);
    expect(st.pendingConfirm).toBeNull();
    expect(sent.at(-1)).toMatchObject({ type: "reject", toolId: "t1" });

    st.pendingConfirm = {
      toolId: "t2",
      toolName: "demo_create",
      routeId: "demo.create",
      risk: "write",
      destructive: false,
      params: {},
      body: {},
    };
    store.resolveConfirm(CONN, true);
    expect(sent.at(-1)).toMatchObject({ type: "confirm", toolId: "t2" });
  });

  it("sends the active conversation id with the message", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    const sent: string[] = [];
    st.socket = { send: (d: string) => sent.push(d) } as unknown as WebSocket;
    st.activeId = "conv-9";
    store.send(CONN, "hello");
    expect(sent).toHaveLength(1);
    expect(JSON.parse(sent[0]).conversationId).toBe("conv-9");
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
    expect(st.model).toBe("gpt-4o-mini");
    expect(st.messages.map((m) => m.content)).toEqual(["second"]);
    expect(st.hasMore).toBe(true);

    messages.mockResolvedValue({
      messages: [storedMsg("m1", "first")],
      hasMore: false,
    });
    await store.loadOlder(CONN);
    // Older page is prepended; loaded count was the current length.
    expect(messages).toHaveBeenCalledWith(CONN, "cv", 1);
    expect(st.messages.map((m) => m.content)).toEqual(["first", "second"]);
    expect(st.hasMore).toBe(false);
  });

  it("queues messages typed mid-stream and flushes on completion", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    st.socket = { send: () => {} } as unknown as WebSocket;

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
  });

  it("sends the selected provider + model", () => {
    const store = useAiChatStore();
    const st = store.state(CONN);
    const sent: Record<string, unknown>[] = [];
    st.socket = {
      send: (d: string) => sent.push(JSON.parse(d)),
    } as unknown as WebSocket;
    store.setProvider(CONN, "p1", "gpt-4o-mini");
    store.send(CONN, "hi");
    expect(sent.at(-1)).toMatchObject({
      providerId: "p1",
      model: "gpt-4o-mini",
    });
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
    const st = store.state(CONN);
    const sent: Record<string, unknown>[] = [];
    st.socket = {
      send: (d: string) => sent.push(JSON.parse(d)),
    } as unknown as WebSocket;

    await store.loadProviders();
    store.send(CONN, "hi");

    expect(sent.at(-1)).toMatchObject({
      providerId: "p-local",
      model: "openai/gpt-4o",
    });
  });
});
