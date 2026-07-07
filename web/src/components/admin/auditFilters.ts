import type { AuditFilters } from "@/types/projection";

export type AuditDateRange = Date | (Date | null)[] | null;

export interface AuditFilterDraft {
  event: string;
  remoteAddr: string;
  result: string | null;
  risk: string | null;
  dateRange: AuditDateRange;
}

export const auditResultOptions = [
  { label: "Allowed", value: "allowed" },
  { label: "Denied", value: "denied" },
  { label: "Error", value: "error" },
];

export const auditRiskOptions = [
  { label: "Safe", value: "safe" },
  { label: "Write", value: "write" },
  { label: "Privileged", value: "privileged" },
  { label: "Destructive", value: "destructive" },
];

export function createAuditFilterDraft(): AuditFilterDraft {
  return {
    event: "",
    remoteAddr: "",
    result: null,
    risk: null,
    dateRange: null,
  };
}

export function activeAuditFilterCount(filters: AuditFilters): number {
  let count = 0;
  if (filters.event?.trim()) count++;
  if (filters.remoteAddr?.trim()) count++;
  if (filters.result?.trim()) count++;
  if (filters.risk?.trim()) count++;
  if (filters.since || filters.until) count++;
  return count;
}

export function resetAuditFilterDraft(draft: AuditFilterDraft): void {
  draft.event = "";
  draft.remoteAddr = "";
  draft.result = null;
  draft.risk = null;
  draft.dateRange = null;
}

export function syncAuditFilterDraft(
  draft: AuditFilterDraft,
  filters: AuditFilters,
): void {
  draft.event = filters.event ?? "";
  draft.remoteAddr = filters.remoteAddr ?? "";
  draft.result = filters.result ?? null;
  draft.risk = filters.risk ?? null;

  const since = parseDate(filters.since);
  const until = parseDate(filters.until);
  if (!since && !until) {
    draft.dateRange = null;
    return;
  }

  const start = since ?? until;
  const end = until ? new Date(until.getTime() - 1) : since;
  draft.dateRange = [start, end].filter(Boolean) as Date[];
}

export function buildAuditFilters(draft: AuditFilterDraft): AuditFilters {
  const filters: AuditFilters = {};
  const event = draft.event.trim();
  const remoteAddr = draft.remoteAddr.trim();

  if (event) filters.event = event;
  if (remoteAddr) filters.remoteAddr = remoteAddr;
  if (draft.result) filters.result = draft.result;
  if (draft.risk) filters.risk = draft.risk;

  const dates = selectedDates(draft.dateRange);
  if (dates.length > 0) {
    const start = dates[0];
    const end = dates[dates.length - 1] ?? start;
    const from = start.getTime() <= end.getTime() ? start : end;
    const to = start.getTime() <= end.getTime() ? end : start;
    filters.since = startOfDay(from).toISOString();
    filters.until = nextDayStart(to).toISOString();
  }

  return filters;
}

function parseDate(value?: string): Date | null {
  if (!value) return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function selectedDates(value: AuditDateRange): Date[] {
  if (!value) return [];
  const dates = Array.isArray(value) ? value : [value];
  return dates.filter((date): date is Date => date instanceof Date);
}

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function nextDayStart(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1);
}
