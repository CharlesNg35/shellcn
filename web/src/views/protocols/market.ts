import type { MarketEntry } from "@/types/projection";

export function marketAction(entry: MarketEntry): string | null {
  if (!entry.compatible) return null;
  if (!entry.managed) return "Install";
  if (entry.updateAvailable) return "Update";
  return null;
}

export function marketStatus(entry: MarketEntry): {
  value: string;
  severity: "success" | "info" | "warn" | "secondary";
} {
  if (!entry.compatible) return { value: "Unavailable", severity: "secondary" };
  if (!entry.managed) return { value: "Not installed", severity: "secondary" };
  if (entry.updateAvailable)
    return { value: "Update available", severity: "warn" };
  return { value: "Installed", severity: "success" };
}

export function marketVersionLabel(value?: string): string {
  return value ? `v${value}` : "No version";
}

export function marketRepoUrl(entry: MarketEntry): string {
  return entry.homepage || `https://${entry.repo}`;
}
