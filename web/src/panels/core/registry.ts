import {
  defineAsyncComponent,
  type AsyncComponentLoader,
  type Component,
} from "vue";
import type { PanelType } from "../../types/projection";
import LoadingPanel from "./LoadingPanel.vue";
import TablePanel from "../table/TablePanel.vue";
import FormPanel from "../form/FormPanel.vue";
import EnrollPanel from "../enroll/EnrollPanel.vue";
import FileBrowserPanel from "../file/FileBrowserPanel.vue";
import DocumentPanel from "../document/DocumentPanel.vue";

const lazy = (loader: AsyncComponentLoader): Component =>
  defineAsyncComponent({ loader, loadingComponent: LoadingPanel });

// Lightweight declarative panels are bundled up front; heavy ones (xterm,
// Monaco, noVNC, charts) are dynamically imported on first use so first paint
// stays constant regardless of how many plugins exist.
export const panelRegistry: Record<string, Component> = {
  table: TablePanel,
  form: FormPanel,
  enroll: EnrollPanel,
  file_browser: FileBrowserPanel,
  document: DocumentPanel,
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
