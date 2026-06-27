import {
  defineAsyncComponent,
  type AsyncComponentLoader,
  type Component,
} from "vue";
import {
  PanelType,
  type PanelType as ProjectionPanelType,
} from "@/types/projection";
import PanelLoader from "@/components/PanelLoader.vue";
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
  [PanelType.Table]: TablePanel,
  [PanelType.Form]: FormPanel,
  [PanelType.Enroll]: EnrollPanel,
  [PanelType.FileBrowser]: FileBrowserPanel,
  [PanelType.Document]: DocumentPanel,
  [PanelType.Dashboard]: DashboardPanel,
  [PanelType.ObjectDetail]: lazy(
    () => import("../specialized/ObjectDetailPanel.vue"),
  ),
  [PanelType.Timeline]: lazy(() => import("../specialized/TimelinePanel.vue")),
  [PanelType.TaskProgress]: lazy(
    () => import("../streaming/TaskProgressPanel.vue"),
  ),
  [PanelType.Split]: lazy(() => import("../specialized/SplitPanel.vue")),
  [PanelType.Canvas]: lazy(() => import("../streaming/CanvasPanel.vue")),
  [PanelType.Wasm]: lazy(() => import("../wasm/WasmPanel.vue")),
  [PanelType.WebProxy]: lazy(() => import("../web/WebProxyPanel.vue")),
  [PanelType.Terminal]: lazy(() => import("../streaming/TerminalPanel.vue")),
  [PanelType.TerminalGrid]: lazy(
    () => import("../streaming/TerminalGridPanel.vue"),
  ),
  [PanelType.LogStream]: lazy(() => import("../streaming/LogStreamPanel.vue")),
  [PanelType.Metrics]: lazy(() => import("../streaming/MetricsPanel.vue")),
  [PanelType.CodeEditor]: lazy(
    () => import("../streaming/CodeEditorPanel.vue"),
  ),
  [PanelType.Diff]: lazy(() => import("../specialized/DiffPanel.vue")),
  [PanelType.QueryEditor]: lazy(
    () => import("../streaming/QueryEditorPanel.vue"),
  ),
  [PanelType.RemoteDesktop]: lazy(
    () => import("../streaming/RemoteDesktopPanel.vue"),
  ),
  [PanelType.Graph]: lazy(() => import("../specialized/GraphPanel.vue")),
  [PanelType.Trace]: lazy(() => import("../specialized/TracePanel.vue")),
  [PanelType.KV]: lazy(() => import("../specialized/KVPanel.vue")),
  [PanelType.HTTPClient]: lazy(
    () => import("../specialized/HTTPClientPanel.vue"),
  ),
};

export function resolvePanel(type: ProjectionPanelType): Component | undefined {
  return panelRegistry[type];
}
