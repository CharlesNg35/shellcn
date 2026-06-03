import type { Role } from "../constants/roles";

export type IconType = "lucide" | "url" | "base64" | "emoji" | "svg";

export interface Icon {
  type: IconType;
  value: string;
}

export interface PluginCategoryInfo {
  key: string;
  label: string;
  icon: Icon;
  order: number;
}

export type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "WS";

export type RiskLevel = "safe" | "write" | "destructive" | "privileged";

export type Transport = "direct" | "agent";

export const TRANSPORT_DIRECT: Transport = "direct";
export const TRANSPORT_AGENT: Transport = "agent";

export const Layout = {
  Tabs: "tabs",
  SidebarTree: "sidebar_tree",
  Dashboard: "dashboard",
  Single: "single",
} as const;
export type Layout = (typeof Layout)[keyof typeof Layout];

export type KnownPanelType =
  | "terminal"
  | "file_browser"
  | "table"
  | "metrics"
  | "log_stream"
  | "code_editor"
  | "document"
  | "query_editor"
  | "remote_desktop"
  | "form"
  | "enroll"
  | "graph"
  | "trace"
  | "kv"
  | "http_client"
  | "dashboard";

export type PanelType = KnownPanelType | (string & {});

export type StreamKind = "terminal" | "logs" | "desktop" | "metrics" | "file";

export type RecordingClass = "terminal" | "desktop";

export type RecordingFormat = "asciicast_v2" | "webm_canvas";

export type RecordingPolicy = "disabled" | "manual" | "auto";

export interface RecordingCapability {
  class: RecordingClass;
  formats: RecordingFormat[];
  authoritative: boolean;
  inputCapture: boolean;
}

export type RecordingStatus =
  | "pending"
  | "active"
  | "finalized"
  | "failed"
  | "discarded";

export interface RecordingSummary {
  id: string;
  userId: string;
  username?: string;
  connectionId: string;
  connectionName?: string;
  protocol: string;
  class: RecordingClass;
  format: RecordingFormat;
  authoritative: boolean;
  status: RecordingStatus;
  title?: string;
  startedAt: string;
  endedAt?: string;
  durationMs: number;
  size: number;
}

export interface RecordingFilters {
  user?: string;
  connection?: string;
  protocol?: string;
  class?: RecordingClass;
  status?: RecordingStatus;
}

export type FieldType =
  | "text"
  | "email"
  | "url"
  | "tel"
  | "number"
  | "stepper"
  | "slider"
  | "password"
  | "select"
  | "radio"
  | "multiselect"
  | "file"
  | "toggle"
  | "textarea"
  | "json"
  | "duration"
  | "credential_ref"
  | "object"
  | "array"
  | "autocomplete"
  | "map";

export interface Option {
  label: string;
  value: string | number | boolean;
}

export type CredentialKind = string;

export interface CredentialKindInfo {
  kind: CredentialKind;
  label: string;
  secretLabel: string;
  secretMultiline?: boolean;
  identityLabel?: string;
  compatibleProtocols?: string[];
}

export interface CredentialSelector {
  kinds: CredentialKind[];
  protocols?: string[];
  required?: boolean;
}

export type Operator = "eq" | "neq" | "in" | "nin" | "empty" | "notEmpty";

export interface Rule {
  field: string;
  op: Operator;
  value?: unknown;
}

export interface Condition {
  allOf?: Rule[];
  anyOf?: Rule[];
}

export type ValidatorType = "min" | "max" | "regex" | "oneOf";

export interface Validator {
  type: ValidatorType;
  value?: unknown;
  message?: string;
}

export interface Field {
  key: string;
  label: string;
  type: FieldType;
  required?: boolean;
  secret?: boolean;
  default?: unknown;
  placeholder?: string;
  help?: string;
  options?: Option[];
  optionsSource?: DataSource;
  credential?: CredentialSelector;
  visibleWhen?: Condition;
  validators?: Validator[];
  step?: number;
  fields?: Field[];
  item?: Field;
  minItems?: number;
  maxItems?: number;
  itemLabel?: string;
  addLabel?: string;
  keyLabel?: string;
  keyPlaceholder?: string;
}

export interface Group {
  name: string;
  fields: Field[];
}

export interface Schema {
  groups: Group[];
}

export interface DataSource {
  routeId: string;
  method?: Method;
  params?: Record<string, string>;
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

export type RowClickAction = "navigate" | "detail" | "select" | "none";

export interface FilterOption {
  value: string;
  label?: string;
}

export interface ScopeFilter {
  param: string;
  label: string;
  icon?: Icon;
  control?: string;
  optionsSource?: DataSource;
  options?: FilterOption[];
  valueField?: string;
  labelField?: string;
  allLabel?: string;
  defaultValue?: string;
}

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
  exportable?: boolean;
}

export interface GraphPanelConfig {
  layout?: "grid" | "manual";
  fitView?: boolean;
  expandRouteId?: string;
  expandParam?: string;
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

export type ColumnType =
  | "text"
  | "badge"
  | "bytes"
  | "datetime"
  | "number"
  | "percent"
  | "bool"
  | "json"
  | "icon";

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

export type Severity = "info" | "success" | "warn" | "danger" | "secondary";

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

export interface DashboardCell {
  key: string;
  label?: string;
  icon?: Icon;
  panel: PanelType;
  source?: DataSource;
  config?: Record<string, unknown>;
  span?: number;
}

export interface DashboardPanelConfig {
  cells: DashboardCell[];
}

export interface TreeGroup {
  key: string;
  label: string;
  icon?: Icon;
  source?: DataSource;
  resourceKind?: string;
  ref?: ResourceRef;
  badge?: Badge;
}

export interface TreeNode {
  key: string;
  label: string;
  icon?: Icon;
  ref?: ResourceRef;
  leaf?: boolean;
  childrenSource?: DataSource;
  badge?: Badge;
  resourceKind?: string;
  listParams?: Record<string, string>;
  data?: Record<string, unknown>;
}

export interface HeaderSpec {
  title?: string;
  statusField?: string;
  severities?: Record<string, Severity>;
}

export interface ResourceActions {
  toolbar?: string[];
  row?: string[];
  detail?: string[];
  selectable?: boolean;
}

export interface DetailView {
  header: HeaderSpec;
  defaultTab?: string;
  tabs: Tab[];
}

export interface ResourceType {
  kind: string;
  title: string;
  list: DataSource;
  watch?: DataSource;
  columns: Column[];
  columnsSource?: DataSource;
  actions?: ResourceActions;
  detail: DetailView;
}

export interface AgentProfile {
  modes: string[];
  riskNote?: string;
}

export interface CredentialSummary {
  id: string;
  name: string;
  kind: CredentialKind;
  ownerId?: string;
  ownerName?: string;
  identity?: string;
  protocols?: string[];
  updatedAt?: string;
}

export interface PluginSummary {
  name: string;
  title: string;
  icon: Icon;
  category: PluginCategoryInfo;
  description?: string;
}

export interface PluginProjection {
  apiVersion: number;
  name: string;
  version: string;
  title: string;
  description: string;
  icon: Icon;
  category: PluginCategoryInfo;
  config: Schema;
  capabilities: string[];
  credentialKinds?: CredentialKindInfo[];
  supportedTransports: Transport[];
  agent?: AgentProfile;
  layout: Layout;
  tabs?: Tab[];
  tree?: TreeGroup[];
  resources?: ResourceType[];
  actions?: Action[];
  headerActions?: string[];
  scope?: ScopeFilter[];
  streams?: Stream[];
  recording?: RecordingCapability[];
}

export interface ConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  transport: Transport;
  online?: boolean;
  status?: "offline";
  canManage?: boolean;
  canShare?: boolean;
  access?: "owner" | "admin" | GrantAccess;
  owned?: boolean;
  ownerName?: string;
  sharedWithMe?: boolean;
  sharedByMe?: boolean;
  recording?: Record<string, string>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
  folderId?: string;
  sortOrder?: number;
}

export type FolderColor =
  | "slate"
  | "blue"
  | "teal"
  | "emerald"
  | "amber"
  | "rose"
  | "violet"
  | "cyan";

export interface ConnectionFolder {
  id: string;
  parentId?: string;
  name: string;
  color: FolderColor;
  sortOrder: number;
}

export type GrantAccess = "use" | "manage";

export interface ShareGrant {
  id: string;
  subjectId: string;
  username?: string;
  displayName?: string;
  access: GrantAccess;
}

export interface UserSummary {
  id: string;
  username: string;
  displayName?: string;
}

export interface AdminUser {
  id: string;
  username: string;
  email?: string;
  displayName?: string;
  roles: Role[];
  disabled: boolean;
  protected: boolean;
  twoFactorEnabled?: boolean;
}

export interface UserConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  createdAt: string;
}

export interface AuditEntry {
  id: string;
  time: string;
  event: string;
  risk?: string;
  result: string;
  connectionId?: string;
  error?: string;
  remoteAddr?: string;
}

export interface AuditPage {
  items: AuditEntry[];
  total: number;
}

export interface InvitationSummary {
  id: string;
  email: string;
  role: string;
  status: string;
  createdAt: string;
  expiresAt: string;
}

export interface InviteResult {
  invitation: InvitationSummary;
  link: string;
  emailSent: boolean;
}

export interface ConnectionDetail {
  id: string;
  name: string;
  protocol: string;
  transport: Transport;
  ownerId: string;
  config: Record<string, unknown>;
  secrets: Record<string, string>;
  credentials?: Record<string, CredentialRefState>;
  recording?: Record<string, string>;
  aiMode?: string;
  aiAllowDestructive?: boolean;
}

export interface CredentialRefState {
  state: "set" | "not_set";
  readable: boolean;
  summary?: CredentialSummary;
}

export interface SortKey {
  field: string;
  desc?: boolean;
}

export interface PageRequest {
  cursor?: string;
  limit?: number;
  filter?: Record<string, string>;
  sort?: SortKey[];
}

export interface Page<T> {
  items: T[];
  nextCursor: string;
  total?: number;
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

export type EventType = "added" | "updated" | "deleted";

export interface ResourceEvent {
  type: EventType;
  ref: ResourceRef;
  resource?: unknown;
}

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
