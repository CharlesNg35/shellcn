package plugin

// PanelType selects which core renderer component a tab/detail panel uses.
type PanelType string

const (
	PanelTerminal      PanelType = "terminal"
	PanelTerminalGrid  PanelType = "terminal_grid"
	PanelFileBrowser   PanelType = "file_browser"
	PanelTable         PanelType = "table"
	PanelMetrics       PanelType = "metrics"
	PanelLogStream     PanelType = "log_stream"
	PanelCodeEditor    PanelType = "code_editor"
	PanelDiff          PanelType = "diff"
	PanelDocument      PanelType = "document"
	PanelQueryEditor   PanelType = "query_editor"
	PanelRemoteDesktop PanelType = "remote_desktop"
	PanelForm          PanelType = "form"
	PanelEnroll        PanelType = "enroll"
	PanelDashboard     PanelType = "dashboard"
	PanelObjectDetail  PanelType = "object_detail"
	PanelTimeline      PanelType = "timeline"
	PanelTaskProgress  PanelType = "task_progress"
	PanelSplit         PanelType = "split"
	PanelCanvas        PanelType = "canvas"

	PanelGraph      PanelType = "graph"
	PanelTrace      PanelType = "trace"
	PanelKV         PanelType = "kv"
	PanelHTTPClient PanelType = "http_client"
)

// PanelConfig is closed to this package so config fields cannot accept arbitrary
// data.
type PanelConfig interface{ panelConfig() }

func (TableConfig) panelConfig()         {}
func (FileBrowserConfig) panelConfig()   {}
func (FormPanelConfig) panelConfig()     {}
func (DashboardConfig) panelConfig()     {}
func (MetricsConfig) panelConfig()       {}
func (GraphConfig) panelConfig()         {}
func (TraceConfig) panelConfig()         {}
func (KVConfig) panelConfig()            {}
func (TerminalConfig) panelConfig()      {}
func (TerminalGridConfig) panelConfig()  {}
func (CodeEditorConfig) panelConfig()    {}
func (DiffConfig) panelConfig()          {}
func (QueryEditorConfig) panelConfig()   {}
func (HTTPClientConfig) panelConfig()    {}
func (RemoteDesktopConfig) panelConfig() {}
func (ObjectDetailConfig) panelConfig()  {}
func (TimelineConfig) panelConfig()      {}
func (TaskProgressConfig) panelConfig()  {}
func (SplitConfig) panelConfig()         {}
func (CanvasConfig) panelConfig()        {}

// StreamKind tags the long-lived channel a panel binds to.
type StreamKind string

const (
	StreamTerminal StreamKind = "terminal"
	StreamLogs     StreamKind = "logs"
	StreamDesktop  StreamKind = "desktop"
	StreamMetrics  StreamKind = "metrics"
	StreamFile     StreamKind = "file"
	StreamTask     StreamKind = "task"
	StreamCanvas   StreamKind = "canvas"
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
	ColumnText         ColumnType = "text"
	ColumnBadge        ColumnType = "badge"
	ColumnBytes        ColumnType = "bytes"
	ColumnDateTime     ColumnType = "datetime"
	ColumnRelativeTime ColumnType = "relative_time"
	ColumnNumber       ColumnType = "number"
	ColumnPercent      ColumnType = "percent"
	ColumnBool         ColumnType = "bool"
	ColumnJSON         ColumnType = "json"
	ColumnIcon         ColumnType = "icon"
)

type Column struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Sortable bool       `json:"sortable,omitempty"`
	Type     ColumnType `json:"type,omitempty"`
	Width    string     `json:"width,omitempty"`
	// ReadOnly keeps server-managed values out of the inline editor. Nullable
	// lets the editor clear a cell to empty/null.
	ReadOnly bool `json:"readOnly,omitempty"`
	Nullable bool `json:"nullable,omitempty"`
	// Precision fixes fraction digits for number/percent cells.
	Precision *int `json:"precision,omitempty"`
	// Severities maps lower-cased badge values to colors.
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

// TableConfig drives the generic table panel. Editable tables use RowKey plus
// Insert/Update/Delete routes with uniform mutation bodies.
type TableConfig struct {
	Columns       []Column    `json:"columns,omitempty"`
	ColumnsSource *DataSource `json:"columnsSource,omitempty"`
	Watch         *DataSource `json:"watch,omitempty"`
	ActionIDs     []string    `json:"actionIds,omitempty"`
	RowActionIDs  []string    `json:"rowActionIds,omitempty"`
	// Selectable makes rows selectable (checkboxes) even without RowActionIDs —
	// for a browse table where actions live in the detail view, not a row bar.
	// Declaring RowActionIDs implies it.
	Selectable bool `json:"selectable,omitempty"`

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

	// StagedEdits batches local row edits until the user commits or discards them.
	StagedEdits bool `json:"stagedEdits,omitempty"`

	// HiddenColumns omits helper fields when columns are inferred from row data.
	HiddenColumns []string `json:"hiddenColumns,omitempty"`

	// Exportable opts the table into the generic CSV/JSON export of loaded rows.
	// Off by default so a plugin must deliberately allow data to leave the grid.
	Exportable bool `json:"exportable,omitempty"`

	// RowClick overrides the automatic row-body click (navigate a navigable row,
	// else select); empty uses that default.
	RowClick RowClickAction `json:"rowClick,omitempty"`
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
	// Bulk-operation slots over a multi-selection; each slot is optional.
	MoveRouteID     string `json:"moveRouteId,omitempty"`
	CopyRouteID     string `json:"copyRouteId,omitempty"`
	ChmodRouteID    string `json:"chmodRouteId,omitempty"`
	ArchiveRouteID  string `json:"archiveRouteId,omitempty"`
	Writable        bool   `json:"writable,omitempty"`
	MultipleUpload  bool   `json:"multipleUpload,omitempty"`
	MaxUploadBytes  int64  `json:"maxUploadBytes,omitempty"`
	UploadFieldName string `json:"uploadFieldName,omitempty"`
}

type FormPanelConfig struct {
	SubmitRouteID string            `json:"submitRouteId,omitempty"`
	SubmitMethod  Method            `json:"submitMethod,omitempty"`
	SubmitLabel   string            `json:"submitLabel,omitempty"`
	Params        map[string]string `json:"params,omitempty"`
}

// DashboardConfig renders multiple panels in one responsive grid.
type DashboardConfig struct {
	Cells []Panel `json:"cells,omitempty"`
}

type ObjectDetailField struct {
	Key        string              `json:"key"`
	Label      string              `json:"label,omitempty"`
	Type       ColumnType          `json:"type,omitempty"`
	Copy       bool                `json:"copy,omitempty"`
	Redacted   bool                `json:"redacted,omitempty"`
	Severities map[string]Severity `json:"severities,omitempty"`
}

type ObjectDetailSection struct {
	Title  string              `json:"title,omitempty"`
	Fields []ObjectDetailField `json:"fields,omitempty"`
}

type ObjectDetailConfig struct {
	Sections  []ObjectDetailSection `json:"sections,omitempty"`
	RawToggle bool                  `json:"rawToggle,omitempty"`
}

type TimelineConfig struct {
	TimestampField    string `json:"timestampField,omitempty"`
	TitleField        string `json:"titleField,omitempty"`
	BodyField         string `json:"bodyField,omitempty"`
	SeverityField     string `json:"severityField,omitempty"`
	IconField         string `json:"iconField,omitempty"`
	ResourceField     string `json:"resourceField,omitempty"`
	EmptyText         string `json:"emptyText,omitempty"`
	RefreshIntervalMs int    `json:"refreshIntervalMs,omitempty"`
}

type TaskProgressConfig struct {
	Title         string `json:"title,omitempty"`
	CancelRouteID string `json:"cancelRouteId,omitempty"`
	RetryRouteID  string `json:"retryRouteId,omitempty"`
}

type SplitOrientation string

const (
	SplitHorizontal SplitOrientation = "horizontal"
	SplitVertical   SplitOrientation = "vertical"
)

type SplitPanel struct {
	Panel
	Size    int `json:"size,omitempty"`
	MinSize int `json:"minSize,omitempty"`
}

type SplitConfig struct {
	Orientation SplitOrientation `json:"orientation,omitempty"`
	Panels      []SplitPanel     `json:"panels,omitempty"`
}

type CanvasConfig struct {
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	HiDPI          bool   `json:"hidpi,omitempty"`
	Interactive    bool   `json:"interactive,omitempty"`
	Keyboard       bool   `json:"keyboard,omitempty"`
	Pointer        bool   `json:"pointer,omitempty"`
	Wheel          bool   `json:"wheel,omitempty"`
	ResizeEvents   bool   `json:"resizeEvents,omitempty"`
	Background     string `json:"background,omitempty"`
	FocusOnPointer bool   `json:"focusOnPointer,omitempty"`
	AriaLabel      string `json:"ariaLabel,omitempty"`
	Instructions   string `json:"instructions,omitempty"`
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

// MetricsConfig selects the stat, gauge, and series keys rendered from metric
// stream frames.
type MetricsConfig struct {
	Stats   []MetricStat   `json:"stats,omitempty"`
	Gauges  []MetricGauge  `json:"gauges,omitempty"`
	Series  []MetricSeries `json:"series,omitempty"`
	History int            `json:"history,omitempty"`
}

type GraphLayout string

const (
	GraphLayoutGrid   GraphLayout = "grid"
	GraphLayoutManual GraphLayout = "manual"
)

type GraphConfig struct {
	Layout  GraphLayout `json:"layout,omitempty"`
	FitView bool        `json:"fitView,omitempty"`
	// ExpandRouteID makes nodes expandable through a read route.
	ExpandRouteID string `json:"expandRouteId,omitempty"`
	ExpandParam   string `json:"expandParam,omitempty"`
	// Exportable controls client-side graph image export. Nil means enabled.
	Exportable *bool `json:"exportable,omitempty"`
}

type TraceConfig struct {
	ServiceField string `json:"serviceField,omitempty"`
}

type KVConfig struct {
	CreateRouteID string `json:"createRouteId,omitempty"`
	ReadRouteID   string `json:"readRouteId,omitempty"`
	WriteRouteID  string `json:"writeRouteId,omitempty"`
	DeleteRouteID string `json:"deleteRouteId,omitempty"`
	KeyParam      string `json:"keyParam,omitempty"`
	Writable      bool   `json:"writable,omitempty"`
	// ValueTypes enables a type picker; empty means plain value editing.
	ValueTypes []string `json:"valueTypes,omitempty"`
}

// TerminalConfig opts a terminal panel into extra controls. Off by default so a
// plugin enables only what its terminal needs.
type TerminalConfig struct {
	Zoom   bool `json:"zoom,omitempty"`   // font-size +/- controls and Ctrl/⌘ +/-/0
	Search bool `json:"search,omitempty"` // scrollback find with match navigation
}

type TerminalGridConfig struct {
	MaxPanes     int  `json:"maxPanes,omitempty"`
	DefaultPanes int  `json:"defaultPanes,omitempty"`
	Zoom         bool `json:"zoom,omitempty"`
	Search       bool `json:"search,omitempty"`
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

type DiffMode string

const (
	DiffSideBySide DiffMode = "side_by_side"
	DiffUnified    DiffMode = "unified"
)

type DiffConfig struct {
	Language          string   `json:"language,omitempty"`
	OriginalField     string   `json:"originalField,omitempty"`
	ModifiedField     string   `json:"modifiedField,omitempty"`
	OriginalLabel     string   `json:"originalLabel,omitempty"`
	ModifiedLabel     string   `json:"modifiedLabel,omitempty"`
	Mode              DiffMode `json:"mode,omitempty"`
	CollapseUnchanged bool     `json:"collapseUnchanged,omitempty"`
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

type RemoteDesktopConfig struct {
	Resize     bool   `json:"resize,omitempty"`
	Clipboard  bool   `json:"clipboard,omitempty"`
	Audio      bool   `json:"audio,omitempty"`
	RepeaterID string `json:"repeaterID,omitempty"`
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

// ResourceRef is a managed object's stable identity and display label. Scope is
// an optional outer container for hierarchies deeper than namespace/name.
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

// Panel is a renderable tab, detail panel, or dashboard cell.
type Panel struct {
	Key    string      `json:"key"`
	Label  string      `json:"label,omitempty"`
	Icon   Icon        `json:"icon,omitzero"`
	Type   PanelType   `json:"panel"`
	Source *DataSource `json:"source,omitempty"`
	Config PanelConfig `json:"config,omitempty"`
	// Span is a dashboard-only sizing hint.
	Span int `json:"span,omitempty"`
}

// TreeGroup is a lazy connection-sidebar root. With Source it expands; without
// Source it opens ResourceKind or Ref directly.
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
	// ResourceKind opens a resource list instead of a single-resource detail.
	ResourceKind string `json:"resourceKind,omitempty"`
	// ListParams merge into the resource list DataSource params.
	ListParams map[string]string `json:"listParams,omitempty"`
	// Data carries row fields for status badges and action gating.
	Data map[string]any `json:"data,omitempty"`
}

// OpenTarget selects where an action's result surfaces.
type OpenTarget string

const (
	OpenView   OpenTarget = "view"
	OpenDock   OpenTarget = "dock"
	OpenDialog OpenTarget = "dialog"
	// OpenURL opens the route-returned URL in a new browser tab.
	OpenURL OpenTarget = "url"
)

// Action is a UI affordance over a route.
type Action struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Icon        Icon              `json:"icon,omitzero"`
	RouteID     string            `json:"routeId"`
	Params      map[string]string `json:"params,omitempty"`
	Confirm     bool              `json:"confirm,omitempty"`
	ConfirmText string            `json:"confirmText,omitempty"`
	OnSuccess   *ActionSuccess    `json:"onSuccess,omitempty"`
	// OpenDock/OpenDialog opens a panel in the workspace dock or a modal.
	Open  OpenTarget `json:"open,omitempty"`
	Panel PanelType  `json:"panel,omitempty"`
	// Config is the panel config for dock/dialog-opened panels.
	Config PanelConfig `json:"config,omitempty"`
	// EnabledWhen disables the button unless active-row fields match.
	EnabledWhen *Condition `json:"enabledWhen,omitempty"`
	// IconOnly renders the button as its icon alone; Label becomes the tooltip.
	IconOnly bool `json:"iconOnly,omitempty"`
	// Group clusters actions into a labeled dropdown.
	Group string `json:"group,omitempty"`
}

// NavigateTarget is where the UI moves after an action succeeds.
type NavigateTarget string

// NavigateList returns from a resource detail to its list.
const NavigateList NavigateTarget = "list"

type ActionSuccess struct {
	SelectTab string `json:"selectTab,omitempty"`
	// Navigate moves the workbench after success.
	Navigate NavigateTarget `json:"navigate,omitempty"`
}

// Stream is a long-lived channel a panel binds to, pointing at a WS route.
type Stream struct {
	ID      string     `json:"id"`
	Kind    StreamKind `json:"kind"`
	RouteID string     `json:"routeId"`
}

// HeaderSpec configures a resource DetailView header. Detail actions live in
// ResourceActions.Detail, not here.
type HeaderSpec struct {
	Title       string `json:"title,omitempty"`
	StatusField string `json:"statusField,omitempty"`
	// Severities colors the status badge by value (same value->severity map as a
	// badge Column); unmapped values stay neutral.
	Severities map[string]Severity `json:"severities,omitempty"`
}

// ResourceActions groups action IDs by render surface.
type ResourceActions struct {
	Toolbar    []string `json:"toolbar,omitempty"`    // list toolbar, no row context (create, prune)
	Row        []string `json:"row,omitempty"`        // bulk over selected rows (delete); implies Selectable
	Detail     []string `json:"detail,omitempty"`     // the one open resource, in its detail header
	Selectable bool     `json:"selectable,omitempty"` // row checkboxes without a row bar; Row implies it
}

// DetailView is opened when a resource row is clicked.
type DetailView struct {
	Header     HeaderSpec `json:"header"`
	DefaultTab string     `json:"defaultTab,omitempty"`
	Tabs       []Panel    `json:"tabs"`
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
	// Actions groups this resource's actions by render surface (toolbar / row /
	// detail). The single action contract for a resource.
	Actions ResourceActions `json:"actions,omitzero"`
	Detail  DetailView      `json:"detail"`
}

// ScopeControl names the scope filter's input widget. Open vocabulary: the
// renderer falls back to a select for names it doesn't recognize.
type ScopeControl string

const (
	ScopeSelect      ScopeControl = "select" // default
	ScopeMultiSelect ScopeControl = "multiselect"
	ScopeSearch      ScopeControl = "search"
	ScopeToggle      ScopeControl = "toggle" // on sets the first Option's value
)

// ScopeSeparator joins multiselect scope values in one route param.
const ScopeSeparator = ","

// ScopeFilter is a global selector injected into read and stream route params.
type ScopeFilter struct {
	Param         string         `json:"param"`
	Label         string         `json:"label"`
	Icon          Icon           `json:"icon,omitzero"`
	Control       ScopeControl   `json:"control,omitempty"`
	OptionsSource *DataSource    `json:"optionsSource,omitempty"`
	Options       []FilterOption `json:"options,omitempty"`
	ValueField    string         `json:"valueField,omitempty"`
	LabelField    string         `json:"labelField,omitempty"`
	AllLabel      string         `json:"allLabel,omitempty"`
	DefaultValue  string         `json:"defaultValue,omitempty"`
}

type FilterOption struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}
