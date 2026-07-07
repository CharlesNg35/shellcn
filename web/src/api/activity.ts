import { api } from "./client";
import type { AuditFilters, AuditPage } from "../types/projection";

export function auditQuery(
  limit: number,
  offset: number,
  filters: AuditFilters = {},
): string {
  const params = new URLSearchParams({
    limit: String(limit),
    offset: String(offset),
  });
  for (const [key, value] of Object.entries(filters)) {
    if (value?.trim()) params.set(key, value.trim());
  }
  return params.toString();
}

// activityApi backs the self-service "My activity" view (the signed-in user's
// own audit trail).
export const activityApi = {
  mine: (limit: number, offset: number, filters?: AuditFilters) =>
    api.get<AuditPage>(`/audit/me?${auditQuery(limit, offset, filters)}`),
};
