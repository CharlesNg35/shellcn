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
	ColumnBool     ColumnType = "bool"
)

type Column struct {
	Key      string     `json:"key"`
	Label    string     `json:"label"`
	Sortable bool       `json:"sortable,omitempty"`
	Type     ColumnType `json:"type,omitempty"`
	Width    string     `json:"width,omitempty"`
}

// TableConfig is the declarative config consumed by the generic table panel.
type TableConfig struct {
	Columns      []Column    `json:"columns,omitempty"`
	Watch        *DataSource `json:"watch,omitempty"`
	ActionIDs    []string    `json:"actionIds,omitempty"`
	RowActionIDs []string    `json:"rowActionIds,omitempty"`
}

func (c TableConfig) Map() map[string]any {
	out := map[string]any{}
	if len(c.Columns) > 0 {
		out["columns"] = c.Columns
	}
	if c.Watch != nil {
		out["watch"] = c.Watch
	}
	if len(c.ActionIDs) > 0 {
		out["actionIds"] = c.ActionIDs
	}
	if len(c.RowActionIDs) > 0 {
		out["rowActionIds"] = c.RowActionIDs
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
}

func (c GraphConfig) Map() map[string]any {
	out := map[string]any{}
	if c.Layout != "" {
		out["layout"] = c.Layout
	}
	if c.FitView {
		out["fitView"] = c.FitView
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
	ReadRouteID   string `json:"readRouteId,omitempty"`
	WriteRouteID  string `json:"writeRouteId,omitempty"`
	DeleteRouteID string `json:"deleteRouteId,omitempty"`
	KeyParam      string `json:"keyParam,omitempty"`
	Writable      bool   `json:"writable,omitempty"`
}

func (c KVConfig) Map() map[string]any {
	out := map[string]any{}
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

// ResourceRef is a managed object's stable identity vs display label.
type ResourceRef struct {
	Kind      string `json:"kind"`
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
}

// TreeGroup is a connection-level sidebar root, loaded lazily.
type TreeGroup struct {
	Key    string     `json:"key"`
	Label  string     `json:"label"`
	Icon   Icon       `json:"icon,omitzero"`
	Source DataSource `json:"source"`
	Badge  *Badge     `json:"badge,omitempty"`
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
}

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
}

type ActionSuccess struct {
	SelectTab string `json:"selectTab,omitempty"`
}

// Stream is a long-lived channel a panel binds to, pointing at a WS route.
type Stream struct {
	ID      string     `json:"id"`
	Kind    StreamKind `json:"kind"`
	RouteID string     `json:"routeId"`
}

// HeaderSpec configures a resource DetailView header.
type HeaderSpec struct {
	Title       string   `json:"title,omitempty"`
	StatusField string   `json:"statusField,omitempty"`
	ActionIDs   []string `json:"actionIds,omitempty"`
}

// DetailView is opened when a resource row is clicked.
type DetailView struct {
	Header HeaderSpec `json:"header"`
	Tabs   []Tab      `json:"tabs"`
}

// ResourceType is a managed object type: columns, actions, detail.
type ResourceType struct {
	Kind      string      `json:"kind"`
	Title     string      `json:"title"`
	List      DataSource  `json:"list"`
	Watch     *DataSource `json:"watch,omitempty"`
	Columns   []Column    `json:"columns"`
	ActionIDs []string    `json:"actionIds"`
	Detail    DetailView  `json:"detail"`
}
