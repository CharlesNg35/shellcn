package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/charlesng35/shellcn/internal/ai/engine"
	"github.com/charlesng35/shellcn/internal/ai/tools"
	"github.com/charlesng35/shellcn/internal/models"
	"github.com/charlesng35/shellcn/internal/pluginregistry"
	"github.com/charlesng35/shellcn/sdk/plugin"
)

type fakeSess struct{}

func (fakeSess) HealthCheck(context.Context) error { return nil }
func (fakeSess) OpenChannel(context.Context, plugin.ChannelRequest) (plugin.Channel, error) {
	return nil, plugin.ErrNotSupported
}
func (fakeSess) Close() error { return nil }

type demoPlugin struct{}

func (demoPlugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		APIVersion: plugin.CurrentAPIVersion, Name: "demo", Version: "0", Title: "Demo",
		Category: plugin.CategoryOther, Layout: plugin.LayoutTabs,
		Tabs: []plugin.Panel{{
			Key:    "items",
			Label:  "Items",
			Type:   plugin.PanelTable,
			Source: &plugin.DataSource{RouteID: "demo.list", Params: map[string]string{"database": "${resource.uid}", "schema": "${resource.name}"}},
			Config: plugin.TableConfig{
				Editable: true,
				Insert: &plugin.DataSource{
					RouteID: "demo.row.insert",
					Method:  plugin.MethodPost,
					Params:  map[string]string{"database": "${resource.scope}", "schema": "${resource.namespace}", "table": "${resource.name}"},
				},
			},
		}},
		Streams: []plugin.Stream{
			{ID: "demo.stream", Kind: plugin.StreamQuery, RouteID: "demo.stream"},
			{ID: "demo.terminal", Kind: plugin.StreamTerminal, RouteID: "demo.terminal"},
		},
		SupportedTransports: []plugin.Transport{plugin.TransportDirect},
	}
}

func (demoPlugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			ID: "demo.list", Method: plugin.MethodGet, Risk: plugin.RiskSafe, Permission: "demo.read", AuditEvent: "demo.list",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.get", Method: plugin.MethodGet, Risk: plugin.RiskSafe, Permission: "demo.read", AuditEvent: "demo.get",
			Path:   "/items/{name}",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.create", Method: plugin.MethodPost, Risk: plugin.RiskWrite, Permission: "demo.write", AuditEvent: "demo.create",
			Input: &plugin.Schema{Groups: []plugin.Group{{Name: "i", Fields: []plugin.Field{
				{Key: "name", Label: "Name", Help: "Human-readable item name.", Type: plugin.FieldText, Required: true, Default: "default-name"},
				{Key: "count", Label: "Count", Type: plugin.FieldNumber, Validators: []plugin.Validator{{Type: plugin.ValidatorMin, Value: 1}, {Type: plugin.ValidatorMax, Value: 100}}},
				{Key: "token", Label: "Token", Type: plugin.FieldPassword, Secret: true},
				{Key: "password", Label: "Password", Type: plugin.FieldPassword},
				{
					Key: "credential_id", Label: "Stored credential", Type: plugin.FieldCredentialRef, Required: true,
					Credential: &plugin.CredentialSelector{Kind: plugin.CredentialKindAPIToken},
				},
				{Key: "profile", Label: "Profile", Type: plugin.FieldObject, Fields: []plugin.Field{
					{Key: "display_name", Label: "Display name", Type: plugin.FieldText, Required: true},
					{Key: "nested_token", Label: "Nested token", Type: plugin.FieldPassword, Secret: true},
					{Key: "nested_credential", Label: "Nested credential", Type: plugin.FieldCredentialRef},
				}},
				{Key: "recovery_codes", Label: "Recovery codes", Type: plugin.FieldArray, Item: &plugin.Field{Type: plugin.FieldPassword}},
			}}}},
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.row.insert", Method: plugin.MethodPost, Risk: plugin.RiskWrite, Permission: "demo.write", AuditEvent: "demo.row.insert",
			Path:   "/tables/{schema}/{table}/rows",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.delete", Method: plugin.MethodDelete, Risk: plugin.RiskDestructive, Permission: "demo.delete", AuditEvent: "demo.delete",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.exec", Method: plugin.MethodPost, Risk: plugin.RiskPrivileged, Permission: "demo.exec", AuditEvent: "demo.exec",
			Handle: func(*plugin.RequestContext) (any, error) { return nil, nil },
		},
		{
			ID: "demo.stream", Method: plugin.MethodWS, Risk: plugin.RiskSafe, Permission: "demo.read", AuditEvent: "demo.stream",
			Input:  &plugin.Schema{Groups: []plugin.Group{{Name: "i", Fields: []plugin.Field{{Key: "tail", Label: "Tail", Type: plugin.FieldNumber}}}}},
			Stream: func(*plugin.RequestContext, plugin.ClientStream) error { return nil },
		},
		{
			ID: "demo.terminal", Method: plugin.MethodWS, Risk: plugin.RiskSafe, Permission: "demo.read", AuditEvent: "demo.terminal",
			Stream: func(*plugin.RequestContext, plugin.ClientStream) error { return nil },
		},
	}
}

func (demoPlugin) Connect(context.Context, plugin.ConnectConfig) (plugin.Session, error) {
	return fakeSess{}, nil
}

type recordingInvoker struct {
	lastRoute  string
	lastParams map[string]string
	lastBody   []byte
	result     any
	err        error
}

func (r *recordingInvoker) InvokeRoute(_ context.Context, _ models.User, _, routeID string, params map[string]string, body []byte) (any, error) {
	r.lastRoute = routeID
	r.lastParams = params
	r.lastBody = body
	return r.result, r.err
}

type recordingStreamInvoker struct {
	recordingInvoker
	lastStreamRoute  string
	lastStreamParams map[string]string
	lastStreamOpts   engine.StreamSampleOptions
}

func (r *recordingStreamInvoker) InvokeStream(_ context.Context, _ models.User, _, routeID string, params map[string]string, opts engine.StreamSampleOptions) (any, error) {
	r.lastStreamRoute = routeID
	r.lastStreamParams = params
	r.lastStreamOpts = opts
	return engine.StreamSample{RouteID: routeID, Data: "line 1\n"}, nil
}

func registry(t *testing.T) *pluginregistry.Registry {
	t.Helper()
	reg := pluginregistry.New()
	reg.MustRegister(demoPlugin{})
	return reg
}

func TestBuildReadOnlyExposesOnlySafeNonStream(t *testing.T) {
	reg := registry(t)
	ts, err := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true}, &recordingInvoker{}, models.User{ID: "u"}, "c1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	names := map[string]bool{}
	for _, s := range ts.Specs() {
		names[s.Name] = true
	}
	if !names["demo_list"] || !names["demo_get"] {
		t.Fatalf("safe read routes missing: %v", names)
	}
	for _, forbidden := range []string{"demo_create", "demo_delete", "demo_exec", "demo_stream"} {
		if names[forbidden] {
			t.Fatalf("read-only set leaked %q: %v", forbidden, names)
		}
	}
	if names["observe_demo_stream"] {
		t.Fatalf("stream observer should require a stream-capable invoker: %v", names)
	}
}

func TestBuildExposesSafeStreamObserverWhenSupported(t *testing.T) {
	reg := registry(t)
	inv := &recordingStreamInvoker{}
	ts, err := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true}, inv, models.User{ID: "u"}, "c1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if ts.Has("demo_stream") || ts.Has("demo_terminal") {
		t.Fatal("raw websocket route must not be exposed")
	}
	if !ts.Has("observe_demo_stream") {
		t.Fatalf("safe stream observer missing: %v", ts.Specs())
	}
	if ts.Has("observe_demo_terminal") {
		t.Fatal("terminal streams must not be exposed as text observers")
	}

	specs := map[string]engine.ToolSpec{}
	for _, s := range ts.Specs() {
		specs[s.Name] = s
	}
	props := specs["observe_demo_stream"].Parameters["properties"].(map[string]any)
	for _, key := range []string{"tail", "duration_ms", "max_bytes", "max_events"} {
		if _, ok := props[key]; !ok {
			t.Fatalf("observer schema missing %s: %+v", key, props)
		}
	}

	out, err := ts.Execute(context.Background(), engine.ToolCall{
		Name:  "observe_demo_stream",
		Input: map[string]any{"tail": 25, "duration_ms": 250, "max_bytes": 1024, "max_events": 3},
	})
	if err != nil {
		t.Fatalf("execute observer: %v", err)
	}
	if inv.lastStreamRoute != "demo.stream" {
		t.Fatalf("wrong stream route: %q", inv.lastStreamRoute)
	}
	if inv.lastStreamParams["tail"] != "25" {
		t.Fatalf("stream input should be routed as params: %+v", inv.lastStreamParams)
	}
	if inv.lastStreamOpts.Duration != 250*time.Millisecond || inv.lastStreamOpts.MaxBytes != 1024 || inv.lastStreamOpts.MaxEvents != 3 {
		t.Fatalf("stream opts not routed: %+v", inv.lastStreamOpts)
	}
	sample, ok := out.(map[string]any)
	if !ok || sample["routeId"] != "demo.stream" || sample["data"] != "line 1\n" {
		t.Fatalf("unexpected observer result: %#v", out)
	}
}

func TestWriteTierExposesWriteNotDestructiveOrPrivileged(t *testing.T) {
	reg := registry(t)
	ts, _ := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}, &recordingInvoker{}, models.User{ID: "u"}, "c1")
	if !ts.Has("demo_create") {
		t.Fatal("write tier should expose demo_create")
	}
	if ts.Has("demo_delete") || ts.Has("demo_exec") {
		t.Fatal("write tier must not expose destructive/privileged")
	}
}

func TestToolSchemaExcludesSensitiveFieldsAndIncludesPathParams(t *testing.T) {
	reg := registry(t)
	ts, _ := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}, &recordingInvoker{}, models.User{ID: "u"}, "c1")

	specs := map[string]engine.ToolSpec{}
	for _, s := range ts.Specs() {
		specs[s.Name] = s
	}

	create := specs["demo_create"].Parameters["properties"].(map[string]any)
	if _, ok := create["name"]; !ok {
		t.Fatal("create tool missing name property")
	}
	if specs["demo_create"].Parameters["additionalProperties"] != false {
		t.Fatalf("tool schema should reject unknown args: %+v", specs["demo_create"].Parameters)
	}
	name := create["name"].(map[string]any)
	if !strings.Contains(name["description"].(string), "Human-readable item name") || name["default"] != "default-name" {
		t.Fatalf("field metadata missing from schema: %+v", name)
	}
	count := create["count"].(map[string]any)
	if count["minimum"] != 1 || count["maximum"] != 100 {
		t.Fatalf("numeric validators missing from schema: %+v", count)
	}
	if _, ok := create["token"]; ok {
		t.Fatal("secret field must not be exposed to the model")
	}
	if _, ok := create["password"]; ok {
		t.Fatal("password field must not be exposed to the model")
	}
	if _, ok := create["credential_id"]; ok {
		t.Fatal("credential_ref field must not be exposed to the model")
	}
	if _, ok := create["recovery_codes"]; ok {
		t.Fatal("array of sensitive values must not be exposed to the model")
	}
	profile := create["profile"].(map[string]any)["properties"].(map[string]any)
	if _, ok := profile["display_name"]; !ok {
		t.Fatal("safe nested field should be exposed to the model")
	}
	if _, ok := profile["nested_token"]; ok {
		t.Fatal("nested secret field must not be exposed to the model")
	}
	if _, ok := profile["nested_credential"]; ok {
		t.Fatal("nested credential_ref field must not be exposed to the model")
	}

	get := specs["demo_get"].Parameters
	required, _ := get["required"].([]string)
	if len(required) != 1 || required[0] != "name" {
		t.Fatalf("path param should be required: %v", get["required"])
	}

	list := specs["demo_list"].Parameters["properties"].(map[string]any)
	if _, ok := list["database"]; !ok {
		t.Fatalf("manifest data-source route param should be exposed: %+v", list)
	}
	if _, ok := list["schema"]; !ok {
		t.Fatalf("manifest data-source route param should be exposed: %+v", list)
	}
	if !strings.Contains(specs["demo_list"].Description, "Route params:") {
		t.Fatalf("route param hint missing from description: %q", specs["demo_list"].Description)
	}

	insert := specs["demo_row_insert"].Parameters
	insertProps := insert["properties"].(map[string]any)
	values := insertProps["values"].(map[string]any)
	if values["type"] != "object" || !strings.Contains(values["description"].(string), "Send an object") {
		t.Fatalf("editable table insert values schema missing: %+v", values)
	}
	required = insert["required"].([]string)
	if !containsString(required, "schema") || !containsString(required, "table") || !containsString(required, "values") {
		t.Fatalf("editable table insert required fields missing: %v", required)
	}
}

func TestExecuteSplitsPathParamsFromBody(t *testing.T) {
	reg := registry(t)
	inv := &recordingInvoker{result: map[string]any{"ok": true}}
	ts, _ := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}, inv, models.User{ID: "u"}, "c1")

	// Path-param route: the {name} param goes to params, not the body.
	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_get", Input: map[string]any{"name": "alpha"}}); err != nil {
		t.Fatalf("execute get: %v", err)
	}
	if inv.lastRoute != "demo.get" || inv.lastParams["name"] != "alpha" {
		t.Fatalf("path param not routed: route=%s params=%v", inv.lastRoute, inv.lastParams)
	}
	if len(inv.lastBody) != 0 {
		t.Fatalf("path-only call should have empty body, got %s", inv.lastBody)
	}

	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_list", Input: map[string]any{"database": "shellcn", "schema": "public"}}); err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if inv.lastRoute != "demo.list" || inv.lastParams["database"] != "shellcn" || inv.lastParams["schema"] != "public" {
		t.Fatalf("manifest params not routed: route=%s params=%v", inv.lastRoute, inv.lastParams)
	}
	if len(inv.lastBody) != 0 {
		t.Fatalf("manifest-param call should have empty body, got %s", inv.lastBody)
	}

	ts.WithConfirmer(&recordingConfirmer{approve: true})
	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_row_insert", Input: map[string]any{
		"database": "shellcn",
		"schema":   "public",
		"table":    "users",
		"values":   `{"name":"alice","age":30}`,
	}}); err != nil {
		t.Fatalf("execute inferred row insert: %v", err)
	}
	if inv.lastRoute != "demo.row.insert" || inv.lastParams["database"] != "shellcn" || inv.lastParams["schema"] != "public" || inv.lastParams["table"] != "users" {
		t.Fatalf("inferred row insert params not routed: route=%s params=%v", inv.lastRoute, inv.lastParams)
	}
	var rowBody map[string]any
	if err := json.Unmarshal(inv.lastBody, &rowBody); err != nil {
		t.Fatalf("row insert body not JSON: %s err=%v", inv.lastBody, err)
	}
	rowValues, ok := rowBody["values"].(map[string]any)
	if !ok || rowValues["name"] != "alice" || rowValues["age"] != float64(30) {
		t.Fatalf("row insert values not normalized as object: %+v", rowBody)
	}

	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_row_insert", Input: map[string]any{
		"schema": "public",
		"table":  "users",
	}}); err == nil || !strings.Contains(err.Error(), "values is required") {
		t.Fatalf("missing values should fail before invoking route, got %v", err)
	}

	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_create", Input: map[string]any{"name": "x"}}); err != nil {
		t.Fatalf("execute create: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(inv.lastBody, &body); err != nil || body["name"] != "x" {
		t.Fatalf("body not marshaled: %s err=%v", inv.lastBody, err)
	}
}

type recordingConfirmer struct {
	approve bool
	calls   []tools.ConfirmRequest
}

func (c *recordingConfirmer) Confirm(_ context.Context, req tools.ConfirmRequest) (bool, error) {
	c.calls = append(c.calls, req)
	return c.approve, nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestConfirmerGatesWritesNotReads(t *testing.T) {
	reg := registry(t)

	inv := &recordingInvoker{result: map[string]any{"ok": true}}
	tsNoConfirm := mustBuild(t, reg, map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}, inv)
	if _, err := tsNoConfirm.Execute(context.Background(), engine.ToolCall{ID: "tc0", Name: "demo_create", Input: map[string]any{"name": "x"}}); err == nil {
		t.Fatal("write tool without confirmer should fail closed")
	}
	if inv.lastRoute != "" {
		t.Fatalf("unguarded write must not invoke the route, got %q", inv.lastRoute)
	}

	cf := &recordingConfirmer{approve: false}
	ts := mustBuild(t, reg, map[plugin.RiskLevel]bool{plugin.RiskSafe: true, plugin.RiskWrite: true}, inv).WithConfirmer(cf)

	out, err := ts.Execute(context.Background(), engine.ToolCall{ID: "tc1", Name: "demo_create", Input: map[string]any{"name": "x"}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(cf.calls) != 1 || cf.calls[0].ToolCallID != "tc1" || cf.calls[0].Risk != plugin.RiskWrite {
		t.Fatalf("confirmer not consulted correctly: %+v", cf.calls)
	}
	if inv.lastRoute != "" {
		t.Fatalf("declined write must not invoke the route, got %q", inv.lastRoute)
	}
	if m, ok := out.(map[string]any); !ok || m["declined"] != true {
		t.Fatalf("declined result expected, got %#v", out)
	}

	// Approved write: the route runs.
	cf.approve = true
	if _, err := ts.Execute(context.Background(), engine.ToolCall{ID: "tc2", Name: "demo_create", Input: map[string]any{"name": "y"}}); err != nil {
		t.Fatalf("approved execute: %v", err)
	}
	if inv.lastRoute != "demo.create" {
		t.Fatalf("approved write should invoke the route, got %q", inv.lastRoute)
	}

	// Reads never reach the confirmer.
	before := len(cf.calls)
	if _, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_list"}); err != nil {
		t.Fatalf("read execute: %v", err)
	}
	if len(cf.calls) != before {
		t.Fatal("a read must not require confirmation")
	}
}

func mustBuild(t *testing.T, reg *pluginregistry.Registry, allowed map[plugin.RiskLevel]bool, inv tools.Invoker) *tools.ToolSet {
	t.Helper()
	ts, err := tools.Build(reg, "demo", allowed, inv, models.User{ID: "u"}, "c1")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return ts
}

func TestExecuteTruncatesLargeResult(t *testing.T) {
	reg := registry(t)
	big := strings.Repeat("x", 20<<10)
	inv := &recordingInvoker{result: map[string]any{"data": big}}
	ts, _ := tools.Build(reg, "demo", map[plugin.RiskLevel]bool{plugin.RiskSafe: true}, inv, models.User{ID: "u"}, "c1")

	out, err := ts.Execute(context.Background(), engine.ToolCall{Name: "demo_list"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok || m["truncated"] != true {
		t.Fatalf("large result should be marked truncated: %#v", out)
	}
	if _, ok := m["data"]; !ok {
		t.Fatalf("structured truncated result should keep data when possible: %#v", out)
	}
	if preview, ok := m["preview"].(string); ok && !utf8.ValidString(preview) {
		t.Fatalf("preview must be valid utf8: %q", preview)
	}
}
