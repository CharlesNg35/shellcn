import type { Component } from "vue";
import * as lucide from "@lucide/vue";

export const FALLBACK_ICON = "circle";

const components = lucide as unknown as Record<string, Component | undefined>;
const resolved = new Map<string, Component>();

// Any non-alphanumeric separator splits segments; numeric suffixes are kept
// (e.g. "trash-2" → "Trash2"), and an already-PascalCase name passes through.
export function toPascalCase(name: string): string {
  return name
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join("");
}

export function iconExists(name: string): boolean {
  return typeof components[toPascalCase(name)] === "function";
}

export function resolveLucideIcon(name?: string | null): Component {
  const key = (name ?? "").trim();
  const cached = resolved.get(key);
  if (cached) return cached;
  const component =
    components[toPascalCase(key)] ?? components[toPascalCase(FALLBACK_ICON)]!;
  resolved.set(key, component);
  return component;
}
