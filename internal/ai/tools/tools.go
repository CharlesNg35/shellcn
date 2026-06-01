// Package tools turns a connection plugin's manifest routes into risk-gated LLM
// tools. It is plugin-agnostic: tools derive purely from each route's Risk,
// Input schema, and path params. Execution runs through the same secure pipeline
// a human request uses (the Invoker), so the agent can never exceed the user's
// RBAC or the route's risk gate.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/plugin"
)

// maxToolResultBytes caps a tool result before it enters the model context, so a
// large listing can't blow the context window. Truncation is signalled, never silent.
const maxToolResultBytes = 8 << 10

// Invoker runs a route as the user through the full secure pipeline. *server.Server
// satisfies it; depending on the interface (not the server) avoids an import cycle.
type Invoker interface {
	InvokeRoute(ctx context.Context, user models.User, connID, routeID string, params map[string]string, body []byte) (any, error)
}

// ConfirmRequest describes a pending write/destructive tool call awaiting the
// user's decision.
type ConfirmRequest struct {
	ToolCallID  string
	ToolName    string
	RouteID     string
	Risk        plugin.RiskLevel
	Destructive bool
	Params      map[string]string
	Body        map[string]any
}

// Confirmer asks the user to approve a mutation before it runs. It returns true
// to proceed, false to decline. Reads never reach a confirmer.
type Confirmer interface {
	Confirm(ctx context.Context, req ConfirmRequest) (bool, error)
}

// RouteSource enumerates a protocol's routes (satisfied by *plugin.Registry).
type RouteSource interface {
	Get(name string) (plugin.Plugin, bool)
}

// binding records how to call a route from a sanitized tool name.
type binding struct {
	routeID    string
	risk       plugin.RiskLevel
	pathParams []string
}

// ToolSet is the risk-gated tool catalogue for one connection. It implements
// engine.ToolExecutor.
type ToolSet struct {
	specs     []engine.ToolSpec
	byName    map[string]binding
	invoker   Invoker
	user      models.User
	connID    string
	confirmer Confirmer
}

// WithConfirmer attaches a confirmer that gates write/destructive tool calls.
func (ts *ToolSet) WithConfirmer(c Confirmer) *ToolSet {
	ts.confirmer = c
	return ts
}

// Build enumerates the protocol's routes, keeps those whose Risk is allowed and
// which are non-streaming, and produces a tool per route. allowed is the set of
// permitted risk tiers (read-only agents pass {RiskSafe:true}).
func Build(src RouteSource, protocol string, allowed map[plugin.RiskLevel]bool, invoker Invoker, user models.User, connID string) (*ToolSet, error) {
	plg, ok := src.Get(protocol)
	if !ok {
		return nil, fmt.Errorf("tools: unknown protocol %q", protocol)
	}
	ts := &ToolSet{byName: map[string]binding{}, invoker: invoker, user: user, connID: connID}
	for _, r := range plg.Routes() {
		if r.IsStream() || !allowed[r.Risk] {
			continue
		}
		name := sanitizeName(r.ID)
		if _, dup := ts.byName[name]; dup {
			continue
		}
		params := templateParamNames(r.Path)
		ts.byName[name] = binding{routeID: r.ID, risk: r.Risk, pathParams: params}
		ts.specs = append(ts.specs, engine.ToolSpec{
			Name:        name,
			Description: describe(r),
			Parameters:  toJSONSchema(r, params),
		})
	}
	sort.Slice(ts.specs, func(i, j int) bool { return ts.specs[i].Name < ts.specs[j].Name })
	return ts, nil
}

// Specs returns the tool catalogue for the provider request.
func (ts *ToolSet) Specs() []engine.ToolSpec { return ts.specs }

// Has reports whether a tool name is in the set.
func (ts *ToolSet) Has(name string) bool { _, ok := ts.byName[name]; return ok }

// Execute runs a tool call: it splits path params from the JSON body, invokes the
// route as the user, and returns a cleaned/truncated result for the model context.
func (ts *ToolSet) Execute(ctx context.Context, call engine.ToolCall) (any, error) {
	b, ok := ts.byName[call.Name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
	params := map[string]string{}
	body := map[string]any{}
	pathSet := map[string]bool{}
	for _, p := range b.pathParams {
		pathSet[p] = true
	}
	for k, v := range call.Input {
		if pathSet[k] {
			params[k] = fmt.Sprint(v)
			continue
		}
		body[k] = v
	}
	// Write/destructive calls pause for the user's confirmation before running.
	if ts.confirmer != nil && (b.risk == plugin.RiskWrite || b.risk == plugin.RiskDestructive) {
		ok, err := ts.confirmer.Confirm(ctx, ConfirmRequest{
			ToolCallID:  call.ID,
			ToolName:    call.Name,
			RouteID:     b.routeID,
			Risk:        b.risk,
			Destructive: b.risk == plugin.RiskDestructive,
			Params:      params,
			Body:        body,
		})
		if err != nil {
			return nil, err
		}
		if !ok {
			return map[string]any{"declined": true, "note": "The user declined this action; it was not performed."}, nil
		}
	}

	var raw []byte
	if len(body) > 0 {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	result, err := ts.invoker.InvokeRoute(ctx, ts.user, ts.connID, b.routeID, params, raw)
	if err != nil {
		return nil, err
	}
	return clean(result), nil
}

// clean truncates an oversized result and marks it, so the model is told the
// data was capped rather than silently losing rows.
func clean(result any) any {
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) <= maxToolResultBytes {
		return result
	}
	return map[string]any{
		"truncated": true,
		"note":      fmt.Sprintf("result truncated to %d bytes; narrow the query for full data", maxToolResultBytes),
		"preview":   string(encoded[:maxToolResultBytes]),
	}
}

// describe builds a concise, model-facing description from the route's stable id,
// method, and risk — no plugin-specific knowledge.
func describe(r plugin.Route) string {
	action := humanize(r.ID)
	verb := map[plugin.Method]string{
		plugin.MethodGet:    "Read",
		plugin.MethodPost:   "Create or run",
		plugin.MethodPut:    "Update",
		plugin.MethodPatch:  "Update",
		plugin.MethodDelete: "Delete",
	}[r.Method]
	if verb == "" {
		verb = "Invoke"
	}
	return fmt.Sprintf("%s: %s (%s, %s)", verb, action, r.Method, r.Risk)
}

func humanize(id string) string {
	return strings.ReplaceAll(strings.ReplaceAll(id, ".", " "), "_", " ")
}

// sanitizeName maps a route id to an LLM-tool-name-safe token ([A-Za-z0-9_-]).
func sanitizeName(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// toJSONSchema flattens the route's Input groups plus its path params into a flat
// JSON Schema object the model can fill. Path params are required strings.
func toJSONSchema(r plugin.Route, pathParams []string) map[string]any {
	props := map[string]any{}
	var required []string

	for _, p := range pathParams {
		props[p] = map[string]any{"type": "string", "description": "path parameter"}
		required = append(required, p)
	}

	if r.Input != nil {
		for _, g := range r.Input.Groups {
			for _, f := range g.Fields {
				if f.Secret {
					continue // never let the model supply secret material
				}
				props[f.Key] = fieldSchema(f)
				if f.Required {
					required = append(required, f.Key)
				}
			}
		}
	}

	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func fieldSchema(f plugin.Field) map[string]any {
	out := map[string]any{}
	if d := strings.TrimSpace(f.Label); d != "" {
		out["description"] = d
	} else if h := strings.TrimSpace(f.Help); h != "" {
		out["description"] = h
	}
	switch f.Type {
	case plugin.FieldNumber, plugin.FieldStepper, plugin.FieldSlider:
		out["type"] = "number"
	case plugin.FieldToggle:
		out["type"] = "boolean"
	case plugin.FieldMultiSelect:
		out["type"] = "array"
		out["items"] = map[string]any{"type": "string"}
	case plugin.FieldArray:
		out["type"] = "array"
		if f.Item != nil {
			out["items"] = fieldSchema(*f.Item)
		} else {
			out["items"] = map[string]any{"type": "string"}
		}
	case plugin.FieldObject, plugin.FieldMap, plugin.FieldJSON:
		out["type"] = "object"
	default:
		out["type"] = "string"
	}
	if len(f.Options) > 0 {
		vals := make([]any, 0, len(f.Options))
		for _, o := range f.Options {
			vals = append(vals, o.Value)
		}
		out["enum"] = vals
	}
	return out
}

// templateParamNames extracts {name} segments from a route path, in order.
func templateParamNames(path string) []string {
	var out []string
	for {
		start := strings.IndexByte(path, '{')
		if start < 0 {
			return out
		}
		path = path[start+1:]
		end := strings.IndexByte(path, '}')
		if end < 0 {
			return out
		}
		if name := strings.TrimSpace(path[:end]); name != "" {
			out = append(out, name)
		}
		path = path[end+1:]
	}
}
