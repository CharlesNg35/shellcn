import { api } from "./client";

export type AiProviderKind =
  | "openai"
  | "openrouter"
  | "anthropic"
  | "google"
  | "openai_compatible";

// Non-secret projection of a user provider — the API never returns the key.
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

// Read-only shared-AI status (presence + provider/model, never the key).
export interface AiGlobalStatus {
  configured: boolean;
  provider?: string;
  kind?: string;
  model?: string;
}

// Write payload. apiKey is write-only; an empty string on update keeps the
// stored key.
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
  // Mint a single-use ticket for the chat WebSocket of one connection.
  chatTicket: (connectionId: string) =>
    api.post<{ ticket: string }>(`/connections/${connectionId}/ai/ticket`),

  // Conversation CRUD (owner-scoped on the server).
  listConversations: (connectionId: string) =>
    api.get<AiConversation[]>(`/connections/${connectionId}/ai/conversations`),
  createConversation: (
    connectionId: string,
    providerId?: string,
    model?: string,
  ) =>
    api.post<AiConversation>(`/connections/${connectionId}/ai/conversations`, {
      providerId: providerId ?? "",
      model: model ?? "",
    }),
  getConversation: (connectionId: string, cid: string) =>
    api.get<{ conversation: AiConversation; page: AiMessagePage }>(
      `/connections/${connectionId}/ai/conversations/${cid}`,
    ),
  messages: (connectionId: string, cid: string, loadedCount: number) =>
    api.get<AiMessagePage>(
      `/connections/${connectionId}/ai/conversations/${cid}/messages?loadedCount=${loadedCount}`,
    ),
  renameConversation: (connectionId: string, cid: string, title: string) =>
    api.put<AiConversation>(
      `/connections/${connectionId}/ai/conversations/${cid}`,
      { title },
    ),
  deleteConversation: (connectionId: string, cid: string) =>
    api.del<void>(`/connections/${connectionId}/ai/conversations/${cid}`),
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

// chatSocketUrl builds the connection's chat WebSocket URL with a redeemed ticket.
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
