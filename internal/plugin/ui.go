package plugin

// PanelType selects which core renderer component a tab/detail panel uses.
type PanelType string

const (
	PanelTerminal      PanelType = "terminal"
	PanelFileBrowser   PanelType = "file_browser"
	PanelTable         PanelType = "table"
	PanelMetrics       PanelType = "metrics"
	PanelLogStream     PanelType = "log_stream"
	PanelCodeEditor    PanelType = "code_editor"
	PanelDocument      PanelType = "document"
	PanelQueryEditor   PanelType = "query_editor"
	PanelRemoteDesktop PanelType = "remote_desktop"
	PanelForm          PanelType = "form"
	PanelEnroll        PanelType = "enroll"
	PanelDashboard     PanelType = "dashboard"

	PanelGraph      PanelType = "graph"
	PanelTrace      PanelType = "trace"
	PanelKV         PanelType = "kv"
	PanelHTTPClient PanelType = "http_client"
)

// StreamKind tags the long-lived channel a panel binds to.
type StreamKind string

const (
	StreamTerminal StreamKind = "terminal"
	StreamLogs     StreamKind = "logs"
	StreamDesktop  StreamKind = "desktop"
	StreamMetrics  StreamKind = "metrics"
	StreamFile     StreamKind = "file"
)

// DataSource binds a panel to a route by id; params interpolate from the active
// resource or static values. The core resolves RouteID + params to a URL.
type DataSource struct {
	RouteID string            `json:"routeId"`
	Method  Method            `json:"method,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// ColumnType selects a cell renderer for a table column.
type ColumnType string

const (
	ColumnText     ColumnType = "text"
	ColumnBadge    ColumnType = "badge"
	ColumnBytes    ColumnType = "bytes"
	ColumnDateTime ColumnType = "datetime"
	ColumnNumber   ColumnType = "number"
	ColumnPercent  ColumnType = "percent"
	ColumnBool     ColumnType = "bool"
	ColumnJSON     ColumnType = "json"
)

type Column struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Sortable bool       `json:"sortable,omitempty"`
	Type     ColumnType `json:"type,omitempty"`
	Width    string     `json:"width,omitempty"`
	// ReadOnly keeps a column non-editable even when its table is Editable
	// (e.g. server-managed values). Nullable lets the inline editor clear a
	// cell to an empty/null value rather than an empty string.
	ReadOnly bool `json:"readOnly,omitempty"`
	Nullable bool `json:"nullable,omitempty"`
	// Precision fixes fraction digits for number/percent cells.
	Precision *int `json:"precision,omitempty"`
	// Severities colors a badge column by value: it maps a lower-cased cell value
	// to a Severity (e.g. "running" -> success). Unmapped values stay neutral;
	// ignored for non-badge columns.
	Severities map[string]Severity `json:"severities,omitempty"`
}

// RowClickAction declares what a click on a table row's body does.
type RowClickAction string

const (
	RowClickNavigate RowClickAction = "navigate" // open the row's ref resource
	RowClickDetail   RowClickAction = "detail"   // open the per-row details dialog
	RowClickSelect   RowClickAction = "select"   // toggle row selection
	RowClickNone     RowClickAction = "none"
)

// TableConfig is the declarative config consumed by the generic table panel.
//
// The editing affordances are plugin-agnostic: when Editable is true and RowKey
// names the columns that uniquely identify a row, the generic data grid offers
// inline cell editing, add-row, and delete-row controls wired to the declared
// Insert/Update/Delete routes. Mutation request bodies are uniform across every
// plugin: Insert sends {"values":{col:val}}, Update sends {"key":{col:val},
// "values":{col:val}}, and Delete sends {"key":{col:val}}.
type TableConfig struct {
	Columns       []Column    `json:"columns,omitempty"`
	ColumnsSource *DataSource `json:"columnsSource,omitempty"`
	Watch         *DataSource `json:"watch,omitempty"`
	ActionIDs     []string    `json:"actionIds,omitempty"`
	RowActionIDs  []string    `json:"rowActionIds,omitempty"`

	// RefreshIntervalMs re-fetches the current page on a cadence and replaces it
	// in place — preferred over Watch for high-churn tables where per-row diffs
	// would flood the client.
	RefreshIntervalMs int `json:"refreshIntervalMs,omitempty"`
	// DefaultSort is the column the table sorts by on first load.
	DefaultSort *SortKey `json:"defaultSort,omitempty"`

	Editable  bool        `json:"editable,omitempty"`
	RowKey    []string    `json:"rowKey,omitempty"`
	Insert    *DataSource `json:"insert,omitempty"`
	Update    *DataSource `json:"update,omitempty"`
	Delete    *DataSource `json:"delete,omitempty"`
	EmptyText string      `json:"emptyText,omitempty"`

	// StagedEdits buffers edits, added rows, and deletions locally so the user
	// reviews them and commits or discards as a batch, rather than each change
	// hitting its route immediately. Opt-in; when off, edits apply on the spot.
	StagedEdits bool `json:"stagedEdits,omitempty"`

	// HiddenColumns lists row field keys to omit when the grid derives its
	// columns from the data (no declared Columns). Lets a plugin keep helper
	// fields out of the view without the renderer hard-coding any field names.
	HiddenColumns []string `json:"hiddenColumns,omitempty"`

	// Exportable opts the table into the generic CSV/JSON export of loaded rows.
	// Off by default so a plugin must deliberately allow data to leave the grid.
	Exportable bool `json:"exportable,omitempty"`

	// RowClick overrides the automatic row-body click (navigate a navigable row,
	// else select); empty uses that default.
	RowClick RowClickAction `json:"rowClick,omitempty"`
}

func (c TableConfig) Map() map[string]any {
	out := map[string]any{}
	if len(c.Columns) > 0 {
		out["columns"] = c.Columns
	}
	if c.ColumnsSource != nil {
		out["columnsSource"] = c.ColumnsSource
	}
	if c.Watch != nil {
		out["watch"] = c.Watch
	}
	if c.RefreshIntervalMs > 0 {
		out["refreshIntervalMs"] = c.RefreshIntervalMs
	}
	if c.DefaultSort != nil {
		out["defaultSort"] = c.DefaultSort
	}
	if len(c.ActionIDs) > 0 {
		out["actionIds"] = c.ActionIDs
	}
	if len(c.RowActionIDs) > 0 {
		out["rowActionIds"] = c.RowActionIDs
	}
	if c.Editable {
		out["editable"] = true
	}
	if len(c.RowKey) > 0 {
		out["rowKey"] = c.RowKey
	}
	if c.Insert != nil {
		out["insert"] = c.Insert
	}
	if c.Update != nil {
		out["update"] = c.Update
	}
	if c.Delete != nil {
		out["delete"] = c.Delete
	}
	if c.EmptyText != "" {
		out["emptyText"] = c.EmptyText
	}
	if c.StagedEdits {
		out["stagedEdits"] = true
	}
	if len(c.HiddenColumns) > 0 {
		out["hiddenColumns"] = c.HiddenColumns
	}
	if c.Exportable {
		out["exportable"] = true
	}
	if c.RowClick != "" {
		out["rowClick"] = c.RowClick
	}
	return out
}

type FileBrowserConfig struct {
	PathParam       string `json:"pathParam,omitempty"`
	ReadRouteID     string `json:"readRouteId,omitempty"`
	DownloadRouteID string `json:"downloadRouteId,omitempty"`
	WriteRouteID    string `json:"writeRouteId,omitempty"`
	UploadRouteID   string `json:"uploadRouteId,omitempty"`
	MkdirRouteID    string `json:"mkdirRouteId,omitempty"`
	RenameRouteID   string `json:"renameRouteId,omitempty"`
	DeleteRouteID   string `json:"deleteRouteId,omitempty"`
	Writable        bool   `json:"writable,omitempty"`
	MultipleUpload  bool   `json:"multipleUpload,omitempty"`
	MaxUploadBytes  int64  `json:"maxUploadBytes,omitempty"`
	UploadFieldName string `json:"uploadFieldName,omitempty"`
}

func (c FileBrowserConfig) Map() map[string]any {
	out := map[string]any{}
	if c.PathParam != "" {
		out["pathParam"] = c.PathParam
	}
	if c.ReadRouteID != "" {
		out["readRouteId"] = c.ReadRouteID
	}
	if c.DownloadRouteID != "" {
		out["downloadRouteId"] = c.DownloadRouteID
	}
	if c.WriteRouteID != "" {
		out["writeRouteId"] = c.WriteRouteID
	}
	if c.UploadRouteID != "" {
		out["uploadRouteId"] = c.UploadRouteID
	}
	if c.MkdirRouteID != "" {
		out["mkdirRouteId"] = c.MkdirRouteID
	}
	if c.RenameRouteID != "" {
		out["renameRouteId"] = c.RenameRouteID
	}
	if c.DeleteRouteID != "" {
		out["deleteRouteId"] = c.DeleteRouteID
	}
	if c.Writable {
		out["writable"] = true
	}
	if c.MultipleUpload {
		out["multipleUpload"] = true
	}
	if c.MaxUploadBytes > 0 {
		out["maxUploadBytes"] = c.MaxUploadBytes
	}
	if c.UploadFieldName != "" {
		out["uploadFieldName"] = c.UploadFieldName
	}
	return out
}

type FormPanelConfig struct {
	SubmitRouteID string            `json:"submitRouteId,omitempty"`
	SubmitMethod  Method            `json:"submitMethod,omitempty"`
	SubmitLabel   string            `json:"submitLabel,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
}

func (c FormPanelConfig) Map() map[string]any {
	out := map[string]any{}
	if c.SubmitRouteID != "" {
		out["submitRouteId"] = c.SubmitRouteID
	}
	if c.SubmitMethod != "" {
		out["submitMethod"] = c.SubmitMethod
	}
	if c.SubmitLabel != "" {
		out["submitLabel"] = c.SubmitLabel
	}
	if len(c.Params) > 0 {
		out["params"] = c.Params
	}
	return out
}

// DashboardCell is one panel inside a PanelDashboard grid. It mirrors a Tab
// minus the tab-bar semantics: any panel type, its own source/config, and an
// optional Span (>= 2 fills the row). Plugin-agnostic — any plugin can compose
// an at-a-glance view from its existing panels and routes.
type DashboardCell struct {
	Key    string         `json:"key"`
	Label  string         `json:"label,omitempty"`
	Icon   Icon           `json:"icon,omitzero"`
	Panel  PanelType      `json:"panel"`
	Source *DataSource    `json:"source,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	Span   int            `json:"span,omitempty"`
}

// DashboardConfig is the declarative config for a PanelDashboard: a responsive
// grid that renders every cell at once. Usable as a detail/connection Tab panel.
type DashboardConfig struct {
	Cells []DashboardCell `json:"cells,omitempty"`
}

func (c DashboardConfig) Map() map[string]any {
	out := map[string]any{}
	if len(c.Cells) > 0 {
		out["cells"] = c.Cells
	}
	return out
}

// MetricStat is one KPI number card in the metrics panel.
type MetricStat struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
	Unit  string `json:"unit,omitempty"`
}

// MetricGauge is one radial/doughnut gauge (current value vs Max; Max 0 = 100,
// i.e. a percentage).
type MetricGauge struct {
	Key   string  `json:"key"`
	Label string  `json:"label,omitempty"`
	Unit  string  `json:"unit,omitempty"`
	Max   float64 `json:"max,omitempty"`
}

// MetricSeries is one line in the metrics time-series chart.
type MetricSeries struct {
	Key   string `json:"key"`
	Label string `json:"label,omitempty"`
	Unit  string `json:"unit,omitempty"`
}

// MetricsConfig drives the generic metrics panel — KPI stat cards, doughnut
// gauges, and a multi-series time-series — all fed by the JSON frames the
// metrics stream emits (each keyed by the configured field keys). Plugin-
// agnostic: the renderer knows no field names; a plugin declares what to show.
type MetricsConfig struct {
	Stats   []MetricStat   `json:"stats,omitempty"`
	Gauges  []MetricGauge  `json:"gauges,omitempty"`
	Series  []MetricSeries `json:"series,omitempty"`
	History int            `json:"history,omitempty"`
}

func (c MetricsConfig) Map() map[string]any {
	out := map[string]any{}
	if len(c.Stats) > 0 {
		out["stats"] = c.Stats
	}
	if len(c.Gauges) > 0 {
		out["gauges"] = c.Gauges
	}
	if len(c.Series) > 0 {
		out["series"] = c.Series
	}
	if c.History > 0 {
		out["history"] = c.History
	}
	return out
}

type GraphLayout string

const (
	GraphLayoutGrid   GraphLayout = "grid"
	GraphLayoutManual GraphLayout = "manual"
)

type GraphConfig struct {
	Layout  GraphLayout `json:"layout,omitempty"`
	FitView bool        `json:"fitView,omitempty"`
	// ExpandRouteID, when set, makes nodes expandable: the panel fetches a node's
	// neighbourhood from this read route (passing the node id as ExpandParam,
	// default "node") and merges the result into the graph.
	ExpandRouteID string `json:"expandRouteId,omitempty"`
	ExpandParam   string `json:"expandParam,omitempty"`
}

func (c GraphConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Layout != "" {
		out["layout"] = c.Layout
	}
	if c.FitView {
		out["fitView"] = c.FitView
	}
	if c.ExpandRouteID != "" {
		out["expandRouteId"] = c.ExpandRouteID
	}
	if c.ExpandParam != "" {
		out["expandParam"] = c.ExpandParam
	}
	return out
}

type TraceConfig struct {
	ServiceField string `json:"serviceField,omitempty"`
}

func (c TraceConfig) Map() map[string]any {
	out := map[string]any{}
	if c.ServiceField != "" {
		out["serviceField"] = c.ServiceField
	}
	return out
}

type KVConfig struct {
	CreateRouteID string `json:"createRouteId,omitempty"`
	ReadRouteID   string `json:"readRouteId,omitempty"`
	WriteRouteID  string `json:"writeRouteId,omitempty"`
	DeleteRouteID string `json:"deleteRouteId,omitempty"`
	KeyParam      string `json:"keyParam,omitempty"`
	Writable      bool   `json:"writable,omitempty"`
	// ValueTypes are the value kinds a plugin's store supports (e.g. Redis
	// string/hash/list/set/zset). The renderer offers them in the type picker;
	// when empty it shows a plain value editor with no type concept.
	ValueTypes []string `json:"valueTypes,omitempty"`
}

func (c KVConfig) Map() map[string]any {
	out := map[string]any{}
	if c.CreateRouteID != "" {
		out["createRouteId"] = c.CreateRouteID
	}
	if c.ReadRouteID != "" {
		out["readRouteId"] = c.ReadRouteID
	}
	if c.WriteRouteID != "" {
		out["writeRouteId"] = c.WriteRouteID
	}
	if c.DeleteRouteID != "" {
		out["deleteRouteId"] = c.DeleteRouteID
	}
	if c.KeyParam != "" {
		out["keyParam"] = c.KeyParam
	}
	if c.Writable {
		out["writable"] = c.Writable
	}
	if len(c.ValueTypes) > 0 {
		out["valueTypes"] = c.ValueTypes
	}
	return out
}

// TerminalConfig opts a terminal panel into extra controls. Off by default so a
// plugin enables only what its terminal needs.
type TerminalConfig struct {
	Zoom   bool `json:"zoom,omitempty"`   // font-size +/- controls and Ctrl/⌘ +/-/0
	Search bool `json:"search,omitempty"` // scrollback find with match navigation
}

func (c TerminalConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Zoom {
		out["zoom"] = true
	}
	if c.Search {
		out["search"] = true
	}
	return out
}

type CodeEditorConfig struct {
	Language       string            `json:"language,omitempty"`
	InitialContent string            `json:"initialContent,omitempty"`
	SaveRouteID    string            `json:"saveRouteId,omitempty"`
	SaveMethod     Method            `json:"saveMethod,omitempty"`
	SaveParams     map[string]string `json:"saveParams,omitempty"`
	SaveBodyKey    string            `json:"saveBodyKey,omitempty"`
	SaveExtra      map[string]any    `json:"saveExtra,omitempty"`
}

type QueryEditorConfig struct {
	Language          string            `json:"language,omitempty"`
	Label             string            `json:"label,omitempty"`
	ExecuteLabel      string            `json:"executeLabel,omitempty"`
	CancelLabel       string            `json:"cancelLabel,omitempty"`
	RunningLabel      string            `json:"runningLabel,omitempty"`
	EmptyText         string            `json:"emptyText,omitempty"`
	InitialQuery      string            `json:"initialQuery,omitempty"`
	CancelRouteID     string            `json:"cancelRouteId,omitempty"`
	CancelParams      map[string]string `json:"cancelParams,omitempty"`
	CompletionRouteID string            `json:"completionRouteId,omitempty"`
	CompletionParams  map[string]string `json:"completionParams,omitempty"`
	Exportable        bool              `json:"exportable,omitempty"`
}

func (c CodeEditorConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Language != "" {
		out["language"] = c.Language
	}
	if c.InitialContent != "" {
		out["initialContent"] = c.InitialContent
	}
	if c.SaveRouteID != "" {
		out["saveRouteId"] = c.SaveRouteID
	}
	if c.SaveMethod != "" {
		out["saveMethod"] = c.SaveMethod
	}
	if len(c.SaveParams) > 0 {
		out["saveParams"] = c.SaveParams
	}
	if c.SaveBodyKey != "" {
		out["saveBodyKey"] = c.SaveBodyKey
	}
	if len(c.SaveExtra) > 0 {
		out["saveExtra"] = c.SaveExtra
	}
	return out
}

func (c QueryEditorConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Language != "" {
		out["language"] = c.Language
	}
	if c.Label != "" {
		out["label"] = c.Label
	}
	if c.ExecuteLabel != "" {
		out["executeLabel"] = c.ExecuteLabel
	}
	if c.CancelLabel != "" {
		out["cancelLabel"] = c.CancelLabel
	}
	if c.RunningLabel != "" {
		out["runningLabel"] = c.RunningLabel
	}
	if c.EmptyText != "" {
		out["emptyText"] = c.EmptyText
	}
	if c.InitialQuery != "" {
		out["initialQuery"] = c.InitialQuery
	}
	if c.CancelRouteID != "" {
		out["cancelRouteId"] = c.CancelRouteID
	}
	if len(c.CancelParams) > 0 {
		out["cancelParams"] = c.CancelParams
	}
	if c.CompletionRouteID != "" {
		out["completionRouteId"] = c.CompletionRouteID
	}
	if len(c.CompletionParams) > 0 {
		out["completionParams"] = c.CompletionParams
	}
	if c.Exportable {
		out["exportable"] = true
	}
	return out
}

type HeaderDefault struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type HTTPClientConfig struct {
	ExecuteRouteID string          `json:"executeRouteId,omitempty"`
	Methods        []string        `json:"methods,omitempty"`
	DefaultMethod  string          `json:"defaultMethod,omitempty"`
	DefaultURL     string          `json:"defaultUrl,omitempty"`
	DefaultHeaders []HeaderDefault `json:"defaultHeaders,omitempty"`
	DefaultBody    string          `json:"defaultBody,omitempty"`
}

func (c HTTPClientConfig) Map() map[string]any {
	out := map[string]any{}
	if c.ExecuteRouteID != "" {
		out["executeRouteId"] = c.ExecuteRouteID
	}
	if len(c.Methods) > 0 {
		out["methods"] = c.Methods
	}
	if c.DefaultMethod != "" {
		out["defaultMethod"] = c.DefaultMethod
	}
	if c.DefaultURL != "" {
		out["defaultUrl"] = c.DefaultURL
	}
	if len(c.DefaultHeaders) > 0 {
		out["defaultHeaders"] = c.DefaultHeaders
	}
	if c.DefaultBody != "" {
		out["defaultBody"] = c.DefaultBody
	}
	return out
}

type RemoteDesktopConfig struct {
	Resize     bool   `json:"resize,omitempty"`
	Clipboard  bool   `json:"clipboard,omitempty"`
	Audio      bool   `json:"audio,omitempty"`
	RepeaterID string `json:"repeaterID,omitempty"`
}

func (c RemoteDesktopConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Resize {
		out["resize"] = c.Resize
	}
	if c.Clipboard {
		out["clipboard"] = c.Clipboard
	}
	if c.Audio {
		out["audio"] = c.Audio
	}
	if c.RepeaterID != "" {
		out["repeaterID"] = c.RepeaterID
	}
	return out
}

// Severity styles a badge.
type Severity string

const (
	SeverityInfo      Severity = "info"
	SeveritySuccess   Severity = "success"
	SeverityWarn      Severity = "warn"
	SeverityDanger    Severity = "danger"
	SeveritySecondary Severity = "secondary"
)

type Badge struct {
	Source   *DataSource `json:"source,omitempty"`
	Value    any         `json:"value,omitempty"`
	Severity Severity    `json:"severity,omitempty"`
}

// ResourceRef is a managed object's stable identity vs display label. Scope is
// an optional outer container the resource belongs to (e.g. a database or
// cluster) for hierarchies deeper than namespace/name; it interpolates as
// ${resource.scope} wherever ${resource.namespace} does.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Scope     string `json:"scope,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// ResourceEvent is emitted by watch streams to patch a resource list.
type ResourceEvent struct {
	Type     string      `json:"type"`
	Ref      ResourceRef `json:"ref"`
	Resource any         `json:"resource,omitempty"`
}

// Tab is one connection-level or resource-level panel.
type Tab struct {
	Key    string         `json:"key"`
	Label  string         `json:"label"`
	Icon   Icon           `json:"icon,omitzero"`
	Panel  PanelType      `json:"panel"`
	Source *DataSource    `json:"source,omitempty"`
	Config map[string]any `json:"config,omitempty"`
	// Span is a sizing hint for the dashboard layout only: a value >= 2 makes
	// the panel fill the row; otherwise it occupies one grid column. Other
	// layouts ignore it.
	Span int `json:"span,omitempty"`
}

// TreeGroup is a connection-level sidebar root, loaded lazily.
//
// A group with a Source is expandable: its children are fetched on expand. Omit
// Source to make it a leaf — a direct destination that opens on click with no
// expandable children: set ResourceKind to open that kind's list, or Ref to open
// a specific resource's detail (e.g. a single dashboard/landing view).
type TreeGroup struct {
	Key          string       `json:"key"`
	Label        string       `json:"label"`
	Icon         Icon         `json:"icon,omitzero"`
	Source       DataSource   `json:"source,omitzero"`
	ResourceKind string       `json:"resourceKind,omitempty"`
	Ref          *ResourceRef `json:"ref,omitempty"`
	Badge        *Badge       `json:"badge,omitempty"`
}

// TreeNode is one node returned by a tree DataSource.
type TreeNode struct {
	Key            string       `json:"key"`
	Label          string       `json:"label"`
	Icon           Icon         `json:"icon,omitzero"`
	Ref            *ResourceRef `json:"ref,omitempty"`
	Leaf           bool         `json:"leaf,omitempty"`
	ChildrenSource *DataSource  `json:"childrenSource,omitempty"`
	Badge          *Badge       `json:"badge,omitempty"`
	// ResourceKind makes the node open that resource type's list view (like a
	// top-level tree group) instead of a single-resource detail — for nesting a
	// category that drills into a kind list.
	ResourceKind string `json:"resourceKind,omitempty"`
	// ListParams scope that list (merged into the resource's list DataSource
	// params), e.g. a namespace — so a nested node opens a filtered list.
	ListParams map[string]string `json:"listParams,omitempty"`
	// Data carries the node's row fields so a tree-opened detail matches a table
	// row (status badge, action gating).
	Data map[string]any `json:"data,omitempty"`
}

// OpenTarget selects where an action's result surfaces. The default (view) runs
// the route (or its form/confirm); Dock opens a panel in the workspace dock;
// Dialog opens a panel in a modal.
type OpenTarget string

const (
	OpenView   OpenTarget = "view"
	OpenDock   OpenTarget = "dock"
	OpenDialog OpenTarget = "dialog"
	// OpenURL runs the action's route, which returns {"url": "..."}, and opens
	// that URL in a new browser tab. The route decides the URL.
	OpenURL OpenTarget = "url"
)

// Action is a UI affordance over a route. Permission/risk/input live on the
// route (single source of truth); this only references it plus UI metadata.
type Action struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Icon        Icon              `json:"icon,omitzero"`
	RouteID     string            `json:"routeId"`
	Params      map[string]string `json:"params,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
	ConfirmText string            `json:"confirmText,omitempty"`
	OnSuccess   *ActionSuccess    `json:"onSuccess,omitempty"`
	// Open=OpenDock/OpenDialog makes the action open Panel (a generic panel type,
	// e.g. terminal/log_stream/code_editor) in the workspace dock or a modal,
	// sourced from the action's route — instead of executing the route inline.
	Open  OpenTarget `json:"open,omitempty"`
	Panel PanelType  `json:"panel,omitempty"`
	// Config is the panel config for a dock/dialog-opened Panel (e.g. a
	// code_editor's saveRouteId), so an action can open an editable panel.
	Config map[string]any `json:"config,omitempty"`
	// EnabledWhen gates the button on the active row's fields (e.g. state ==
	// "running"); false shows it disabled, not hidden. Empty = always enabled.
	EnabledWhen *Condition `json:"enabledWhen,omitempty"`
}

// NavigateTarget is where the UI moves after an action succeeds.
type NavigateTarget string

// NavigateList returns from a resource detail to its list — e.g. after a delete,
// so the now-gone resource's detail doesn't linger.
const NavigateList NavigateTarget = "list"

type ActionSuccess struct {
	SelectTab string `json:"selectTab,omitempty"`
	// Navigate moves the workbench after success (e.g. a deleted resource's detail
	// returns to the list). Empty leaves the current view in place.
	Navigate NavigateTarget `json:"navigate,omitempty"`
}

// Stream is a long-lived channel a panel binds to, pointing at a WS route.
type Stream struct {
	ID      string     `json:"id"`
	Kind    StreamKind `json:"kind"`
	RouteID string     `json:"routeId"`
}

// HeaderSpec configures a resource DetailView header.
type HeaderSpec struct {
	Title       string `json:"title,omitempty"`
	StatusField string `json:"statusField,omitempty"`
	// Severities colors the status badge by value (same value->severity map as a
	// badge Column); unmapped values stay neutral.
	Severities map[string]Severity `json:"severities,omitempty"`
	ActionIDs  []string            `json:"actionIds,omitempty"`
}

// DetailView is opened when a resource row is clicked.
type DetailView struct {
	Header     HeaderSpec `json:"header"`
	DefaultTab string     `json:"defaultTab,omitempty"`
	Tabs       []Tab      `json:"tabs"`
}

// ResourceType is a managed object type: columns, actions, detail.
type ResourceType struct {
	Kind  string      `json:"kind"`
	Title string      `json:"title"`
	List  DataSource  `json:"list"`
	Watch *DataSource `json:"watch,omitempty"`
	// Columns are the static list columns. Leave empty and set ColumnsSource to
	// derive columns at runtime (e.g. a CRD's own printer columns).
	Columns []Column `json:"columns"`
	// ColumnsSource is an optional route returning column definitions (rows with
	// name/label) for lists whose columns are only known at runtime. The list's
	// scoping params are merged in, so one generic type can serve many shapes.
	ColumnsSource *DataSource `json:"columnsSource,omitempty"`
	ActionIDs     []string    `json:"actionIds"`
	ListActionIDs []string    `json:"listActionIds,omitempty"`
	RowActionIDs  []string    `json:"rowActionIds,omitempty"`
	Detail        DetailView  `json:"detail"`
	// Filters are toolbar controls (e.g. a namespace selector) that scope the list
	// by setting a list route param.
	Filters []ResourceFilter `json:"filters,omitempty"`
}

// ResourceFilter is a list toolbar control that sets a route param (Param) to a
// chosen value. Choices come from OptionsSource rows (ValueField/LabelField) or
// static Options; an empty choice (AllLabel) clears the filter.
type ResourceFilter struct {
	Key           string         `json:"key"`
	Label         string         `json:"label"`
	Param         string         `json:"param"`
	OptionsSource *DataSource    `json:"optionsSource,omitempty"`
	Options       []FilterOption `json:"options,omitempty"`
	ValueField    string         `json:"valueField,omitempty"`
	LabelField    string         `json:"labelField,omitempty"`
	AllLabel      string         `json:"allLabel,omitempty"`
}

type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}
