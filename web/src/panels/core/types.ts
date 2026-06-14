import type { RecordingDescriptor } from "@/composables/useRecordingControl";
import type {
  Action,
  DataSource,
  ResourceIdentity,
  Row,
} from "@/types/projection";

// Every panel component receives this shape; PanelHost binds it uniformly.
export interface PanelProps {
  connectionId: string;
  panelKey?: string;
  source?: DataSource;
  config?: Record<string, unknown>;
  recording?: RecordingDescriptor | null;
  resource?: ResourceIdentity | null;
  record?: Row | null;
  actions?: Action[];
}
