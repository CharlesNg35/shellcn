// Package pluginux checks plugin manifests against generic renderer UX rules.
package pluginux

import (
	"fmt"
	"strings"

	"github.com/charlesng35/shellcn/sdk/plugin"
)

type Level string

const (
	Error   Level = "error"
	Warning Level = "warning"
)

type Finding struct {
	Level   Level
	Path    string
	Message string
}

func (f Finding) Error() string {
	if f.Path == "" {
		return f.Message
	}
	return f.Path + ": " + f.Message
}

func Lint(m plugin.Manifest, routes []plugin.Route) []Finding {
	l := linter{
		manifest: m,
		routes:   map[string]plugin.Route{},
		streams:  map[string]plugin.Stream{},
	}
	for _, r := range routes {
		l.routes[r.ID] = r
	}
	for _, s := range m.Streams {
		l.streams[s.RouteID] = s
	}
	l.actions()
	l.schema("config", m.Config)
	for _, r := range routes {
		if r.Input != nil {
			l.schema("route "+r.ID+" input", *r.Input)
		}
	}
	l.panels("connection", m.Tabs)
	for _, r := range m.Resources {
		l.resource(r)
		l.panels("resource "+r.Kind+" detail", r.Detail.Tabs)
	}
	return l.findings
}

func Errors(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Level == Error {
			out = append(out, f)
		}
	}
	return out
}

type linter struct {
	manifest plugin.Manifest
	routes   map[string]plugin.Route
	streams  map[string]plugin.Stream
	findings []Finding
}

func (l *linter) add(level Level, path, msg string, args ...any) {
	l.findings = append(l.findings, Finding{Level: level, Path: path, Message: fmt.Sprintf(msg, args...)})
}

func (l *linter) actions() {
	for _, a := range l.manifest.Actions {
		path := "action " + a.ID
		rt := l.routes[a.RouteID]
		if strings.TrimSpace(a.Label) == "" {
			l.add(Error, path, "must declare a label")
		}
		if a.Icon.Value == "" {
			l.add(Warning, path, "should declare an icon")
		}
		if (rt.Risk == plugin.RiskDestructive || rt.Risk == plugin.RiskPrivileged) && !a.Confirm {
			l.add(Error, path, "%s action must require confirmation", rt.Risk)
		}
		if (a.Open == plugin.OpenDock || a.Open == plugin.OpenDialog) && a.Panel == "" {
			l.add(Error, path, "opens %q without a panel", a.Open)
		}
		if a.Open == plugin.OpenDock && !longLivedPanel(a.Panel) {
			l.add(Error, path, "OpenDock is only for long-lived interactive panels")
		}
		if a.Open == plugin.OpenDock && a.OnSuccess != nil {
			l.add(Warning, path, "dock actions usually should not navigate or switch tabs on success")
		}
		if a.Open == plugin.OpenDialog && a.OnSuccess == nil && rt.Risk != plugin.RiskSafe {
			l.add(Warning, path, "write dialog should declare an onSuccess refresh/navigation hint when useful")
		}
		if a.Open == plugin.OpenURL && rt.Input != nil && hasRequiredField(*rt.Input) {
			l.add(Error, path, "OpenURL action input fields are submitted as route params; required body fields would fail core validation")
		}
		l.panel(path+" panel", plugin.Panel{Key: a.ID, Type: a.Panel, Source: &plugin.DataSource{RouteID: a.RouteID, Method: rt.Method}, Config: a.Config})
	}
}

func hasRequiredField(schema plugin.Schema) bool {
	for _, group := range schema.Groups {
		if fieldsHaveRequired(group.Fields) {
			return true
		}
	}
	return false
}

func fieldsHaveRequired(fields []plugin.Field) bool {
	for _, field := range fields {
		if field.Required || fieldsHaveRequired(field.Fields) {
			return true
		}
		if field.Item != nil && fieldsHaveRequired([]plugin.Field{*field.Item}) {
			return true
		}
	}
	return false
}

func longLivedPanel(panel plugin.PanelType) bool {
	switch panel {
	case plugin.PanelTerminal, plugin.PanelTerminalGrid, plugin.PanelRemoteDesktop, plugin.PanelLogStream, plugin.PanelMetrics, plugin.PanelTaskProgress, plugin.PanelCanvas:
		return true
	default:
		return false
	}
}

func (l *linter) resource(r plugin.ResourceType) {
	path := "resource " + r.Kind
	if len(r.Columns) == 0 && r.ColumnsSource == nil {
		l.add(Warning, path, "list should declare static columns or a columnsSource")
	}
	if r.Watch == nil {
		l.add(Warning, path, "live resource lists should declare watch or use a table refresh interval")
	}
	for _, c := range r.Columns {
		if c.Type == "" || c.Type == plugin.ColumnText {
			switch strings.ToLower(c.Key) {
			case "age", "createdat", "created", "updated", "updatedat", "time", "timestamp":
				l.add(Warning, path+" column "+c.Key, "time-like columns should use datetime or relative_time")
			case "status", "state", "phase", "type":
				l.add(Warning, path+" column "+c.Key, "state-like columns should use badge severities when values are bounded")
			}
		}
	}
}

func (l *linter) schema(path string, s plugin.Schema) {
	for _, g := range s.Groups {
		for _, f := range g.Fields {
			l.field(path+"."+g.Name+"."+f.Key, f)
		}
	}
}

func (l *linter) field(path string, f plugin.Field) {
	if f.Type == plugin.FieldText && (len(f.Options) > 0 || f.OptionsSource != nil) {
		l.add(Error, path, "text field with options should use select/radio for closed values or autocomplete for suggested values")
	}
	if f.Type == plugin.FieldText {
		switch strings.ToLower(f.Key) {
		case "type", "kind", "mode", "driver", "auth", "tls_mode", "protocol", "signal":
			l.add(Warning, path, "bounded-looking field should usually use select/radio or autocomplete")
		}
	}
	for _, v := range f.Validators {
		if f.Type == plugin.FieldText && v.Type == plugin.ValidatorOneOf {
			l.add(Warning, path, "oneOf text field should usually use select/radio")
		}
	}
	for _, child := range f.Fields {
		l.field(path+"."+child.Key, child)
	}
	if f.Item != nil {
		l.field(path+"[]", *f.Item)
	}
}

func (l *linter) panels(path string, panels []plugin.Panel) {
	for _, p := range panels {
		l.panel(path+" panel "+p.Key, p)
	}
}

func (l *linter) panel(path string, p plugin.Panel) {
	l.streamMatch(path, p)
	switch c := p.Config.(type) {
	case plugin.TableConfig:
		if len(c.Columns) == 0 && c.ColumnsSource == nil {
			l.add(Warning, path, "table should declare columns or columnsSource")
		}
		if c.EmptyText == "" {
			l.add(Warning, path, "table should declare an emptyText tailored to the resource")
		}
		if c.Watch == nil && c.RefreshIntervalMs == 0 {
			l.add(Warning, path, "live tables should declare watch or refreshIntervalMs")
		}
	case plugin.DashboardConfig:
		l.panels(path+" dashboard", c.Cells)
	case plugin.SplitConfig:
		if len(c.Panels) < 2 {
			l.add(Error, path, "split panel requires at least two child panels")
		}
		for _, child := range c.Panels {
			l.panel(path+" split "+child.Key, child.Panel)
		}
	}
}

func (l *linter) streamMatch(path string, p plugin.Panel) {
	if p.Source == nil {
		return
	}
	expected, ok := expectedStreamKind(p.Type)
	if !ok {
		return
	}
	stream, ok := l.streams[p.Source.RouteID]
	if !ok {
		l.add(Error, path, "stream panel route %q is not declared in manifest streams", p.Source.RouteID)
		return
	}
	if stream.Kind != expected {
		l.add(Error, path, "stream route %q is %q but panel %q requires %q", p.Source.RouteID, stream.Kind, p.Type, expected)
	}
}

func expectedStreamKind(panel plugin.PanelType) (plugin.StreamKind, bool) {
	switch panel {
	case plugin.PanelTerminal, plugin.PanelTerminalGrid:
		return plugin.StreamTerminal, true
	case plugin.PanelLogStream:
		return plugin.StreamLogs, true
	case plugin.PanelRemoteDesktop:
		return plugin.StreamDesktop, true
	case plugin.PanelMetrics:
		return plugin.StreamMetrics, true
	case plugin.PanelTaskProgress:
		return plugin.StreamTask, true
	case plugin.PanelCanvas:
		return plugin.StreamCanvas, true
	default:
		return "", false
	}
}
