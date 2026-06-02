import { api } from "./client";

function conversationPath(connectionId: string, cid: string): string {
  if (!cid) throw new Error("Conversation id is required");
  return `/connections/${connectionId}/ai/conversations/${cid}`;
}

export type AiProviderKind =
  | "openai"
  | "openrouter"
  | "anthropic"
  | "google"
  | "openai_compatible";

export interface AiProviderSummary {
  id: string;
  kind: AiProviderKind;
  name: string;
  baseUrl?: string;
  models: string[];
  model: string;
  hasKey: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface AiGlobalStatus {
  configured: boolean;
  provider?: string;
  kind?: string;
  model?: string;
}

export interface AiProviderInput {
  kind: AiProviderKind;
  name: string;
  baseUrl?: string;
  apiKey?: string;
  models: string[];
  model: string;
}

export const aiApi = {
  global: () => api.get<AiGlobalStatus>("/ai/global"),
  list: () => api.get<AiProviderSummary[]>("/me/ai/config"),
  create: (body: AiProviderInput) =>
    api.post<AiProviderSummary>("/me/ai/config", body),
  previewModels: (body: AiProviderInput) =>
    api.post<{ models: string[] }>("/me/ai/models", body),
  update: (id: string, body: AiProviderInput) =>
    api.put<AiProviderSummary>(`/me/ai/config/${id}`, body),
  remove: (id: string) => api.del<void>(`/me/ai/config/${id}`),
  models: (id: string) =>
    api.get<{ models: string[] }>(`/me/ai/config/${id}/models`),
  testProviderDraft: (body: AiProviderInput) =>
    api.post<{ ok: boolean; error?: string }>("/me/ai/test", body),
  testProvider: (id: string) =>
    api.post<{ ok: boolean; error?: string }>(`/me/ai/config/${id}/test`),
  chatTicket: (connectionId: string) =>
    api.post<{ ticket: string }>(`/connections/${connectionId}/ai/ticket`),

  listConversations: (connectionId: string) =>
    api.get<AiConversation[]>(`/connections/${connectionId}/ai/conversations`),
  createConversation: (connectionId: string, providerId?: string) =>
    api.post<AiConversation>(`/connections/${connectionId}/ai/conversations`, {
      providerId: providerId ?? "",
    }),
  getConversation: (connectionId: string, cid: string) =>
    api.get<{ conversation: AiConversation; page: AiMessagePage }>(
      conversationPath(connectionId, cid),
    ),
  messages: (connectionId: string, cid: string, loadedCount: number) =>
    api.get<AiMessagePage>(
      `${conversationPath(connectionId, cid)}/messages?loadedCount=${loadedCount}`,
    ),
  renameConversation: (connectionId: string, cid: string, title: string) =>
    api.put<AiConversation>(conversationPath(connectionId, cid), { title }),
  deleteConversation: (connectionId: string, cid: string) =>
    api.del<void>(conversationPath(connectionId, cid)),
};

export interface AiConversation {
  id: string;
  ownerId: string;
  connectionId: string;
  title: string;
  autoTitled: boolean;
  providerId: string;
  model: string;
  createdAt: string;
  updatedAt: string;
}

export interface AiStoredToolCall {
  id: string;
  name: string;
  input?: unknown;
  output?: unknown;
  err?: string;
}

export interface AiStoredMessage {
  id: string;
  conversationId: string;
  seq: number;
  role: "user" | "assistant";
  content: string;
  toolCalls?: AiStoredToolCall[];
  reasoning?: string;
  truncated?: boolean;
  createdAt: string;
}

export interface AiMessagePage {
  messages: AiStoredMessage[];
  loadedCount: number;
  totalCount: number;
  hasMore: boolean;
}

export function chatSocketUrl(connectionId: string, ticket: string): string {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${location.host}/api/connections/${connectionId}/ai/chat?ticket=${encodeURIComponent(ticket)}`;
}

// StreamEvent mirrors internal/ai/engine.StreamEvent (the chat WS wire frame).
export interface AiStreamEvent {
  type:
    | "text_delta"
    | "reasoning_delta"
    | "tool_call"
    | "tool_result"
    | "step"
    | "error"
    | "done";
  text?: string;
  toolName?: string;
  toolId?: string;
  input?: Record<string, unknown>;
  output?: unknown;
  err?: string;
  subagent?: string;
  truncated?: boolean;
  usage?: { inputTokens: number; outputTokens: number };
}
