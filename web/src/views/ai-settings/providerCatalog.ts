import type { AiProviderKind } from "../../api/ai";

export interface ProviderPreset {
  kind: AiProviderKind;
  label: string;
  defaultName: string;
  defaultModel: string;
  models: string[];
  baseUrl?: string;
  apiKeyUrl?: string;
  requiresBaseUrl?: boolean;
  custom?: boolean;
}

export const providerPresets: ProviderPreset[] = [
  {
    kind: "openrouter",
    label: "OpenRouter",
    defaultName: "OpenRouter",
    defaultModel: "openai/gpt-4o",
    models: [
      "openai/gpt-4o",
      "anthropic/claude-sonnet-4.5",
      "google/gemini-2.5-pro",
    ],
    apiKeyUrl: "https://openrouter.ai/keys",
  },
  {
    kind: "openai",
    label: "OpenAI",
    defaultName: "OpenAI",
    defaultModel: "gpt-4o",
    models: ["gpt-4o", "gpt-4o-mini", "o3-mini"],
    apiKeyUrl: "https://platform.openai.com/api-keys",
  },
  {
    kind: "anthropic",
    label: "Anthropic",
    defaultName: "Anthropic",
    defaultModel: "claude-sonnet-4-5",
    models: ["claude-opus-4-1", "claude-sonnet-4-5", "claude-haiku-4-5"],
    apiKeyUrl: "https://console.anthropic.com/settings/keys",
  },
  {
    kind: "google",
    label: "Google",
    defaultName: "Google",
    defaultModel: "gemini-2.5-pro",
    models: ["gemini-2.5-pro", "gemini-2.5-flash"],
    apiKeyUrl: "https://aistudio.google.com/app/apikey",
  },
  {
    kind: "openai_compatible",
    label: "OpenAI-compatible",
    defaultName: "Custom provider",
    defaultModel: "",
    models: [],
    baseUrl: "http://127.0.0.1:11434/v1",
    requiresBaseUrl: true,
    custom: true,
  },
];

export function providerPreset(kind: AiProviderKind): ProviderPreset {
  return providerPresets.find((p) => p.kind === kind) ?? providerPresets[0];
}

export function providerKindLabel(kind: AiProviderKind | string): string {
  return (
    providerPresets.find((p) => p.kind === kind)?.label ??
    kind.replaceAll("_", " ")
  );
}
