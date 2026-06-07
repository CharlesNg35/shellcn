import type { DataSource, Icon, Method, RiskLevel, SortKey } from "./core";
import type { Condition, Schema } from "./schema";

export const KNOWN_PANEL_TYPES = [
  "terminal",
  "file_browser",
  "table",
  "metrics",
  "log_stream",
  "terminal_grid",
  "code_editor",
  "diff",
  "document",
  "query_editor",
  "remote_desktop",
  "form",
  "enroll",
  "object_detail",
  "timeline",
  "task_progress",
  "split",
  "canvas",
  "graph",
  "trace",
  "kv",
  "http_client",
  "dashboard",
] as const;

export type KnownPanelType = (typeof KNOWN_PANEL_TYPES)[number];

export type PanelType = KnownPanelType | (string & {});

export type StreamKind =
  | "terminal"
  | "logs"
  | "desktop"
  | "metrics"
  | "file"
  | "task"
  | "canvas";

export type ColumnType =
  | "text"
  | "badge"
  | "bytes"
  | "datetime"
  | "relative_time"
  | "number"
  | "percent"
  | "bool"
  | "json"
  | "icon";

export type Severity = "info" | "success" | "warn" | "danger" | "secondary";

export interface Column {
  key: string;
  label: string;
  sortable?: boolean;
  type?: ColumnType;
  width?: string;
  readOnly?: boolean;
  nullable?: boolean;
  precision?: number;
  severities?: Record<string, Severity>;
}

export type RowClickAction = "navigate" | "detail" | "select" | "none";

export interface TablePanelConfig {
  columns?: Column[];
  columnsSource?: DataSource;
  watch?: DataSource;
  refreshIntervalMs?: number;
  defaultSort?: SortKey;
  actionIds?: string[];
  rowActionIds?: string[];
  selectable?: boolean;
  editable?: boolean;
  rowKey?: string[];
  insert?: DataSource;
  update?: DataSource;
  delete?: DataSource;
  emptyText?: string;
  stagedEdits?: boolean;
  hiddenColumns?: string[];
  exportable?: boolean;
  rowClick?: RowClickAction;
}

export interface FileBrowserConfig {
  pathParam?: string;
  readRouteId?: string;
  downloadRouteId?: string;
  writeRouteId?: string;
  uploadRouteId?: string;
  mkdirRouteId?: string;
  renameRouteId?: string;
  deleteRouteId?: string;
  moveRouteId?: string;
  copyRouteId?: string;
  chmodRouteId?: string;
  archiveRouteId?: string;
  writable?: boolean;
  multipleUpload?: boolean;
  maxUploadBytes?: number;
  uploadFieldName?: string;
}

export interface FormPanelConfig {
  submitRouteId?: string;
  submitMethod?: Exclude<Method, "GET" | "WS">;
  submitLabel?: string;
  params?: Record<string, string>;
}

export interface CodeEditorConfig {
  language?: string;
  initialContent?: string;
  saveRouteId?: string;
  saveMethod?: Exclude<Method, "GET" | "WS">;
  saveParams?: Record<string, string>;
  saveBodyKey?: string;
  saveExtra?: Record<string, unknown>;
}

export type DiffMode = "side_by_side" | "unified";

export interface DiffPanelConfig {
  language?: string;
  originalField?: string;
  modifiedField?: string;
  originalLabel?: string;
  modifiedLabel?: string;
  mode?: DiffMode;
  collapseUnchanged?: boolean;
}

export interface QueryEditorConfig {
  language?: string;
  label?: string;
  executeLabel?: string;
  cancelLabel?: string;
  runningLabel?: string;
  emptyText?: string;
  initialQuery?: string;
  cancelRouteId?: string;
  cancelParams?: Record<string, string>;
  completionRouteId?: string;
  completionParams?: Record<string, string>;
  exportable?: boolean | null;
}

export interface GraphPanelConfig {
  layout?: "grid" | "manual";
  fitView?: boolean;
  expandRouteId?: string;
  expandParam?: string;
  exportable?: boolean;
}

export interface TracePanelConfig {
  serviceField?: string;
}

export interface MetricStat {
  key: string;
  label?: string;
  unit?: string;
}

export interface MetricGauge {
  key: string;
  label?: string;
  unit?: string;
  max?: number;
}

export interface MetricSeries {
  key: string;
  label?: string;
  unit?: string;
}

export interface MetricsPanelConfig {
  stats?: MetricStat[];
  gauges?: MetricGauge[];
  series?: MetricSeries[];
  history?: number;
}

export interface TerminalPanelConfig {
  zoom?: boolean;
  search?: boolean;
}

export interface TerminalGridPanelConfig extends TerminalPanelConfig {
  maxPanes?: number;
  defaultPanes?: number;
}

export interface KVPanelConfig {
  createRouteId?: string;
  readRouteId?: string;
  writeRouteId?: string;
  deleteRouteId?: string;
  keyParam?: string;
  writable?: boolean;
  valueTypes?: string[];
}

export interface HTTPClientConfig {
  executeRouteId?: string;
  methods?: string[];
  defaultMethod?: string;
  defaultUrl?: string;
  defaultHeaders?: Array<{ key: string; value: string }>;
  defaultBody?: string;
}

export interface RemoteDesktopPanelConfig {
  resize?: boolean;
  clipboard?: boolean;
  audio?: boolean;
  repeaterID?: string;
}

export interface ObjectDetailField {
  key: string;
  label?: string;
  type?: ColumnType;
  copy?: boolean;
  redacted?: boolean;
  severities?: Record<string, Severity>;
}

export interface ObjectDetailSection {
  title?: string;
  fields?: ObjectDetailField[];
}

export interface ObjectDetailPanelConfig {
  sections?: ObjectDetailSection[];
  rawToggle?: boolean;
}

export interface TimelinePanelConfig {
  timestampField?: string;
  titleField?: string;
  bodyField?: string;
  severityField?: string;
  iconField?: string;
  resourceField?: string;
  emptyText?: string;
  refreshIntervalMs?: number;
}

export interface TaskProgressPanelConfig {
  title?: string;
  cancelRouteId?: string;
  retryRouteId?: string;
}

export interface DashboardCell {
  key: string;
  label?: string;
  icon?: Icon;
  panel: PanelType;
  source?: DataSource;
  config?: Record<string, unknown>;
  span?: number;
}

export interface SplitChildPanel extends DashboardCell {
  size?: number;
  minSize?: number;
}

export interface SplitPanelConfig {
  orientation?: "horizontal" | "vertical";
  panels?: SplitChildPanel[];
}

export interface CanvasPanelConfig {
  width?: number;
  height?: number;
  hidpi?: boolean;
  interactive?: boolean;
  keyboard?: boolean;
  pointer?: boolean;
  wheel?: boolean;
  resizeEvents?: boolean;
  background?: string;
  focusOnPointer?: boolean;
  ariaLabel?: string;
  instructions?: string;
}

export interface Badge {
  source?: DataSource;
  value?: string | number;
  severity?: Severity;
}

export interface ResourceRef {
  kind: string;
  scope?: string;
  namespace?: string;
  name: string;
  uid: string;
}

export interface ActionSuccess {
  selectTab?: string;
  navigate?: "list";
}

export interface Action {
  id: string;
  label: string;
  icon?: Icon;
  routeId: string;
  method?: Method;
  params?: Record<string, string>;
  risk: RiskLevel;
  requiresConfirm: boolean;
  confirmText?: string;
  input?: Schema;
  onSuccess?: ActionSuccess;
  open?: "view" | "dock" | "dialog" | "url";
  panel?: PanelType;
  config?: Record<string, unknown>;
  enabledWhen?: Condition;
  iconOnly?: boolean;
  group?: string;
}

export interface Stream {
  id: string;
  kind: StreamKind;
  routeId: string;
}

export interface Tab {
  key: string;
  label: string;
  icon?: Icon;
  panel: PanelType;
  source?: DataSource;
  config?: Record<string, unknown>;
  span?: number;
}

export interface DashboardPanelConfig {
  cells: DashboardCell[];
}

export interface PanelConfigProperty {
  type: "string" | "number" | "boolean" | "object" | "array";
  items?: PanelConfigProperty;
  properties?: Record<string, PanelConfigProperty>;
  enum?: string[];
  required?: string[];
}

export interface PanelConfigSchema {
  type: "object";
  properties?: Record<string, PanelConfigProperty>;
  required?: string[];
}

export type Row = Record<string, unknown> & {
  ref?: ResourceRef;
  _key?: Record<string, unknown>;
  _links?: Record<string, ResourceRef>;
};

export interface FileEntry {
  name: string;
  path: string;
  isDir: boolean;
  size?: number;
  mime?: string;
  modTime?: string;
  mode?: string;
  symlink?: string;
}

export interface FileContent {
  path: string;
  mime?: string;
  encoding?: "utf8" | "base64" | "url" | "binary";
  content?: string;
  url?: string;
  size?: number;
  truncated?: boolean;
}
