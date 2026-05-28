// The single FE/BE contract: the browser projection the renderer consumes.
// Mirrors the Go manifest projection. Carries no server-only fields
// (handlers, raw mount paths, permission keys, audit-event names).

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

export type Layout = "tabs" | "sidebar_tree" | "dashboard";

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

// Open union: the renderer must handle a type it does not recognize.
export type PanelType = KnownPanelType | (string & {});

export type StreamKind = "terminal" | "logs" | "desktop" | "metrics" | "file";

// Recording (plugin-declared capability + per-connection policy)

export type RecordingClass = "terminal" | "desktop";

export type RecordingFormat = "asciicast_v2" | "webm_canvas";

export type RecordingPolicy = "disabled" | "manual" | "auto";

// What a plugin can record for one stream class. The browser never sees the
// server-only stream binding — only which classes/formats are offered.
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

// Schema & declarative form

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
  | "credential_ref";

export interface Option {
  label: string;
  value: string | number | boolean;
}

// Credential kinds are registry data, not a frontend enum.
export type CredentialKind = string;

export interface CredentialKindInfo {
  kind: CredentialKind;
  label: string;
  secretLabel: string;
  secretMultiline?: boolean;
  identityLabel?: string;
  compatibleProtocols?: string[];
}

// Describes which reusable credentials a `credential_ref` field accepts. The
// field carries only the chosen credential's id — never secret material.
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
  // Encrypted at rest, write-only over the API: never carries a value back.
  secret?: boolean;
  default?: unknown;
  placeholder?: string;
  help?: string;
  options?: Option[];
  // Populates choices from a route at form-open time (rows -> {value,label});
  // its params interpolate ${resource.*} from the form's resource context.
  optionsSource?: DataSource;
  credential?: CredentialSelector;
  visibleWhen?: Condition;
  validators?: Validator[];
  // Increment for number/slider inputs (defaults to 1); min/max come from the
  // min/max validators.
  step?: number;
}

export interface Group {
  name: string;
  fields: Field[];
}

export interface Schema {
  groups: Group[];
}

// Data binding

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
  writable?: boolean;
  multipleUpload?: boolean;
  maxUploadBytes?: number;
  uploadFieldName?: string;
}

export interface TablePanelConfig {
  columns?: Column[];
  columnsSource?: DataSource;
  watch?: DataSource;
  actionIds?: string[];
  rowActionIds?: string[];
  // Inline data-grid editing (plugin-agnostic). When `editable` is set and
  // `rowKey` names the identifying columns, the grid offers cell editing,
  // add-row, and delete-row wired to these mutation routes. Bodies are uniform:
  // insert {values}, update {key, values}, delete {key}.
  editable?: boolean;
  rowKey?: string[];
  insert?: DataSource;
  update?: DataSource;
  delete?: DataSource;
  emptyText?: string;
  // Opt-in: buffer edits/inserts/deletes locally for review and commit or
  // discard them as a batch instead of applying each change immediately.
  stagedEdits?: boolean;
  // Row field keys to omit when the grid derives columns from the data.
  hiddenColumns?: string[];
  // Opt-in: show the generic CSV/JSON export control for loaded rows.
  exportable?: boolean;
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
  // Opt-in: show the CSV/JSON export control for query results.
  exportable?: boolean;
}

export interface GraphPanelConfig {
  layout?: "grid" | "manual";
  fitView?: boolean;
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
  | "bool"
  | "json";

export interface Column {
  key: string;
  label: string;
  sortable?: boolean;
  type?: ColumnType;
  width?: string;
  // readOnly keeps a column non-editable even when its table is editable.
  // nullable lets the inline editor clear the cell to an empty/null value.
  readOnly?: boolean;
  nullable?: boolean;
  // Maps a lower-cased badge value to a severity for color (e.g. running →
  // success); unmapped values render neutral.
  severities?: Record<string, Severity>;
}

export type Severity = "info" | "success" | "warn" | "danger" | "secondary";

export interface Badge {
  source?: DataSource;
  value?: string | number;
  severity?: Severity;
}

// Resources

export interface ResourceRef {
  kind: string;
  // Optional outer container (e.g. database/cluster) for hierarchies deeper than
  // namespace/name. Interpolates as ${resource.scope}.
  scope?: string;
  namespace?: string;
  name: string;
  uid: string;
}

export interface ActionSuccess {
  selectTab?: string;
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
  // open="dock"/"dialog" opens `panel` (sourced from this action's route) in the
  // workspace dock or a modal instead of executing the route inline.
  open?: "view" | "dock" | "dialog" | "url";
  panel?: PanelType;
  config?: Record<string, unknown>; // panel config for a dock/dialog-opened panel
  enabledWhen?: Condition; // gate on the active row's fields; false = disabled
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
  // Dashboard-layout sizing hint: >= 2 fills the row, otherwise one column.
  span?: number;
}

// One panel inside a `dashboard` panel grid (mirrors a Tab minus tab-bar
// semantics). Generic — any plugin composes an at-a-glance view from its panels.
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
  // A group with a source is expandable (children load on expand). Omit it for a
  // leaf that opens directly: resourceKind opens that kind's list, ref opens a
  // specific resource's detail.
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
  // Opens the resource type's list view (like a top-level group) instead of a
  // single-resource detail; listParams scope that list (e.g. a namespace).
  resourceKind?: string;
  listParams?: Record<string, string>;
  data?: Record<string, unknown>; // row fields, so a tree-opened detail matches a table row
}

export interface HeaderSpec {
  title?: string;
  statusField?: string;
  // Colors the status badge by value (same value->severity map as a badge column).
  severities?: Record<string, Severity>;
  actionIds?: string[];
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
  columnsSource?: DataSource; // runtime column defs when columns is empty (e.g. CRDs)
  actionIds: string[];
  listActionIds?: string[];
  rowActionIds?: string[];
  detail: DetailView;
}

// Plugin projection

export interface AgentProfile {
  modes: string[];
  riskNote?: string;
}

export interface CredentialSummary {
  id: string;
  name: string;
  kind: CredentialKind;
  ownerId?: string;
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
  streams?: Stream[];
  recording?: RecordingCapability[];
}

// Connection instances (stored configs the user reaches the plugin through)

export interface ConnectionSummary {
  id: string;
  name: string;
  protocol: string;
  icon?: Icon;
  transport: Transport;
  // online gates the agent enroll panel; "offline" (agent with no tunnel) shows
  // a red dot. The green "connected" state is client-side (the connect gate).
  online?: boolean;
  status?: "offline";
  canManage?: boolean;
  access?: "owner" | "admin" | GrantAccess;
  owned?: boolean;
  sharedWithMe?: boolean;
  sharedByMe?: boolean;
  recording?: Record<string, string>;
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

// A sharing grant on a connection or credential, with the subject resolved for
// display. Never carries secret material.
export interface ShareGrant {
  id: string;
  subjectId: string;
  username?: string;
  displayName?: string;
  access: GrantAccess;
}

// Minimal subject record returned by the user-lookup endpoint for grant assignment.
export interface UserSummary {
  id: string;
  username: string;
  displayName?: string;
}

// Admin account management.
export interface AdminUser {
  id: string;
  username: string;
  email?: string;
  displayName?: string;
  roles: string[];
  disabled: boolean;
  protected: boolean;
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

// The edit/detail read: non-secret config plus a per-secret-field presence map
// ("set" / "not set"). Secret values are never carried back.
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
}

export interface CredentialRefState {
  state: "set" | "not_set";
  readable: boolean;
  summary?: CredentialSummary;
}

// Lists, pagination, watch

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

// A table row. Beyond display columns, a row may carry reserved framework keys
// the generic grid understands (and never renders as columns):
//   `ref`   — the row's own resource identity (row-click navigation).
//   `_key`  — opaque key map identifying the row for inline edit/delete.
//   `_links`— map of column key -> related resource ref; the grid renders those
//             cells as links that open the related resource.
export type Row = Record<string, unknown> & {
  ref?: ResourceRef;
  _key?: Record<string, unknown>;
  _links?: Record<string, ResourceRef>;
};

// One entry in a file_browser directory listing.
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

// Returned by a file_browser readRouteId for inline preview.
export interface FileContent {
  path: string;
  mime?: string;
  encoding?: "utf8" | "base64" | "url";
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

// Agent enrollment

export interface InstallArtifact {
  label: string;
  kind: string;
  command?: string;
  url?: string;
}

export interface Enrollment {
  enrollmentId: string;
  expiresAt: string;
  artifacts: InstallArtifact[];
}

export type AgentStatus = "pending" | "online" | "offline" | "error";

export interface AgentState {
  status: AgentStatus;
  message?: string;
}
