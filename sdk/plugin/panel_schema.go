package plugin

type PanelConfigProperty struct {
	Type       string                         `json:"type"`
	Items      *PanelConfigProperty           `json:"items,omitempty"`
	Properties map[string]PanelConfigProperty `json:"properties,omitempty"`
	Enum       []string                       `json:"enum,omitempty"`
	Required   []string                       `json:"required,omitempty"`
}

type PanelConfigSchema struct {
	Type       string                         `json:"type"`
	Properties map[string]PanelConfigProperty `json:"properties,omitempty"`
	Required   []string                       `json:"required,omitempty"`
}

func PanelConfigSchemas() map[PanelType]PanelConfigSchema {
	return map[PanelType]PanelConfigSchema{
		PanelTable: {
			Type: "object",
			Properties: props(
				prop("columns", array(object())),
				prop("columnsSource", dataSource()),
				prop("watch", dataSource()),
				prop("refreshIntervalMs", number()),
				prop("defaultSort", object()),
				prop("actionIds", array(stringProp())),
				prop("rowActionIds", array(stringProp())),
				prop("selectable", boolProp()),
				prop("editable", boolProp()),
				prop("rowKey", array(stringProp())),
				prop("insert", dataSource()),
				prop("update", dataSource()),
				prop("delete", dataSource()),
				prop("emptyText", stringProp()),
				prop("stagedEdits", boolProp()),
				prop("hiddenColumns", array(stringProp())),
				prop("exportable", boolProp()),
				prop("rowClick", enum("navigate", "detail", "select", "none")),
			),
		},
		PanelFileBrowser: {
			Type: "object",
			Properties: props(
				prop("pathParam", stringProp()),
				prop("readRouteId", stringProp()),
				prop("downloadRouteId", stringProp()),
				prop("writeRouteId", stringProp()),
				prop("uploadRouteId", stringProp()),
				prop("mkdirRouteId", stringProp()),
				prop("renameRouteId", stringProp()),
				prop("deleteRouteId", stringProp()),
				prop("moveRouteId", stringProp()),
				prop("copyRouteId", stringProp()),
				prop("chmodRouteId", stringProp()),
				prop("archiveRouteId", stringProp()),
				prop("writable", boolProp()),
				prop("multipleUpload", boolProp()),
				prop("maxUploadBytes", number()),
				prop("uploadFieldName", stringProp()),
			),
		},
		PanelForm: {
			Type: "object",
			Properties: props(
				prop("submitRouteId", stringProp()),
				prop("submitMethod", enum("POST", "PUT", "PATCH", "DELETE")),
				prop("submitLabel", stringProp()),
				prop("params", stringMap()),
			),
		},
		PanelDashboard: {
			Type:       "object",
			Properties: props(prop("cells", array(panelObject()))),
		},
		PanelMetrics: {
			Type: "object",
			Properties: props(
				prop("stats", array(keyedObject())),
				prop("gauges", array(keyedObject())),
				prop("series", array(keyedObject())),
				prop("history", number()),
			),
		},
		PanelGraph: {
			Type: "object",
			Properties: props(
				prop("layout", enum("grid", "manual")),
				prop("fitView", boolProp()),
				prop("expandRouteId", stringProp()),
				prop("expandParam", stringProp()),
			),
		},
		PanelTrace: {Type: "object", Properties: props(prop("serviceField", stringProp()))},
		PanelKV: {
			Type: "object",
			Properties: props(
				prop("createRouteId", stringProp()),
				prop("readRouteId", stringProp()),
				prop("writeRouteId", stringProp()),
				prop("deleteRouteId", stringProp()),
				prop("keyParam", stringProp()),
				prop("writable", boolProp()),
				prop("valueTypes", array(stringProp())),
			),
		},
		PanelTerminal: {Type: "object", Properties: props(prop("zoom", boolProp()), prop("search", boolProp()))},
		PanelCodeEditor: {
			Type: "object",
			Properties: props(
				prop("language", stringProp()),
				prop("initialContent", stringProp()),
				prop("saveRouteId", stringProp()),
				prop("saveMethod", enum("POST", "PUT", "PATCH", "DELETE")),
				prop("saveParams", stringMap()),
				prop("saveBodyKey", stringProp()),
				prop("saveExtra", object()),
			),
		},
		PanelQueryEditor: {
			Type: "object",
			Properties: props(
				prop("language", stringProp()),
				prop("label", stringProp()),
				prop("executeLabel", stringProp()),
				prop("cancelLabel", stringProp()),
				prop("runningLabel", stringProp()),
				prop("emptyText", stringProp()),
				prop("initialQuery", stringProp()),
				prop("cancelRouteId", stringProp()),
				prop("cancelParams", stringMap()),
				prop("completionRouteId", stringProp()),
				prop("completionParams", stringMap()),
				prop("exportable", boolProp()),
			),
		},
		PanelHTTPClient: {
			Type: "object",
			Properties: props(
				prop("executeRouteId", stringProp()),
				prop("methods", array(stringProp())),
				prop("defaultMethod", stringProp()),
				prop("defaultUrl", stringProp()),
				prop("defaultHeaders", array(object())),
				prop("defaultBody", stringProp()),
			),
		},
		PanelRemoteDesktop: {
			Type: "object",
			Properties: props(
				prop("resize", boolProp()),
				prop("clipboard", boolProp()),
				prop("audio", boolProp()),
				prop("repeaterID", stringProp()),
			),
		},
		PanelObjectDetail: {
			Type: "object",
			Properties: props(
				prop("sections", array(object())),
				prop("rawToggle", boolProp()),
			),
		},
		PanelTimeline: {
			Type: "object",
			Properties: props(
				prop("timestampField", stringProp()),
				prop("titleField", stringProp()),
				prop("bodyField", stringProp()),
				prop("severityField", stringProp()),
				prop("iconField", stringProp()),
				prop("resourceField", stringProp()),
				prop("emptyText", stringProp()),
				prop("refreshIntervalMs", number()),
			),
		},
		PanelTaskProgress: {
			Type: "object",
			Properties: props(
				prop("title", stringProp()),
				prop("cancelRouteId", stringProp()),
				prop("retryRouteId", stringProp()),
			),
		},
		PanelSplit: {
			Type: "object",
			Properties: props(
				prop("orientation", enum("horizontal", "vertical")),
				prop("panels", array(object())),
			),
		},
	}
}

type schemaProp struct {
	key string
	val PanelConfigProperty
}

func prop(key string, val PanelConfigProperty) schemaProp {
	return schemaProp{key: key, val: val}
}

func props(items ...schemaProp) map[string]PanelConfigProperty {
	out := make(map[string]PanelConfigProperty, len(items))
	for _, item := range items {
		out[item.key] = item.val
	}
	return out
}

func stringProp() PanelConfigProperty { return PanelConfigProperty{Type: "string"} }
func boolProp() PanelConfigProperty   { return PanelConfigProperty{Type: "boolean"} }
func number() PanelConfigProperty     { return PanelConfigProperty{Type: "number"} }
func object() PanelConfigProperty     { return PanelConfigProperty{Type: "object"} }

func array(item PanelConfigProperty) PanelConfigProperty {
	return PanelConfigProperty{Type: "array", Items: &item}
}

func enum(values ...string) PanelConfigProperty {
	return PanelConfigProperty{Type: "string", Enum: values}
}

func stringMap() PanelConfigProperty {
	return PanelConfigProperty{Type: "object", Properties: map[string]PanelConfigProperty{"*": stringProp()}}
}

func dataSource() PanelConfigProperty {
	return PanelConfigProperty{
		Type: "object",
		Properties: props(
			prop("routeId", stringProp()),
			prop("method", enum("GET", "POST", "PUT", "PATCH", "DELETE", "WS")),
			prop("params", stringMap()),
		),
		Required: []string{"routeId"},
	}
}

func keyedObject() PanelConfigProperty {
	return PanelConfigProperty{
		Type:       "object",
		Properties: props(prop("key", stringProp())),
		Required:   []string{"key"},
	}
}

func panelObject() PanelConfigProperty {
	return PanelConfigProperty{
		Type: "object",
		Properties: props(
			prop("key", stringProp()),
			prop("label", stringProp()),
			prop("panel", stringProp()),
			prop("source", dataSource()),
			prop("config", object()),
			prop("span", number()),
		),
		Required: []string{"key", "panel"},
	}
}
