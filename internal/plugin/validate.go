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
	return ValidateWithCredentialKinds(m, routes, mustCredentialKindSet(builtInCredentialKindCatalog))
}

// ValidateWithCredentialKinds checks a manifest against an existing credential
// catalog. Plugin-declared kinds are added to a local copy before credential_ref
// selectors are validated, so a plugin may use the kinds it declares.
func ValidateWithCredentialKinds(m Manifest, routes []Route, existing CredentialKindCatalog) error {
	var errs []error
	add := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf(format, args...))
	}
	if existing == nil {
		existing = mustCredentialKindSet(builtInCredentialKindCatalog)
	}
	catalog, err := newCredentialKindSet(credentialKindDefinitions(existing.CredentialKinds()))
	if err != nil {
		add("credential kind catalog is invalid: %v", err)
		catalog = mustCredentialKindSet(nil)
	}
	declaredCredentialKinds := map[CredentialKind]bool{}
	for _, info := range m.CredentialKinds {
		if err := catalog.add(info); err != nil {
			add("%v", err)
		}
		declaredCredentialKinds[normalizeCredentialKindInfo(info).Kind] = true
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
	if m.Category == "" {
		add("Category is required")
	} else if _, ok := CategoryLookup(m.Category); !ok {
		add("Category %q is not a built-in category", m.Category)
	}
	switch m.Layout {
	case LayoutTabs, LayoutSidebarTree, LayoutDashboard:
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
	actionIDs := validateActions(m, routesByID, collectTabKeys(m), add)
	streamsByID := validateStreams(m, routesByID, add)
	validateLayout(m, routesByID, actionIDs, add)
	validateRecording(m, streamsByID, add)
	usedCredentialKinds := validateCredentialSelectors(m.Config, catalog, add)
	for kind := range declaredCredentialKinds {
		if !usedCredentialKinds[kind] {
			add("credential kind %q is declared but not used by any credential_ref selector", kind)
		}
	}

	return errors.Join(errs...)
}

func validateCredentialSelectors(schema Schema, catalog CredentialKindCatalog, add func(string, ...any)) map[CredentialKind]bool {
	used := map[CredentialKind]bool{}
	for _, group := range schema.Groups {
		for _, field := range group.Fields {
			if field.Type != FieldCredentialRef {
				continue
			}
			if field.Credential == nil {
				add("credential_ref field %q is missing Credential selector", field.Key)
				continue
			}
			if len(field.Credential.Kinds) == 0 {
				add("credential_ref field %q declares no accepted credential kinds", field.Key)
			}
			for _, kind := range field.Credential.Kinds {
				used[kind] = true
				if _, ok := catalog.CredentialKindLookup(kind); !ok {
					add("credential_ref field %q declares unknown credential kind %q", field.Key, kind)
				}
			}
		}
	}
	return used
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
func validateActions(m Manifest, routes map[string]Route, tabs map[string]bool, add func(string, ...any)) map[string]bool {
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
		if a.OnSuccess != nil && a.OnSuccess.SelectTab != "" && !tabs[a.OnSuccess.SelectTab] {
			add("action %q onSuccess.selectTab references unknown tab %q", a.ID, a.OnSuccess.SelectTab)
		}
		if (a.Open == OpenDock || a.Open == OpenDialog) && a.Panel == "" {
			add("action %q opens a panel (%s) but declares no panel type", a.ID, a.Open)
		}
	}
	return ids
}

func collectTabKeys(m Manifest) map[string]bool {
	keys := map[string]bool{}
	for _, tab := range m.Tabs {
		if tab.Key != "" {
			keys[tab.Key] = true
		}
	}
	for _, res := range m.Resources {
		for _, tab := range res.Detail.Tabs {
			if tab.Key != "" {
				keys[tab.Key] = true
			}
		}
	}
	return keys
}

// validateStreams checks stream ids reference existing WS routes and returns the
// declared streams indexed by id (for recording validation).
func validateStreams(m Manifest, routes map[string]Route, add func(string, ...any)) map[string]Stream {
	seen := make(map[string]Stream, len(m.Streams))
	for _, s := range m.Streams {
		if s.ID == "" {
			add("a stream is missing an ID")
			continue
		}
		if _, dup := seen[s.ID]; dup {
			add("duplicate stream ID %q", s.ID)
		}
		seen[s.ID] = s
		switch {
		case s.RouteID == "":
			add("stream %q is missing a RouteID", s.ID)
		case routes[s.RouteID].ID == "":
			add("stream %q references unknown route %q", s.ID, s.RouteID)
		case routes[s.RouteID].Method != MethodWS:
			add("stream %q references non-WS route %q", s.ID, s.RouteID)
		}
	}
	return seen
}

// validateRecording checks recording declarations: known classes (no dupes),
// class/format compatibility, and that StreamIDs reference declared streams whose
// kind matches the class. It rejects shapes that could enable unsupported
// recording, but never asserts a default policy (recording stays off by default).
func validateRecording(m Manifest, streams map[string]Stream, add func(string, ...any)) {
	seenClass := map[RecordingClass]bool{}
	for _, c := range m.Recording {
		switch c.Class {
		case RecordingTerminal, RecordingDesktop:
		default:
			add("recording capability has invalid class %q", c.Class)
			continue
		}
		if seenClass[c.Class] {
			add("duplicate recording class %q", c.Class)
		}
		seenClass[c.Class] = true

		if len(c.Formats) == 0 {
			add("recording class %q declares no formats", c.Class)
		}
		if len(c.StreamIDs) == 0 {
			add("recording class %q declares no streams", c.Class)
		}
		for _, f := range c.Formats {
			if !recordingFormatValidForClass(c.Class, f) {
				add("recording class %q does not support format %q", c.Class, f)
			}
		}

		wantKind := streamKindForClass(c.Class)
		for _, id := range c.StreamIDs {
			s, ok := streams[id]
			if !ok {
				add("recording class %q references unknown stream %q", c.Class, id)
				continue
			}
			if s.Kind != wantKind {
				add("recording class %q references stream %q with incompatible kind %q", c.Class, id, s.Kind)
			}
		}
	}
}

func recordingFormatValidForClass(class RecordingClass, f RecordingFormat) bool {
	switch class {
	case RecordingTerminal:
		return terminalFormats[f]
	case RecordingDesktop:
		return desktopFormats[f]
	default:
		return false
	}
}

func streamKindForClass(class RecordingClass) StreamKind {
	if class == RecordingDesktop {
		return StreamDesktop
	}
	return StreamTerminal
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
	checkStreamSource := func(ctx string, ds *DataSource) {
		if ds == nil {
			add("%s is missing a source", ctx)
			return
		}
		if ds.RouteID == "" {
			add("%s is missing a RouteID", ctx)
			return
		}
		rt, ok := routes[ds.RouteID]
		if !ok {
			add("%s references unknown route %q", ctx, ds.RouteID)
			return
		}
		if rt.Method != MethodWS {
			add("%s references route %q with invalid stream method %q", ctx, ds.RouteID, rt.Method)
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
			if t.Source != nil && t.Panel != PanelRemoteDesktop {
				checkDS(fmt.Sprintf("%s tab %q source", ctx, t.Key), *t.Source)
			}
			checkActionIDs(fmt.Sprintf("%s tab %q actionIds", ctx, t.Key), stringConfigList(t.Config, "actionIds"))
			checkActionIDs(fmt.Sprintf("%s tab %q rowActionIds", ctx, t.Key), stringConfigList(t.Config, "rowActionIds"))
			checkPanelConfigRoutes(
				fmt.Sprintf("%s tab %q", ctx, t.Key),
				t,
				checkRouteID,
				checkWriteRouteID,
				checkMultipartRouteID,
				checkStreamSource,
				add,
			)
		}
	}

	checkTabs("connection", m.Tabs)
	resourceKinds := map[string]bool{}
	for _, rt := range m.Resources {
		resourceKinds[rt.Kind] = true
	}
	for _, g := range m.Tree {
		checkDS(fmt.Sprintf("tree group %q source", g.Key), g.Source)
		if g.ResourceKind != "" && !resourceKinds[g.ResourceKind] {
			add("tree group %q references unknown resource kind %q", g.Key, g.ResourceKind)
		}
	}
	for _, rt := range m.Resources {
		checkDS(fmt.Sprintf("resource %q list", rt.Kind), rt.List)
		if rt.Watch != nil {
			checkDS(fmt.Sprintf("resource %q watch", rt.Kind), *rt.Watch)
		}
		if rt.ColumnsSource != nil {
			checkDS(fmt.Sprintf("resource %q columnsSource", rt.Kind), *rt.ColumnsSource)
		}
		checkActionIDs(fmt.Sprintf("resource %q", rt.Kind), rt.ActionIDs)
		checkActionIDs(fmt.Sprintf("resource %q list", rt.Kind), rt.ListActionIDs)
		checkActionIDs(fmt.Sprintf("resource %q row", rt.Kind), rt.RowActionIDs)
		checkActionIDs(fmt.Sprintf("resource %q header", rt.Kind), rt.Detail.Header.ActionIDs)
		checkTabs(fmt.Sprintf("resource %q detail", rt.Kind), rt.Detail.Tabs)
	}
}

func stringConfigList(config map[string]any, key string) []string {
	if config == nil {
		return nil
	}
	switch v := config[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func checkPanelConfigRoutes(
	ctx string,
	tab Tab,
	checkRouteID func(string, string),
	checkWriteRouteID func(string, string),
	checkMultipartRouteID func(string, string),
	checkStreamSource func(string, *DataSource),
	add func(string, ...any),
) {
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
		checkWriteRouteID(ctx+" writeRouteId", route("writeRouteId"))
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
		checkRouteID(ctx+" completionRouteId", route("completionRouteId"))
	case PanelKV:
		checkWriteRouteID(ctx+" createRouteId", route("createRouteId"))
		checkRouteID(ctx+" readRouteId", route("readRouteId"))
		checkWriteRouteID(ctx+" writeRouteId", route("writeRouteId"))
		checkWriteRouteID(ctx+" deleteRouteId", route("deleteRouteId"))
	case PanelHTTPClient:
		checkWriteRouteID(ctx+" executeRouteId", route("executeRouteId"))
	case PanelRemoteDesktop:
		checkStreamSource(ctx+" source", tab.Source)
		validateRemoteDesktopConfig(ctx, tab.Config, add)
	}
}

func validateRemoteDesktopConfig(ctx string, config map[string]any, add func(string, ...any)) {
	if _, ok := config["engine"]; ok {
		add("%s config no longer accepts remote desktop engine; desktop rendering is core-owned", ctx)
	}
}
