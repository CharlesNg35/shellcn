import type { DataSource, Icon, Method, RiskLevel, SortKey } from "./core";
import type { Condition, Option, Schema } from "./schema";

export const PanelType = {
  Terminal: "terminal",
  FileBrowser: "file_browser",
  Table: "table",
  Metrics: "metrics",
  LogStream: "log_stream",
  TerminalGrid: "terminal_grid",
  CodeEditor: "code_editor",
  Diff: "diff",
  Document: "document",
  QueryEditor: "query_editor",
  RemoteDesktop: "remote_desktop",
  Form: "form",
  Enroll: "enroll",
  ObjectDetail: "object_detail",
  Timeline: "timeline",
  TaskProgress: "task_progress",
  Split: "split",
  Canvas: "canvas",
  Wasm: "wasm",
  WebProxy: "web_proxy",
  Graph: "graph",
  Trace: "trace",
  KV: "kv",
  HTTPClient: "http_client",
  Dashboard: "dashboard",
} as const;

export const KNOWN_PANEL_TYPES = Object.values(PanelType);

export type KnownPanelType = (typeof PanelType)[keyof typeof PanelType];

export type PanelType = KnownPanelType | (string & {});

export const StreamKind = {
  Terminal: "terminal",
  Logs: "logs",
  Query: "query",
  Desktop: "desktop",
  Metrics: "metrics",
  Task: "task",
  Canvas: "canvas",
  Resource: "resource",
} as const;
export type StreamKind = (typeof StreamKind)[keyof typeof StreamKind];

export const ColumnType = {
  Text: "text",
  Badge: "badge",
  Bytes: "bytes",
  DateTime: "datetime",
  RelativeTime: "relative_time",
  Number: "number",
  Percent: "percent",
  Bool: "bool",
  Json: "json",
  Icon: "icon",
} as const;
export type ColumnType = (typeof ColumnType)[keyof typeof ColumnType];

export const ColumnEditor = {
  Text: "text",
  Textarea: "textarea",
  Number: "number",
  Toggle: "toggle",
  Select: "select",
  Json: "json",
} as const;
export type ColumnEditor = (typeof ColumnEditor)[keyof typeof ColumnEditor];

export const Severity = {
  Info: "info",
  Success: "success",
  Warn: "warn",
  Danger: "danger",
  Secondary: "secondary",
} as const;
export type Severity = (typeof Severity)[keyof typeof Severity];

export interface Column {
  key: string;
  label: string;
  sortable?: boolean;
  type?: ColumnType;
  width?: string;
  editable?: boolean;
  editor?: ColumnEditor;
  options?: Option[];
  readOnly?: boolean;
  nullable?: boolean;
  precision?: number;
  severities?: Record<string, Severity>;
}

export const RowClickAction = {
  Navigate: "navigate",
  Detail: "detail",
  Select: "select",
  None: "none",
} as const;
export type RowClickAction =
  (typeof RowClickAction)[keyof typeof RowClickAction];

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
  routes?: FileBrowserRoutes;
  upload?: FileUploadConfig;
  writable?: boolean;
  controls?: StreamControl[];
}

export interface FileBrowserRoutes {
  read?: string;
  download?: string;
  write?: string;
  mkdir?: string;
  rename?: string;
  delete?: string;
  move?: string;
  copy?: string;
  chmod?: string;
  archive?: string;
}

export interface FileUploadConfig {
  routeId?: string;
  fieldName?: string;
  multiple?: boolean;
  maxBytes?: number;
}

export const FileOperation = {
  Move: "move",
  Copy: "copy",
} as const;

export type FileOperation = (typeof FileOperation)[keyof typeof FileOperation];

export interface SaveToast {
  summary?: string;
  detail?: string;
  severity?: Severity;
}

export const SaveDismiss = { Close: "close" } as const;
export type SaveDismiss = (typeof SaveDismiss)[keyof typeof SaveDismiss] | "";

export interface FormPanelConfig {
  submitRouteId?: string;
  submitMethod?: Exclude<Method, "GET" | "WS">;
  submitLabel?: string;
  params?: Record<string, string>;
  saveToast?: SaveToast;
  saveDismiss?: SaveDismiss;
}

export interface CodeEditorConfig {
  language?: string;
  initialContent?: string;
  saveRouteId?: string;
  saveMethod?: Exclude<Method, "GET" | "WS">;
  saveParams?: Record<string, string>;
  saveBodyKey?: string;
  saveExtra?: Record<string, unknown>;
  watch?: DataSource;
  refreshField?: string;
  dryRunKey?: string;
  saveToast?: SaveToast;
  saveDismiss?: SaveDismiss;
}

export interface StreamControl {
  param: string;
  label?: string;
  optionsSource?: DataSource;
}

export interface LogStreamConfig {
  controls?: StreamControl[];
  allowPrevious?: boolean;
}

export const DiffMode = {
  SideBySide: "side_by_side",
  Unified: "unified",
} as const;
export type DiffMode = (typeof DiffMode)[keyof typeof DiffMode];

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
  layout?: GraphLayout;
  fitView?: boolean;
  expandRouteId?: string;
  expandParam?: string;
  exportable?: boolean;
}

export const GraphLayout = {
  Grid: "grid",
  Manual: "manual",
} as const;
export type GraphLayout = (typeof GraphLayout)[keyof typeof GraphLayout];

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
  usage?: MetricUsage[];
  series?: MetricSeries[];
  history?: number;
}

export interface TerminalPanelConfig {
  zoom?: boolean;
  search?: boolean;
  controls?: StreamControl[];
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
  usage?: UsageSpec;
  copy?: boolean;
  redacted?: boolean;
  severities?: Record<string, Severity>;
}

export interface UsageSpec {
  percentKey?: string;
  usedKey?: string;
  totalKey?: string;
  usedType?: ColumnType;
  totalType?: ColumnType;
  unit?: string;
  totalLabel?: string;
  warnAt?: number;
  criticalAt?: number;
}

export type MetricUsage = ObjectDetailField;

export interface ObjectDetailSection {
  title?: string;
  fields?: ObjectDetailField[];
}

export interface ObjectDetailPanelConfig {
  sections?: ObjectDetailSection[];
  rawToggle?: boolean;
  watch?: DataSource;
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
  watch?: DataSource;
}

export interface TaskProgressPanelConfig {
  title?: string;
  cancelRouteId?: string;
  retryRouteId?: string;
}

export interface PanelVariant {
  panel: PanelType;
  config?: Record<string, unknown>;
  visibleWhen?: Condition;
}

export interface DashboardCell {
  key: string;
  label?: string;
  icon?: Icon;
  panel: PanelType;
  source?: DataSource;
  config?: Record<string, unknown>;
  variants?: PanelVariant[];
  visibleWhen?: Condition;
  span?: number;
}

export interface SplitChildPanel extends DashboardCell {
  size?: number;
  minSize?: number;
}

export interface SplitPanelConfig {
  orientation?: SplitOrientation;
  panels?: SplitChildPanel[];
}

export const SplitOrientation = {
  Horizontal: "horizontal",
  Vertical: "vertical",
} as const;
export type SplitOrientation =
  (typeof SplitOrientation)[keyof typeof SplitOrientation];

export interface CanvasPanelConfig {
  width?: number;
  height?: number;
  scaleMode?: CanvasScaleMode;
  minScale?: number;
  maxScale?: number;
  hidpi?: boolean;
  interactive?: boolean;
  keyboard?: boolean;
  pointer?: boolean;
  wheelMode?: CanvasWheelMode;
  resizeEvents?: boolean;
  background?: string;
  focusOnPointer?: boolean;
  ariaLabel?: string;
  instructions?: string;
}

export const CanvasScaleMode = {
  Resize: "resize",
  Fit: "fit",
  Scroll: "scroll",
} as const;
export type CanvasScaleMode =
  (typeof CanvasScaleMode)[keyof typeof CanvasScaleMode];

export const CanvasWheelMode = {
  Auto: "auto",
  Capture: "capture",
  Modified: "modified",
  None: "none",
} as const;
export type CanvasWheelMode =
  (typeof CanvasWheelMode)[keyof typeof CanvasWheelMode];

export const WasmRuntime = {
  Go: "go",
  Generic: "generic",
} as const;
export type WasmRuntime = (typeof WasmRuntime)[keyof typeof WasmRuntime];

export type WasmScaleMode = CanvasScaleMode;

export interface WasmAsset {
  path: string;
  mime?: string;
  source: DataSource;
}

export interface WasmBoot {
  scripts?: string[];
}

export interface WasmCapabilities {
  keyboard?: boolean;
  pointer?: boolean;
  audio?: boolean;
  fullscreen?: boolean;
  pointerLock?: boolean;
  gamepad?: boolean;
}

export interface WasmBridgeRoute {
  routeId: string;
  method?: Exclude<Method, "WS">;
  params?: Record<string, string>;
}

export interface WasmBridgeStream {
  routeId: string;
  params?: Record<string, string>;
}

export interface WasmBridge {
  routes?: WasmBridgeRoute[];
  streams?: WasmBridgeStream[];
}

export interface WasmPanelConfig {
  entry: string;
  runtime?: WasmRuntime;
  boot?: WasmBoot;
  assets?: WasmAsset[];
  width?: number;
  height?: number;
  scaleMode?: WasmScaleMode;
  capabilities?: WasmCapabilities;
  bridge?: WasmBridge;
  ariaLabel?: string;
  instructions?: string;
}

export const WebProxyCapability = {
  Clipboard: "clipboard",
  Downloads: "downloads",
  Fullscreen: "fullscreen",
  Popups: "popups",
  SameOrigin: "same_origin",
} as const;
export type WebProxyCapability =
  (typeof WebProxyCapability)[keyof typeof WebProxyCapability];

export interface WebProxyPanelConfig {
  path?: string;
  capabilities?: WebProxyCapability[];
  openExternal?: boolean;
  ariaLabel?: string;
  instructions?: string;
}

export interface Badge {
  source?: DataSource;
  value?: string | number;
  severity?: Severity;
}

// ResourceIdentity marks a row/tree node as a real navigable resource. Plain
// table rows use their own fields and record-context params instead.
export interface ResourceIdentity {
  kind: string;
  scope?: string;
  namespace?: string;
  name: string;
  uid: string;
}

export interface ActionSuccess {
  selectTab?: string;
  navigate?: NavigateTarget;
  effects?: ActionEffect[];
}

export const ActionEffectType = {
  TerminalInput: "terminal_input",
  OpenPanel: "open_panel",
} as const;
export type ActionEffectType =
  (typeof ActionEffectType)[keyof typeof ActionEffectType];

export interface ActionEffect {
  type: ActionEffectType;
  terminalInput?: TerminalInputEffect;
  openPanel?: OpenPanelEffect;
}

export interface TerminalInputEffect {
  tab?: string;
  text?: string;
  resultField?: string;
  appendNewline?: boolean;
}

export interface OpenPanelEffect {
  open: OpenTarget;
  panel: PanelType;
  title?: string;
  icon?: Icon;
  source?: DataSource;
  config?: Record<string, unknown>;
}

export const NavigateTarget = {
  List: "list",
} as const;
export type NavigateTarget =
  (typeof NavigateTarget)[keyof typeof NavigateTarget];

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
  open?: OpenTarget;
  panel?: PanelType;
  config?: Record<string, unknown>;
  enabledWhen?: Condition;
  visibleWhen?: Condition;
  iconOnly?: boolean;
  group?: string;
}

export const OpenTarget = {
  View: "view",
  Dock: "dock",
  Dialog: "dialog",
  Url: "url",
} as const;
export type OpenTarget = (typeof OpenTarget)[keyof typeof OpenTarget];

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
  variants?: PanelVariant[];
  visibleWhen?: Condition;
  span?: number;
}

export interface DashboardPanelConfig {
  cells: DashboardCell[];
}

export interface PanelConfigProperty {
  type: PanelConfigPropertyType;
  items?: PanelConfigProperty;
  properties?: Record<string, PanelConfigProperty>;
  enum?: string[];
  required?: string[];
}

export interface PanelConfigSchema {
  type: typeof PanelConfigPropertyType.Object;
  properties?: Record<string, PanelConfigProperty>;
  required?: string[];
}

export const PanelConfigPropertyType = {
  String: "string",
  Number: "number",
  Boolean: "boolean",
  Object: "object",
  Array: "array",
} as const;
export type PanelConfigPropertyType =
  (typeof PanelConfigPropertyType)[keyof typeof PanelConfigPropertyType];

export type Row = Record<string, unknown> & {
  ref?: ResourceIdentity;
  _key?: Record<string, unknown>;
  _links?: Record<string, ResourceIdentity>;
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
  encoding?: FileContentEncoding;
  content?: string;
  url?: string;
  size?: number;
  truncated?: boolean;
}

export const FileContentEncoding = {
  UTF8: "utf8",
  Base64: "base64",
  Url: "url",
  Binary: "binary",
} as const;
export type FileContentEncoding =
  (typeof FileContentEncoding)[keyof typeof FileContentEncoding];
