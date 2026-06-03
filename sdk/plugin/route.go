package plugin

import "time"

// Method is the HTTP verb (or WS) a route is mounted under.
type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodPatch  Method = "PATCH"
	MethodDelete Method = "DELETE"
	MethodWS     Method = "WS"
)

// RiskLevel is enforced by the route wrapper and projected (read-only) to the UI.
type RiskLevel string

const (
	RiskSafe        RiskLevel = "safe"        // read-only (list, describe)
	RiskWrite       RiskLevel = "write"       // create/update
	RiskDestructive RiskLevel = "destructive" // delete, truncate, restore
	RiskPrivileged  RiskLevel = "privileged"  // shell, exec, raw socket
)

// Handler is a plugin's pure business logic for an HTTP route. It never sees
// http.ResponseWriter, status codes, headers, cookies, or auth.
type Handler func(rc *RequestContext) (any, error)

// StreamHandler is a plugin's logic for a WS route, bridging to the browser.
type StreamHandler func(rc *RequestContext, client ClientStream) error

// Route is a typed server endpoint with the metadata the core enforces. It is
// the ONE behavior mechanism: no HandleAction, no plugin-owned HTTP.
type Route struct {
	ID         string    `json:"id"`
	Method     Method    `json:"method"`
	Path       string    `json:"path"`
	Permission string    `json:"permission"`
	Risk       RiskLevel `json:"risk"`
	AuditEvent string    `json:"auditEvent"`
	Input      *Schema   `json:"input,omitempty"`

	Timeout time.Duration `json:"timeout,omitempty"`

	Handle Handler       `json:"-"`
	Stream StreamHandler `json:"-"`
}

// IsStream reports whether the route is a WebSocket route.
func (r Route) IsStream() bool { return r.Method == MethodWS }
