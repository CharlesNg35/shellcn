import type { Severity } from "@/types/projection";

// Severity token -> Tailwind badge classes, shared by every value-driven status
// (tables, detail headers) so the color contract lives in one place.
const BADGE_SEVERITY: Record<Severity, string> = {
  success:
    "bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300",
  warn: "bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300",
  danger: "bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300",
  info: "bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300",
  secondary:
    "bg-surface-100 text-surface-600 dark:bg-surface-800 dark:text-surface-300",
};

// Resolves a status value to its badge classes; unmapped values (or no map)
// fall back to the neutral "secondary" style.
export function badgeClassFor(
  severities: Record<string, Severity> | undefined,
  value: unknown,
): string {
  const sev = severities?.[String(value ?? "").toLowerCase()];
  return BADGE_SEVERITY[sev ?? "secondary"];
}
