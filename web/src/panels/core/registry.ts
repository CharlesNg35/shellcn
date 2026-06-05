import {
  defineAsyncComponent,
  type AsyncComponentLoader,
  type Component,
} from "vue";
import type { PanelType } from "../../types/projection";
import PanelLoader from "../../components/PanelLoader.vue";
import TablePanel from "../table/TablePanel.vue";
import FormPanel from "../form/FormPanel.vue";
import EnrollPanel from "../enroll/EnrollPanel.vue";
import FileBrowserPanel from "../file/FileBrowserPanel.vue";
import DocumentPanel from "../document/DocumentPanel.vue";
import DashboardPanel from "../dashboard/DashboardPanel.vue";

const lazy = (loader: AsyncComponentLoader): Component =>
  defineAsyncComponent({ loader, loadingComponent: PanelLoader });

// Lightweight declarative panels are bundled up front; heavy panel engines are
// dynamically imported on first use so first paint stays constant regardless of
// how many plugins exist.
export const panelRegistry: Record<string, Component> = {
  table: TablePanel,
  form: FormPanel,
  enroll: EnrollPanel,
  file_browser: FileBrowserPanel,
  document: DocumentPanel,
  dashboard: DashboardPanel,
  object_detail: lazy(() => import("../specialized/ObjectDetailPanel.vue")),
  timeline: lazy(() => import("../specialized/TimelinePanel.vue")),
  task_progress: lazy(() => import("../streaming/TaskProgressPanel.vue")),
  split: lazy(() => import("../specialized/SplitPanel.vue")),
  terminal: lazy(() => import("../streaming/TerminalPanel.vue")),
  log_stream: lazy(() => import("../streaming/LogStreamPanel.vue")),
  metrics: lazy(() => import("../streaming/MetricsPanel.vue")),
  code_editor: lazy(() => import("../streaming/CodeEditorPanel.vue")),
  query_editor: lazy(() => import("../streaming/QueryEditorPanel.vue")),
  remote_desktop: lazy(() => import("../streaming/RemoteDesktopPanel.vue")),
  graph: lazy(() => import("../specialized/GraphPanel.vue")),
  trace: lazy(() => import("../specialized/TracePanel.vue")),
  kv: lazy(() => import("../specialized/KVPanel.vue")),
  http_client: lazy(() => import("../specialized/HTTPClientPanel.vue")),
};

export function resolvePanel(type: PanelType): Component | undefined {
  return panelRegistry[type];
}
