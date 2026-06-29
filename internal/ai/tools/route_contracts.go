package tools

import (
	"reflect"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type routeContracts struct {
	params       map[string]map[string]bool
	inputs       map[string]*plugin.Schema
	bodyDefaults map[string]map[string]any
}

func inferRouteContracts(m plugin.Manifest) routeContracts {
	c := routeContracts{
		params:       map[string]map[string]bool{},
		inputs:       map[string]*plugin.Schema{},
		bodyDefaults: map[string]map[string]any{},
	}
	walkManifest(reflect.ValueOf(m), func(v reflect.Value) {
		if v.Kind() == reflect.Struct {
			c.extractRouteRefs(v)
			c.extractRendererBodies(v)
		}
	})
	return c
}

func (c *routeContracts) extractRouteRefs(v reflect.Value) {
	if routeID := stringField(v, "RouteID"); routeID != "" {
		c.addParams(routeID, stringMapField(v, "Params"))
	}
	for _, field := range exportedFields(v) {
		if !strings.HasSuffix(field.Name, "RouteID") {
			continue
		}
		routeID := stringValue(v.FieldByIndex(field.Index))
		if routeID == "" {
			continue
		}
		prefix := strings.TrimSuffix(field.Name, "RouteID")
		params := stringMapField(v, prefix+"Params")
		if len(params) == 0 && field.Name == "SubmitRouteID" {
			params = stringMapField(v, "Params")
		}
		c.addParams(routeID, params)
	}

	keyParam := stringField(v, "KeyParam")
	if keyParam != "" {
		keyParams := singleParam(keyParam)
		for _, name := range []string{"CreateRouteID", "ReadRouteID", "WriteRouteID", "DeleteRouteID"} {
			c.addParams(stringField(v, name), keyParams)
		}
	}
}

func (c *routeContracts) extractRendererBodies(v reflect.Value) {
	for _, spec := range []struct {
		field  string
		schema *plugin.Schema
	}{
		{field: "Insert", schema: rowMutationInput(false, true)},
		{field: "Update", schema: rowMutationInput(true, true)},
		{field: "Delete", schema: rowMutationInput(true, false)},
	} {
		c.addInput(routeRefID(v.FieldByName(spec.field)), spec.schema)
	}

	if saveRouteID := stringField(v, "SaveRouteID"); saveRouteID != "" {
		c.addInput(saveRouteID, codeEditorSaveInput(stringField(v, "SaveBodyKey")))
		c.addBodyDefaults(saveRouteID, anyMapField(v, "SaveExtra"))
	}

	if stringField(v, "KeyParam") != "" {
		c.addInput(stringField(v, "CreateRouteID"), kvMutationInput())
		c.addInput(stringField(v, "WriteRouteID"), kvMutationInput())
	}

	if executeRouteID := stringField(v, "ExecuteRouteID"); executeRouteID != "" {
		c.addInput(executeRouteID, httpClientInput())
	}
}

func (c *routeContracts) addParams(routeID string, params map[string]string) {
	if strings.TrimSpace(routeID) == "" {
		return
	}
	for key := range params {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if c.params[routeID] == nil {
			c.params[routeID] = map[string]bool{}
		}
		c.params[routeID][key] = true
	}
}

func (c *routeContracts) addInput(routeID string, schema *plugin.Schema) {
	if strings.TrimSpace(routeID) == "" || schema == nil || c.inputs[routeID] != nil {
		return
	}
	c.inputs[routeID] = schema
}

func (c *routeContracts) addBodyDefaults(routeID string, defaults map[string]any) {
	if strings.TrimSpace(routeID) == "" || len(defaults) == 0 || c.bodyDefaults[routeID] != nil {
		return
	}
	next := make(map[string]any, len(defaults))
	for key, value := range defaults {
		key = strings.TrimSpace(key)
		if key != "" {
			next[key] = value
		}
	}
	if len(next) > 0 {
		c.bodyDefaults[routeID] = next
	}
}

func walkManifest(v reflect.Value, visit func(reflect.Value)) {
	if !v.IsValid() {
		return
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	visit(v)
	switch v.Kind() {
	case reflect.Struct:
		for _, field := range exportedFields(v) {
			walkManifest(v.FieldByIndex(field.Index), visit)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walkManifest(v.Index(i), visit)
		}
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			walkManifest(iter.Value(), visit)
		}
	}
}

func exportedFields(v reflect.Value) []reflect.StructField {
	if v.Kind() != reflect.Struct {
		return nil
	}
	typ := v.Type()
	fields := make([]reflect.StructField, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath == "" {
			fields = append(fields, field)
		}
	}
	return fields
}

func routeRefID(v reflect.Value) string {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if !v.IsValid() || v.Kind() != reflect.Struct {
		return ""
	}
	return stringField(v, "RouteID")
}

func stringField(v reflect.Value, name string) string {
	return stringValue(v.FieldByName(name))
}

func stringValue(v reflect.Value) string {
	if !v.IsValid() || v.Kind() != reflect.String {
		return ""
	}
	return strings.TrimSpace(v.String())
}

func stringMapField(v reflect.Value, name string) map[string]string {
	field := v.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.Map || field.Type().Key().Kind() != reflect.String || field.Type().Elem().Kind() != reflect.String {
		return nil
	}
	out := map[string]string{}
	iter := field.MapRange()
	for iter.Next() {
		key := strings.TrimSpace(iter.Key().String())
		if key != "" {
			out[key] = iter.Value().String()
		}
	}
	return out
}

func anyMapField(v reflect.Value, name string) map[string]any {
	field := v.FieldByName(name)
	if !field.IsValid() || field.Kind() != reflect.Map || field.Type().Key().Kind() != reflect.String {
		return nil
	}
	out := map[string]any{}
	iter := field.MapRange()
	for iter.Next() {
		key := strings.TrimSpace(iter.Key().String())
		if key != "" && iter.Value().CanInterface() {
			out[key] = iter.Value().Interface()
		}
	}
	return out
}

func rowMutationInput(key, values bool) *plugin.Schema {
	fields := make([]plugin.Field, 0, 2)
	if key {
		fields = append(fields, plugin.Field{
			Key:      "key",
			Label:    "Row key",
			Type:     plugin.FieldJSON,
			Required: true,
			Help:     "JSON object whose keys are identifying column names and values are the row values to match.",
		})
	}
	if values {
		fields = append(fields, plugin.Field{
			Key:      "values",
			Label:    "Column values",
			Type:     plugin.FieldJSON,
			Required: true,
			Help:     "JSON object whose keys are column names and values are the values to write. Send an object, not a JSON string.",
		})
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Row", Fields: fields}}}
}

func codeEditorSaveInput(bodyKey string) *plugin.Schema {
	key := strings.TrimSpace(bodyKey)
	if key == "" {
		return &plugin.Schema{Groups: []plugin.Group{{Name: "Editor", Fields: []plugin.Field{{
			Key:      "content",
			Label:    "Content",
			Type:     plugin.FieldTextarea,
			Required: true,
			Help:     "Raw editor content to save.",
		}}}}}
	}
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Editor", Fields: []plugin.Field{{
		Key:      key,
		Label:    key,
		Type:     plugin.FieldJSON,
		Required: true,
		Help:     "Parsed JSON editor document to save. Send JSON as an object, not a quoted string or SQL fragment.",
	}}}}}
}

func kvMutationInput() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Key", Fields: []plugin.Field{
		{
			Key:   "type",
			Label: "Value type",
			Type:  plugin.FieldText,
			Help:  "Optional storage type such as string, hash, list, set, zset, or a plugin-defined type.",
		},
		{
			Key:      "value",
			Label:    "Value",
			Type:     plugin.FieldTextarea,
			Required: true,
			Help:     "Value to write for the selected key. For structured values, send JSON text in the format expected by the plugin.",
		},
	}}}}
}

func httpClientInput() *plugin.Schema {
	return &plugin.Schema{Groups: []plugin.Group{{Name: "Request", Fields: []plugin.Field{
		{Key: "method", Label: "Method", Type: plugin.FieldText, Required: true, Help: "HTTP method to send."},
		{Key: "url", Label: "URL", Type: plugin.FieldText, Required: true, Help: "Absolute URL or plugin-supported relative URL."},
		{Key: "headers", Label: "Headers", Type: plugin.FieldArray, Item: &plugin.Field{Type: plugin.FieldObject, Fields: []plugin.Field{
			{Key: "key", Label: "Name", Type: plugin.FieldText, Required: true},
			{Key: "value", Label: "Value", Type: plugin.FieldText, Required: true},
		}}},
		{Key: "body", Label: "Body", Type: plugin.FieldTextarea, Help: "Request body text."},
	}}}}
}

func singleParam(key string) map[string]string {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	return map[string]string{key: ""}
}
