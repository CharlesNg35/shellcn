import { api } from "./client";
import type { AgentState, Enrollment } from "../types/projection";

export const agentApi = {
  state: (connectionId: string) =>
    api.get<AgentState>(`/connections/${connectionId}/agent/state`),
  enroll: (connectionId: string) =>
    api.post<Enrollment>(`/connections/${connectionId}/agent/enrollments`),
};
