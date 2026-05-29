import { api } from "./client";
import type { AuditPage } from "../types/projection";

// activityApi backs the self-service "My activity" view (the signed-in user's
// own audit trail).
export const activityApi = {
  mine: (limit: number, offset: number) =>
    api.get<AuditPage>(`/audit/me?limit=${limit}&offset=${offset}`),
};
