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

export type AgentStatus = "pending" | "online" | "offline" | "error";

export interface AgentState {
  status: AgentStatus;
  message?: string;
}
