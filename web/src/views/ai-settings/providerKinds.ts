import type { AiProviderKind } from "../../api/ai";

const labels: Record<AiProviderKind, string> = {
  openrouter: "OpenRouter",
  openai: "OpenAI",
  anthropic: "Anthropic",
  google: "Google",
  openai_compatible: "OpenAI-compatible",
};

export const providerKindOptions = Object.entries(labels).map(
  ([value, label]) => ({
    label,
    value: value as AiProviderKind,
  }),
);

export function providerKindLabel(kind: AiProviderKind | string): string {
  return labels[kind as AiProviderKind] ?? kind.replaceAll("_", " ");
}

export function defaultProviderName(kind: AiProviderKind): string {
  return kind === "openai_compatible"
    ? "Custom provider"
    : providerKindLabel(kind);
}

export function requiresBaseUrl(kind: AiProviderKind): boolean {
  return kind === "openai_compatible";
}

export function requiresApiKey(kind: AiProviderKind): boolean {
  return kind !== "openai_compatible";
}
