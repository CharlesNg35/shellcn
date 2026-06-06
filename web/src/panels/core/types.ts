import type { Action, DataSource, ResourceRef } from "../../types/projection";
import type { RecordingDescriptor } from "../../composables/useRecordingControl";

// Every panel component receives this shape; PanelHost binds it uniformly.
export interface PanelProps {
  connectionId: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  recording?: RecordingDescriptor | null;
  resource?: ResourceRef | null;
  actions?: Action[];
}
