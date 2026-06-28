import {
  API_BASE,
  api,
  apiErrorFromResponse,
  ApiError,
  getCsrfToken,
  reportApiError,
} from "./client";

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
  turnControl: (
    connectionId: string,
    turnId: string,
    body: AiTurnControlRequest,
  ) =>
    api.post<void>(
      `/connections/${connectionId}/ai/turns/${turnId}/control`,
      body,
    ),

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

export interface AiTurnRequest {
  content: string;
  providerId: string;
  conversationId: string;
}

export interface AiTurnControlRequest {
  type: "stop" | "confirm" | "reject";
  toolId?: string;
}

export interface AiConversation {
  id: string;
  ownerId: string;
  connectionId: string;
  title: string;
  titleResolved: boolean;
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

// StreamEvent mirrors internal/ai/engine.StreamEvent.
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

export type AiTurnStreamEvent =
  | AiStreamEvent
  | { type: "turn"; turnId: string }
  | { type: "conversation"; conversationId: string; title?: string }
  | ({ type: "needs_confirmation"; turnId: string } & AiPendingConfirm);

export interface AiPendingConfirm {
  toolId: string;
  toolName: string;
  routeId: string;
  risk: string;
  destructive: boolean;
  params: Record<string, string>;
  body: Record<string, unknown>;
}

export interface StreamAiTurnOptions {
  signal: AbortSignal;
  onTurnId?: (turnId: string) => void;
  onEvent: (event: AiTurnStreamEvent) => void;
}

export async function streamAiTurn(
  connectionId: string,
  body: AiTurnRequest,
  options: StreamAiTurnOptions,
): Promise<void> {
  const headers = new Headers({ "Content-Type": "application/json" });
  const csrf = getCsrfToken();
  if (csrf) headers.set("X-CSRF-Token", csrf);

  let response: Response;
  try {
    response = await fetch(`${API_BASE}/connections/${connectionId}/ai/turns`, {
      method: "POST",
      headers,
      body: JSON.stringify(body),
      signal: options.signal,
    });
  } catch (err) {
    if (isAbort(err)) throw err;
    const apiErr = new ApiError(0, "Network error. Is the gateway reachable?");
    reportApiError(apiErr);
    throw apiErr;
  }

  if (!response.ok) {
    const err = apiErrorFromResponse(
      response.status,
      response.statusText,
      await response.text(),
      response.headers.get("X-ShellCN-Auth") === "required",
    );
    reportApiError(err);
    throw err;
  }

  const turnId = response.headers.get("X-ShellCN-AI-Turn-ID") ?? "";
  if (turnId) options.onTurnId?.(turnId);
  await readNDJSON(response, options.onEvent);
}

async function readNDJSON(
  response: Response,
  onEvent: (event: AiTurnStreamEvent) => void,
): Promise<void> {
  const reader = response.body?.getReader();
  if (!reader) throw new Error("Streaming response is not available.");

  const decoder = new TextDecoder();
  let buffer = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    buffer = drainLines(buffer, onEvent);
  }
  buffer += decoder.decode();
  drainLines(buffer, onEvent, true);
}

function drainLines(
  input: string,
  onEvent: (event: AiTurnStreamEvent) => void,
  final = false,
): string {
  const lines = input.split(/\r?\n/);
  const rest = final ? "" : (lines.pop() ?? "");
  for (const line of lines) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    onEvent(JSON.parse(trimmed) as AiTurnStreamEvent);
  }
  return rest;
}

export function isAbort(err: unknown): boolean {
  return (typeof DOMException !== "undefined" && err instanceof DOMException) ||
    (typeof err === "object" && err !== null && "name" in err)
    ? (err as { name?: string }).name === "AbortError"
    : false;
}
