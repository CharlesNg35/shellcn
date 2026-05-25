import type { DataSource, ResourceRef } from "../types/projection";

// Every panel component receives this shape; PanelHost binds it uniformly.
export interface PanelProps {
  connectionId: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  resource?: ResourceRef | null;
}
