import { computed, reactive } from "vue";
import { defineStore } from "pinia";
import type { RiskLevel } from "@/types/projection";

export interface WorkspaceInvalidation {
  connectionId: string;
  routeId: string;
  risk: RiskLevel | string;
  params?: Record<string, string>;
  toolName?: string;
  toolId?: string;
  source?: "ai" | "ui";
  at?: number;
}

interface InvalidationState {
  version: number;
  last: WorkspaceInvalidation | null;
}

export const useWorkspaceInvalidationStore = defineStore(
  "workspaceInvalidation",
  () => {
    const byConnection = reactive<Record<string, InvalidationState>>({});

    function state(connectionId: string): InvalidationState {
      if (!byConnection[connectionId]) {
        byConnection[connectionId] = { version: 0, last: null };
      }
      return byConnection[connectionId];
    }

    function invalidate(event: WorkspaceInvalidation): void {
      const connectionId = event.connectionId;
      if (!connectionId) return;
      const st = state(connectionId);
      st.version += 1;
      st.last = { ...event, at: event.at ?? Date.now() };
    }

    function version(connectionId: string): number {
      return byConnection[connectionId]?.version ?? 0;
    }

    function last(connectionId: string): WorkspaceInvalidation | null {
      return byConnection[connectionId]?.last ?? null;
    }

    return {
      byConnection,
      invalidate,
      version,
      last,
      versionRef: (connectionId: string) =>
        computed(() => byConnection[connectionId]?.version ?? 0),
    };
  },
);
