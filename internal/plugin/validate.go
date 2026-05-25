package plugin

import (
	"errors"
	"fmt"
)

// CurrentAPIVersion is the plugin contract version this core supports.
const CurrentAPIVersion = 1

var validRisks = map[RiskLevel]bool{
	RiskSafe: true, RiskWrite: true, RiskDestructive: true, RiskPrivileged: true,
}

var validMethods = map[Method]bool{
	MethodGet: true, MethodPost: true, MethodPut: true,
	MethodPatch: true, MethodDelete: true, MethodWS: true,
}

// Validate checks a manifest + its routes at registration, returning an
// aggregate of all actionable problems found (not just the first).
func Validate(m Manifest, routes []Route) error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}

	if m.APIVersion != CurrentAPIVersion {
		add("APIVersion %d unsupported (core supports %d)", m.APIVersion, CurrentAPIVersion)
	}
	if m.Name == "" {
		add("Name is required")
	}
	if m.Title == "" {
		add("Title is required")
	}
	switch m.Layout {
	case LayoutTabs, LayoutSidebarTree:
	default:
		add("Layout %q is not a valid layout", m.Layout)
	}

	if !m.SupportsTransport(TransportDirect) {
		add("SupportedTransports must include %q", TransportDirect)
	}
	if m.SupportsTransport(TransportAgent) && m.Agent == nil {
		add("AgentProfile is required when transport %q is declared", TransportAgent)
	}
	if !m.SupportsTransport(TransportAgent) && m.Agent != nil {
		add("AgentProfile declared but transport %q is not supported", TransportAgent)
	}

	routesByID := validateRoutes(routes, add)
	actionIDs := validateActions(m, routesByID, add)
	validateStreams(m, routesByID, add)
	validateLayout(m, routesByID, actionIDs, add)

	return errors.Join(errs...)
}

// validateRoutes checks route shape and returns the set of route ids.
func validateRoutes(routes []Route, add func(string, ...any)) map[string]Route {
	ids := make(map[string]Route, len(routes))
	for _, rt := range routes {
		if rt.ID == "" {
			add("a route is missing an ID")
			continue
		}
		if _, exists := ids[rt.ID]; exists {
			add("duplicate route ID %q", rt.ID)
		}
		ids[rt.ID] = rt

		if !validMethods[rt.Method] {
			add("route %q has invalid method %q", rt.ID, rt.Method)
		}
		if rt.Permission == "" {
			add("route %q is missing a Permission", rt.ID)
		}
		if !validRisks[rt.Risk] {
			add("route %q has invalid risk %q", rt.ID, rt.Risk)
		}
		if rt.Method == MethodWS {
			if rt.Stream == nil {
				add("WS route %q is missing a Stream handler", rt.ID)
			}
		} else if rt.Handle == nil {
			add("route %q is missing a Handle func", rt.ID)
		}
	}
	return ids
}

// validateActions checks action ids are unique and reference existing routes.
func validateActions(m Manifest, routes map[string]Route, add func(string, ...any)) map[string]bool {
	ids := make(map[string]bool, len(m.Actions))
	for _, a := range m.Actions {
		if a.ID == "" {
			add("an action is missing an ID")
			continue
		}
		if ids[a.ID] {
			add("duplicate action ID %q", a.ID)
		}
		ids[a.ID] = true
		if a.RouteID == "" {
			add("action %q is missing a RouteID", a.ID)
		} else if _, ok := routes[a.RouteID]; !ok {
			add("action %q references unknown route %q", a.ID, a.RouteID)
		}
	}
	return ids
}

// validateStreams checks stream ids reference existing WS routes.
func validateStreams(m Manifest, routes map[string]Route, add func(string, ...any)) {
	seen := make(map[string]bool, len(m.Streams))
	for _, s := range m.Streams {
		if s.ID == "" {
			add("a stream is missing an ID")
			continue
		}
		if seen[s.ID] {
			add("duplicate stream ID %q", s.ID)
		}
		seen[s.ID] = true
		switch {
		case s.RouteID == "":
			add("stream %q is missing a RouteID", s.ID)
		case routes[s.RouteID].ID == "":
			add("stream %q references unknown route %q", s.ID, s.RouteID)
		case routes[s.RouteID].Method != MethodWS:
			add("stream %q references non-WS route %q", s.ID, s.RouteID)
		}
	}
}

// validateLayout checks every DataSource RouteID and ActionID reference resolves.
func validateLayout(m Manifest, routes map[string]Route, actionIDs map[string]bool, add func(string, ...any)) {
	checkDS := func(ctx string, ds DataSource) {
		if ds.RouteID == "" {
			add("%s is missing a RouteID", ctx)
		} else if _, ok := routes[ds.RouteID]; !ok {
			add("%s references unknown route %q", ctx, ds.RouteID)
		}
	}
	checkRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		if _, ok := routes[routeID]; !ok {
			add("%s references unknown route %q", ctx, routeID)
		}
	}
	checkWriteRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		rt, ok := routes[routeID]
		if !ok {
			add("%s references unknown route %q", ctx, routeID)
			return
		}
		if rt.Method == MethodGet || rt.Method == MethodWS {
			add("%s references route %q with invalid write method %q", ctx, routeID, rt.Method)
		}
	}
	checkMultipartRouteID := func(ctx string, routeID string) {
		if routeID == "" {
			return
		}
		rt, ok := routes[routeID]
		if !ok {
			add("%s references unknown route %q", ctx, routeID)
			return
		}
		if rt.Method == MethodGet || rt.Method == MethodWS {
			add("%s references route %q with invalid upload method %q", ctx, routeID, rt.Method)
		}
		if rt.Input == nil || !rt.Input.HasFileField() {
			add("%s references route %q without a file input schema", ctx, routeID)
		}
	}
	checkActionIDs := func(ctx string, ids []string) {
		for _, id := range ids {
			if !actionIDs[id] {
				add("%s references unknown action %q", ctx, id)
			}
		}
	}
	checkTabs := func(ctx string, tabs []Tab) {
		for _, t := range tabs {
			if t.Source != nil {
				checkDS(fmt.Sprintf("%s tab %q source", ctx, t.Key), *t.Source)
			}
			checkPanelConfigRoutes(fmt.Sprintf("%s tab %q", ctx, t.Key), t, checkRouteID, checkWriteRouteID, checkMultipartRouteID)
		}
	}

	checkTabs("connection", m.Tabs)
	for _, g := range m.Tree {
		checkDS(fmt.Sprintf("tree group %q source", g.Key), g.Source)
	}
	for _, rt := range m.Resources {
		checkDS(fmt.Sprintf("resource %q list", rt.Kind), rt.List)
		if rt.Watch != nil {
			checkDS(fmt.Sprintf("resource %q watch", rt.Kind), *rt.Watch)
		}
		checkActionIDs(fmt.Sprintf("resource %q", rt.Kind), rt.ActionIDs)
		checkActionIDs(fmt.Sprintf("resource %q header", rt.Kind), rt.Detail.Header.ActionIDs)
		checkTabs(fmt.Sprintf("resource %q detail", rt.Kind), rt.Detail.Tabs)
	}
}

func checkPanelConfigRoutes(ctx string, tab Tab, checkRouteID, checkWriteRouteID, checkMultipartRouteID func(string, string)) {
	route := func(key string) string {
		if tab.Config == nil {
			return ""
		}
		v, _ := tab.Config[key].(string)
		return v
	}
	switch tab.Panel {
	case PanelFileBrowser:
		checkRouteID(ctx+" readRouteId", route("readRouteId"))
		checkRouteID(ctx+" downloadRouteId", route("downloadRouteId"))
		checkMultipartRouteID(ctx+" uploadRouteId", route("uploadRouteId"))
		checkWriteRouteID(ctx+" mkdirRouteId", route("mkdirRouteId"))
		checkWriteRouteID(ctx+" renameRouteId", route("renameRouteId"))
		checkWriteRouteID(ctx+" deleteRouteId", route("deleteRouteId"))
	case PanelForm:
		checkWriteRouteID(ctx+" submitRouteId", route("submitRouteId"))
	case PanelCodeEditor:
		checkWriteRouteID(ctx+" saveRouteId", route("saveRouteId"))
	case PanelQueryEditor:
		checkWriteRouteID(ctx+" cancelRouteId", route("cancelRouteId"))
	}
}
