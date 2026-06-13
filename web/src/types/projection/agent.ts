export interface InstallArtifact {
  label: string;
  kind: string;
  command?: string;
  url?: string;
  content?: string;
  filename?: string;
}

export interface Enrollment {
  enrollmentId: string;
  expiresAt: string;
  artifacts: InstallArtifact[];
  downloadUrl: string;
}

export const AgentStatus = {
  Pending: "pending",
  Online: "online",
  Offline: "offline",
  Error: "error",
} as const;
export type AgentStatus = (typeof AgentStatus)[keyof typeof AgentStatus];

export interface AgentState {
  status: AgentStatus;
  message?: string;
}
