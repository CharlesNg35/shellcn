// Package tools exposes risk-gated connection routes as agent tools.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

// maxToolResultBytes caps tool output before it enters model context.
const (
	maxToolResultBytes  = 8 << 10
	maxToolStringBytes  = 2 << 10
	maxToolArrayItems   = 50
	routeToolTimeout    = 60 * time.Second
	truncatedResultNote = "result was truncated before being shown to the model; narrow the query for full data"
)

// Invoker runs a route as the user through the secure pipeline.
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

// RouteSource enumerates a protocol's routes.
type RouteSource interface {
	Get(name string) (plugin.Plugin, bool)
}

// binding records how to call a route from a sanitized tool name.
type binding struct {
	routeID string
	risk    plugin.RiskLevel
	params  map[string]bool
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

// Build produces tools from allowed, non-streaming protocol routes.
func Build(src RouteSource, protocol string, allowed map[plugin.RiskLevel]bool, invoker Invoker, user models.User, connID string) (*ToolSet, error) {
	plg, ok := src.Get(protocol)
	if !ok {
		return nil, fmt.Errorf("tools: unknown protocol %q", protocol)
	}
	ts := &ToolSet{byName: map[string]binding{}, invoker: invoker, user: user, connID: connID}
	manifestParams := routeParamIndex(plg.Manifest())
	for _, r := range plg.Routes() {
		if r.IsStream() || !allowed[r.Risk] {
			continue
		}
		name := sanitizeName(r.ID)
		if _, dup := ts.byName[name]; dup {
			continue
		}
		params := templateParamNames(r.Path)
		routeParams := mergeRouteParams(params, manifestParams[r.ID])
		ts.byName[name] = binding{routeID: r.ID, risk: r.Risk, params: routeParamSet(routeParams)}
		ts.specs = append(ts.specs, engine.ToolSpec{
			Name:        name,
			Description: describe(r, routeParams),
			Parameters:  toJSONSchema(r, params, routeParams),
		})
	}
	sort.Slice(ts.specs, func(i, j int) bool { return ts.specs[i].Name < ts.specs[j].Name })
	return ts, nil
}

// Specs returns the tool catalogue for the provider request.
func (ts *ToolSet) Specs() []engine.ToolSpec { return ts.specs }

// Has reports whether a tool name is in the set.
func (ts *ToolSet) Has(name string) bool { _, ok := ts.byName[name]; return ok }

// Execute invokes a route tool and returns a model-safe result.
func (ts *ToolSet) Execute(ctx context.Context, call engine.ToolCall) (any, error) {
	b, ok := ts.byName[call.Name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q", call.Name)
	}
	params := map[string]string{}
	body := map[string]any{}
	for k, v := range call.Input {
		if b.params[k] {
			params[k] = fmt.Sprint(v)
			continue
		}
		body[k] = v
	}
	if b.risk == plugin.RiskWrite || b.risk == plugin.RiskDestructive {
		if ts.confirmer == nil {
			return nil, fmt.Errorf("tool %q requires user confirmation", call.Name)
		}
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
	routeCtx, cancel := context.WithTimeout(ctx, routeToolTimeout)
	defer cancel()
	result, err := ts.invoker.InvokeRoute(routeCtx, ts.user, ts.connID, b.routeID, params, raw)
	if err != nil {
		return nil, err
	}
	return clean(result), nil
}

// clean marks and truncates oversized tool output.
func clean(result any) any {
	result, changed := compactValue(result)
	encoded, err := json.Marshal(result)
	if err != nil || len(encoded) <= maxToolResultBytes {
		if changed {
			return map[string]any{
				"truncated": true,
				"note":      truncatedResultNote,
				"data":      result,
			}
		}
		return result
	}
	return map[string]any{
		"truncated": true,
		"note":      truncatedResultNote,
		"preview":   safeTruncate(string(encoded), maxToolResultBytes),
	}
}

func compactValue(v any) (any, bool) {
	switch x := v.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return v, false
	case string:
		if len(x) <= maxToolStringBytes {
			return x, false
		}
		return safeTruncate(x, maxToolStringBytes), true
	case []any:
		return compactSlice(x)
	case map[string]any:
		return compactMap(x)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return v, false
		}
		var normalized any
		if err := json.Unmarshal(raw, &normalized); err != nil {
			return v, false
		}
		switch normalized.(type) {
		case string, []any, map[string]any:
			return compactValue(normalized)
		default:
			return v, false
		}
	}
}

func compactMap(in map[string]any) (map[string]any, bool) {
	out := make(map[string]any, len(in))
	changed := false
	for k, v := range in {
		next, ok := compactValue(v)
		if ok {
			changed = true
		}
		out[k] = next
	}
	return out, changed
}

func compactSlice(in []any) ([]any, bool) {
	limit := len(in)
	changed := false
	if limit > maxToolArrayItems {
		limit = maxToolArrayItems
		changed = true
	}
	out := make([]any, 0, limit+1)
	for _, v := range in[:limit] {
		next, ok := compactValue(v)
		if ok {
			changed = true
		}
		out = append(out, next)
	}
	if len(in) > limit {
		out = append(out, map[string]any{
			"truncated":      true,
			"remainingItems": len(in) - limit,
		})
	}
	return out, changed
}

func safeTruncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if limit <= 0 {
		return "…"
	}
	next := s[:limit]
	for !utf8.ValidString(next) && len(next) > 0 {
		next = next[:len(next)-1]
	}
	return next + "…"
}

// describe builds a model-facing description from route metadata.
func describe(r plugin.Route, routeParams []string) string {
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
	desc := fmt.Sprintf("%s: %s (%s, %s). Route: %s. Permission: %s.", verb, action, r.Method, r.Risk, r.ID, r.Permission)
	if len(routeParams) > 0 {
		desc += " Route params: " + strings.Join(routeParams, ", ") + ". Use these to preserve the same database/schema/resource scope as the user's request."
	}
	return desc
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

// toJSONSchema flattens route input and route params for the model.
func toJSONSchema(r plugin.Route, pathParams, routeParams []string) map[string]any {
	props := map[string]any{}
	var required []string

	for _, p := range pathParams {
		props[p] = map[string]any{"type": "string", "description": "path parameter"}
		required = append(required, p)
	}
	pathSet := routeParamSet(pathParams)
	for _, p := range routeParams {
		if pathSet[p] {
			continue
		}
		props[p] = map[string]any{"type": "string", "description": "route scope parameter from the plugin manifest"}
	}

	if r.Input != nil {
		for _, g := range r.Input.Groups {
			for _, f := range g.Fields {
				schema, ok := fieldSchema(f)
				if !ok {
					continue
				}
				props[f.Key] = schema
				if f.Required {
					required = append(required, f.Key)
				}
			}
		}
	}

	schema := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func routeParamIndex(m plugin.Manifest) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	add := func(routeID string, params map[string]string) {
		if strings.TrimSpace(routeID) == "" {
			return
		}
		for key := range params {
			if strings.TrimSpace(key) == "" {
				continue
			}
			if out[routeID] == nil {
				out[routeID] = map[string]bool{}
			}
			out[routeID][key] = true
		}
	}

	dataSourceType := reflect.TypeOf(plugin.DataSource{})
	actionType := reflect.TypeOf(plugin.Action{})
	var walk func(reflect.Value)
	walk = func(v reflect.Value) {
		if !v.IsValid() {
			return
		}
		for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if v.Type() == dataSourceType {
			ds := v.Interface().(plugin.DataSource)
			add(ds.RouteID, ds.Params)
			return
		}
		if v.Type() == actionType {
			a := v.Interface().(plugin.Action)
			add(a.RouteID, a.Params)
		}
		switch v.Kind() {
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).PkgPath != "" {
					continue
				}
				walk(v.Field(i))
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		case reflect.Map:
			iter := v.MapRange()
			for iter.Next() {
				walk(iter.Value())
			}
		}
	}
	walk(reflect.ValueOf(m))
	return out
}

func mergeRouteParams(pathParams []string, manifestParams map[string]bool) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(pathParams)+len(manifestParams))
	for _, p := range pathParams {
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	extra := make([]string, 0, len(manifestParams))
	for p := range manifestParams {
		if !seen[p] {
			extra = append(extra, p)
		}
	}
	sort.Strings(extra)
	return append(out, extra...)
}

func routeParamSet(params []string) map[string]bool {
	out := make(map[string]bool, len(params))
	for _, p := range params {
		out[p] = true
	}
	return out
}

func fieldSchema(f plugin.Field) (map[string]any, bool) {
	if sensitiveField(f) {
		return nil, false
	}
	out := map[string]any{}
	if d := fieldDescription(f); d != "" {
		out["description"] = d
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
			item, ok := fieldSchema(*f.Item)
			if !ok {
				return nil, false
			}
			out["items"] = item
		} else {
			out["items"] = map[string]any{"type": "string"}
		}
	case plugin.FieldObject:
		out["type"] = "object"
		props := map[string]any{}
		var required []string
		for _, child := range f.Fields {
			schema, ok := fieldSchema(child)
			if !ok {
				continue
			}
			props[child.Key] = schema
			if child.Required {
				required = append(required, child.Key)
			}
		}
		if len(props) > 0 {
			out["properties"] = props
			out["additionalProperties"] = false
		}
		if len(required) > 0 {
			out["required"] = required
		}
	case plugin.FieldMap:
		out["type"] = "object"
		if f.Item != nil {
			item, ok := fieldSchema(*f.Item)
			if !ok {
				return nil, false
			}
			out["additionalProperties"] = item
		}
	case plugin.FieldJSON:
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
	if f.Default != nil {
		out["default"] = f.Default
	}
	if f.MinItems > 0 {
		out["minItems"] = f.MinItems
	}
	if f.MaxItems > 0 {
		out["maxItems"] = f.MaxItems
	}
	for _, v := range f.Validators {
		applyValidator(out, v)
	}
	return out, true
}

func fieldDescription(f plugin.Field) string {
	var parts []string
	if label := strings.TrimSpace(f.Label); label != "" {
		parts = append(parts, label)
	}
	if help := strings.TrimSpace(f.Help); help != "" && help != strings.TrimSpace(f.Label) {
		parts = append(parts, help)
	}
	if f.OptionsSource != nil && strings.TrimSpace(f.OptionsSource.RouteID) != "" {
		parts = append(parts, "Options are loaded dynamically from route "+f.OptionsSource.RouteID+".")
	}
	return strings.Join(parts, " ")
}

func applyValidator(out map[string]any, v plugin.Validator) {
	switch v.Type {
	case plugin.ValidatorMin:
		out["minimum"] = v.Value
	case plugin.ValidatorMax:
		out["maximum"] = v.Value
	case plugin.ValidatorRegex:
		if s, ok := v.Value.(string); ok && s != "" {
			out["pattern"] = s
		}
	case plugin.ValidatorOneOf:
		out["enum"] = v.Value
	}
}

func sensitiveField(f plugin.Field) bool {
	return f.Secret || f.Type == plugin.FieldPassword || f.Type == plugin.FieldCredentialRef
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
